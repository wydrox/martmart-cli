package mcpserver

import (
	"testing"

	checkoutcore "github.com/wydrox/martmart-cli/internal/checkout"
)

func TestCheckoutGuardFromPreview(t *testing.T) {
	preview := &checkoutcore.CheckoutPreview{
		CartID:    "cart-1",
		ItemCount: 3,
		Total:     &checkoutcore.Money{Amount: 42.5, Currency: "PLN"},
	}

	guard := checkoutGuardFromPreview(preview)
	if guard == nil {
		t.Fatal("guard is nil")
	}
	if guard.ExpectedCartID != "cart-1" {
		t.Fatalf("ExpectedCartID = %q, want cart-1", guard.ExpectedCartID)
	}
	if guard.ExpectedItemCount == nil || *guard.ExpectedItemCount != 3 {
		t.Fatalf("ExpectedItemCount = %v, want 3", guard.ExpectedItemCount)
	}
	if guard.ExpectedTotal == nil || *guard.ExpectedTotal != 42.5 {
		t.Fatalf("ExpectedTotal = %v, want 42.5", guard.ExpectedTotal)
	}
}

func TestCheckoutGuardFromFinalizeInput_EmptyReturnsNil(t *testing.T) {
	if got := checkoutGuardFromFinalizeInput(checkoutFinalizeIn{}); got != nil {
		t.Fatalf("guard = %#v, want nil", got)
	}
}

func TestCheckoutGuardFromFinalizeInput_UsesExplicitFields(t *testing.T) {
	count := 2
	total := 19.99
	guard := checkoutGuardFromFinalizeInput(checkoutFinalizeIn{
		ExpectedCartID:    "cart-2",
		ExpectedItemCount: &count,
		ExpectedTotal:     &total,
	})
	if guard == nil {
		t.Fatal("guard is nil")
	}
	if guard.ExpectedCartID != "cart-2" || guard.ExpectedItemCount == nil || *guard.ExpectedItemCount != 2 || guard.ExpectedTotal == nil || *guard.ExpectedTotal != 19.99 {
		t.Fatalf("unexpected guard: %#v", guard)
	}
}
