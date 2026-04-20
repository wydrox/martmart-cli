package login

import (
	"errors"
	"fmt"
	"testing"
)

func TestNoRemoteDebugEndpointIsWrappable(t *testing.T) {
	wrapped := fmt.Errorf("firstAvailableRemoteDebugEndpoint: %w", errNoRemoteDebugEndpoint)
	if !errors.Is(wrapped, errNoRemoteDebugEndpoint) {
		t.Fatalf("errors.Is should report wrapped error as errNoRemoteDebugEndpoint, got false")
	}
}
