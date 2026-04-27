package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/httpclient"
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

type remoteDebugBrowserInfo struct {
	BrowserApp         string
	BrowserUserDataDir string
	ProfileDirectory   string
}

var errNoRemoteDebugEndpoint = errors.New("no browser remote debugging endpoint available on port 9222")

const remoteDebugCaptureWait = 3 * time.Second

func runWithRemoteDebugBrowser(ctx context.Context, opts Options) (*Result, error) {
	provider := session.NormalizeProvider(opts.Provider)
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
	debugBase, version, err := firstAvailableRemoteDebugEndpoint()
	var browserInfo *remoteDebugBrowserInfo
	if err != nil {
		debugBase, browserInfo, err = ensurePreferredBrowserRemoteDebugOn9222(ctx, opts)
		if err != nil {
			return nil, err
		}
		version, err = waitForRemoteDebugVersion(ctx, debugBase, 20*time.Second)
		if err != nil {
			return nil, err
		}
	}

	loginDebugf(opts, "using remote debug endpoint %s for %s", debugBase, provider)
	if err := openLoginPageOnRemoteDebugBrowser(ctx, version.WebSocketDebuggerURL, loginURL); err != nil {
		return nil, err
	}

	targetInfo, err := waitForProviderRemoteDebugTarget(ctx, debugBase, provider, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if err := waitRemoteDebugCaptureWindow(ctx, opts, remoteDebugCaptureWait); err != nil {
		return nil, err
	}

	result, firstErr := captureAndSaveRemoteDebugSession(ctx, opts, provider, s, baseURL, loginURL, version.WebSocketDebuggerURL, targetInfo.ID)
	if firstErr == nil {
		applyRemoteDebugBrowserInfo(result, browserInfo)
		return result, nil
	}
	loginDebugf(opts, "first %s capture/verify failed: %v", provider, firstErr)

	if err := reloadRemoteDebugTarget(ctx, version.WebSocketDebuggerURL, targetInfo.ID, loginURL); err != nil {
		return nil, fmt.Errorf("first %s capture/verify failed: %v; reload failed: %w", providerDisplayName(provider), firstErr, err)
	}
	retargeted, err := waitForProviderRemoteDebugTarget(ctx, debugBase, provider, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("first %s capture/verify failed: %v; provider tab not found after reload: %w", providerDisplayName(provider), firstErr, err)
	}
	if err := waitRemoteDebugCaptureWindow(ctx, opts, remoteDebugCaptureWait); err != nil {
		return nil, err
	}
	result, secondErr := captureAndSaveRemoteDebugSession(ctx, opts, provider, s, baseURL, loginURL, version.WebSocketDebuggerURL, retargeted.ID)
	if secondErr == nil {
		applyRemoteDebugBrowserInfo(result, browserInfo)
		return result, nil
	}
	return nil, fmt.Errorf("%s login capture/verify failed after one reload retry: first error: %v; second error: %w", providerDisplayName(provider), firstErr, secondErr)
}

func applyRemoteDebugBrowserInfo(result *Result, info *remoteDebugBrowserInfo) {
	if result == nil || info == nil {
		return
	}
	if strings.TrimSpace(info.BrowserApp) != "" {
		result.BrowserApp = strings.TrimSpace(info.BrowserApp)
	}
	if strings.TrimSpace(info.BrowserUserDataDir) != "" {
		result.BrowserUserDataDir = strings.TrimSpace(info.BrowserUserDataDir)
	}
	if strings.TrimSpace(info.ProfileDirectory) != "" {
		result.ProfileDirectory = strings.TrimSpace(info.ProfileDirectory)
	}
}

func waitForProviderRemoteDebugTarget(ctx context.Context, debugBase, provider string, timeout time.Duration) (*remoteDebugTarget, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		targetInfo, err := findProviderRemoteDebugTarget(debugBase, provider)
		if err == nil && targetInfo != nil {
			return targetInfo, nil
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("could not find %s page on remote debug endpoint: %w", providerDisplayName(provider), lastErr)
	}
	return nil, fmt.Errorf("opened browser tab for %s was not detected on the remote debug target list", providerDisplayName(provider))
}

func waitRemoteDebugCaptureWindow(ctx context.Context, opts Options, d time.Duration) error {
	loginDebugf(opts, "waiting %s before auth capture", d.Round(100*time.Millisecond))
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func captureAndSaveRemoteDebugSession(ctx context.Context, opts Options, provider string, s *session.Session, baseURL, loginURL, browserWSURL, targetID string) (*Result, error) {
	switch provider {
	case session.ProviderDelio:
		capture, err := captureDelioSessionFromRemoteTarget(ctx, browserWSURL, targetID, loginURL)
		if err != nil {
			return nil, err
		}
		if !hasDelioAuthCookie(capture.CookieHeader) {
			return nil, errors.New("Delio auth cookies not detected in the open browser tab after 3s")
		}
		return saveDelioSessionFromCapture(s, baseURL, "", capture)
	default:
		capture, err := captureFriscoSessionFromRemoteTarget(ctx, browserWSURL, targetID, loginURL)
		if err != nil {
			return nil, err
		}
		if capture.AccessToken == "" && capture.RefreshToken == "" {
			return nil, errors.New("Frisco access/refresh token not detected in the open browser tab after 3s")
		}
		return saveFriscoSessionFromCapture(s, baseURL, "", capture)
	}
}

func reloadRemoteDebugTarget(ctx context.Context, browserWSURL, targetID, loginURL string) error {
	if strings.TrimSpace(targetID) == "" {
		return openLoginPageOnRemoteDebugBrowser(ctx, browserWSURL, loginURL)
	}
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, browserWSURL)
	defer cancelAlloc()
	taskCtx, cancelTask := chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(targetID)))
	defer cancelTask()
	if err := chromedp.Run(taskCtx, chromedp.Reload()); err == nil {
		return nil
	}
	if err := chromedp.Run(taskCtx, chromedp.Navigate(loginURL)); err == nil {
		return nil
	}
	return openLoginPageOnRemoteDebugBrowser(ctx, browserWSURL, loginURL)
}

