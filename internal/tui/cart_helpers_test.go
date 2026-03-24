package tui

import (
	"encoding/json"
	"testing"
)

func TestClampCursor(t *testing.T) {
	cases := []struct {
		c, n, want int
	}{
		{0, 0, 0},
		{5, 0, 0},
		{-1, 5, 0},
		{3, 5, 3},
		{10, 5, 4},
		{4, 5, 4},
	}
	for _, tc := range cases {
		if got := clampCursor(tc.c, tc.n); got != tc.want {
			t.Errorf("clampCursor(%d, %d) = %d, want %d", tc.c, tc.n, got, tc.want)
		}
	}
}

func TestParseCartPayload(t *testing.T) {
	data := map[string]any{
		"products": []any{
			map[string]any{
				"productId": "123",
				"quantity":  float64(2),
			},
		},
	}
	lines, err := parseCartPayload(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].productID != "123" || lines[0].quantity != 2 {
		t.Errorf("unexpected line: %+v", lines[0])
	}
}

func TestParseCartPayload_NilAndEmpty(t *testing.T) {
	lines, err := parseCartPayload(nil)
	if err != nil || lines != nil {
		t.Errorf("nil input: lines=%v, err=%v", lines, err)
	}

	lines, err = parseCartPayload(map[string]any{})
	if err != nil || lines != nil {
		t.Errorf("empty map: lines=%v, err=%v", lines, err)
	}
}

func TestParseCartPayload_NotMap(t *testing.T) {
	_, err := parseCartPayload("not a map")
	if err == nil {
		t.Error("expected error for non-map")
	}
}

func TestFirstArray(t *testing.T) {
	root := map[string]any{
		"products": []any{1, 2},
		"other":    "string",
	}
	got := firstArray(root, "items", "products")
	if len(got) != 2 {
		t.Errorf("expected 2 items, got %d", len(got))
	}

	got = firstArray(root, "missing")
	if got != nil {
		t.Errorf("expected nil for missing key")
	}
}

func TestAnyToInt(t *testing.T) {
	cases := []struct {
		val  any
		want int
		ok   bool
	}{
		{int(5), 5, true},
		{int32(10), 10, true},
		{int64(20), 20, true},
		{float64(3.0), 3, true},
		{float32(4.0), 4, true},
		{json.Number("7"), 7, true},
		{"nope", 0, false},
		{nil, 0, false},
	}
	for _, tc := range cases {
		got, ok := anyToInt(tc.val)
		if ok != tc.ok || got != tc.want {
			t.Errorf("anyToInt(%v) = (%d, %v), want (%d, %v)", tc.val, got, ok, tc.want, tc.ok)
		}
	}
}

func TestIntField(t *testing.T) {
	m := map[string]any{
		"quantity": float64(3),
	}
	got, ok := intField(m, "qty", "quantity")
	if !ok || got != 3 {
		t.Errorf("got %d, %v", got, ok)
	}

	_, ok = intField(m, "missing")
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestLineTotalPrice(t *testing.T) {
	cases := []struct {
		qty   int
		price string
		want  string
	}{
		{2, "10.00", "20.00"},
		{3, "5,50", "16.50"},
		{0, "10.00", "—"},
		{1, "", "—"},
		{1, "—", "—"},
		{1, "invalid", "—"},
	}
	for _, tc := range cases {
		if got := lineTotalPrice(tc.qty, tc.price); got != tc.want {
			t.Errorf("lineTotalPrice(%d, %q) = %q, want %q", tc.qty, tc.price, got, tc.want)
		}
	}
}

func TestFormatUnitPrice(t *testing.T) {
	// Direct numeric price
	got := formatUnitPrice(map[string]any{"price": 4.59})
	if got != "4.59" {
		t.Errorf("expected 4.59, got %q", got)
	}

	// Nested price object
	got = formatUnitPrice(map[string]any{
		"unitPrice": map[string]any{"gross": 5.39},
	})
	if got != "5.39" {
		t.Errorf("expected 5.39, got %q", got)
	}

	// Empty map
	got = formatUnitPrice(map[string]any{})
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestMissingDetailsProductIDs(t *testing.T) {
	lines := []cartLine{
		{productID: "A", name: "Full", unitPrice: "10.00"},
		{productID: "B", name: "Has name"},
		{productID: "C"},
		{productID: "C"}, // duplicate
		{productID: ""},  // no ID
	}
	got := missingDetailsProductIDs(lines)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(got), got)
	}
}

func TestParseProductDetailsPayload(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{
				"productId":   "100",
				"displayName": "Test Product",
				"price":       map[string]any{"price": 9.99},
			},
		},
	}
	got := parseProductDetailsPayload(data, []string{"100"})
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got["100"].name != "Test Product" {
		t.Errorf("name = %q", got["100"].name)
	}
}

func TestParseProductDetailsPayload_Nil(t *testing.T) {
	got := parseProductDetailsPayload(nil, []string{"100"})
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	got = parseProductDetailsPayload(map[string]any{}, nil)
	if got != nil {
		t.Errorf("expected nil for empty ids, got %v", got)
	}
}
