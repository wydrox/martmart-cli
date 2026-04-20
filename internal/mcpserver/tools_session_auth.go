package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/login"
	"github.com/wydrox/martmart-cli/internal/session"
)

// registerSessionAuthTools registers all session and auth MCP tools.
func registerSessionAuthTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_login",
		Description: "Opens the provider page in the user's default Chromium-based browser app with temporary remote debugging, captures auth session data automatically, and saves the session.",
	}, toolSessionLogin)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_show",
		Description: "Current session with secrets redacted (same as CLI session show).",
	}, toolSessionShow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_from_curl",
		Description: "Parse curl, ApplyFromCurl, Save (mirrors CLI session from-curl).",
	}, toolSessionFromCurl)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_refresh_token",
		Description: "POST /app/commerce/connect/token with refresh_token grant.",
	}, toolAuthRefreshToken)
}

// sessionShowIn is the (empty) input type for the session_show tool.
type sessionShowIn struct{}

func toolSessionShow(_ context.Context, _ *mcp.CallToolRequest, _ sessionShowIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(session.RedactedCopy(s))
}

// sessionFromCurlIn is the input type for the session_from_curl tool.
type sessionFromCurlIn struct {
	Curl string `json:"curl"`
}

func toolSessionFromCurl(_ context.Context, _ *mcp.CallToolRequest, in sessionFromCurlIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.Curl) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("curl is required")
	}
	cd, err := session.ParseCurl(in.Curl)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	session.ApplyFromCurl(s, cd)
	if err := session.Save(s); err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":         true,
		"base_url":      s.BaseURL,
		"user_id":       s.UserID,
		"token_saved":   mcpASATokenSaved(s),
		"headers_saved": mcpASAHeaderKeysSorted(s.Headers),
	})
}

// authRefreshTokenIn is the input type for the session_refresh_token tool.
type authRefreshTokenIn struct {
	RefreshToken string `json:"refresh_token,omitempty" jsonschema:"optional; else session refresh_token"`
}

func toolAuthRefreshToken(_ context.Context, _ *mcp.CallToolRequest, in authRefreshTokenIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	rt := strings.TrimSpace(in.RefreshToken)
	if rt == "" {
		rt = session.RefreshTokenString(s)
	}
	if rt == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("missing refresh_token (argument or session)")
	}
	payload := map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": rt,
	}
	result, err := httpclient.RequestJSON(s, "POST", "/app/commerce/connect/token", httpclient.RequestOpts{
		Data:       payload,
		DataFormat: httpclient.FormatForm,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if m, ok := result.(map[string]any); ok {
		expiresIn := m["expires_in"]
		if at, ok := mcpASAStringField(m["access_token"]); ok && at != "" {
			s.Token = at
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			s.Headers["Authorization"] = "Bearer " + at
		}
		if nr, ok := mcpASAStringField(m["refresh_token"]); ok && nr != "" {
			s.RefreshToken = nr
		}
		if err := session.Save(s); err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(map[string]any{
			"saved":               true,
			"token_saved":         mcpASATokenSaved(s),
			"refresh_token_saved": session.RefreshTokenString(s) != "",
			"expires_in":          expiresIn,
		})
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":               false,
		"token_saved":         mcpASATokenSaved(s),
		"refresh_token_saved": session.RefreshTokenString(s) != "",
		"message":             "Unexpected token endpoint payload shape; session not updated.",
	})
}

// sessionLoginIn is the input type for the session_login tool.
type sessionLoginIn struct {
	TimeoutSec *int `json:"timeout_sec,omitempty" jsonschema:"Login timeout in seconds; default 180"`
}

func toolSessionLogin(ctx context.Context, _ *mcp.CallToolRequest, in sessionLoginIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	timeout := 10
	if in.TimeoutSec != nil && *in.TimeoutSec > 0 {
		timeout = *in.TimeoutSec
	}
	result, err := login.Run(ctx, login.Options{Provider: session.CurrentProvider(), TimeoutSec: timeout})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":               result.Saved,
		"base_url":            result.BaseURL,
		"user_id":             result.UserID,
		"token_saved":         result.TokenSaved,
		"refresh_token_saved": result.RefreshTokenSaved,
		"cookie_saved":        result.CookieSaved,
	})
}

// mcpASAStringField converts v to a trimmed string and reports whether it is non-empty.
func mcpASAStringField(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t), strings.TrimSpace(t) != ""
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		return s, s != ""
	}
}

// mcpASATokenSaved reports whether the session contains a non-empty access token.
func mcpASATokenSaved(s *session.Session) bool {
	if s == nil || s.Token == nil {
		return false
	}
	if str, ok := s.Token.(string); ok {
		return str != ""
	}
	return true
}

// mcpASAHeaderKeysSorted returns the header map keys in sorted order.
func mcpASAHeaderKeysSorted(h map[string]string) []string {
	if len(h) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mcpASAGetStringAny returns the first non-empty string field by keys.
func mcpASAGetStringAny(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := mcpASAStringField(v); ok {
				return s
			}
		}
	}
	return ""
}

// mcpASAGetBoolAny returns first bool-like field by keys.
func mcpASAGetBoolAny(m map[string]any, keys ...string) (bool, bool) {
	if m == nil {
		return false, false
	}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t, true
		case string:
			switch strings.ToLower(strings.TrimSpace(t)) {
			case "true", "1", "yes":
				return true, true
			case "false", "0", "no":
				return false, true
			}
		}
	}
	return false, false
}
