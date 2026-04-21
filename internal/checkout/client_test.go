package checkout

import (
	"testing"

	"github.com/wydrox/martmart-cli/internal/session"
)

func TestNewClientRoutesByProvider(t *testing.T) {
	if _, ok := NewClient(session.ProviderDelio).(*DelioClient); !ok {
		t.Fatalf("NewClient(delio) did not return DelioClient")
	}
	if _, ok := NewClient(session.ProviderFrisco).(*FriscoClient); !ok {
		t.Fatalf("NewClient(frisco) did not return FriscoClient")
	}
	if _, ok := NewClient("").(*FriscoClient); !ok {
		t.Fatalf("NewClient(\"\") did not fall back to FriscoClient")
	}
}
