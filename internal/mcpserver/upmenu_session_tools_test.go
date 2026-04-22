package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/wydrox/martmart-cli/internal/session"
)

func TestMCPResolveSessionProvider_AllowsUpMenu(t *testing.T) {
	got, err := mcpResolveSessionProvider("upmenu")
	if err != nil {
		t.Fatalf("mcpResolveSessionProvider: %v", err)
	}
	if got != session.ProviderUpMenu {
		t.Fatalf("provider = %q, want %q", got, session.ProviderUpMenu)
	}
}

func TestSessionStatusProviders_AllowsUpMenu(t *testing.T) {
	providers, err := sessionStatusProviders("upmenu")
	if err != nil {
		t.Fatalf("sessionStatusProviders: %v", err)
	}
	if len(providers) != 1 || providers[0] != session.ProviderUpMenu {
		t.Fatalf("providers = %v", providers)
	}
}

func TestSessionStatusEntry_UpMenuDoesNotSuggestInteractiveLogin(t *testing.T) {
	entry := sessionStatusEntry(session.ProviderUpMenu, &session.Session{BaseURL: session.DefaultBaseURLForProvider(session.ProviderUpMenu)}, "")
	if entry["interactive_login_hint"] != false {
		t.Fatalf("interactive_login_hint = %#v", entry["interactive_login_hint"])
	}
}

func TestToolSessionLogin_UpMenuUnsupported(t *testing.T) {
	_, _, err := toolSessionLogin(context.Background(), nil, sessionLoginIn{Provider: session.ProviderUpMenu})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolAuthRefreshToken_UpMenuUnsupported(t *testing.T) {
	_, _, err := toolAuthRefreshToken(context.Background(), nil, authRefreshTokenIn{Provider: session.ProviderUpMenu})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}
