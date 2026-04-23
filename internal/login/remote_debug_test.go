package login

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wydrox/martmart-cli/internal/session"
)

func makeJWTForTest(t *testing.T, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON) + ".sig"
}

func TestRefreshFriscoAccessToken_UpdatesSessionFromTokenResponse(t *testing.T) {
	newToken := makeJWTForTest(t, map[string]any{"user_id": "u-123"})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/commerce/connect/token" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/app/commerce/connect/token")
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if got := r.Header.Get("Content-Type"); !strings.Contains(strings.ToLower(got), "application/x-www-form-urlencoded") {
			t.Fatalf("content-type = %q, want form", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Fatalf("grant_type = %q, want refresh_token", got)
		}
		if got := r.Form.Get("refresh_token"); got != "old-refresh" {
			t.Fatalf("refresh_token = %q, want %q", got, "old-refresh")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  newToken,
			"refresh_token": "new-refresh",
		})
	}))
	defer server.Close()

	s := &session.Session{
		BaseURL:      server.URL,
		Headers:      map[string]string{"Cookie": "rtokenN=1|old-refresh"},
		RefreshToken: "old-refresh",
	}

	if err := refreshFriscoAccessToken(s); err != nil {
		t.Fatalf("refreshFriscoAccessToken: %v", err)
	}
	if got := session.TokenString(s); got != newToken {
		t.Fatalf("token = %q, want %q", got, newToken)
	}
	if got := session.RefreshTokenString(s); got != "new-refresh" {
		t.Fatalf("refresh_token = %q, want %q", got, "new-refresh")
	}
	if got := session.UserIDString(s); got != "u-123" {
		t.Fatalf("user_id = %q, want %q", got, "u-123")
	}
	if got := session.HeaderValue(s, "Authorization"); got != "Bearer "+newToken {
		t.Fatalf("authorization = %q, want bearer token", got)
	}
}

func TestVerifyFriscoCapturedSession_UsesUserIDFromJWT(t *testing.T) {
	token := makeJWTForTest(t, map[string]any{"user_id": "u-456"})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/commerce/api/v1/users/u-456/cart" {
			t.Fatalf("path = %q, want user cart path", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	s := &session.Session{
		BaseURL: server.URL,
		Token:   token,
		Headers: map[string]string{},
	}

	if err := verifyFriscoCapturedSession(s); err != nil {
		t.Fatalf("verifyFriscoCapturedSession: %v", err)
	}
	if got := session.UserIDString(s); got != "u-456" {
		t.Fatalf("user_id = %q, want %q", got, "u-456")
	}
}

func TestVerifyDelioCapturedSession_RejectsGraphQLErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/proxy/delio" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/proxy/delio")
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if got := r.Header.Get("Cookie"); !strings.Contains(got, "authtoken=") {
			t.Fatalf("cookie = %q, want auth cookie", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
	}))
	defer server.Close()

	s := &session.Session{
		BaseURL: server.URL,
		Headers: map[string]string{"Cookie": "authtoken=abc; idtoken=def"},
	}

	err := verifyDelioCapturedSession(s)
	if err == nil {
		t.Fatal("verifyDelioCapturedSession returned nil, want error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "graphql errors") {
		t.Fatalf("error = %v, want graphql errors", err)
	}
}
