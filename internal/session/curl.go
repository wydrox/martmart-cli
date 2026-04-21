package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/shlex"
)

// CurlData holds the parsed components of a curl command.
type CurlData struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    *string
}

// userPathRe matches the numeric user ID segment in a Frisco API path.
var userPathRe = regexp.MustCompile(`/users/(\d+)`)

// rtokenCookieRe matches a refresh token cookie name/value pair.
var rtokenCookieRe = regexp.MustCompile(`(?i)([a-z0-9_-]*rtoken[a-z0-9_-]*)=([^;\s,]+)`)

// ParseCurl parses a curl command string into its URL, method, headers, and body.
func ParseCurl(curlCommand string) (*CurlData, error) {
	tokens, err := shlex.Split(curlCommand)
	if err != nil {
		return nil, fmt.Errorf("shlex: %w", err)
	}
	if len(tokens) == 0 {
		return nil, errors.New("empty curl command")
	}
	if tokens[0] != "curl" {
		return nil, errors.New("command must start with 'curl'")
	}

	method := "GET"
	rawURL := ""
	headers := map[string]string{}
	var body *string

	i := 1
	for i < len(tokens) {
		token := tokens[i]
		var nxt string
		if i+1 < len(tokens) {
			nxt = tokens[i+1]
		}

		switch {
		case (token == "-X" || token == "--request") && nxt != "":
			method = strings.ToUpper(nxt)
			i += 2
			continue
		case (token == "-H" || token == "--header") && nxt != "":
			if idx := strings.IndexByte(nxt, ':'); idx >= 0 {
				k := strings.TrimSpace(nxt[:idx])
				v := strings.TrimSpace(nxt[idx+1:])
				headers[k] = v
			}
			i += 2
			continue
		case (token == "--data" || token == "--data-raw" || token == "--data-binary" || token == "-d") && nxt != "":
			body = &nxt
			if method == "GET" {
				method = "POST"
			}
			i += 2
			continue
		case token == "--url" && nxt != "":
			rawURL = nxt
			i += 2
			continue
		case strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://"):
			rawURL = token
			i++
			continue
		}
		i++
	}

	if rawURL == "" {
		return nil, errors.New("could not find URL in curl command")
	}

	return &CurlData{Method: method, URL: rawURL, Headers: headers, Body: body}, nil
}

