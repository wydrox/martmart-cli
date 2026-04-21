package commands

import (
	"slices"
	"strings"
	"testing"

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
