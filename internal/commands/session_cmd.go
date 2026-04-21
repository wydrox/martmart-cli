package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/login"
	"github.com/wydrox/martmart-cli/internal/session"
)

// session login defaults to a provider-specific start URL when --login-url is not set.

var (
	sessionLoginRun      = login.Run
	sessionLoginTryReuse = tryReuseExistingSession
)

type reusedSessionResult struct {
	Provider          string
	SessionFile       string
	BaseURL           string
	UserID            string
	TokenSaved        bool
	RefreshTokenSaved bool
	CookieSaved       bool
}

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage provider sessions (tokens, headers, user_id).",
	}
	cmd.AddCommand(newSessionFromCurlCmd(), newSessionListCmd(), newSessionVerifyCmd(), newSessionLoginCmd(), newSessionRefreshTokenCmd())
	return cmd
}

func newSessionFromCurlCmd() *cobra.Command {
	var curlStr string
	c := &cobra.Command{
		Use:   "from-curl",
		Short: "Load session from curl command.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cd, err := session.ParseCurl(curlStr)
			if err != nil {
				return err
			}
			provider, s, err := loadSessionForRequest(cmd)
			if err != nil {
				return err
			}
			session.ApplyFromCurlForProvider(s, cd, provider)
			if err := session.SaveProvider(provider, s); err != nil {
				return err
			}
			_, _ = cmd.OutOrStdout().Write([]byte("Session saved from curl.\n"))
			return printJSON(map[string]any{
				"base_url":      s.BaseURL,
				"user_id":       s.UserID,
				"token_saved":   tokenSaved(s),
				"headers_saved": headerKeysSorted(s.Headers),
			})
		},
	}
	c.Flags().StringVar(&curlStr, "curl", "", "Full curl command in quotes.")
	_ = c.MarkFlagRequired("curl")
	return c
}

type sessionListEntry struct {
	Provider          string `json:"provider"`
	Saved             bool   `json:"saved"`
	AuthPresent       bool   `json:"auth_present"`
	BaseURL           string `json:"base_url"`
	UserID            string `json:"user_id"`
	TokenSaved        bool   `json:"token_saved"`
	RefreshTokenSaved bool   `json:"refresh_token_saved"`
	CookieSaved       bool   `json:"cookie_saved"`
	SessionFile       string `json:"session_file,omitempty"`
}

func newSessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List sessions for all providers.",
		RunE: func(_ *cobra.Command, _ []string) error {
			entries, err := collectSessionListEntries()
			if err != nil {
				return err
			}
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(entries)
			}
			return printSessionListTable(entries)
		},
	}
}

func collectSessionListEntries() ([]sessionListEntry, error) {
	providers := session.SupportedProviders()
	entries := make([]sessionListEntry, 0, len(providers))
	for _, provider := range providers {
		s, path, err := session.LoadProviderWithPath(provider)
		if err != nil {
			return nil, err
		}
		entries = append(entries, sessionListEntry{
			Provider:          provider,
			Saved:             path != "",
			AuthPresent:       session.IsAuthenticated(s),
			BaseURL:           s.BaseURL,
			UserID:            session.UserIDString(s),
			TokenSaved:        session.TokenString(s) != "",
			RefreshTokenSaved: session.RefreshTokenString(s) != "",
			CookieSaved:       session.HeaderValue(s, "Cookie") != "",
			SessionFile:       path,
		})
	}
	return entries, nil
}

func printSessionListTable(entries []sessionListEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "provider\tsaved\tauth_present\tbase_url\tuser_id\ttoken_saved\trefresh_token_saved\tcookie_saved\tsession_file")
	for _, entry := range entries {
		_, _ = fmt.Fprintf(
			w,
			"%s\t%t\t%t\t%s\t%s\t%t\t%t\t%t\t%s\n",
			entry.Provider,
			entry.Saved,
			entry.AuthPresent,
			cellValue(entry.BaseURL),
			cellValue(entry.UserID),
			entry.TokenSaved,
			entry.RefreshTokenSaved,
			entry.CookieSaved,
			cellValue(entry.SessionFile),
		)
	}
	return w.Flush()
}

func newSessionVerifyCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify the provider session for this request.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, s, err := loadSessionForRequest(cmd)
			if err != nil {
				return err
			}
			if err := verifyLoadedSession(provider, s, userID); err != nil {
				return err
			}
			switch provider {
			case session.ProviderDelio:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Session OK: Delio currentCart responded successfully.")
			case session.ProviderUpMenu:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Session OK: UpMenu session request responded successfully.")
			default:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Session OK: cart API responded successfully.")
			}
			return nil
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func verifyLoadedSession(provider string, s *session.Session, userID string) error {
	switch provider {
	case session.ProviderDelio:
		if session.HeaderValue(s, "Cookie") == "" {
			return errors.New("no Cookie header in Delio session. Use 'session from-curl' with a copied Delio API request")
		}
		payload, err := delio.CurrentCart(s)
		if err != nil {
			return err
		}
		_, err = delio.ExtractCurrentCart(payload)
		return err
	case session.ProviderUpMenu:
		if session.HeaderValue(s, "Cookie") == "" && session.HeaderValue(s, "Authorization") == "" {
			return errors.New("no auth headers in UpMenu session. Use 'session from-curl' with a copied authenticated UpMenu request")
		}
		_, err := httpclient.RequestJSON(s, "GET", "/", httpclient.RequestOpts{})
		return err
	default:
		if session.TokenString(s) == "" {
			return errors.New("no token in session. Use 'session from-curl' or 'session login'")
		}
		uid, err := session.RequireUserID(s, userID)
		if err != nil {
			return err
		}
		path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
		_, err = httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
		return err
	}
}

