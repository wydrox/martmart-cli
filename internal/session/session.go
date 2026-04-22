// Package session manages provider-specific persistent CLI sessions stored under
// ~/.martmart-cli/ with legacy read fallback from ~/.frisco-cli/.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	// ProviderFrisco identifies the Frisco backend.
	ProviderFrisco = "frisco"
	// ProviderDelio identifies the Delio backend.
	ProviderDelio = "delio"
	// ProviderUpMenu identifies the UpMenu backend.
	ProviderUpMenu = "upmenu"

	// DefaultBaseURL is the base URL for the Frisco API.
	DefaultBaseURL = "https://www.frisco.pl"
	// DefaultDelioBaseURL is the base URL for the Delio API.
	DefaultDelioBaseURL = "https://delio.com.pl"
	// DefaultUpMenuBaseURL is the default base URL for the Dobra Buła UpMenu MVP.
	DefaultUpMenuBaseURL = "https://dobrabula.orderwebsite.com"
)

var (
	sessionDir       string
	legacySessionDir string
	sessionFile      string
)

func init() {
	home, _ := os.UserHomeDir()
	sessionDir = filepath.Join(home, ".martmart-cli")
	legacySessionDir = filepath.Join(home, ".frisco-cli")
	sessionFile = filepath.Join(sessionDir, "frisco-session.json")
}

// Session is persisted per provider in ~/.martmart-cli/.
type Session struct {
	BaseURL      string            `json:"base_url"`
	Headers      map[string]string `json:"headers"`
	Token        any               `json:"token"`
	UserID       any               `json:"user_id"`
	RefreshToken any               `json:"refresh_token"`
}

func defaultSession() *Session {
	return defaultSessionForProvider(ProviderFrisco)
}

func defaultSessionForProvider(provider string) *Session {
	return &Session{
		BaseURL: DefaultBaseURLForProvider(provider),
		Headers: map[string]string{},
		Token:   nil,
		UserID:  nil,
	}
}

// StorageDir returns the session/config root directory.
func StorageDir() string {
	return sessionDir
}

// SupportedProviders returns the sorted list of supported backend providers.
func SupportedProviders() []string {
	providers := []string{ProviderFrisco, ProviderDelio, ProviderUpMenu}
	sort.Strings(providers)
	return providers
}

// SupportedProvidersFlagHelp returns a flag/help-friendly provider list.
func SupportedProvidersFlagHelp() string {
	return strings.Join(SupportedProviders(), " or ")
}

// ProviderDisplayName returns a human-friendly provider name.
func ProviderDisplayName(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderDelio:
		return "Delio"
	case ProviderUpMenu:
		return "UpMenu"
	default:
		return "Frisco"
	}
}

// NormalizeProvider lowercases and trims provider names.
func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

// ValidateProvider validates a provider identifier.
func ValidateProvider(provider string) error {
	switch NormalizeProvider(provider) {
	case ProviderFrisco, ProviderDelio, ProviderUpMenu:
		return nil
	default:
		return fmt.Errorf("unsupported provider %q (use one of: %s)", provider, strings.Join(SupportedProviders(), ", "))
	}
}

// DefaultBaseURLForProvider returns the default base URL for the given provider.
func DefaultBaseURLForProvider(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderDelio:
		return DefaultDelioBaseURL
	case ProviderUpMenu:
		return DefaultUpMenuBaseURL
	default:
		return DefaultBaseURL
	}
}

// ProviderForURL guesses the provider from an absolute URL.
func ProviderForURL(rawURL string) string {
	if u, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
		host := strings.ToLower(strings.TrimSpace(u.Hostname()))
		switch {
		case host == "delio.com.pl" || strings.HasSuffix(host, ".delio.com.pl"):
			return ProviderDelio
		case host == "upmenu.com" || strings.HasSuffix(host, ".upmenu.com"):
			return ProviderUpMenu
		case host == "orderwebsite.com" || strings.HasSuffix(host, ".orderwebsite.com"):
			return ProviderUpMenu
		case host == "frisco.pl" || strings.HasSuffix(host, ".frisco.pl"):
			return ProviderFrisco
		}
	}
	return ProviderFrisco
}

// ProviderForBaseURL guesses the provider from a session base URL.
func ProviderForBaseURL(baseURL string) string {
	return ProviderForURL(baseURL)
}

