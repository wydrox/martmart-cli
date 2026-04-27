//go:build darwin

package login

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"

	"github.com/wydrox/martmart-cli/internal/session"
)

type activeBrowserProfile struct {
	AppName          string
	BundleID         string
	ExecPath         string
	UserDataDir      string
	ProfileDirectory string
	KeychainService  string
	KeychainAccount  string
}

type browserCookie struct {
	HostKey           string
	Name              string
	Value             string
	EncryptedValueHex string
	Path              string
}

func runWithExistingBrowser(ctx context.Context, opts Options) (*Result, error) {
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
	timeoutSec := opts.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 180
	}

	profile, err := detectPreferredBrowserProfile(opts)
	if err != nil {
		return nil, err
	}
	loginDebugf(opts, "provider=%s login_url=%s timeout=%ds browser=%s user_data_dir=%s profile=%s", provider, loginURL, timeoutSec, profile.AppName, profile.UserDataDir, profile.ProfileDirectory)

	browserProfile, cleanupProfile, err := prepareActiveBrowserSnapshot(profile)
	if err != nil {
		return nil, err
	}
	shouldCleanup := true
	defer func() {
		if !shouldCleanup {
			loginDebugf(opts, "keeping temporary browser profile for inspection: %s", browserProfile.SnapshotUserData)
			return
		}
		cleanupProfile()
	}()

	debugBase, cleanupBrowser, err := launchBrowserWithRemoteDebug(browserProfile, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !shouldCleanup {
			loginDebugf(opts, "keeping spawned browser window open for inspection")
			return
		}
		cleanupBrowser()
	}()

	version, err := waitForRemoteDebugVersion(ctx, debugBase, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		if opts.keepBrowserOnFailureEnabled() {
			shouldCleanup = false
		}
		return nil, err
	}
	loginDebugf(opts, "remote debugging endpoint ready: %s", debugBase)
	if err := openLoginPageOnRemoteDebugBrowser(ctx, version.WebSocketDebuggerURL, loginURL); err != nil {
		if opts.keepBrowserOnFailureEnabled() {
			shouldCleanup = false
		}
		return nil, err
	}
	loginDebugf(opts, "opened login page in spawned browser")
	openedAt := time.Now()

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			if opts.keepBrowserOnFailureEnabled() {
				shouldCleanup = false
			}
			return nil, ctx.Err()
		default:
		}

		targetInfo, err := findProviderRemoteDebugTarget(debugBase, provider)
		if err != nil {
			lastErr = err
			loginDebugf(opts, "target discovery error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		switch provider {
		case session.ProviderDelio:
			if targetInfo == nil {
				lastErr = fmt.Errorf("waiting for a Delio page in %s profile %q", profile.AppName, profile.ProfileDirectory)
				loginDebugf(opts, "%v", lastErr)
				time.Sleep(1 * time.Second)
				continue
			}
			capture, err := captureDelioSessionFromRemoteTarget(ctx, version.WebSocketDebuggerURL, targetInfo.ID, loginURL)
			if err == nil && hasDelioAuthCookie(capture.CookieHeader) {
				result, saveErr := saveDelioSessionFromCapture(s, baseURL, profile.ProfileDirectory, capture)
				if saveErr == nil && result != nil {
					result.BrowserApp = profile.AppName
					result.BrowserUserDataDir = profile.UserDataDir
				}
				keepLoginPageOpenBriefly(opts, openedAt)
				return result, saveErr
			}
			if err != nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("Delio auth cookies not detected in %s profile %q yet", profile.AppName, profile.ProfileDirectory)
			}
			loginDebugf(opts, "%v", lastErr)
		default:
			targetID := ""
			if targetInfo != nil {
				targetID = targetInfo.ID
			}
			capture, err := captureFriscoSessionFromRemoteTarget(ctx, version.WebSocketDebuggerURL, targetID, loginURL)
			if err == nil && (capture.AccessToken != "" || capture.RefreshToken != "") {
				result, saveErr := saveFriscoSessionFromCapture(s, baseURL, profile.ProfileDirectory, capture)
				if saveErr == nil && result != nil {
					result.BrowserApp = profile.AppName
					result.BrowserUserDataDir = profile.UserDataDir
				}
				keepLoginPageOpenBriefly(opts, openedAt)
				return result, saveErr
			}
			if err != nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("Frisco auth data not detected in %s profile %q yet", profile.AppName, profile.ProfileDirectory)
			}
			loginDebugf(opts, "%v", lastErr)
		}

		time.Sleep(1 * time.Second)
	}

	if opts.keepBrowserOnFailureEnabled() {
		shouldCleanup = false
	}
	if lastErr != nil {
		return nil, lastErr
	}
	if provider == session.ProviderDelio {
		return nil, fmt.Errorf("Delio auth cookies not detected in %s profile %q within %ds", profile.AppName, profile.ProfileDirectory, timeoutSec)
	}
	return nil, fmt.Errorf("Frisco auth data not detected in %s profile %q within %ds", profile.AppName, profile.ProfileDirectory, timeoutSec)
}

