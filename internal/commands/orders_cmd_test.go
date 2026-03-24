package commands

import (
	"testing"
)

func TestExtractOrdersList_Array(t *testing.T) {
	data := []any{
		map[string]any{"id": "1"},
		map[string]any{"id": "2"},
	}
	got := extractOrdersList(data)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestExtractOrdersList_MapWithItems(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"id": "10"},
		},
	}
	got := extractOrdersList(data)
	if len(got) != 1 || got[0]["id"] != "10" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestExtractOrdersList_Nil(t *testing.T) {
	got := extractOrdersList(nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestExtractOrderDatetime(t *testing.T) {
	order := map[string]any{
		"createdAt": "2024-01-15T10:30:00Z",
	}
	if got := extractOrderDatetime(order); got != "2024-01-15T10:30:00Z" {
		t.Errorf("got %q", got)
	}

	// Fallback to "date"
	order2 := map[string]any{"date": "2024-02-20"}
	if got := extractOrderDatetime(order2); got != "2024-02-20" {
		t.Errorf("got %q", got)
	}

	// Empty
	if got := extractOrderDatetime(map[string]any{}); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractOrderTotal(t *testing.T) {
	order := map[string]any{
		"total": 125.50,
	}
	got := extractOrderTotal(order)
	if got == nil || *got != 125.50 {
		t.Errorf("expected 125.50, got %v", got)
	}
}

func TestExtractOrderTotal_NestedPricing(t *testing.T) {
	order := map[string]any{
		"pricing": map[string]any{
			"totalPayment": 200.0,
		},
	}
	got := extractOrderTotal(order)
	if got == nil || *got != 200.0 {
		t.Errorf("expected 200.0, got %v", got)
	}
}

func TestExtractOrderTotal_Nil(t *testing.T) {
	got := extractOrderTotal(map[string]any{})
	if got != nil {
		t.Errorf("expected nil, got %v", *got)
	}
}

func TestAddNumber(t *testing.T) {
	var c []float64
	addNumber(float64(1.5), &c)
	addNumber(int(2), &c)
	addNumber(int64(3), &c)
	addNumber("skip", &c)
	addNumber(nil, &c)
	if len(c) != 3 {
		t.Fatalf("expected 3, got %d", len(c))
	}
}

func TestExtractOrderProducts(t *testing.T) {
	order := map[string]any{
		"products": []any{
			map[string]any{
				"productId": "P1",
				"quantity":  float64(2),
				"price":     float64(10.0),
				"total":     float64(20.0),
				"product": map[string]any{
					"name":          map[string]any{"pl": "Mleko", "en": "Milk"},
					"grammage":      float64(1000),
					"unitOfMeasure": "Kilogram",
				},
			},
		},
	}
	products := extractOrderProducts(order)
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
	p := products[0]
	if p.ProductID != "P1" {
		t.Errorf("productID = %q", p.ProductID)
	}
	if p.Name != "Mleko" {
		t.Errorf("name = %q", p.Name)
	}
	if p.Quantity != 2 {
		t.Errorf("qty = %f", p.Quantity)
	}
	if p.Total != 20.0 {
		t.Errorf("total = %f", p.Total)
	}
	if p.Unit != "Kilogram" {
		t.Errorf("unit = %q", p.Unit)
	}
}

func TestExtractOrderProducts_NoProducts(t *testing.T) {
	got := extractOrderProducts(map[string]any{})
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
