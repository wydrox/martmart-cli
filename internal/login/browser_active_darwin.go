//go:build darwin

package login

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"

	"github.com/wydrox/martmart-cli/internal/session"
)

type activeBrowserProfile struct {
	AppName          string
	UserDataDir      string
	ProfileDirectory string
	KeychainService  string
	KeychainAccount  string
}

type chromiumCookie struct {
	HostKey           string
	Name              string
	Value             string
	EncryptedValueHex string
	Path              string
}

func runWithExistingBrowser(ctx context.Context, opts Options) (*Result, error) {
	provider := session.NormalizeProvider(opts.Provider)
	if provider != session.ProviderDelio {
		return nil, fmt.Errorf("existing-browser login is only implemented for Delio")
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

	profile, err := detectActiveBrowserProfile(opts.UserDataDir, opts.ProfileDirectory)
	if err != nil {
		return nil, err
	}
	if err := openURLInBrowser(profile.AppName, loginURL); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	var cookieHeader string
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
		cookieHeader, err = cookieHeaderFromChromiumProfile(profile, "delio.com.pl")
		if err == nil && hasDelioAuthCookie(cookieHeader) {
			s.BaseURL = baseURL
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			s.Headers["Cookie"] = cookieHeader
			s.Token = nil
			delete(s.Headers, "Authorization")
			s.RefreshToken = nil
			if err := session.SaveProvider(provider, s); err != nil {
				return nil, err
			}
			return &Result{
				Saved:            true,
				BaseURL:          s.BaseURL,
				UserID:           s.UserID,
				TokenSaved:       false,
				CookieSaved:      true,
				Provider:         provider,
				ProfileDirectory: profile.ProfileDirectory,
			}, nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("could not read cookies from %s profile %q: %w", profile.AppName, profile.ProfileDirectory, err)
	}
	return nil, fmt.Errorf("Delio auth cookies not detected in %s profile %q — open a logged-in Delio page in that browser and wait for the session to load", profile.AppName, profile.ProfileDirectory)
}

func detectActiveBrowserProfile(userDataDir, profileDir string) (*activeBrowserProfile, error) {
	if strings.TrimSpace(userDataDir) != "" {
		resolved, err := resolveUserDataDir(userDataDir)
		if err != nil {
			return nil, err
		}
		prof := strings.TrimSpace(profileDir)
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

	appName, err := frontmostMacBrowserName()
	if err != nil {
		return nil, err
	}
	profile, ok := knownDarwinBrowsers[strings.TrimSpace(appName)]
	if !ok {
		appName, err = defaultMacBrowserName()
		if err != nil {
			return nil, fmt.Errorf("frontmost app %q is not a supported browser and default browser lookup failed: %w", appName, err)
		}
		profile, ok = knownDarwinBrowsers[strings.TrimSpace(appName)]
		if !ok {
			return nil, fmt.Errorf("browser %q is not supported yet; use --user-data-dir/--profile-directory for a Chromium browser profile", appName)
		}
	}
	profile.ProfileDirectory = strings.TrimSpace(profileDir)
	if profile.ProfileDirectory == "" {
		profile.ProfileDirectory = detectLastUsedProfile(profile.UserDataDir)
	}
	if profile.ProfileDirectory == "" {
		profile.ProfileDirectory = "Default"
	}
	if err := ensureProfileExists(profile.UserDataDir, profile.ProfileDirectory); err != nil {
		return nil, err
	}
	copy := profile
	return &copy, nil
}

var knownDarwinBrowsers = map[string]activeBrowserProfile{
	"Google Chrome": {
		AppName:         "Google Chrome",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Google", "Chrome"),
		KeychainService: "Chrome Safe Storage",
		KeychainAccount: "Chrome",
	},
	"Chromium": {
		AppName:         "Chromium",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Chromium"),
		KeychainService: "Chromium Safe Storage",
		KeychainAccount: "Chromium",
	},
	"Brave Browser": {
		AppName:         "Brave Browser",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "BraveSoftware", "Brave-Browser"),
		KeychainService: "Brave Safe Storage",
		KeychainAccount: "Brave",
	},
	"Arc": {
		AppName:         "Arc",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Arc"),
		KeychainService: "Arc Safe Storage",
		KeychainAccount: "Arc",
	},
	"Microsoft Edge": {
		AppName:         "Microsoft Edge",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Microsoft Edge"),
		KeychainService: "Microsoft Edge Safe Storage",
		KeychainAccount: "Microsoft Edge",
	},
	"Helium": {
		AppName:         "Helium",
		UserDataDir:     filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "net.imput.helium"),
		KeychainService: "Chromium Safe Storage",
		KeychainAccount: "Chromium",
	},
}

func inferProfileMetadataFromUserDataDir(userDataDir string) activeBrowserProfile {
	resolved := strings.TrimSpace(userDataDir)
	for _, profile := range knownDarwinBrowsers {
		if resolved == profile.UserDataDir {
			return profile
		}
	}
	base := filepath.Base(resolved)
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

func defaultMacBrowserName() (string, error) {
	cmd := exec.Command("osascript", "-e", `POSIX path of (path to default application for URL "https://delio.com.pl")`)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not detect default browser: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errors.New("empty default browser path")
	}
	base := strings.TrimSuffix(filepath.Base(strings.TrimSuffix(path, "/")), ".app")
	if base == "" {
		return "", errors.New("could not parse default browser name")
	}
	return base, nil
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

func cookieHeaderFromChromiumProfile(profile *activeBrowserProfile, domain string) (string, error) {
	if profile == nil {
		return "", errors.New("missing browser profile")
	}
	cookies, err := readChromiumCookies(profile, domain)
	if err != nil {
		return "", err
	}
	if len(cookies) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(cookies))
	for _, ck := range cookies {
		value := ck.Value
		if value == "" && ck.EncryptedValueHex != "" {
			value, err = decryptChromiumCookieValue(profile, ck.EncryptedValueHex)
			if err != nil || value == "" {
				continue
			}
		}
		parts = append(parts, ck.Name+"="+value)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; "), nil
}

func readChromiumCookies(profile *activeBrowserProfile, domain string) ([]chromiumCookie, error) {
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
	cookies := make([]chromiumCookie, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		for len(parts) < 5 {
			parts = append(parts, "")
		}
		cookies = append(cookies, chromiumCookie{
			HostKey:           parts[0],
			Name:              parts[1],
			Value:             parts[2],
			EncryptedValueHex: parts[3],
			Path:              parts[4],
		})
	}
	return cookies, nil
}

func decryptChromiumCookieValue(profile *activeBrowserProfile, encryptedHex string) (string, error) {
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
		passphrase, err := chromiumSafeStoragePassword(profile)
		if err != nil {
			return "", err
		}
		return decryptChromiumV10(encrypted[3:], passphrase)
	}
	return string(encrypted), nil
}

func chromiumSafeStoragePassword(profile *activeBrowserProfile) (string, error) {
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

func decryptChromiumV10(ciphertext []byte, passphrase string) (string, error) {
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