func detectPreferredBrowserProfile(opts Options) (*activeBrowserProfile, error) {
	if strings.TrimSpace(opts.UserDataDir) != "" {
		resolved, err := resolveUserDataDir(opts.UserDataDir)
		if err != nil {
			return nil, err
		}
		resolved = normalizeBrowserUserDataDir(resolved)
		prof := strings.TrimSpace(opts.ProfileDirectory)
		if prof == "" {
			prof = detectLastUsedProfile(resolved)
		}
		if prof == "" {
			prof = "Default"
		}
		if err := ensureProfileExists(resolved, prof); err != nil {
			return nil, err
		}
		ap := inferProfileMetadataFromUserDataDir(resolved)
		ap.UserDataDir = resolved
		ap.ProfileDirectory = prof
		if ap.AppName == "" {
			ap.AppName = "Browser"
		}
		return &ap, nil
	}

	bundleID, err := defaultMacBrowserBundleID()
	if err != nil {
		return nil, err
	}
	profile, ok := knownDarwinBrowsersByBundleID[strings.TrimSpace(bundleID)]
	if !ok {
		return nil, fmt.Errorf("default browser %q is not supported yet; use --user-data-dir/--profile-directory", bundleID)
	}
	return finalizeBrowserProfile(profile, opts.ProfileDirectory)
}

var knownDarwinBrowsers = map[string]activeBrowserProfile{
	"Google Chrome": {
		AppName:         "Google Chrome",
		BundleID:        "com.google.Chrome",
		ExecPath:        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Google", "Chrome"),
		KeychainService: "Chrome Safe Storage",
		KeychainAccount: "Chrome",
	},
	"Chromium": {
		AppName:         "Chromium",
		BundleID:        "org.chromium.Chromium",
		ExecPath:        "/Applications/Chromium.app/Contents/MacOS/Chromium",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Chromium"),
		KeychainService: "Chromium Safe Storage",
		KeychainAccount: "Chromium",
	},
	"Brave Browser": {
		AppName:         "Brave Browser",
		BundleID:        "com.brave.Browser",
		ExecPath:        "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "BraveSoftware", "Brave-Browser"),
		KeychainService: "Brave Safe Storage",
		KeychainAccount: "Brave",
	},
	"Arc": {
		AppName:         "Arc",
		BundleID:        "company.thebrowser.Browser",
		ExecPath:        "/Applications/Arc.app/Contents/MacOS/Arc",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Arc", "User Data"),
		KeychainService: "Arc Safe Storage",
		KeychainAccount: "Arc",
	},
	"Microsoft Edge": {
		AppName:         "Microsoft Edge",
		BundleID:        "com.microsoft.edgemac",
		ExecPath:        "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Microsoft Edge"),
		KeychainService: "Microsoft Edge Safe Storage",
		KeychainAccount: "Microsoft Edge",
	},
	"Helium": {
		AppName:         "Helium",
		BundleID:        "net.imput.helium",
		ExecPath:        "/Applications/Helium.app/Contents/MacOS/Helium",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "net.imput.helium"),
		KeychainService: "Chromium Safe Storage",
		KeychainAccount: "Chromium",
	},
}

var knownDarwinBrowsersByBundleID = map[string]activeBrowserProfile{
	"com.google.Chrome":          knownDarwinBrowsers["Google Chrome"],
	"org.chromium.Chromium":      knownDarwinBrowsers["Chromium"],
	"com.brave.Browser":          knownDarwinBrowsers["Brave Browser"],
	"company.thebrowser.Browser": knownDarwinBrowsers["Arc"],
	"com.microsoft.edgemac":      knownDarwinBrowsers["Microsoft Edge"],
	"net.imput.helium":           knownDarwinBrowsers["Helium"],
}

