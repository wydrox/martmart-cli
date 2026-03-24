package commands

import (
	"encoding/json"
	"testing"
)

func TestIsScalar(t *testing.T) {
	cases := []struct {
		val  any
		want bool
	}{
		{nil, true},
		{"hello", true},
		{true, true},
		{42, true},
		{int32(1), true},
		{int64(2), true},
		{float32(1.5), true},
		{float64(2.5), true},
		{json.Number("3"), true},
		{[]any{1}, false},
		{map[string]any{"a": 1}, false},
	}
	for _, tc := range cases {
		if got := isScalar(tc.val); got != tc.want {
			t.Errorf("isScalar(%v) = %v, want %v", tc.val, got, tc.want)
		}
	}
}

func TestCellValue(t *testing.T) {
	cases := []struct {
		val  any
		want string
	}{
		{nil, "—"},
		{"", "—"},
		{"   ", "—"},
		{"hello", "hello"},
		{42, "42"},
	}
	for _, tc := range cases {
		if got := cellValue(tc.val); got != tc.want {
			t.Errorf("cellValue(%v) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestHhmm(t *testing.T) {
	cases := []struct {
		val  any
		want string
	}{
		{nil, "—"},
		{"2024-03-01T14:30:00Z", "14:30"},
		{"2024-03-01T09:00:00+02:00", "09:00"},
		{"noT", ""},
		{"2024-03-01T9", "9"},
	}
	for _, tc := range cases {
		if got := hhmm(tc.val); got != tc.want {
			t.Errorf("hhmm(%v) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestListOfMaps(t *testing.T) {
	input := []any{
		map[string]any{"a": 1},
		"skip me",
		map[string]any{"b": 2},
		42,
	}
	got := listOfMaps(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 maps, got %d", len(got))
	}
}

func TestInferColumns(t *testing.T) {
	rows := []map[string]any{
		{"name": "A", "quantity": 1, "extra": "x"},
		{"name": "B", "status": "ok"},
	}
	cols := inferColumns(rows)
	if len(cols) == 0 {
		t.Fatal("expected columns")
	}
	// "name" and "status" are priority keys and should appear early
	found := map[string]bool{}
	for _, c := range cols {
		found[c] = true
	}
	if !found["name"] || !found["status"] {
		t.Errorf("missing expected columns in %v", cols)
	}
}

func TestInferColumns_Empty(t *testing.T) {
	cols := inferColumns(nil)
	if len(cols) != 1 || cols[0] != "value" {
		t.Errorf("expected [value], got %v", cols)
	}
}

func TestStringField(t *testing.T) {
	s, ok := stringField("hello")
	if !ok || s != "hello" {
		t.Errorf("got %q, %v", s, ok)
	}
	s, ok = stringField(nil)
	if ok || s != "" {
		t.Errorf("got %q, %v for nil", s, ok)
	}
	s, ok = stringField(42)
	if !ok || s != "42" {
		t.Errorf("got %q, %v for 42", s, ok)
	}
}
