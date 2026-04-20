package commands

import (
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