func finalizeBrowserProfile(profile activeBrowserProfile, requestedProfileDir string) (*activeBrowserProfile, error) {
	profile.UserDataDir = normalizeBrowserUserDataDir(profile.UserDataDir)
	if strings.TrimSpace(profile.ProfileDirectory) == "" {
		profile.ProfileDirectory = strings.TrimSpace(requestedProfileDir)
	}
	if strings.TrimSpace(profile.ProfileDirectory) == "" {
		profile.ProfileDirectory = detectLastUsedProfile(profile.UserDataDir)
	}
	if strings.TrimSpace(profile.ProfileDirectory) == "" {
		profile.ProfileDirectory = "Default"
	}
	if strings.TrimSpace(profile.ExecPath) != "" {
		if _, err := os.Stat(profile.ExecPath); err != nil {
			return nil, fmt.Errorf("browser executable not found for %s: %s", profile.AppName, profile.ExecPath)
		}
	}
	if err := ensureProfileExists(profile.UserDataDir, profile.ProfileDirectory); err != nil {
		return nil, err
	}
	copy := profile
	return &copy, nil
}

func normalizeBrowserUserDataDir(userDataDir string) string {
	resolved := strings.TrimSpace(userDataDir)
	if resolved == "" {
		return ""
	}
	if info, err := os.Stat(filepath.Join(resolved, "User Data")); err == nil && info.IsDir() {
		if _, err := os.Stat(filepath.Join(resolved, "Local State")); err != nil {
			return filepath.Join(resolved, "User Data")
		}
	}
	return resolved
}

func inferProfileMetadataFromUserDataDir(userDataDir string) activeBrowserProfile {
	resolved := normalizeBrowserUserDataDir(userDataDir)
	for _, profile := range knownDarwinBrowsers {
		if resolved == normalizeBrowserUserDataDir(profile.UserDataDir) {
			return profile
		}
	}
	base := filepath.Base(resolved)
	if strings.EqualFold(base, "User Data") {
		base = filepath.Base(filepath.Dir(resolved))
	}
	if strings.EqualFold(base, "Arc") {
		return knownDarwinBrowsers["Arc"]
	}
	if strings.EqualFold(base, "net.imput.helium") {
		return knownDarwinBrowsers["Helium"]
	}
	if strings.EqualFold(base, "Chromium") {
		return knownDarwinBrowsers["Chromium"]
	}
	if strings.EqualFold(base, "Chrome") {
		return knownDarwinBrowsers["Google Chrome"]
	}
	return activeBrowserProfile{}
}

func frontmostMacBrowserName() (string, error) {
	cmd := exec.Command("osascript", "-e", `tell application "System Events" to get name of first application process whose frontmost is true`)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not detect frontmost macOS application: %w", err)
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return "", errors.New("could not detect frontmost browser")
	}
	return name, nil
}

type launchServicesHandler struct {
	RoleAll   string `json:"LSHandlerRoleAll"`
	URLScheme string `json:"LSHandlerURLScheme"`
}

func defaultMacBrowserBundleID() (string, error) {
	path := filepath.Join(os.Getenv("HOME"), "Library", "Preferences", "com.apple.LaunchServices", "com.apple.launchservices.secure.plist")
	cmd := exec.Command("plutil", "-extract", "LSHandlers", "json", "-o", "-", path)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not detect default browser: %w", err)
	}
	var handlers []launchServicesHandler
	if err := json.Unmarshal(out, &handlers); err != nil {
		return "", fmt.Errorf("could not parse default browser handlers: %w", err)
	}
	for _, handler := range handlers {
		if (handler.URLScheme == "https" || handler.URLScheme == "http") && strings.TrimSpace(handler.RoleAll) != "" {
			return strings.TrimSpace(handler.RoleAll), nil
		}
	}
	return "", errors.New("could not detect default browser bundle id")
}

func openURLInDefaultBrowser(url string) error {
	if err := exec.Command("open", url).Run(); err != nil {
		return fmt.Errorf("could not open login URL in default browser: %w", err)
	}
	return nil
}

func openURLInBrowser(appName, url string) error {
	args := []string{}
	if strings.TrimSpace(appName) != "" && appName != "Browser" {
		args = append(args, "-a", appName)
	}
	args = append(args, url)
	if err := exec.Command("open", args...).Run(); err != nil {
		return fmt.Errorf("could not open login URL in %s: %w", appName, err)
	}
	return nil
}

