package commands

import (
	"testing"
)

func TestParseGrammageKg(t *testing.T) {
	cases := []struct {
		input string
		want  float64
		ok    bool
	}{
		{"500 g", 0.5, true},
		{"1 kg", 1.0, true},
		{"250ml", 0.25, true},
		{"1.5 l", 1.5, true},
		{"750 ml", 0.75, true},
		{"2 kg", 2.0, true},
		{"100g", 0.1, true},
		{"", 0, false},
		{"szt", 0, false},
		{"5 szt", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseGrammageKg(tc.input)
		if ok != tc.ok {
			t.Errorf("parseGrammageKg(%q): ok=%v, want %v", tc.input, ok, tc.ok)
			continue
		}
		if ok && (got-tc.want > 0.001 || tc.want-got > 0.001) {
			t.Errorf("parseGrammageKg(%q) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

func TestParseMoneyFloat(t *testing.T) {
	cases := []struct {
		input string
		want  float64
		ok    bool
	}{
		{"12.34", 12.34, true},
		{"12,34", 12.34, true},
		{"0.99", 0.99, true},
		{"", 0, false},
		{"-", 0, false},
		{"  7.50  ", 7.50, true},
	}
	for _, tc := range cases {
		got, ok := parseMoneyFloat(tc.input)
		if ok != tc.ok {
			t.Errorf("parseMoneyFloat(%q): ok=%v, want %v", tc.input, ok, tc.ok)
			continue
		}
		if ok && (got-tc.want > 0.001 || tc.want-got > 0.001) {
			t.Errorf("parseMoneyFloat(%q) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

func TestAsString(t *testing.T) {
	if got := asString(nil); got != "" {
		t.Errorf("asString(nil) = %q", got)
	}
	if got := asString("  hello  "); got != "hello" {
		t.Errorf("asString(\" hello \") = %q", got)
	}
	if got := asString(42); got != "42" {
		t.Errorf("asString(42) = %q", got)
	}
}

func TestAsInt(t *testing.T) {
	cases := []struct {
		val  any
		want int
	}{
		{int(5), 5},
		{int32(10), 10},
		{int64(20), 20},
		{float32(3.7), 3},
		{float64(4.9), 4},
		{"nope", 0},
		{nil, 0},
	}
	for _, tc := range cases {
		if got := asInt(tc.val); got != tc.want {
			t.Errorf("asInt(%v) = %d, want %d", tc.val, got, tc.want)
		}
	}
}

func TestFallbackDash(t *testing.T) {
	if got := fallbackDash(""); got != "-" {
		t.Errorf("fallbackDash(\"\") = %q", got)
	}
	if got := fallbackDash("  "); got != "-" {
		t.Errorf("fallbackDash(\"  \") = %q", got)
	}
	if got := fallbackDash("kg"); got != "kg" {
		t.Errorf("fallbackDash(\"kg\") = %q", got)
	}
}

func TestQuantitiesFromCartGET(t *testing.T) {
	data := map[string]any{
		"products": []any{
			map[string]any{"productId": "A", "quantity": float64(2)},
			map[string]any{"productId": "B", "quantity": float64(1)},
			map[string]any{
				"product":  map[string]any{"productId": "C"},
				"quantity": float64(3),
			},
		},
	}
	got := quantitiesFromCartGET(data)
	if got["A"] != 2 || got["B"] != 1 || got["C"] != 3 {
		t.Errorf("unexpected: %v", got)
	}
}

func TestQuantitiesFromCartGET_BadInput(t *testing.T) {
	got := quantitiesFromCartGET("not a map")
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestMergedCartProductsSlice(t *testing.T) {
	qtyMap := map[string]int{
		"B": 2,
		"A": 1,
		"C": 0, // should be excluded
	}
	got := mergedCartProductsSlice(qtyMap)
	if len(got) != 2 {
		t.Fatalf("expected 2 products, got %d", len(got))
	}
	// Should be sorted by ID
	first := got[0].(map[string]any)
	second := got[1].(map[string]any)
	if first["productId"] != "A" || second["productId"] != "B" {
		t.Errorf("unexpected order: %v, %v", first, second)
	}
}
