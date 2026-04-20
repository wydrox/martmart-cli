package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/wydrox/martmart-cli/internal/session"
)

type remoteDebugVersion struct {
	Browser              string `json:"Browser"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type remoteDebugTarget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

type remoteDebugDelioCapture struct {
	Headers      map[string]string
	CookieHeader string
	UserID       string
}

type remoteDebugFriscoCapture struct {
	CookieHeader string
	AccessToken  string
	RefreshToken string
	UserID       string
}

type remoteDebugFetchResult struct {
	Status int    `json:"status"`
	Text   string `json:"text"`
}

var errNoRemoteDebugEndpoint = errors.New("no Chromium remote debugging endpoint available")

func runWithRemoteDebugBrowser(ctx context.Context, opts Options) (*Result, error) {
	provider := session.NormalizeProvider(opts.Provider)
	if provider == "" {
		provider = session.CurrentProvider()
	}
	if provider == "" {
		provider = session.ProviderFrisco
	}
	if err := session.ValidateProvider(provider); err != nil {
		return nil, err
	}

	s, err := session.LoadProvider(provider)
	if err != nil {
		return nil, err
	}
	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = session.DefaultBaseURLForProvider(provider)
	}
	loginURL := strings.TrimSpace(opts.LoginURL)
	if loginURL == "" {
		loginURL = defaultStartURL(provider)
	}
	timeoutSec := opts.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 180
	}

	debugBase, version, err := firstAvailableRemoteDebugEndpoint()
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		targetInfo, err := findProviderRemoteDebugTarget(debugBase, provider)
		if err != nil {
			lastErr = err
		} else {
			switch provider {
			case session.ProviderDelio:
				if targetInfo == nil {
					lastErr = errors.New("open a logged-in Delio tab in your current Chromium browser and keep it open for session capture")
					break
				}
				capture, err := captureDelioSessionFromRemoteTarget(ctx, version.WebSocketDebuggerURL, targetInfo.ID, loginURL)
				if err == nil && hasDelioAuthCookie(capture.CookieHeader) {
					return saveDelioSessionFromCapture(s, baseURL, "", capture)
				}
				if err != nil {
					lastErr = err
				} else {
					lastErr = errors.New("Delio auth cookies not detected in the open browser tab yet")
				}
			default:
				targetID := ""
				if targetInfo != nil {
					targetID = targetInfo.ID
				}
				capture, err := captureFriscoSessionFromRemoteTarget(ctx, version.WebSocketDebuggerURL, targetID, loginURL)
				if err == nil && capture.AccessToken != "" {
					return saveFriscoSessionFromCapture(s, baseURL, "", capture)
				}
				if err != nil {
					lastErr = err
				} else {
					lastErr = errors.New("Frisco session token not detected in the current Chromium browser yet")
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timed out waiting for a logged-in %s tab on the Chromium remote debugging endpoint", providerDisplayName(provider))
	}
	return nil, lastErr
}

func openLoginPageOnRemoteDebugBrowser(ctx context.Context, browserWSURL, loginURL string) error {
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, browserWSURL)
	defer cancelAlloc()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	targetID, err := target.CreateTarget(loginURL).WithNewWindow(true).Do(taskCtx)
	if err != nil {
		return fmt.Errorf("could not open login page in remote-debug browser: %w", err)
	}
	if err := target.ActivateTarget(targetID).Do(taskCtx); err != nil {
		return fmt.Errorf("could not activate login page target: %w", err)
	}
	return nil
}

func providerDisplayName(provider string) string {
	switch session.NormalizeProvider(provider) {
	case session.ProviderDelio:
		return "Delio"
	default:
		return "Frisco"
	}
}

func firstAvailableRemoteDebugEndpoint() (string, *remoteDebugVersion, error) {
	var lastErr error
	for _, base := range remoteDebugBaseURLs() {
		var version remoteDebugVersion
		if err := remoteDebugGetJSON(base+"/json/version", &version); err != nil {
			lastErr = err
			continue
		}
		if strings.TrimSpace(version.WebSocketDebuggerURL) == "" {
			lastErr = fmt.Errorf("missing webSocketDebuggerUrl on %s/json/version", base)
			continue
		}
		return base, &version, nil
	}
	if lastErr == nil {
		lastErr = errNoRemoteDebugEndpoint
	}
	return "", nil, lastErr
}

func remoteDebugBaseURLs() []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, raw := range []string{
		strings.TrimSpace(strings.TrimRight(urlFromEnv("MARTMART_REMOTE_DEBUG_URL"), "/")),
		"http://127.0.0.1:9222",
		"http://localhost:9222",
	} {
		if raw == "" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out
}

func urlFromEnv(key string) string {
	return strings.TrimSpace(strings.TrimRight(os.Getenv(key), "/"))
}

func remoteDebugGetJSON(rawURL string, dest any) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned %s", rawURL, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func findProviderRemoteDebugTarget(debugBase, provider string) (*remoteDebugTarget, error) {
	var targets []remoteDebugTarget
	if err := remoteDebugGetJSON(debugBase+"/json/list", &targets); err != nil {
		return nil, err
	}
	prefix := providerRemoteDebugURLPrefix(provider)
	for i := range targets {
		if targets[i].Type != "page" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(targets[i].URL), prefix) {
			copy := targets[i]
			return &copy, nil
		}
	}
	return nil, nil
}

func providerRemoteDebugURLPrefix(provider string) string {
	switch session.NormalizeProvider(provider) {
	case session.ProviderDelio:
		return "https://delio.com.pl/"
	default:
		return "https://www.frisco.pl/"
	}
}

func captureDelioSessionFromRemoteTarget(ctx context.Context, browserWSURL, targetID, loginURL string) (*remoteDebugDelioCapture, error) {
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, browserWSURL)
	defer cancelAlloc()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(targetID)))
	defer cancelTask()

	headersCh := make(chan map[string]string, 2)
	var mu sync.Mutex
	seenRequestIDs := map[network.RequestID]struct{}{}

	chromedp.ListenTarget(taskCtx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if strings.Contains(strings.TrimSpace(e.Request.URL), "/api/proxy/delio") {
				mu.Lock()
				seenRequestIDs[e.RequestID] = struct{}{}
				mu.Unlock()
			}
		case *network.EventRequestWillBeSentExtraInfo:
			mu.Lock()
			_, ok := seenRequestIDs[e.RequestID]
			mu.Unlock()
			if !ok && strings.TrimSpace(headerStringValue(e.Headers, ":path")) != "/api/proxy/delio" {
				return
			}
			headers := networkHeadersToStringMap(e.Headers)
			select {
			case headersCh <- headers:
			default:
			}
		}
	})

	var fetchResult remoteDebugFetchResult
	fetchExpr := `(async () => {
		const response = await fetch('https://delio.com.pl/api/proxy/delio', {
			method: 'POST',
			headers: {
				'content-type': 'application/json',
				'x-platform': 'web',
				'x-api-version': '4.0',
				'x-app-version': '7.32.6',
				'x-csrf-protected': ''
			},
			body: JSON.stringify({
				operationName: 'CurrentCart',
				variables: {},
				query: 'query CurrentCart { currentCart { id shippingAddress { lat long } } }'
			})
		});
		return { status: response.status, text: await response.text() };
	})()`

	var cookies []*network.Cookie
	err := chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.Evaluate(fetchExpr, &fetchResult, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().WithURLs([]string{loginURL}).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, err
	}
	if fetchResult.Status < 200 || fetchResult.Status >= 300 {
		return nil, fmt.Errorf("Delio fetch via remote debug returned %d: %s", fetchResult.Status, strings.TrimSpace(fetchResult.Text))
	}
	cookieHeader := cookieHeaderFromCDPCookies(cookies, hostForURL(loginURL))

	headers := map[string]string{}
	select {
	case headers = <-headersCh:
	case <-time.After(2 * time.Second):
	}

	capture := &remoteDebugDelioCapture{
		Headers:      filterDelioSessionHeaders(headers),
		CookieHeader: strings.TrimSpace(cookieHeader),
		UserID:       extractDelioUserIDFromCookieHeader(cookieHeader),
	}
	if capture.CookieHeader == "" {
		capture.CookieHeader = strings.TrimSpace(headerMapValue(headers, "Cookie"))
	}
	if capture.UserID == "" {
		capture.UserID = extractDelioUserIDFromCookieHeader(capture.CookieHeader)
	}
	return capture, nil
}

func captureFriscoSessionFromRemoteTarget(ctx context.Context, browserWSURL, targetID, loginURL string) (*remoteDebugFriscoCapture, error) {
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, browserWSURL)
	defer cancelAlloc()

	var taskCtx context.Context
	var cancelTask context.CancelFunc
	if strings.TrimSpace(targetID) != "" {
		taskCtx, cancelTask = chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(targetID)))
	} else {
		taskCtx, cancelTask = chromedp.NewContext(allocCtx)
	}
	defer cancelTask()

	var cookies []*network.Cookie
	err := chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().WithURLs([]string{loginURL}).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, err
	}

	host := hostForURL(loginURL)
	cookieHeader := cookieHeaderFromCDPCookies(cookies, host)
	cookieMap := cookieMapFromCDPCookies(cookies, host)
	accessToken := extractFriscoAccessToken(cookieMap)
	refreshToken := extractFriscoRefreshToken(cookieMap, cookieHeader)
	userID := extractFriscoUserID(cookieMap, accessToken)

	return &remoteDebugFriscoCapture{
		CookieHeader: cookieHeader,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       userID,
	}, nil
}

func networkHeadersToStringMap(headers network.Headers) map[string]string {
	out := map[string]string{}
	for k := range headers {
		out[k] = strings.TrimSpace(fmt.Sprint(headers[k]))
	}
	return out
}

func filterDelioSessionHeaders(headers map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range headers {
		lk := strings.ToLower(strings.TrimSpace(k))
		if _, ok := delioSessionHeaderAllow[lk]; !ok {
			continue
		}
		out[lk] = strings.TrimSpace(v)
	}
	if out["accept"] == "" {
		out["accept"] = "*/*"
	}
	if out["content-type"] == "" {
		out["content-type"] = "application/json"
	}
	if out["origin"] == "" {
		out["origin"] = "https://delio.com.pl"
	}
	if out["referer"] == "" {
		out["referer"] = "https://delio.com.pl/"
	}
	if out["x-api-version"] == "" {
		out["x-api-version"] = "4.0"
	}
	if out["x-app-version"] == "" {
		out["x-app-version"] = "7.32.6"
	}
	if out["x-platform"] == "" {
		out["x-platform"] = "web"
	}
	if _, ok := out["x-csrf-protected"]; !ok {
		out["x-csrf-protected"] = ""
	}
	return out
}

var delioSessionHeaderAllow = map[string]struct{}{
	"accept":             {},
	"accept-language":    {},
	"baggage":            {},
	"content-type":       {},
	"cookie":             {},
	"origin":             {},
	"priority":           {},
	"referer":            {},
	"sec-ch-ua":          {},
	"sec-ch-ua-mobile":   {},
	"sec-ch-ua-platform": {},
	"sec-fetch-dest":     {},
	"sec-fetch-mode":     {},
	"sec-fetch-site":     {},
	"sentry-trace":       {},
	"user-agent":         {},
	"x-api-version":      {},
	"x-app-version":      {},
	"x-csrf-protected":   {},
	"x-platform":         {},
}

func cookieHeaderFromCDPCookies(cookies []*network.Cookie, host string) string {
	parts := make([]string, 0, len(cookies))
	for _, ck := range cookies {
		if ck == nil || ck.Name == "" {
			continue
		}
		if host != "" && !cookieMatchesHost(ck.Domain, host) {
			continue
		}
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func cookieMapFromCDPCookies(cookies []*network.Cookie, host string) map[string]string {
	out := map[string]string{}
	for _, ck := range cookies {
		if ck == nil || strings.TrimSpace(ck.Name) == "" {
			continue
		}
		if host != "" && !cookieMatchesHost(ck.Domain, host) {
			continue
		}
		out[ck.Name] = ck.Value
	}
	return out
}

func saveDelioSessionFromCapture(s *session.Session, baseURL, profileDirectory string, capture *remoteDebugDelioCapture) (*Result, error) {
	if capture == nil {
		return nil, errors.New("missing Delio capture")
	}
	if !hasDelioAuthCookie(capture.CookieHeader) {
		return nil, errors.New("Delio auth cookies not detected in captured browser session")
	}
	s.BaseURL = baseURL
	s.Headers = session.NormalizeHeaders(capture.Headers)
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	s.Headers["Cookie"] = capture.CookieHeader
	s.Token = nil
	delete(s.Headers, "Authorization")
	s.RefreshToken = nil
	if strings.TrimSpace(capture.UserID) != "" {
		s.UserID = strings.TrimSpace(capture.UserID)
	}
	if err := session.SaveProvider(session.ProviderDelio, s); err != nil {
		return nil, err
	}
	return &Result{
		Saved:            true,
		BaseURL:          s.BaseURL,
		UserID:           s.UserID,
		TokenSaved:       false,
		CookieSaved:      session.HeaderValue(s, "Cookie") != "",
		Provider:         session.ProviderDelio,
		ProfileDirectory: profileDirectory,
	}, nil
}

func saveFriscoSessionFromCapture(s *session.Session, baseURL, profileDirectory string, capture *remoteDebugFriscoCapture) (*Result, error) {
	if capture == nil {
		return nil, errors.New("missing Frisco capture")
	}
	if strings.TrimSpace(capture.AccessToken) == "" {
		return nil, errors.New("Frisco access token not detected in captured browser session")
	}
	s.BaseURL = baseURL
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	if strings.TrimSpace(capture.CookieHeader) != "" {
		s.Headers["Cookie"] = strings.TrimSpace(capture.CookieHeader)
	}
	s.Token = strings.TrimSpace(capture.AccessToken)
	s.Headers["Authorization"] = "Bearer " + strings.TrimSpace(capture.AccessToken)
	if strings.TrimSpace(capture.RefreshToken) != "" {
		s.RefreshToken = strings.TrimSpace(capture.RefreshToken)
	} else {
		s.RefreshToken = nil
	}
	if strings.TrimSpace(capture.UserID) != "" {
		s.UserID = strings.TrimSpace(capture.UserID)
	}
	if err := session.SaveProvider(session.ProviderFrisco, s); err != nil {
		return nil, err
	}
	return &Result{
		Saved:             true,
		BaseURL:           s.BaseURL,
		UserID:            s.UserID,
		TokenSaved:        session.TokenString(s) != "",
		RefreshTokenSaved: session.RefreshTokenString(s) != "",
		CookieSaved:       session.HeaderValue(s, "Cookie") != "",
		Provider:          session.ProviderFrisco,
		ProfileDirectory:  profileDirectory,
	}, nil
}

func extractFriscoAccessToken(cookieMap map[string]string) string {
	for _, name := range []string{"sessionIdN", "sessionId"} {
		if value := strings.TrimSpace(cookieMap[name]); value != "" {
			return value
		}
	}
	return ""
}

func extractFriscoRefreshToken(cookieMap map[string]string, cookieHeader string) string {
	for _, name := range []string{"rtokenN", "rtoken"} {
		if value := strings.TrimSpace(cookieMap[name]); value != "" {
			return value
		}
	}
	return session.ExtractRefreshTokenFromHeaderValue(cookieHeader)
}

func extractFriscoUserID(cookieMap map[string]string, accessToken string) string {
	for _, name := range []string{"userIdN", "userId"} {
		if value := strings.TrimSpace(cookieMap[name]); value != "" {
			return value
		}
	}
	return extractUserIDFromJWT(accessToken)
}

func headerMapValue(headers map[string]string, name string) string {
	for k, v := range headers {
		if strings.EqualFold(strings.TrimSpace(k), name) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func extractDelioUserIDFromCookieHeader(cookieHeader string) string {
	parts := strings.Split(cookieHeader, ";")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if !strings.HasPrefix(strings.ToLower(trimmed), "idtoken=") {
			continue
		}
		token := strings.TrimSpace(trimmed[len("idToken="):])
		if token == "" {
			continue
		}
		if userID := extractUserIDFromJWT(token); userID != "" {
			return userID
		}
	}
	return ""
}

func extractUserIDFromJWT(token string) string {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return ""
	}
	payload := strings.TrimSpace(parts[1])
	if payload == "" {
		return ""
	}
	if rem := len(payload) % 4; rem != 0 {
		payload += strings.Repeat("=", 4-rem)
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
		if err != nil {
			return ""
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}
	if userID, ok := claims["user_id"].(string); ok {
		return strings.TrimSpace(userID)
	}
	if userID, ok := claims["sub"].(string); ok {
		return strings.TrimSpace(userID)
	}
	return ""
}