func prepareActiveBrowserSnapshot(profile *activeBrowserProfile) (*browserProfile, func(), error) {
	if profile == nil {
		return nil, nil, errors.New("missing browser profile")
	}
	execPath := strings.TrimSpace(profile.ExecPath)
	if execPath == "" {
		return nil, nil, fmt.Errorf("browser executable is not configured for %s", profile.AppName)
	}
	if _, err := os.Stat(execPath); err != nil {
		return nil, nil, fmt.Errorf("browser executable not found for %s: %s", profile.AppName, execPath)
	}
	snapshotUserData, err := os.MkdirTemp("", "martmart-browser-profile-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(snapshotUserData) }
	if err := copyBrowserProfileSnapshot(profile.UserDataDir, snapshotUserData, profile.ProfileDirectory); err != nil {
		cleanup()
		return nil, nil, err
	}
	return &browserProfile{
		ExecPath:         execPath,
		SourceUserData:   profile.UserDataDir,
		SnapshotUserData: snapshotUserData,
		ProfileDirectory: profile.ProfileDirectory,
	}, cleanup, nil
}

func launchBrowserWithRemoteDebug(profile *browserProfile, opts Options) (string, func(), error) {
	if profile == nil {
		return "", nil, errors.New("missing browser profile")
	}
	remoteDebugPort, err := pickFreeLocalhostPort()
	if err != nil {
		return "", nil, fmt.Errorf("could not allocate remote debug port: %w", err)
	}
	debugBase := "http://127.0.0.1:" + remoteDebugPort
	args := []string{
		"--remote-debugging-port=" + remoteDebugPort,
		"--user-data-dir=" + profile.SnapshotUserData,
		"--profile-directory=" + profile.ProfileDirectory,
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		"about:blank",
	}
	loginDebugf(opts, "launching browser executable=%s remote_debug_port=%s snapshot_user_data=%s profile=%s", profile.ExecPath, remoteDebugPort, profile.SnapshotUserData, profile.ProfileDirectory)
	cmd := exec.Command(profile.ExecPath, args...)
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("could not start browser with remote debugging: %w", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()
	cleanup := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-waitCh:
		case <-time.After(2 * time.Second):
		}
	}
	return debugBase, cleanup, nil
}

func ensurePreferredBrowserRemoteDebugOn9222(ctx context.Context, opts Options) (string, *remoteDebugBrowserInfo, error) {
	profile, err := detectPreferredBrowserProfile(opts)
	if err != nil {
		return "", nil, err
	}
	if err := restartBrowserOnRemoteDebugPort(ctx, profile, opts, "9222"); err != nil {
		return "", nil, err
	}
	return "http://127.0.0.1:9222", &remoteDebugBrowserInfo{
		BrowserApp:         profile.AppName,
		BrowserUserDataDir: profile.UserDataDir,
		ProfileDirectory:   profile.ProfileDirectory,
	}, nil
}

func restartBrowserOnRemoteDebugPort(ctx context.Context, profile *activeBrowserProfile, opts Options, port string) error {
	if profile == nil {
		return errors.New("missing browser profile")
	}
	port = strings.TrimSpace(port)
	if port == "" {
		return errors.New("missing remote debug port")
	}
	loginDebugf(opts, "restarting %s on remote debug port %s using user_data_dir=%s profile=%s", profile.AppName, port, profile.UserDataDir, profile.ProfileDirectory)
	if err := quitBrowserApp(profile); err != nil {
		return err
	}
	if err := waitForBrowserStopped(ctx, profile, 15*time.Second); err != nil {
		return err
	}
	if err := launchBrowserOnRemoteDebugPort(profile, port, opts); err != nil {
		return err
	}
	return nil
}

