package login

import (
	"testing"
	"time"
)

func TestRemainingLoginPageVisibleDelay(t *testing.T) {
	orig := loginSuccessOpenGracePeriod
	defer func() { loginSuccessOpenGracePeriod = orig }()

	loginSuccessOpenGracePeriod = 5 * time.Second

	if got := remainingLoginPageVisibleDelay(time.Time{}); got != 0 {
		t.Fatalf("zero openedAt delay = %v, want 0", got)
	}

	if got := remainingLoginPageVisibleDelay(time.Now().Add(-10 * time.Second)); got != 0 {
		t.Fatalf("expired delay = %v, want 0", got)
	}

	got := remainingLoginPageVisibleDelay(time.Now().Add(-2 * time.Second))
	if got <= 0 || got > 4*time.Second {
		t.Fatalf("delay = %v, want between 0 and 4s", got)
	}
}