// ExtractToken from Authorization: Bearer ...
func ExtractToken(headers map[string]string) string {
	for k, v := range headers {
		if !strings.EqualFold(k, "authorization") {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(v), " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// ExtractUserID from URL path /users/{id}/...
func ExtractUserID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	m := userPathRe.FindStringSubmatch(u.Path)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// fromCurlHeaderAllow is the set of header names (lowercase) that are imported
// into the session when parsing a curl command.
var fromCurlHeaderAllow = map[string]struct{}{
	"authorization":      {},
	"content-type":       {},
	"cookie":             {},
	"x-api-version":      {},
	"x-requested-with":   {},
	"x-platform":         {},
	"x-app-version":      {},
	"x-csrf-protected":   {},
	"accept":             {},
	"accept-language":    {},
	"origin":             {},
	"referer":            {},
	"user-agent":         {},
	"sec-ch-ua":          {},
	"sec-ch-ua-mobile":   {},
	"sec-ch-ua-platform": {},
	"sec-fetch-dest":     {},
	"sec-fetch-mode":     {},
	"sec-fetch-site":     {},
	"baggage":            {},
	"sentry-trace":       {},
	"priority":           {},
}

// ApplyFromCurl updates a session with data extracted from a parsed curl command.
// It infers the provider from the curl URL when possible and otherwise falls
// back to the existing session contents.
func ApplyFromCurl(s *Session, c *CurlData) {
	provider := ProviderForSession(s, "")
	if c != nil && strings.TrimSpace(c.URL) != "" {
		provider = ProviderForURL(c.URL)
	}
	ApplyFromCurlForProvider(s, c, provider)
}

// ApplyFromCurlForProvider updates a session with data extracted from a parsed
// curl command, using the provided backend provider to decide how to trust the
// target host and how to preserve provider-specific fields.
func ApplyFromCurlForProvider(s *Session, c *CurlData, provider string) {
	if s == nil || c == nil {
		return
	}
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	for k, v := range c.Headers {
		if _, ok := fromCurlHeaderAllow[strings.ToLower(k)]; ok {
			s.Headers[k] = v
		}
	}
	s.Headers = NormalizeHeaders(s.Headers)
	if t := ExtractToken(c.Headers); t != "" {
		s.Token = t
		s.Headers["Authorization"] = "Bearer " + t
	}
	if rt := ExtractRefreshTokenFromCurlBody(c.Body); rt != "" {
		s.RefreshToken = rt
	}
	cookie := c.Headers["cookie"]
	if cookie == "" {
		cookie = c.Headers["Cookie"]
	}
	if rt := ExtractRefreshTokenFromCookie(cookie); rt != "" {
		s.RefreshToken = rt
	}
	if uid := ExtractUserID(c.URL); uid != "" {
		s.UserID = uid
	}
	if u, err := url.Parse(c.URL); err == nil && u.Scheme != "" && u.Host != "" {
		if isTrustedHostForProvider(provider, u.Hostname()) {
			s.BaseURL = u.Scheme + "://" + u.Host
		}
	}
}

func isTrustedHostForProvider(provider, host string) bool {
	if host == "" {
		return false
	}
	h := strings.ToLower(host)
	switch NormalizeProvider(provider) {
	case ProviderDelio:
		if h == "delio.com.pl" {
			return true
		}
		return strings.HasSuffix(h, ".delio.com.pl")
	default:
		if h == "frisco.pl" {
			return true
		}
		return strings.HasSuffix(h, ".frisco.pl")
	}
}

func isTrustedFriscoHost(host string) bool {
	return isTrustedHostForProvider(ProviderFrisco, host)
}

// ExtractRefreshTokenFromCurlBody parses application/x-www-form-urlencoded body.
func ExtractRefreshTokenFromCurlBody(body *string) string {
	if body == nil {
		return ""
	}
	if vals, err := url.ParseQuery(*body); err == nil {
		if v := vals.Get("refresh_token"); v != "" {
			return v
		}
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(*body), &obj); err == nil {
		if v, ok := obj["refresh_token"].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// extractRefreshTokenValue URL-decodes raw and strips a leading pipe-delimited
// prefix (Frisco encodes the token as "userId|token").
func extractRefreshTokenValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if dec, err := url.QueryUnescape(raw); err == nil && dec != "" {
		raw = dec
	}
	if i := strings.IndexByte(raw, '|'); i >= 0 {
		return strings.TrimSpace(raw[i+1:])
	}
	return raw
}

// ExtractRefreshTokenFromCookie reads token from cookie-like header values.
func ExtractRefreshTokenFromCookie(cookieHeader string) string {
	if cookieHeader == "" {
		return ""
	}
	// Handle both "Cookie: a=b; rtokenN=..." and Set-Cookie-like chunks.
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(part[:idx])
		v := strings.TrimSpace(part[idx+1:])
		lk := strings.ToLower(k)
		if strings.Contains(lk, "rtoken") {
			if token := extractRefreshTokenValue(v); token != "" {
				return token
			}
		}
	}

	// Fallback: Set-Cookie can appear as multiline/combined string.
	if m := rtokenCookieRe.FindStringSubmatch(cookieHeader); len(m) > 2 {
		return extractRefreshTokenValue(m[2])
	}
	return ""
}

// ExtractRefreshTokenFromHeaderValue extracts a refresh token from a raw
// Cookie or Set-Cookie header value string.
func ExtractRefreshTokenFromHeaderValue(value string) string {
	if value == "" {
		return ""
	}
	if t := ExtractRefreshTokenFromCookie(value); t != "" {
		return t
	}
	return ""
}
