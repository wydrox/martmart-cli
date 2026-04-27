// Package login implements interactive browser-based session capture using
// chromedp. It reuses a snapshot of the user's current browser profile so
// existing logged-in browser sessions can be imported for both Frisco and Delio.
package login

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/wydrox/martmart-cli/internal/session"
)

// Options configure browser-based session capture.
type Options struct {
	Provider             string
	LoginURL             string
	TimeoutSec           int
	UserDataDir          string
	ProfileDirectory     string
	Debug                bool
	KeepBrowserOnFailure bool
}

// Result holds captured authentication data from browser login.
type Result struct {
	Saved              bool
	BaseURL            string
	UserID             any
	TokenSaved         bool
	RefreshTokenSaved  bool
	CookieSaved        bool
	Provider           string
	ProfileDirectory   string
	BrowserApp         string
	BrowserUserDataDir string
}

type authCapture struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	CookieHeader string
}

type browserProfile struct {
	ExecPath         string
	SourceUserData   string
	SnapshotUserData string
	ProfileDirectory string
}

type browserLocalState struct {
	Profile struct {
		LastUsed string `json:"last_used"`
	} `json:"profile"`
}

func (opts Options) debugEnabled() bool {
	return opts.Debug || envBool("MARTMART_LOGIN_DEBUG")
}

func (opts Options) keepBrowserOnFailureEnabled() bool {
	return opts.KeepBrowserOnFailure || envBool("MARTMART_LOGIN_KEEP_BROWSER_ON_FAILURE") || envBool("MARTMART_LOGIN_KEEP_BROWSER")
}

func loginDebugf(opts Options, format string, args ...any) {
	if !opts.debugEnabled() {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "[martmart login] "+format+"\n", args...)
}

var loginSuccessOpenGracePeriod = 5 * time.Second