func quitBrowserApp(profile *activeBrowserProfile) error {
	if profile == nil {
		return errors.New("missing browser profile")
	}
	scriptName := appleScriptQuoted(profile.AppName)
	target := fmt.Sprintf("application %q", scriptName)
	if strings.TrimSpace(profile.BundleID) != "" {
		target = fmt.Sprintf("application id %q", appleScriptQuoted(profile.BundleID))
	}
	script := fmt.Sprintf(`tell application "System Events"
	if exists application process %q then
		tell %s to quit
	end if
end tell`, scriptName, target)
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not quit %s before restart: %w (%s)", profile.AppName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func waitForBrowserStopped(ctx context.Context, profile *activeBrowserProfile, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	forcedTERM := false
	forcedKILL := false
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		running, err := isBrowserRunning(profile)
		if err != nil {
			return err
		}
		if !running {
			return nil
		}
		remaining := time.Until(deadline)
		if !forcedTERM && remaining <= 10*time.Second {
			_ = forceSignalBrowser(profile, "-TERM")
			forcedTERM = true
		}
		if !forcedKILL && remaining <= 4*time.Second {
			_ = forceSignalBrowser(profile, "-KILL")
			forcedKILL = true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("%s did not stop before relaunch on port 9222", profile.AppName)
}

func isBrowserRunning(profile *activeBrowserProfile) (bool, error) {
	if profile == nil {
		return false, errors.New("missing browser profile")
	}
	cmd := exec.Command("pgrep", "-x", profile.AppName)
	if err := cmd.Run(); err == nil {
		return true, nil
	} else if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}

func forceSignalBrowser(profile *activeBrowserProfile, signal string) error {
	if profile == nil {
		return errors.New("missing browser profile")
	}
	signal = strings.TrimSpace(signal)
	if signal == "" {
		signal = "-TERM"
	}
	cmd := exec.Command("pkill", signal, "-x", profile.AppName)
	if err := cmd.Run(); err == nil {
		return nil
	} else if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return nil
	} else {
		return err
	}
}

func launchBrowserOnRemoteDebugPort(profile *activeBrowserProfile, port string, opts Options) error {
	if profile == nil {
		return errors.New("missing browser profile")
	}
	if strings.TrimSpace(profile.ExecPath) == "" {
		return fmt.Errorf("browser executable is not configured for %s", profile.AppName)
	}
	args := []string{}
	if strings.TrimSpace(profile.BundleID) != "" {
		args = append(args, "-n", "-b", strings.TrimSpace(profile.BundleID), "--args")
	} else {
		args = append(args, "-n", "-a", profile.AppName, "--args")
	}
	args = append(args,
		"--remote-debugging-port="+strings.TrimSpace(port),
		"--user-data-dir="+profile.UserDataDir,
		"--profile-directory="+profile.ProfileDirectory,
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		"about:blank",
	)
	loginDebugf(opts, "launching %s via open with remote_debug_port=%s user_data_dir=%s profile=%s", profile.AppName, strings.TrimSpace(port), profile.UserDataDir, profile.ProfileDirectory)
	if out, err := exec.Command("open", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("could not launch %s on port %s: %w (%s)", profile.AppName, port, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func appleScriptQuoted(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), `"`, `\\"`)
}

func pickFreeLocalhostPort() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || tcpAddr == nil || tcpAddr.Port <= 0 {
		return "", errors.New("could not determine TCP port")
	}
	return strconv.Itoa(tcpAddr.Port), nil
}

func waitForRemoteDebugVersion(ctx context.Context, debugBase string, timeout time.Duration) (*remoteDebugVersion, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var version remoteDebugVersion
		if err := remoteDebugGetJSON(debugBase+"/json/version", &version); err == nil {
			if strings.TrimSpace(version.WebSocketDebuggerURL) != "" {
				return &version, nil
			}
			lastErr = fmt.Errorf("missing webSocketDebuggerURL on %s/json/version", debugBase)
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out waiting for browser remote debugging on %s", debugBase)
	}
	return nil, lastErr
}

func cookieHeaderAndMapFromBrowserProfile(profile *activeBrowserProfile, domain string) (string, map[string]string, error) {
	if profile == nil {
		return "", nil, errors.New("missing browser profile")
	}
	cookies, err := readBrowserCookies(profile, domain)
	if err != nil {
		return "", nil, err
	}
	if len(cookies) == 0 {
		return "", map[string]string{}, nil
	}
	parts := make([]string, 0, len(cookies))
	cookieMap := map[string]string{}
	for _, ck := range cookies {
		value := ck.Value
		if value == "" && ck.EncryptedValueHex != "" {
			value, err = decryptBrowserCookieValue(profile, ck.EncryptedValueHex)
			if err != nil || value == "" {
				continue
			}
		}
		parts = append(parts, ck.Name+"="+value)
		cookieMap[ck.Name] = value
	}
	sort.Strings(parts)
	return strings.Join(parts, "; "), cookieMap, nil
}