func tryReuseExistingSession(provider string) (*reusedSessionResult, error) {
	s, path, err := session.LoadProviderWithPath(provider)
	if err != nil {
		return nil, err
	}
	if path == "" || !session.IsAuthenticated(s) {
		return nil, nil
	}
	if err := verifyLoadedSession(provider, s, ""); err != nil {
		if shouldRetrySessionLoginInBrowser(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("existing %s session found at %s but verification failed: %w", provider, path, err)
	}
	return &reusedSessionResult{
		Provider:          provider,
		SessionFile:       path,
		BaseURL:           s.BaseURL,
		UserID:            session.UserIDString(s),
		TokenSaved:        session.TokenString(s) != "",
		RefreshTokenSaved: session.RefreshTokenString(s) != "",
		CookieSaved:       session.HeaderValue(s, "Cookie") != "",
	}, nil
}

func shouldRetrySessionLoginInBrowser(err error) bool {
	if err == nil {
		return false
	}
	if details, ok := httpclient.ParseError(err); ok {
		if details.Status == 429 || details.Status >= 500 {
			return false
		}
		if details.Status >= 400 && details.Status < 500 {
			return true
		}
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "no token in session") ||
		strings.Contains(msg, "no cookie header") ||
		strings.Contains(msg, "no auth headers in upmenu session") ||
		strings.Contains(msg, "missing user_id")
}

// tokenSaved reports whether the session contains a non-empty access token.
func tokenSaved(s *session.Session) bool {
	if s == nil || s.Token == nil {
		return false
	}
	if str, ok := s.Token.(string); ok {
		return str != ""
	}
	return true
}

// headerKeysSorted returns the header map keys in sorted order.
func headerKeysSorted(h map[string]string) []string {
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

func newSessionRefreshTokenCmd() *cobra.Command {
	var refresh string
	c := &cobra.Command{
		Use:   "refresh-token",
		Short: "Refresh access token via refresh token.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, s, err := loadSessionForRequest(cmd)
			if err != nil {
				return err
			}
			switch provider {
			case session.ProviderDelio:
				return errors.New("session refresh-token is not implemented for Delio; import fresh cookies with 'session from-curl'")
			case session.ProviderUpMenu:
				return errors.New("session refresh-token is not implemented for UpMenu; import a fresh authenticated request with 'session from-curl'")
			}
			rt := refresh
			if rt == "" {
				rt = session.RefreshTokenString(s)
			}
			if rt == "" {
				return errors.New("missing refresh token. Use --refresh-token or load session with session from-curl")
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
				return err
			}
			saved := false
			expiresIn := any(nil)
			if m, ok := result.(map[string]any); ok {
				expiresIn = m["expires_in"]
				if at, ok := stringField(m["access_token"]); ok && at != "" {
					s.Token = at
					if s.Headers == nil {
						s.Headers = map[string]string{}
					}
					s.Headers["Authorization"] = "Bearer " + at
				}
				if nr, ok := stringField(m["refresh_token"]); ok && nr != "" {
					s.RefreshToken = nr
				}
				if err := session.SaveProvider(provider, s); err != nil {
					return err
				}
				saved = true
			}
			return printJSON(map[string]any{
				"saved":               saved,
				"token_saved":         session.TokenString(s) != "",
				"refresh_token_saved": session.RefreshTokenString(s) != "",
				"expires_in":          expiresIn,
			})
		},
	}
	c.Flags().StringVar(&refresh, "refresh-token", "", "Optional refresh token (otherwise from session).")
	return c
}

func newSessionLoginCmd() *cobra.Command {
	var loginURL string
	var timeoutSec int
	var userDataDir string
	var profileDirectory string
	var debugLogin bool
	var keepOpenOnFailure bool
	var forceLogin bool

	c := &cobra.Command{
		Use:   "login",
		Short: "Open the provider website in your browser and save the detected session.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, err := selectedProvider(cmd)
			if err != nil {
				return err
			}
			if provider == session.ProviderUpMenu {
				return errors.New("session login is not implemented for UpMenu; import a fresh authenticated request with 'session from-curl'")
			}
			if !forceLogin {
				reused, err := sessionLoginTryReuse(provider)
				if err != nil {
					return err
				}
				if reused != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Existing %s session OK: using %s. Pass --force to re-login in a browser.\n", provider, reused.SessionFile)
					return printJSON(map[string]any{
						"saved":                   true,
						"provider":                reused.Provider,
						"reused_existing_session": true,
						"session_file":            reused.SessionFile,
						"base_url":                reused.BaseURL,
						"user_id":                 reused.UserID,
						"token_saved":             reused.TokenSaved,
						"refresh_token_saved":     reused.RefreshTokenSaved,
						"cookie_saved":            reused.CookieSaved,
					})
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"Opening %s in your default browser app with temporary remote debugging and importing the detected session.\n",
				session.ProviderDisplayName(provider),
			)

			result, err := sessionLoginRun(context.Background(), login.Options{
				Provider:             provider,
				LoginURL:             loginURL,
				TimeoutSec:           timeoutSec,
				UserDataDir:          userDataDir,
				ProfileDirectory:     profileDirectory,
				Debug:                debugLogin,
				KeepBrowserOnFailure: keepOpenOnFailure,
			})
			if err != nil {
				return err
			}
			return printJSON(map[string]any{
				"saved":                 result.Saved,
				"provider":              result.Provider,
				"browser_app":           result.BrowserApp,
				"browser_user_data_dir": result.BrowserUserDataDir,
				"profile_directory":     result.ProfileDirectory,
				"base_url":              result.BaseURL,
				"user_id":               result.UserID,
				"token_saved":           result.TokenSaved,
				"refresh_token_saved":   result.RefreshTokenSaved,
				"cookie_saved":          result.CookieSaved,
			})
		},
	}

	c.Flags().StringVar(&loginURL, "login-url", "", "Optional start URL (default depends on provider).")
	c.Flags().IntVar(&timeoutSec, "timeout", 180, "Maximum wait time for session detection (seconds).")
	c.Flags().StringVar(&userDataDir, "user-data-dir", "", "Optional browser user data dir to reuse.")
	c.Flags().StringVar(&profileDirectory, "profile-directory", "", "Optional browser profile directory name, e.g. Default or Profile 1.")
	c.Flags().BoolVar(&debugLogin, "debug", false, "Enable verbose session-login diagnostics.")
	c.Flags().BoolVar(&keepOpenOnFailure, "keep-open-on-failure", false, "Keep the spawned browser window/profile open on login failure for manual inspection.")
	c.Flags().BoolVar(&forceLogin, "force", false, "Always open the browser login flow even if a saved session already verifies successfully.")
	return c
}