func envBool(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func remainingLoginPageVisibleDelay(openedAt time.Time) time.Duration {
	if loginSuccessOpenGracePeriod <= 0 || openedAt.IsZero() {
		return 0
	}
	remaining := time.Until(openedAt.Add(loginSuccessOpenGracePeriod))
	if remaining < 0 {
		return 0
	}
	return remaining
}

func keepLoginPageOpenBriefly(opts Options, openedAt time.Time) {
	if remaining := remainingLoginPageVisibleDelay(openedAt); remaining > 0 {
		loginDebugf(opts, "keeping login page open for %s before cleanup", remaining.Round(100*time.Millisecond))
		time.Sleep(remaining)
	}
}

// Run captures a provider session.
// On macOS this uses the default browser app via a temporary remote-debugging
// session against a copied profile snapshot, without reading cookies directly
// from the browser database.
func Run(ctx context.Context, opts Options) (*Result, error) {
	provider := session.NormalizeProvider(opts.Provider)
	if provider == "" {
		provider = session.ProviderFrisco
	}
	if err := session.ValidateProvider(provider); err != nil {
		return nil, err
	}
	if provider == session.ProviderUpMenu {
		return nil, fmt.Errorf("browser login is not supported for %s; import a fresh authenticated request with 'session from-curl'", session.ProviderDisplayName(provider))
	}

	if runtime.GOOS == "darwin" {
		return runWithRemoteDebugBrowser(ctx, opts)
	}

	return runWithSnapshotBrowser(ctx, opts, provider)
}

func runWithSnapshotBrowser(ctx context.Context, opts Options, provider string) (*Result, error) {
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

	profile, cleanup, err := prepareBrowserProfile(opts.UserDataDir, opts.ProfileDirectory)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	captured := authCapture{}
	var mu sync.Mutex

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(profile.ExecPath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("user-data-dir", profile.SnapshotUserData),
		chromedp.Flag("profile-directory", profile.ProfileDirectory),
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
				if provider == session.ProviderDelio {
					if hasDelioAuthCookie(cookie) {
						captured.CookieHeader = cookie
					}
				} else if captured.CookieHeader == "" {
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
	openedAt := time.Now()

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
		gotCookie := captured.CookieHeader != ""
		mu.Unlock()
		if gotToken && accessDetectedAt.IsZero() {
			accessDetectedAt = time.Now()
		}
		if provider == session.ProviderDelio {
			if gotCookie {
				break
			}
			continue
		}
		if provider != session.ProviderDelio && gotRefresh {
			break
		}
		if gotToken && !accessDetectedAt.IsZero() && time.Since(accessDetectedAt) > 8*time.Second {
			break
		}
	}

	cookieHeader, refreshToken := collectCookies(cdpCtx, baseURL, provider)
	mu.Lock()
	if captured.CookieHeader == "" && cookieHeader != "" {
		captured.CookieHeader = cookieHeader
	}
	if captured.RefreshToken == "" && refreshToken != "" {
		captured.RefreshToken = refreshToken
	}
	accessToken := captured.AccessToken
	refreshToken = captured.RefreshToken
	userID := captured.UserID
	cookieHeader = captured.CookieHeader
	mu.Unlock()

	if provider == session.ProviderDelio {
		if !hasDelioAuthCookie(cookieHeader) {
			return nil, fmt.Errorf("Delio auth cookies not detected — open a logged-in Delio page or sign in in the opened browser window and wait for the session to load")
		}
		result, saveErr := saveDelioSessionFromCapture(s, baseURL, profile.ProfileDirectory, &remoteDebugDelioCapture{
			Headers:      map[string]string{},
			CookieHeader: cookieHeader,
			UserID:       userID,
		})
		if saveErr == nil && result != nil {
			result.BrowserApp = filepath.Base(profile.ExecPath)
			result.BrowserUserDataDir = profile.SourceUserData
		}
		keepLoginPageOpenBriefly(opts, openedAt)
		return result, saveErr
	}
	if accessToken == "" && refreshToken == "" {
		return nil, fmt.Errorf("Frisco auth data not detected — open a logged-in Frisco page or sign in and navigate to cart/account to trigger API requests")
	}
	result, saveErr := saveFriscoSessionFromCapture(s, baseURL, profile.ProfileDirectory, &remoteDebugFriscoCapture{
		CookieHeader: cookieHeader,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       userID,
	})
	if saveErr == nil && result != nil {
		result.BrowserApp = filepath.Base(profile.ExecPath)
		result.BrowserUserDataDir = profile.SourceUserData
	}
	keepLoginPageOpenBriefly(opts, openedAt)
	return result, saveErr
}

func defaultStartURL(provider string) string {
	switch session.NormalizeProvider(provider) {
	case session.ProviderDelio:
		return "https://delio.com.pl/"
	default:
		return "https://www.frisco.pl/login"
	}
}

func collectCookies(cdpCtx context.Context, baseURL, provider string) (string, string) {
	cookies, err := network.GetCookies().Do(cdpCtx)
	if err != nil || len(cookies) == 0 {
		return "", ""
	}
	host := hostForURL(baseURL)
	pairs := make([]string, 0, len(cookies))
	refreshToken := ""
	for _, ck := range cookies {
		if ck == nil || ck.Name == "" {
			continue
		}
		if host != "" && !cookieMatchesHost(ck.Domain, host) {
			continue
		}
		pair := ck.Name + "=" + ck.Value
		if session.NormalizeProvider(provider) != session.ProviderDelio && refreshToken == "" {
			if rt := session.ExtractRefreshTokenFromHeaderValue(pair); rt != "" {
				refreshToken = rt
			}
		}
		pairs = append(pairs, pair)
	}
	return strings.Join(pairs, "; "), refreshToken
}

func hostForURL(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(strings.TrimSpace(raw), "://")
	if len(parts) != 2 {
		return ""
	}
	hostPart := parts[1]
	if idx := strings.IndexByte(hostPart, '/'); idx >= 0 {
		hostPart = hostPart[:idx]
	}
	if idx := strings.IndexByte(hostPart, ':'); idx >= 0 {
		hostPart = hostPart[:idx]
	}
	return strings.ToLower(strings.TrimSpace(hostPart))
}

func cookieMatchesHost(cookieDomain, host string) bool {
	cd := normalizeCookieHost(cookieDomain)
	h := normalizeCookieHost(host)
	if cd == "" || h == "" {
		return false
	}
	return h == cd || strings.HasSuffix(h, "."+cd)
}

func normalizeCookieHost(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	raw = strings.TrimPrefix(raw, ".")
	raw = strings.TrimPrefix(raw, "www.")
	return raw
}

func hasDelioAuthCookie(cookieHeader string) bool {
	parts := strings.Split(cookieHeader, ";")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(trimmed[:idx])
		if isDelioAuthCookieName(name) {
			return true
		}
	}
	return false
}

func isDelioAuthCookieName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authtoken", "idtoken", "refreshtoken":
		return true
	default:
		return false
	}
}

