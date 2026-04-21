package commands

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/login"
	"github.com/wydrox/martmart-cli/internal/session"
)

func TestTokenSaved(t *testing.T) {
	if tokenSaved(nil) {
		t.Error("nil session should be false")
	}
	if tokenSaved(&session.Session{}) {
		t.Error("nil token should be false")
	}
	if !tokenSaved(&session.Session{Token: "abc"}) {
		t.Error("non-empty token should be true")
	}
	if tokenSaved(&session.Session{Token: ""}) {
		t.Error("empty string token should be false")
	}
}

func TestHeaderKeysSorted(t *testing.T) {
	got := headerKeysSorted(map[string]string{"C": "3", "A": "1", "B": "2"})
	if len(got) != 3 || got[0] != "A" || got[1] != "B" || got[2] != "C" {
		t.Errorf("unexpected: %v", got)
	}
	got = headerKeysSorted(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestPrintSessionListTable(t *testing.T) {
	outputFormat = "table"
	defer func() { outputFormat = "table" }()

	entries := []sessionListEntry{{
		Provider:          session.ProviderFrisco,
		Saved:             true,
		AuthPresent:       true,
		BaseURL:           session.DefaultBaseURL,
		UserID:            "646456",
		TokenSaved:        true,
		RefreshTokenSaved: true,
		CookieSaved:       false,
		SessionFile:       "/Users/test/.martmart-cli/frisco-session.json",
	}}

	out := captureStdout(func() {
		if err := printSessionListTable(entries); err != nil {
			t.Fatalf("printSessionListTable returned error: %v", err)
		}
	})

	for _, want := range []string{"provider", "saved", "auth_present", "frisco", "646456", "frisco-session.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got: %q", want, out)
		}
	}
}

func TestNewSessionCmdSubcommands(t *testing.T) {
	cmd := newSessionCmd()
	names := make([]string, 0, len(cmd.Commands()))
	for _, subcmd := range cmd.Commands() {
		names = append(names, subcmd.Name())
	}

	for _, want := range []string{"from-curl", "list", "verify", "login", "refresh-token"} {
		if !slices.Contains(names, want) {
			t.Fatalf("expected session command to include %q, got %v", want, names)
		}
	}
	if slices.Contains(names, "show") {
		t.Fatalf("expected session command to omit %q, got %v", "show", names)
	}
}

func TestSessionLogin_ReusesExistingVerifiedSession(t *testing.T) {
	origReuse := sessionLoginTryReuse
	origRun := sessionLoginRun
	defer func() {
		sessionLoginTryReuse = origReuse
		sessionLoginRun = origRun
	}()

	calledRun := false
	sessionLoginTryReuse = func(provider string) (*reusedSessionResult, error) {
		if provider != session.ProviderFrisco {
			t.Fatalf("provider = %q, want %q", provider, session.ProviderFrisco)
		}
		return &reusedSessionResult{
			Provider:          provider,
			SessionFile:       "/tmp/frisco-session.json",
			BaseURL:           session.DefaultBaseURL,
			UserID:            "646456",
			TokenSaved:        true,
			RefreshTokenSaved: true,
			CookieSaved:       true,
		}, nil
	}
	sessionLoginRun = func(context.Context, login.Options) (*login.Result, error) {
		calledRun = true
		return nil, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "session", "login"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	if calledRun {
		t.Fatal("expected browser login not to run when an existing session is reusable")
	}
	for _, want := range []string{"reused_existing_session", "/tmp/frisco-session.json", "646456"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got %q", want, out)
		}
	}
}

func TestSessionLogin_FallsBackToBrowserWhenReuseMisses(t *testing.T) {
	origReuse := sessionLoginTryReuse
	origRun := sessionLoginRun
	defer func() {
		sessionLoginTryReuse = origReuse
		sessionLoginRun = origRun
	}()

	calledRun := false
	sessionLoginTryReuse = func(string) (*reusedSessionResult, error) {
		return nil, nil
	}
	sessionLoginRun = func(_ context.Context, opts login.Options) (*login.Result, error) {
		calledRun = true
		if opts.Provider != session.ProviderFrisco {
			t.Fatalf("opts.Provider = %q, want %q", opts.Provider, session.ProviderFrisco)
		}
		return &login.Result{
			Saved:             true,
			Provider:          session.ProviderFrisco,
			BaseURL:           session.DefaultBaseURL,
			UserID:            "646456",
			TokenSaved:        true,
			RefreshTokenSaved: true,
			CookieSaved:       true,
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "session", "login", "--timeout", "1"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	if !calledRun {
		t.Fatal("expected browser login to run when there is no reusable session")
	}
	if !strings.Contains(out, "Opening Frisco") {
		t.Fatalf("expected browser login banner in output, got %q", out)
	}
}

func TestShouldRetrySessionLoginInBrowser(t *testing.T) {
	if !shouldRetrySessionLoginInBrowser(&httpclient.ErrorDetails{Status: 401}) {
		t.Fatal("401 should trigger browser login retry")
	}
	if !shouldRetrySessionLoginInBrowser(errors.New("no token in session. Use 'session from-curl' or 'session login'")) {
		t.Fatal("missing token should trigger browser login retry")
	}
	if shouldRetrySessionLoginInBrowser(&httpclient.ErrorDetails{Status: 429}) {
		t.Fatal("429 should not trigger browser login retry")
	}
	if shouldRetrySessionLoginInBrowser(&httpclient.ErrorDetails{Status: 503}) {
		t.Fatal("5xx should not trigger browser login retry")
	}
}
