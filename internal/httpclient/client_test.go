package httpclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wydrox/martmart-cli/internal/session"
)

func TestMakeURL(t *testing.T) {
	cases := []struct {
		base, path, want string
	}{
		{"https://api.frisco.pl", "/v1/cart", "https://api.frisco.pl/v1/cart"},
		{"https://api.frisco.pl", "https://other.com/foo", "https://other.com/foo"},
		{"https://api.frisco.pl/base/", "rel/path", "https://api.frisco.pl/base/rel/path"},
	}
	for _, tc := range cases {
		got, err := makeURL(tc.base, tc.path)
		if err != nil {
			t.Fatalf("makeURL(%q, %q): %v", tc.base, tc.path, err)
		}
		if got != tc.want {
			t.Errorf("makeURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}

func TestHeaderKeyPresent(t *testing.T) {
	h := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer xxx",
	}
	if !headerKeyPresent(h, "content-type") {
		t.Error("expected content-type to match")
	}
	if !headerKeyPresent(h, "AUTHORIZATION") {
		t.Error("expected AUTHORIZATION to match")
	}
	if headerKeyPresent(h, "Accept") {
		t.Error("Accept should not match")
	}
	if headerKeyPresent(nil, "anything") {
		t.Error("nil map should not match")
	}
}

func TestSanitizeErrorURL(t *testing.T) {
	got := sanitizeErrorURL("https://api.frisco.pl/path?token=secret&foo=bar#frag")
	if strings.Contains(got, "secret") || strings.Contains(got, "frag") {
		t.Errorf("expected sanitized URL, got %q", got)
	}
	if got != "https://api.frisco.pl/path" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestSanitizeErrorBody(t *testing.T) {
	body := `{"access_token": "eyJhbGciOi...", "refresh_token": "dGVzdA=="}`
	got := sanitizeErrorBody(body)
	if strings.Contains(got, "eyJhbGciOi") {
		t.Errorf("access_token not redacted: %s", got)
	}
	if strings.Contains(got, "dGVzdA") {
		t.Errorf("refresh_token not redacted: %s", got)
	}
}

func TestSanitizeErrorBody_Empty(t *testing.T) {
	if got := sanitizeErrorBody(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := sanitizeErrorBody("   "); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestSanitizeErrorBody_Truncation(t *testing.T) {
	long := strings.Repeat("x", 2000)
	got := sanitizeErrorBody(long)
	if len(got) > maxErrorBodyLen+20 {
		t.Errorf("expected truncation, got len=%d", len(got))
	}
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Error("expected truncation suffix")
	}
}

func TestIsTokenEndpoint(t *testing.T) {
	if !isTokenEndpoint("https://api.frisco.pl/app/commerce/connect/token") {
		t.Error("expected true for token endpoint")
	}
	if isTokenEndpoint("https://api.frisco.pl/app/commerce/api/v1/cart") {
		t.Error("expected false for cart")
	}
}

func TestRequestJSON_GET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("foo") != "bar" {
			t.Errorf("expected query foo=bar, got %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	result, err := RequestJSON(s, "GET", "/test", RequestOpts{
		Query:  []string{"foo=bar"},
		Client: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["status"] != "ok" {
		t.Errorf("unexpected: %v", m)
	}
}

func TestRequestJSON_POST_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected json content-type, got %s", ct)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["key"] != "val" {
			t.Errorf("unexpected body: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	result, err := RequestJSON(s, "POST", "/submit", RequestOpts{
		Data:       map[string]any{"key": "val"},
		DataFormat: FormatJSON,
		Client:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["received"] != true {
		t.Errorf("unexpected: %v", m)
	}
}

func TestRequestJSON_POST_Form(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/x-www-form-urlencoded") {
			t.Errorf("expected form content-type, got %s", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	_, err := RequestJSON(s, "POST", "/form", RequestOpts{
		Data:       map[string]any{"grant_type": "refresh_token"},
		DataFormat: FormatForm,
		Client:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestJSON_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	_, err := RequestJSON(s, "GET", "/missing", RequestOpts{Client: srv.Client()})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("error should mention 404: %v", err)
	}
	details, ok := ParseError(err)
	if !ok {
		t.Fatalf("ParseError should succeed: %v", err)
	}
	if details.Status != http.StatusNotFound || details.Reason != "Not Found" {
		t.Fatalf("unexpected details: %+v", details)
	}
}

func TestRequestJSON_NonJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(`hello world`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	result, err := RequestJSON(s, "GET", "/text", RequestOpts{Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["body"] != "hello world" {
		t.Errorf("unexpected body: %v", m)
	}
}

func TestRequestJSON_BadQueryParam(t *testing.T) {
	s := &session.Session{BaseURL: "https://example.com"}
	_, err := RequestJSON(s, "GET", "/test", RequestOpts{
		Query: []string{"no-equals-sign"},
	})
	if err == nil {
		t.Fatal("expected error for bad query param")
	}
}

func TestRequestJSON_AuthorizationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL, Token: "test-token"}
	_, err := RequestJSON(s, "GET", "/auth", RequestOpts{Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestJSON_FormData_String(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-www-form-urlencoded" {
			t.Errorf("expected form content-type, got %q", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	result, err := RequestJSON(s, "POST", "/form", RequestOpts{
		Data:       "grant_type=client_credentials",
		DataFormat: FormatForm,
		Client:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok || m["ok"] != true {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestRequestJSON_RawFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	result, err := RequestJSON(s, "POST", "/raw", RequestOpts{
		Data:       "raw body content",
		DataFormat: FormatRaw,
		Client:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok || m["received"] != true {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestRequestJSON_RawFormat_NotString(t *testing.T) {
	s := &session.Session{BaseURL: "https://example.com"}
	_, err := RequestJSON(s, "POST", "/raw", RequestOpts{
		Data:       map[string]any{"key": "val"},
		DataFormat: FormatRaw,
	})
	if err == nil {
		t.Fatal("expected error when passing non-string to FormatRaw")
	}
	if !strings.Contains(err.Error(), "raw") {
		t.Errorf("error should mention raw, got: %v", err)
	}
}

func TestRequestJSON_FormData_BadType(t *testing.T) {
	s := &session.Session{BaseURL: "https://example.com"}
	_, err := RequestJSON(s, "POST", "/form", RequestOpts{
		Data:       []string{"not", "a", "map"},
		DataFormat: FormatForm,
	})
	if err == nil {
		t.Fatal("expected error when passing unsupported type to FormatForm")
	}
	if !strings.Contains(err.Error(), "form") {
		t.Errorf("error should mention form, got: %v", err)
	}
}

func TestRequestJSON_UnsupportedFormat(t *testing.T) {
	s := &session.Session{BaseURL: "https://example.com"}
	_, err := RequestJSON(s, "POST", "/test", RequestOpts{
		Data:       "some data",
		DataFormat: "bad",
	})
	if err == nil {
		t.Fatal("expected error for unsupported data_format")
	}
	if !strings.Contains(err.Error(), "unsupported data_format") {
		t.Errorf("error should mention unsupported data_format, got: %v", err)
	}
}

func TestRequestJSON_EmptyJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// write no body
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	result, err := RequestJSON(s, "GET", "/empty", RequestOpts{Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestRequestJSON_ExtraHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("expected X-Custom-Header=custom-value, got %q", r.Header.Get("X-Custom-Header"))
		}
		if r.Header.Get("X-Another") != "another-value" {
			t.Errorf("expected X-Another=another-value, got %q", r.Header.Get("X-Another"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	s := &session.Session{BaseURL: srv.URL}
	_, err := RequestJSON(s, "GET", "/headers", RequestOpts{
		ExtraHeaders: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Another":       "another-value",
		},
		Client: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestJSON_DefaultBaseURL(t *testing.T) {
	// Session with empty BaseURL should fall back to DefaultBaseURL.
	// The request will fail to connect but we can inspect the error message
	// to confirm it targeted the correct host.
	s := &session.Session{BaseURL: ""}
	_, err := RequestJSON(s, "GET", "/test", RequestOpts{})
	if err == nil {
		t.Fatal("expected connection error for default base URL (no server running)")
	}
	if !strings.Contains(err.Error(), "frisco.pl") {
		t.Errorf("expected error to mention frisco.pl (default base URL), got: %v", err)
	}
}

func TestSanitizeErrorBody_BearerToken(t *testing.T) {
	body := `Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig`
	got := sanitizeErrorBody(body)
	if strings.Contains(got, "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("Bearer token not redacted: %s", got)
	}
	if !strings.Contains(got, "Bearer ***") {
		t.Errorf("expected 'Bearer ***' placeholder, got: %s", got)
	}
}

func TestSanitizeErrorBody_CookieToken(t *testing.T) {
	body := `Cookie: rtokenN=supersecretvalue; other=stuff`
	got := sanitizeErrorBody(body)
	if strings.Contains(got, "supersecretvalue") {
		t.Errorf("cookie token not redacted: %s", got)
	}
	if !strings.Contains(got, "rtokenN=***") {
		t.Errorf("expected 'rtokenN=***' placeholder, got: %s", got)
	}
}

func TestParseError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		wantOK bool
		want   int
	}{
		{"401", &ErrorDetails{Status: 401, Reason: "Unauthorized", Body: `{"error":"auth"}`}, true, 401},
		{"404", &ErrorDetails{Status: 404, Reason: "Not Found", Body: `{"error":"missing"}`}, true, 404},
		{"429", &ErrorDetails{Status: 429, Reason: "Too Many Requests"}, true, 429},
		{"5xx", &ErrorDetails{Status: 503, Reason: "Service Unavailable"}, true, 503},
		{"malformed", json.Unmarshal([]byte(`{`), &map[string]any{}), false, 0},
		{"plain", assertPlainError("boom"), false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			details, ok := ParseError(tc.err)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if details.Status != tc.want {
				t.Fatalf("status=%d want %d", details.Status, tc.want)
			}
		})
	}
}

func assertPlainError(msg string) error { return &plainError{msg: msg} }

type plainError struct{ msg string }

func (e *plainError) Error() string { return e.msg }