func prepareBrowserProfile(userDataDir, profileDir string) (*browserProfile, func(), error) {
	execPath, err := findBrowserExecutable()
	if err != nil {
		return nil, nil, err
	}
	sourceUserData, err := resolveUserDataDir(userDataDir)
	if err != nil {
		return nil, nil, err
	}
	profileDirectory := strings.TrimSpace(profileDir)
	if profileDirectory == "" {
		profileDirectory = detectLastUsedProfile(sourceUserData)
	}
	if profileDirectory == "" {
		profileDirectory = "Default"
	}
	if err := ensureProfileExists(sourceUserData, profileDirectory); err != nil {
		return nil, nil, err
	}
	snapshotUserData, err := os.MkdirTemp("", "martmart-browser-profile-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(snapshotUserData) }
	if err := copyBrowserProfileSnapshot(sourceUserData, snapshotUserData, profileDirectory); err != nil {
		cleanup()
		return nil, nil, err
	}
	return &browserProfile{
		ExecPath:         execPath,
		SourceUserData:   sourceUserData,
		SnapshotUserData: snapshotUserData,
		ProfileDirectory: profileDirectory,
	}, cleanup, nil
}

func findBrowserExecutable() (string, error) {
	for _, p := range BrowserCandidates() {
		if resolved, err := exec.LookPath(p); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf(
		"supported browser executable not found. Checked: %s",
		strings.Join(BrowserCandidates(), ", "),
	)
}

// CheckBrowserInstalled verifies that a supported browser executable is available.
func CheckBrowserInstalled() error {
	_, err := findBrowserExecutable()
	return err
}

// BrowserCandidates returns platform-specific browser executable paths.
func BrowserCandidates() []string {
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

func resolveUserDataDir(explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		if info, err := os.Stat(explicit); err == nil && info.IsDir() {
			return explicit, nil
		}
		return "", fmt.Errorf("browser user data dir not found: %s", explicit)
	}
	for _, candidate := range browserUserDataDirCandidates() {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("browser user data dir not found. Checked: %s", strings.Join(browserUserDataDirCandidates(), ", "))
}

func browserUserDataDirCandidates() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome"),
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome Canary"),
			filepath.Join(home, "Library", "Application Support", "Chromium"),
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		return []string{
			filepath.Join(localAppData, "Google", "Chrome", "User Data"),
			filepath.Join(localAppData, "Chromium", "User Data"),
		}
	default:
		return []string{
			filepath.Join(home, ".config", "google-chrome"),
			filepath.Join(home, ".config", "google-chrome-beta"),
			filepath.Join(home, ".config", "chromium"),
		}
	}
}

func detectLastUsedProfile(userDataDir string) string {
	data, err := os.ReadFile(filepath.Join(userDataDir, "Local State"))
	if err != nil {
		return ""
	}
	var state browserLocalState
	if err := json.Unmarshal(data, &state); err != nil {
		return ""
	}
	return strings.TrimSpace(state.Profile.LastUsed)
}

func ensureProfileExists(userDataDir, profileDirectory string) error {
	profilePath := filepath.Join(userDataDir, profileDirectory)
	if info, err := os.Stat(profilePath); err == nil && info.IsDir() {
		return nil
	}
	return fmt.Errorf("browser profile %q not found in %s", profileDirectory, userDataDir)
}

func copyBrowserProfileSnapshot(srcUserData, dstUserData, profileDirectory string) error {
	if err := os.MkdirAll(dstUserData, 0o700); err != nil {
		return err
	}
	localStateSrc := filepath.Join(srcUserData, "Local State")
	localStateDst := filepath.Join(dstUserData, "Local State")
	if _, err := os.Stat(localStateSrc); err == nil {
		if err := copyFile(localStateSrc, localStateDst); err != nil {
			return err
		}
	}
	srcProfile := filepath.Join(srcUserData, profileDirectory)
	dstProfile := filepath.Join(dstUserData, profileDirectory)
	return copyDirFiltered(srcProfile, dstProfile, func(rel string, d fs.DirEntry) bool {
		name := d.Name()
		skipNames := map[string]struct{}{
			"Cache":         {},
			"Code Cache":    {},
			"GPUCache":      {},
			"DawnCache":     {},
			"GrShaderCache": {},
			"ShaderCache":   {},
			"Crashpad":      {},
			"Crash Reports": {},
			"Media Cache":   {},
		}
		_, skip := skipNames[name]
		return skip && d.IsDir()
	})
}

func copyDirFiltered(src, dst string, skip func(rel string, d fs.DirEntry) bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o700)
		}
		if skip != nil && skip(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(info.Mode())
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