// ProviderForSession infers the provider from persisted session fields and
// falls back to fallbackProvider (or Frisco when empty/unknown).
func ProviderForSession(s *Session, fallbackProvider string) string {
	fallbackProvider = NormalizeProvider(fallbackProvider)
	if s != nil {
		if strings.TrimSpace(s.BaseURL) != "" {
			return ProviderForBaseURL(s.BaseURL)
		}
		if provider := providerFromHeaders(s.Headers); provider != "" {
			return provider
		}
		if TokenString(s) != "" || RefreshTokenString(s) != "" {
			return ProviderFrisco
		}
	}
	switch fallbackProvider {
	case ProviderDelio:
		return ProviderDelio
	case ProviderUpMenu:
		return ProviderUpMenu
	default:
		return ProviderFrisco
	}
}

// SessionFilePath returns the provider-specific session file path.
func SessionFilePath(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderDelio:
		return filepath.Join(sessionDir, "delio-session.json")
	case ProviderUpMenu:
		return filepath.Join(sessionDir, "upmenu-session.json")
	default:
		if strings.TrimSpace(sessionFile) != "" {
			return sessionFile
		}
		return filepath.Join(sessionDir, "frisco-session.json")
	}
}

func appendUniquePath(paths []string, path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return paths
	}
	for _, existing := range paths {
		if existing == path {
			return paths
		}
	}
	return append(paths, path)
}

func sessionLoadPaths(provider string) []string {
	provider = NormalizeProvider(provider)
	paths := []string{}
	paths = appendUniquePath(paths, SessionFilePath(provider))

	switch provider {
	case ProviderDelio:
		paths = appendUniquePath(paths, filepath.Join(legacySessionDir, "delio-session.json"))
	case ProviderUpMenu:
		// UpMenu support is new; there is no legacy session filename.
	default:
		paths = appendUniquePath(paths, filepath.Join(sessionDir, "session.json"))
		paths = appendUniquePath(paths, filepath.Join(legacySessionDir, "frisco-session.json"))
		paths = appendUniquePath(paths, filepath.Join(legacySessionDir, "session.json"))
	}
	return paths
}

// EnsureDir creates ~/.martmart-cli with 0700 permissions if it does not exist.
func EnsureDir() error {
	return os.MkdirAll(sessionDir, 0o700)
}

// LoadProviderWithPath reads the provider session file and returns a Session plus
// the path it was loaded from. The returned path is empty when no persisted
// session exists and the default session is returned instead.
func LoadProviderWithPath(provider string) (*Session, string, error) {
	provider = NormalizeProvider(provider)
	if provider == "" {
		provider = ProviderFrisco
	}
	if err := ValidateProvider(provider); err != nil {
		return nil, "", err
	}

	for _, path := range sessionLoadPaths(provider) {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", err
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, path, err
		}
		if s.BaseURL == "" {
			s.BaseURL = DefaultBaseURLForProvider(provider)
		}
		if s.Headers == nil {
			s.Headers = map[string]string{}
		}
		s.Headers = NormalizeHeaders(s.Headers)
		return &s, path, nil
	}

	return defaultSessionForProvider(provider), "", nil
}

// LoadProvider reads the provider session file and returns a Session.
func LoadProvider(provider string) (*Session, error) {
	s, _, err := LoadProviderWithPath(provider)
	return s, err
}

// Load reads the Frisco session file.
func Load() (*Session, error) {
	return LoadProvider(ProviderFrisco)
}

// SaveProvider persists s to the provider session file with 0600 permissions.
func SaveProvider(provider string, s *Session) error {
	provider = NormalizeProvider(provider)
	if provider == "" {
		provider = ProviderFrisco
	}
	if err := ValidateProvider(provider); err != nil {
		return err
	}
	if err := EnsureDir(); err != nil {
		return err
	}
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	s.Headers = NormalizeHeaders(s.Headers)
	if s.BaseURL == "" {
		s.BaseURL = DefaultBaseURLForProvider(provider)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SessionFilePath(provider), data, 0o600)
}

// Save persists s to a provider-specific session file with 0600 permissions.
// It infers the provider from the session base URL when possible.
func Save(s *Session) error {
	return SaveProvider(ProviderForSession(s, ""), s)
}

// IsAuthenticated reports whether the session has a token, Authorization header,
// or Cookie header (used by Delio).
func IsAuthenticated(s *Session) bool {
	if s == nil {
		return false
	}
	if TokenString(s) != "" {
		return true
	}
	if HeaderValue(s, "Authorization") != "" {
		return true
	}
	if HeaderValue(s, "Cookie") != "" {
		return true
	}
	return false
}

