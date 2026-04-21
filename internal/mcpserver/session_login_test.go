package mcpserver

import "testing"

func TestSessionLoginTimeoutSec_Default(t *testing.T) {
	if got := sessionLoginTimeoutSec(sessionLoginIn{}); got != defaultSessionLoginTimeoutSec {
		t.Fatalf("expected default timeout %d, got %d", defaultSessionLoginTimeoutSec, got)
	}
}

func TestSessionLoginTimeoutSec_Override(t *testing.T) {
	timeout := 240
	if got := sessionLoginTimeoutSec(sessionLoginIn{TimeoutSec: &timeout}); got != 240 {
		t.Fatalf("expected override timeout 240, got %d", got)
	}
}
