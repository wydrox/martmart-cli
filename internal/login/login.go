// Package login implements interactive browser login for Frisco using chromedp.
// Shared between CLI and MCP server.
package login

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/rrudol/frisco/internal/session"
)

// Result holds captured authentication data from browser login.
type Result struct {
	Saved             bool
	BaseURL           string
	UserID            any
	TokenSaved        bool
	RefreshTokenSaved bool
	CookieSaved       bool
}

// Run opens a Chrome browser for interactive Frisco login, captures auth
// credentials from network traffic, and saves the session. It blocks until
// credentials are captured or the timeout expires.
func Run(ctx context.Context, loginURL string, timeoutSec int) (*Result, error) {
	s, err := session.Load()
	if err != nil {
		return nil, err
	}

	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = session.DefaultBaseURL
	}
	if loginURL == "" {
		loginURL = "https://www.frisco.pl/login"
	}
	if timeoutSec <= 0 {
		timeoutSec = 180
	}

	if err := CheckChromeInstalled(); err != nil {
		return nil, err
	}

	type authCapture struct {
		AccessToken  string
		RefreshToken string
		UserID       string
		CookieHeader string
	}
	captured := authCapture{}
	var mu sync.Mutex

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	cdpCtx, cancelCdp := chromedp.NewContext(allocCtx)
	defer cancelCdp()

	chromedp.ListenTarget(cdpCtx, func(ev any) {
		mu.Lock()
		defer mu.Unlock()

		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if captured.UserID == "" {
				if uid := session.ExtractUserID(e.Request.URL); uid != "" {
					captured.UserID = uid
				}
			}
		case *network.EventRequestWillBeSentExtraInfo:
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
		case *network.EventResponseReceivedExtraInfo:
			if captured.RefreshToken == "" {
				if rt := refreshTokenFromHeaders(e.Headers); rt != "" {
					captured.RefreshToken = rt
				}
			}
		}
	})

	if err := chromedp.Run(cdpCtx,
		network.Enable(),
		chromedp.Navigate(loginURL),
	); err != nil {
		return nil, fmt.Errorf("could not start login browser: %w", err)
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	var accessDetectedAt time.Time
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
		mu.Lock()
		gotToken := captured.AccessToken != ""
		gotRefresh := captured.RefreshToken != ""
		mu.Unlock()
		if gotToken && accessDetectedAt.IsZero() {
			accessDetectedAt = time.Now()
		}
		if gotToken && gotRefresh {
			break
		}
		if gotToken && !accessDetectedAt.IsZero() && time.Since(accessDetectedAt) > 8*time.Second {
			break
		}
	}

	allCookies, err := network.GetCookies().Do(cdpCtx)
	if err == nil && len(allCookies) > 0 {
		pairs := make([]string, 0, len(allCookies))
		for _, ck := range allCookies {
			if ck == nil || ck.Name == "" {
				continue
			}
			pairs = append(pairs, ck.Name+"="+ck.Value)
		}
		if len(pairs) > 0 {
			mu.Lock()
			captured.CookieHeader = strings.Join(pairs, "; ")
			if captured.RefreshToken == "" {
				if rt := session.ExtractRefreshTokenFromHeaderValue(captured.CookieHeader); rt != "" {
					captured.RefreshToken = rt
				}
			}
			mu.Unlock()
		}
	}

	mu.Lock()
	accessToken := captured.AccessToken
	refreshToken := captured.RefreshToken
	userID := captured.UserID
	cookieHeader := captured.CookieHeader
	mu.Unlock()

	if accessToken == "" {
		return nil, fmt.Errorf("access token not detected — log in on frisco.pl and navigate to your cart or account page to trigger API requests")
	}

	s.BaseURL = baseURL
	s.Token = accessToken
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	s.Headers["Authorization"] = "Bearer " + accessToken
	if cookieHeader != "" {
		s.Headers["Cookie"] = cookieHeader
	}
	if refreshToken != "" {
		s.RefreshToken = refreshToken
	}
	if userID != "" {
		s.UserID = userID
	}

	if err := session.Save(s); err != nil {
		return nil, err
	}

	return &Result{
		Saved:             true,
		BaseURL:           s.BaseURL,
		UserID:            s.UserID,
		TokenSaved:        session.TokenString(s) != "",
		RefreshTokenSaved: session.RefreshTokenString(s) != "",
		CookieSaved:       s.Headers["Cookie"] != "",
	}, nil
}

// CheckChromeInstalled verifies that a Chrome/Chromium browser is available.
func CheckChromeInstalled() error {
	paths := ChromeCandidates()
	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			return nil
		}
	}
	return fmt.Errorf(
		"Chrome/Chromium not found. Install from: https://www.google.com/chrome/\nChecked: %s",
		strings.Join(paths, ", "),
	)
}

// ChromeCandidates returns platform-specific Chrome/Chromium executable paths.
func ChromeCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"google-chrome",
			"chromium",
		}
	case "windows":
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			"chrome",
		}
	default:
		return []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
		}
	}
}

func bearerFromHeaders(headers network.Headers) string {
	for k := range headers {
		if !strings.EqualFold(k, "authorization") {
			continue
		}
		value := strings.TrimSpace(fmt.Sprint(headers[k]))
		parts := strings.SplitN(value, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func headerStringValue(headers network.Headers, name string) string {
	for k := range headers {
		if strings.EqualFold(k, name) {
			return strings.TrimSpace(fmt.Sprint(headers[k]))
		}
	}
	return ""
}

func refreshTokenFromHeaders(headers network.Headers) string {
	for k := range headers {
		if strings.EqualFold(k, "set-cookie") || strings.EqualFold(k, "cookie") {
			raw := strings.TrimSpace(fmt.Sprint(headers[k]))
			if token := session.ExtractRefreshTokenFromHeaderValue(raw); token != "" {
				return token
			}
		}
	}
	return ""
}