// HeaderValue returns the first header value matching key case-insensitively.
func HeaderValue(s *Session, key string) string {
	if s == nil || len(s.Headers) == 0 {
		return ""
	}
	for k, v := range s.Headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// UserIDString returns session user_id as string or empty.
func UserIDString(s *Session) string {
	if s == nil || s.UserID == nil {
		return ""
	}
	switch v := s.UserID.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

// RequireUserID returns the explicit user ID if given, otherwise falls back to the session user ID.
func RequireUserID(s *Session, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	uid := UserIDString(s)
	if uid == "" {
		return "", errors.New("missing user_id: import session with 'session from-curl' using /users/{id}/... endpoint or pass --user-id")
	}
	return uid, nil
}

// TokenString returns bearer token as string (JSON may unmarshal as string).
func TokenString(s *Session) string {
	if s == nil || s.Token == nil {
		return ""
	}
	switch v := s.Token.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// RefreshTokenString returns refresh token from session.
func RefreshTokenString(s *Session) string {
	if s == nil || s.RefreshToken == nil {
		return ""
	}
	switch v := s.RefreshToken.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// NormalizeHeaders deduplicates headers case-insensitively and uses canonical keys.
func NormalizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	type candidate struct {
		value string
		rank  int
	}
	best := map[string]candidate{}
	orig := map[string]string{}

	for k, v := range headers {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "" {
			continue
		}
		canon := canonicalHeaderKey(lk, k)
		rank := 0
		if k == canon {
			rank = 2
		} else if strings.EqualFold(k, canon) {
			rank = 1
		}
		if prev, ok := best[lk]; !ok || rank > prev.rank || (rank == prev.rank && len(v) > len(prev.value)) {
			best[lk] = candidate{value: v, rank: rank}
			orig[lk] = canon
		}
	}

	out := make(map[string]string, len(best))
	for lk, cand := range best {
		out[orig[lk]] = cand.value
	}
	return out
}

// canonicalHeaderKey returns the canonical form of a known header name, or
// original when the key is not in the recognised set.
func providerFromHeaders(headers map[string]string) string {
	cookie := ""
	for k, v := range headers {
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "cookie":
			cookie = v
		case "authorization":
			if strings.TrimSpace(v) != "" {
				return ProviderFrisco
			}
		case "origin", "referer":
			if provider := ProviderForURL(v); provider == ProviderDelio || provider == ProviderUpMenu {
				return provider
			}
		}
	}
	seenUpMenuCookie := false
	for _, part := range strings.Split(cookie, ";") {
		trimmed := strings.TrimSpace(part)
		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
		switch name {
		case "authtoken", "idtoken", "refreshtoken":
			return ProviderDelio
		case "xsrf-token", "laravel_session":
			seenUpMenuCookie = true
		}
		if strings.Contains(name, "rtoken") || strings.Contains(name, "sessionid") || strings.Contains(name, "userid") {
			return ProviderFrisco
		}
		if strings.Contains(name, "upmenu") {
			seenUpMenuCookie = true
		}
	}
	if seenUpMenuCookie {
		return ProviderUpMenu
	}
	return ""
}

func canonicalHeaderKey(lowerKey, original string) string {
	switch lowerKey {
	case "authorization":
		return "Authorization"
	case "cookie":
		return "Cookie"
	case "content-type":
		return "Content-Type"
	case "accept":
		return "Accept"
	case "accept-language":
		return "Accept-Language"
	case "origin":
		return "Origin"
	case "referer":
		return "Referer"
	case "user-agent":
		return "User-Agent"
	case "sec-ch-ua":
		return "Sec-CH-UA"
	case "sec-ch-ua-mobile":
		return "Sec-CH-UA-Mobile"
	case "sec-ch-ua-platform":
		return "Sec-CH-UA-Platform"
	case "sec-fetch-dest":
		return "Sec-Fetch-Dest"
	case "sec-fetch-mode":
		return "Sec-Fetch-Mode"
	case "sec-fetch-site":
		return "Sec-Fetch-Site"
	case "baggage":
		return "Baggage"
	case "sentry-trace":
		return "Sentry-Trace"
	case "priority":
		return "Priority"
	case "x-api-version":
		return "X-Api-Version"
	case "x-requested-with":
		return "X-Requested-With"
	case "x-frisco-visitorid":
		return "X-Frisco-VisitorId"
	case "x-platform":
		return "X-Platform"
	case "x-app-version":
		return "X-App-Version"
	case "x-csrf-protected":
		return "X-Csrf-Protected"
	default:
		return original
	}
}
