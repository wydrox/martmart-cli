package delio

import "testing"

func TestExtractUpdatedCart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		got, err := ExtractUpdatedCart(map[string]any{
			"data": map[string]any{"updateCart": map[string]any{"id": "cart-1"}},
		})
		if err != nil {
			t.Fatalf("ExtractUpdatedCart: %v", err)
		}
		if got["id"] != "cart-1" {
			t.Fatalf("id=%v want cart-1", got["id"])
		}
	})

	t.Run("graphql errors", func(t *testing.T) {
		_, err := ExtractUpdatedCart(map[string]any{
			"errors": []any{map[string]any{"message": "out of stock"}},
			"data":   map[string]any{"updateCart": map[string]any{"id": "cart-1"}},
		})
		if !IsUpdateCurrentCartBusinessError(err) {
			t.Fatalf("expected business error, got %v", err)
		}
	})

	t.Run("missing updateCart", func(t *testing.T) {
		_, err := ExtractUpdatedCart(map[string]any{
			"data": map[string]any{},
		})
		if !IsUpdateCurrentCartBusinessError(err) {
			t.Fatalf("expected business error, got %v", err)
		}
	})
}