func openLoginPageOnRemoteDebugBrowser(ctx context.Context, browserWSURL, loginURL string) error {
	loginURL = strings.TrimSpace(loginURL)
	if loginURL == "" {
		return errors.New("login URL is required")
	}

	var lastOpenErr error
	if base := remoteDebugBaseFromWebSocket(browserWSURL); base != "" {
		if _, err := openLoginPageViaRemoteDebugHTTP(base, loginURL); err == nil {
			return nil
		} else {
			lastOpenErr = err
		}
	}

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, browserWSURL)
	defer cancelAlloc()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	// Fallback path: navigate current/first page and, if needed, create a new target.
	// Retry briefly, because DevTools target attachment can be momentarily unavailable
	// right after browser start.
	var lastErr error
	for i := 0; i < 5; i++ {
		if i > 0 {
			time.Sleep(250 * time.Millisecond)
		}
		if err := chromedp.Run(taskCtx, chromedp.Navigate(loginURL)); err == nil {
			return nil
		} else {
			lastErr = err
		}

		targetID, createErr := target.CreateTarget(loginURL).WithNewWindow(true).Do(taskCtx)
		if createErr == nil {
			if err := target.ActivateTarget(targetID).Do(taskCtx); err == nil {
				return nil
			} else {
				lastErr = err
			}
		} else {
			lastErr = createErr
		}
	}

	if lastErr != nil {
		if lastOpenErr != nil {
			return fmt.Errorf("could not open login page in remote-debug browser: %w; initial attempt via /json/new failed: %v", lastErr, lastOpenErr)
		}
		return fmt.Errorf("could not open login page in remote-debug browser: %w", lastErr)
	}
	return errors.New("could not open login page in remote-debug browser")
}

func remoteDebugBaseFromWebSocket(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(u.Host)
	if host == "" {
		return ""
	}
	out := *u
	switch strings.ToLower(u.Scheme) {
	case "ws":
		out.Scheme = "http"
	case "wss":
		out.Scheme = "https"
	default:
		if out.Scheme == "" {
			return ""
		}
	}
	out.Host = host
	out.Path = ""
	out.RawQuery = ""
	out.Fragment = ""
	return strings.TrimRight(out.String(), "/")
}

