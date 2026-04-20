package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/login"
	"github.com/wydrox/martmart-cli/internal/session"
)

// session login defaults to a provider-specific start URL when --login-url is not set.

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage session (token, headers, user_id).",
	}
	cmd.AddCommand(newSessionFromCurlCmd(), newSessionShowCmd(), newSessionVerifyCmd(), newSessionLoginCmd(), newSessionRefreshTokenCmd())
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
			s, err := session.Load()
			if err != nil {
				return err
			}
			session.ApplyFromCurl(s, cd)
			if err := session.Save(s); err != nil {
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

func newSessionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current session (sensitive values redacted).",
		RunE: func(_ *cobra.Command, _ []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			return printJSON(session.RedactedCopy(s))
		},
	}
}

func newSessionVerifyCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify session has token and user_id; GET cart must succeed.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			if providerIs(session.ProviderDelio) {
				if session.HeaderValue(s, "Cookie") == "" {
					return errors.New("no Cookie header in Delio session. Use 'session from-curl' with a copied Delio API request")
				}
				if _, err := delio.CurrentCart(s); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Session OK: Delio currentCart responded successfully.")
				return nil
			}
			if session.TokenString(s) == "" {
				return errors.New(
					"no token in session. Use 'session from-curl' or 'session login'",
				)
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			_, err = httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(
				cmd.OutOrStdout(),
				"Session OK: cart API responded successfully.",
			)
			return nil
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
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
		RunE: func(_ *cobra.Command, _ []string) error {
			if providerIs(session.ProviderDelio) {
				return errors.New("session refresh-token is not implemented for Delio; import fresh cookies with 'session from-curl'")
			}
			s, err := session.Load()
			if err != nil {
				return err
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
				if err := session.Save(s); err != nil {
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

	c := &cobra.Command{
		Use:   "login",
		Short: "Open the provider website in your browser and save the detected session.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(),
				"Opening the provider URL in your browser and waiting for session capture. If needed, sign in and open a logged-in page for that provider.")

			result, err := login.Run(context.Background(), login.Options{
				Provider:         session.CurrentProvider(),
				LoginURL:         loginURL,
				TimeoutSec:       timeoutSec,
				UserDataDir:      userDataDir,
				ProfileDirectory: profileDirectory,
			})
			if err != nil {
				return err
			}
			return printJSON(map[string]any{
				"saved":               result.Saved,
				"provider":            result.Provider,
				"profile_directory":   result.ProfileDirectory,
				"base_url":            result.BaseURL,
				"user_id":             result.UserID,
				"token_saved":         result.TokenSaved,
				"refresh_token_saved": result.RefreshTokenSaved,
				"cookie_saved":        result.CookieSaved,
			})
		},
	}

	c.Flags().StringVar(&loginURL, "login-url", "", "Optional start URL (default depends on provider).")
	c.Flags().IntVar(&timeoutSec, "timeout", 180, "Maximum wait time for session detection (seconds).")
	c.Flags().StringVar(&userDataDir, "user-data-dir", "", "Optional Chrome/Chromium user data dir to reuse.")
	c.Flags().StringVar(&profileDirectory, "profile-directory", "", "Optional Chrome profile directory name, e.g. Default or Profile 1.")
	return c
}
