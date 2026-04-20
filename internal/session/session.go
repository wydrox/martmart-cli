// Package session manages provider-specific persistent CLI sessions stored under
// ~/.martmart-cli/ with legacy read fallback from ~/.frisco-cli/.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
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

	// DefaultBaseURL is the base URL for the Frisco API.
	DefaultBaseURL = "https://www.frisco.pl"
	// DefaultDelioBaseURL is the base URL for the Delio API.
	DefaultDelioBaseURL = "https://delio.com.pl"
)

var (
	sessionDir       string
	legacySessionDir string
	sessionFile      string
	currentProvider  = ProviderFrisco
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
	return defaultSessionForProvider(currentProvider)
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
	providers := []string{ProviderFrisco, ProviderDelio}
	sort.Strings(providers)
	return providers
}

// NormalizeProvider lowercases and trims provider names.
func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

// ValidateProvider validates a provider identifier.
func ValidateProvider(provider string) error {
	switch NormalizeProvider(provider) {
	case ProviderFrisco, ProviderDelio:
		return nil
	default:
		return fmt.Errorf("unsupported provider %q (use one of: %s)", provider, strings.Join(SupportedProviders(), ", "))
	}
}

// CurrentProvider returns the process-wide active provider.
func CurrentProvider() string {
	return currentProvider
}

// SetCurrentProvider switches the process-wide active provider.
func SetCurrentProvider(provider string) error {
	provider = NormalizeProvider(provider)
	if provider == "" {
		provider = ProviderFrisco
	}
	if err := ValidateProvider(provider); err != nil {
		return err
	}
	currentProvider = provider
	return nil
}

// DefaultBaseURLForProvider returns the default base URL for the given provider.
func DefaultBaseURLForProvider(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderDelio:
		return DefaultDelioBaseURL
	default:
		return DefaultBaseURL
	}
}

// SessionFilePath returns the provider-specific session file path.
func SessionFilePath(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderDelio:
		return filepath.Join(sessionDir, "delio-session.json")
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

// LoadProvider reads the provider session file and returns a Session.
func LoadProvider(provider string) (*Session, error) {
	provider = NormalizeProvider(provider)
	if provider == "" {
		provider = ProviderFrisco
	}
	if err := ValidateProvider(provider); err != nil {
		return nil, err
	}

	for _, path := range sessionLoadPaths(provider) {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		if s.BaseURL == "" {
			s.BaseURL = DefaultBaseURLForProvider(provider)
		}
		if s.Headers == nil {
			s.Headers = map[string]string{}
		}
		s.Headers = NormalizeHeaders(s.Headers)
		return &s, nil
	}

	return defaultSessionForProvider(provider), nil
}

// Load reads the active provider's session file.
func Load() (*Session, error) {
	return LoadProvider(currentProvider)
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

// Save persists s to the active provider's session file with 0600 permissions.
func Save(s *Session) error {
	return SaveProvider(currentProvider, s)
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
	case "origin":
		return "Origin"
	case "referer":
		return "Referer"
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