func cookieHeaderFromBrowserProfile(profile *activeBrowserProfile, domain string) (string, error) {
	header, _, err := cookieHeaderAndMapFromBrowserProfile(profile, domain)
	return header, err
}

func readBrowserCookies(profile *activeBrowserProfile, domain string) ([]browserCookie, error) {
	cookieDB := filepath.Join(profile.UserDataDir, profile.ProfileDirectory, "Network", "Cookies")
	if _, err := os.Stat(cookieDB); err != nil {
		cookieDB = filepath.Join(profile.UserDataDir, profile.ProfileDirectory, "Cookies")
	}
	if _, err := os.Stat(cookieDB); err != nil {
		return nil, fmt.Errorf("cookies database not found in %s profile %q", profile.UserDataDir, profile.ProfileDirectory)
	}
	tmpDir, err := os.MkdirTemp("", "martmart-cookies-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	tmpDB := filepath.Join(tmpDir, "Cookies.sqlite")
	if err := copyFile(cookieDB, tmpDB); err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`SELECT host_key, name, value, hex(encrypted_value), path FROM cookies WHERE host_key = '%[1]s' OR host_key = '.%[1]s' OR host_key LIKE '%%.%[1]s';`, strings.ReplaceAll(domain, "'", "''"))
	cmd := exec.Command("/usr/bin/sqlite3", "-readonly", "-separator", "\t", tmpDB, query)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sqlite3 query failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	cookies := make([]browserCookie, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		for len(parts) < 5 {
			parts = append(parts, "")
		}
		cookies = append(cookies, browserCookie{
			HostKey:           parts[0],
			Name:              parts[1],
			Value:             parts[2],
			EncryptedValueHex: parts[3],
			Path:              parts[4],
		})
	}
	return cookies, nil
}

func decryptBrowserCookieValue(profile *activeBrowserProfile, encryptedHex string) (string, error) {
	encryptedHex = strings.TrimSpace(encryptedHex)
	if encryptedHex == "" {
		return "", nil
	}
	encrypted, err := decodeHex(encryptedHex)
	if err != nil {
		return "", err
	}
	if len(encrypted) == 0 {
		return "", nil
	}
	if bytes.HasPrefix(encrypted, []byte("v10")) || bytes.HasPrefix(encrypted, []byte("v11")) {
		passphrase, err := browserSafeStoragePassword(profile)
		if err != nil {
			return "", err
		}
		return decryptBrowserCookieValueV10(encrypted[3:], passphrase)
	}
	return string(encrypted), nil
}

func browserSafeStoragePassword(profile *activeBrowserProfile) (string, error) {
	service := strings.TrimSpace(profile.KeychainService)
	if service == "" {
		service = strings.TrimSpace(profile.AppName) + " Safe Storage"
	}
	accountCandidates := []string{strings.TrimSpace(profile.KeychainAccount), strings.TrimSpace(profile.AppName), ""}
	for _, account := range accountCandidates {
		args := []string{"find-generic-password", "-w", "-s", service}
		if account != "" {
			args = append(args, "-a", account)
		}
		out, err := exec.Command("security", args...).Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	return "", fmt.Errorf("could not read %q from macOS Keychain", service)
}

func decryptBrowserCookieValueV10(ciphertext []byte, passphrase string) (string, error) {
	key := pbkdf2.Key([]byte(passphrase), []byte("saltysalt"), 1003, 16, sha1.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("invalid encrypted cookie length")
	}
	iv := bytes.Repeat([]byte(" "), aes.BlockSize)
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)
	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, errors.New("invalid PKCS#7 data")
	}
	pad := int(b[len(b)-1])
	if pad == 0 || pad > blockSize || pad > len(b) {
		return nil, errors.New("invalid PKCS#7 padding")
	}
	for _, v := range b[len(b)-pad:] {
		if int(v) != pad {
			return nil, errors.New("invalid PKCS#7 padding")
		}
	}
	return b[:len(b)-pad], nil
}

func decodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errors.New("invalid hex length")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var v byte
		for j := 0; j < 2; j++ {
			c := s[i+j]
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= c - '0'
			case c >= 'a' && c <= 'f':
				v |= c - 'a' + 10
			case c >= 'A' && c <= 'F':
				v |= c - 'A' + 10
			default:
				return nil, fmt.Errorf("invalid hex character %q", c)
			}
		}
		out[i/2] = v
	}
	return out, nil
}
