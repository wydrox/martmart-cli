package mcpserver

import (
	"reflect"
	"testing"

	"github.com/wydrox/martmart-cli/internal/session"
)

func TestMcpDelioCartItemQuantity(t *testing.T) {
	cart := map[string]any{
		"lineItems": []any{
			map[string]any{
				"quantity": float64(2),
				"product":  map[string]any{"sku": "SKU-1"},
			},
			map[string]any{
				"quantity": int(5),
				"product":  map[string]any{"sku": "SKU-2"},
			},
		},
	}

	if got := mcpDelioCartItemQuantity(cart, "sku-1"); got != 2 {
		t.Fatalf("mcpDelioCartItemQuantity(sku-1) = %d, want 2", got)
	}
	if got := mcpDelioCartItemQuantity(cart, "SKU-2"); got != 5 {
		t.Fatalf("mcpDelioCartItemQuantity(SKU-2) = %d, want 5", got)
	}
	if got := mcpDelioCartItemQuantity(cart, "missing"); got != 0 {
		t.Fatalf("mcpDelioCartItemQuantity(missing) = %d, want 0", got)
	}
}

func TestMcpDelioSetCartQuantity_NoChangeSkipsUpdate(t *testing.T) {
	oldCurrent := mcpDelioCurrentCartFn
	oldExtractCurrent := mcpDelioExtractCurrentCartFn
	oldUpdate := mcpDelioUpdateCurrentCartFn
	oldExtractUpdated := mcpDelioExtractUpdatedCartFn
	t.Cleanup(func() {
		mcpDelioCurrentCartFn = oldCurrent
		mcpDelioExtractCurrentCartFn = oldExtractCurrent
		mcpDelioUpdateCurrentCartFn = oldUpdate
		mcpDelioExtractUpdatedCartFn = oldExtractUpdated
	})

	currentPayload := map[string]any{"kind": "current"}
	mcpDelioCurrentCartFn = func(*session.Session) (any, error) {
		return currentPayload, nil
	}
	mcpDelioExtractCurrentCartFn = func(any) (map[string]any, error) {
		return map[string]any{
			"id": "cart-1",
			"lineItems": []any{
				map[string]any{"quantity": float64(2), "product": map[string]any{"sku": "SKU-1"}},
			},
		}, nil
	}
	updated := false
	mcpDelioUpdateCurrentCartFn = func(*session.Session, string, []map[string]any) (any, error) {
		updated = true
		return nil, nil
	}
	mcpDelioExtractUpdatedCartFn = func(v any) (map[string]any, error) {
		return map[string]any{}, nil
	}

	got, err := mcpDelioSetCartQuantity(&session.Session{}, "SKU-1", 2)
	if err != nil {
		t.Fatalf("mcpDelioSetCartQuantity returned error: %v", err)
	}
	if updated {
		t.Fatal("expected no update call when target quantity matches current quantity")
	}
	if !reflect.DeepEqual(got, currentPayload) {
		t.Fatalf("result = %#v, want %#v", got, currentPayload)
	}
}

func TestMcpDelioSetCartQuantity_UsesDeltaUpdate(t *testing.T) {
	oldCurrent := mcpDelioCurrentCartFn
	oldExtractCurrent := mcpDelioExtractCurrentCartFn
	oldUpdate := mcpDelioUpdateCurrentCartFn
	oldExtractUpdated := mcpDelioExtractUpdatedCartFn
	t.Cleanup(func() {
		mcpDelioCurrentCartFn = oldCurrent
		mcpDelioExtractCurrentCartFn = oldExtractCurrent
		mcpDelioUpdateCurrentCartFn = oldUpdate
		mcpDelioExtractUpdatedCartFn = oldExtractUpdated
	})

	mcpDelioCurrentCartFn = func(*session.Session) (any, error) {
		return map[string]any{"kind": "current"}, nil
	}
	mcpDelioExtractCurrentCartFn = func(any) (map[string]any, error) {
		return map[string]any{
			"id": "cart-42",
			"lineItems": []any{
				map[string]any{"quantity": float64(1), "product": map[string]any{"sku": "SKU-9"}},
			},
		}, nil
	}
	var gotCartID string
	var gotActions []map[string]any
	mcpDelioUpdateCurrentCartFn = func(_ *session.Session, cartID string, actions []map[string]any) (any, error) {
		gotCartID = cartID
		gotActions = actions
		return map[string]any{"kind": "updated"}, nil
	}
	mcpDelioExtractUpdatedCartFn = func(v any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}

	got, err := mcpDelioSetCartQuantity(&session.Session{}, "SKU-9", 3)
	if err != nil {
		t.Fatalf("mcpDelioSetCartQuantity returned error: %v", err)
	}
	if gotCartID != "cart-42" {
		t.Fatalf("cartID = %q, want cart-42", gotCartID)
	}
	wantActions := []map[string]any{{
		"AddLineItem": map[string]any{"quantity": 2, "sku": "SKU-9"},
	}}
	if !reflect.DeepEqual(gotActions, wantActions) {
		t.Fatalf("actions = %#v, want %#v", gotActions, wantActions)
	}
	wantResult := map[string]any{"kind": "updated"}
	if !reflect.DeepEqual(got, wantResult) {
		t.Fatalf("result = %#v, want %#v", got, wantResult)
	}
}