func openLoginPageViaRemoteDebugHTTP(debugBase, loginURL string) (string, error) {
	if strings.TrimSpace(debugBase) == "" {
		return "", errors.New("empty remote debug base URL")
	}
	parsed, err := url.Parse(loginURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported login URL scheme %q", parsed.Scheme)
	}

	endpoint := debugBase + "/json/new?url=" + url.QueryEscape(parsed.String())
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s returned %s", endpoint, resp.Status)
	}

	var opened remoteDebugTarget
	if err := json.NewDecoder(resp.Body).Decode(&opened); err != nil {
		return "", fmt.Errorf("decode /json/new response: %w", err)
	}
	if strings.TrimSpace(opened.ID) == "" {
		return "", fmt.Errorf("remote debug /json/new did not return a target id")
	}
	return opened.ID, nil
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
	return []string{
		"http://127.0.0.1:9222",
		"http://localhost:9222",
	}
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
		target := targets[i]
		if strings.TrimSpace(target.Type) != "page" {
			continue
		}
		host := normalizeCookieHost(hostForURL(strings.TrimSpace(target.URL)))
		if host == "" {
			continue
		}
		if host == prefix || strings.HasSuffix(host, "."+prefix) {
			copy := target
			return &copy, nil
		}
	}
	return nil, nil
}

func providerRemoteDebugURLPrefix(provider string) string {
	switch session.NormalizeProvider(provider) {
	case session.ProviderDelio:
		return "delio.com.pl"
	default:
		return "frisco.pl"
	}
}

func captureDelioSessionFromRemoteTarget(ctx context.Context, browserWSURL, targetID, loginURL string) (*remoteDebugDelioCapture, error) {
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
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().WithURLs([]string{loginURL}).Do(ctx)
			return err
		}),
	)
	if err != nil {
		if strings.TrimSpace(targetID) != "" && isNoTargetRemoteDebugErr(err) {
			return captureDelioSessionFromRemoteTarget(ctx, browserWSURL, "", loginURL)
		}
		return nil, err
	}

	// Best-effort request to capture request headers required for Delio API calls.
	// This endpoint is not required for cookie-based auth capture, so we ignore failures
	// and keep the function usable even if the page blocks or changes the endpoint.
	_ = chromedp.Run(taskCtx, chromedp.Evaluate(fetchExpr, &fetchResult, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
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

	captured := remoteDebugFriscoCapture{}
	var mu sync.Mutex

	chromedp.ListenTarget(taskCtx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if uid := session.ExtractUserID(e.Request.URL); uid != "" {
				mu.Lock()
				if captured.UserID == "" {
					captured.UserID = uid
				}
				mu.Unlock()
			}
		case *network.EventRequestWillBeSentExtraInfo:
			mu.Lock()
			if captured.AccessToken == "" {
				if token := bearerFromHeaders(e.Headers); token != "" {
					captured.AccessToken = token
				}
			}
			if cookie := headerStringValue(e.Headers, "Cookie"); cookie != "" {
				if captured.CookieHeader == "" {
					captured.CookieHeader = cookie
				}
				if captured.RefreshToken == "" {
					if rt := session.ExtractRefreshTokenFromHeaderValue(cookie); rt != "" {
						captured.RefreshToken = rt
					}
				}
			}
			mu.Unlock()
		case *network.EventResponseReceivedExtraInfo:
			if rt := refreshTokenFromHeaders(e.Headers); rt != "" {
				mu.Lock()
				if captured.RefreshToken == "" {
					captured.RefreshToken = rt
				}
				mu.Unlock()
			}
		}
	})

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
		if strings.TrimSpace(targetID) != "" && isNoTargetRemoteDebugErr(err) {
			return captureFriscoSessionFromRemoteTarget(ctx, browserWSURL, "", loginURL)
		}
		return nil, err
	}

	host := hostForURL(loginURL)
	cookieHeader := cookieHeaderFromCDPCookies(cookies, host)
	cookieMap := cookieMapFromCDPCookies(cookies, host)

	mu.Lock()
	if strings.TrimSpace(captured.CookieHeader) == "" && cookieHeader != "" {
		captured.CookieHeader = cookieHeader
	}
	if strings.TrimSpace(captured.AccessToken) == "" {
		captured.AccessToken = extractFriscoAccessToken(cookieMap)
	}
	if strings.TrimSpace(captured.RefreshToken) == "" {
		captured.RefreshToken = extractFriscoRefreshToken(cookieMap, captured.CookieHeader)
	}
	if strings.TrimSpace(captured.UserID) == "" {
		captured.UserID = extractFriscoUserID(cookieMap, captured.AccessToken)
	}
	result := captured
	mu.Unlock()

	return &remoteDebugFriscoCapture{
		CookieHeader: strings.TrimSpace(result.CookieHeader),
		AccessToken:  strings.TrimSpace(result.AccessToken),
		RefreshToken: strings.TrimSpace(result.RefreshToken),
		UserID:       strings.TrimSpace(result.UserID),
	}, nil
}

func isNoTargetRemoteDebugErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "no target with given id found")
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
	if err := verifyDelioCapturedSession(s); err != nil {
		return nil, err
	}
	if err := session.SaveProvider(session.ProviderForSession(s, session.ProviderDelio), s); err != nil {
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
	s.BaseURL = baseURL
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	if strings.TrimSpace(capture.CookieHeader) != "" {
		s.Headers["Cookie"] = strings.TrimSpace(capture.CookieHeader)
	}
	if strings.TrimSpace(capture.AccessToken) != "" {
		s.Token = strings.TrimSpace(capture.AccessToken)
		s.Headers["Authorization"] = "Bearer " + strings.TrimSpace(capture.AccessToken)
	} else {
		s.Token = nil
		delete(s.Headers, "Authorization")
	}
	if strings.TrimSpace(capture.RefreshToken) != "" {
		s.RefreshToken = strings.TrimSpace(capture.RefreshToken)
	} else {
		s.RefreshToken = nil
	}
	if strings.TrimSpace(capture.UserID) != "" {
		s.UserID = strings.TrimSpace(capture.UserID)
	}
	if session.RefreshTokenString(s) != "" && (session.TokenString(s) == "" || session.UserIDString(s) == "") {
		if err := refreshFriscoAccessToken(s); err != nil {
			return nil, err
		}
	}
	if session.TokenString(s) == "" {
		return nil, errors.New("Frisco access token not detected in captured browser session")
	}
	if session.UserIDString(s) == "" {
		if uid := extractUserIDFromJWT(session.TokenString(s)); uid != "" {
			s.UserID = uid
		}
	}
	if session.UserIDString(s) != "" {
		if err := verifyFriscoCapturedSession(s); err != nil {
			return nil, err
		}
	}
	if err := session.SaveProvider(session.ProviderForSession(s, session.ProviderFrisco), s); err != nil {
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

func verifyDelioCapturedSession(s *session.Session) error {
	if s == nil {
		return errors.New("missing Delio session")
	}
	payload, err := delio.CurrentCart(s)
	if err != nil {
		return err
	}
	if _, err := delio.ExtractCurrentCart(payload); err != nil {
		return err
	}
	return nil
}

func refreshFriscoAccessToken(s *session.Session) error {
	if s == nil {
		return errors.New("missing Frisco session")
	}
	rt := strings.TrimSpace(session.RefreshTokenString(s))
	if rt == "" {
		return errors.New("missing Frisco refresh token")
	}
	result, err := httpclient.RequestJSON(s, http.MethodPost, "/app/commerce/connect/token", httpclient.RequestOpts{
		Data: map[string]any{
			"grant_type":    "refresh_token",
			"refresh_token": rt,
		},
		DataFormat: httpclient.FormatForm,
	})
	if err != nil {
		return err
	}
	payload, ok := result.(map[string]any)
	if !ok {
		return errors.New("unexpected Frisco token refresh response")
	}
	accessToken, _ := payload["access_token"].(string)
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return errors.New("missing access_token in Frisco token refresh response")
	}
	s.Token = accessToken
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	s.Headers["Authorization"] = "Bearer " + accessToken
	if refreshToken, ok := payload["refresh_token"].(string); ok {
		refreshToken = strings.TrimSpace(refreshToken)
		if refreshToken != "" {
			s.RefreshToken = refreshToken
		}
	}
	if uid := extractUserIDFromJWT(accessToken); uid != "" {
		s.UserID = uid
	}
	return nil
}

func verifyFriscoCapturedSession(s *session.Session) error {
	if s == nil {
		return errors.New("missing Frisco session")
	}
	uid := session.UserIDString(s)
	if uid == "" {
		if uid = extractUserIDFromJWT(session.TokenString(s)); uid != "" {
			s.UserID = uid
		}
	}
	if uid == "" {
		return errors.New("Frisco user_id not detected in captured browser session")
	}
	_, err := httpclient.RequestJSON(s, http.MethodGet, fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid), httpclient.RequestOpts{})
	return err
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
