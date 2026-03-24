package commands

import (
	"testing"
)

func TestTruthy(t *testing.T) {
	if !truthy(true) {
		t.Error("expected true")
	}
	if truthy(false) {
		t.Error("expected false for false")
	}
	if truthy("true") {
		t.Error("string should not be truthy")
	}
	if truthy(nil) {
		t.Error("nil should not be truthy")
	}
}

func TestNonEmptyStr(t *testing.T) {
	s, ok := nonEmptyStr("hello")
	if !ok || s != "hello" {
		t.Errorf("got %q, %v", s, ok)
	}
	_, ok = nonEmptyStr(nil)
	if ok {
		t.Error("nil should be empty")
	}
	s, ok = nonEmptyStr(42)
	if !ok || s != "42" {
		t.Errorf("got %q, %v", s, ok)
	}
	_, ok = nonEmptyStr("   ")
	if ok {
		t.Error("whitespace should be empty")
	}
}

func TestDateAndHHMMFromISO(t *testing.T) {
	cases := []struct {
		input    string
		wantDate string
		wantTime string
	}{
		{"2024-03-15T14:30:00Z", "2024-03-15", "14:30"},
		{"2024-03-15T09:00:00+02:00", "2024-03-15", "09:00"},
		{"no-t-here", "no-t-here", ""},
	}
	for _, tc := range cases {
		d, h := dateAndHHMMFromISO(tc.input)
		if d != tc.wantDate || h != tc.wantTime {
			t.Errorf("dateAndHHMMFromISO(%q) = (%q, %q), want (%q, %q)",
				tc.input, d, h, tc.wantDate, tc.wantTime)
		}
	}
}

func TestExtractDeliveryWindows(t *testing.T) {
	data := []any{
		map[string]any{
			"startsAt":       "2024-03-15T10:00:00Z",
			"endsAt":         "2024-03-15T12:00:00Z",
			"deliveryMethod": "Van",
			"warehouse":      "WAR1",
		},
		map[string]any{
			"startsAt":       "2024-03-15T14:00:00Z",
			"endsAt":         "2024-03-15T16:00:00Z",
			"deliveryMethod": "Van",
			"warehouse":      "WAR1",
		},
	}
	got := extractDeliveryWindows(data)
	if len(got) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(got))
	}
}

func TestExtractReservableWindows(t *testing.T) {
	data := []any{
		map[string]any{
			"startsAt":       "2024-03-15T10:00:00Z",
			"endsAt":         "2024-03-15T12:00:00Z",
			"deliveryMethod": "Van",
			"warehouse":      "WAR1",
		},
		map[string]any{
			"startsAt": "2024-03-15T14:00:00Z",
			"endsAt":   "2024-03-15T16:00:00Z",
			// missing deliveryMethod + warehouse
		},
	}
	got := extractReservableWindows(data)
	if len(got) != 1 {
		t.Fatalf("expected 1 reservable window, got %d", len(got))
	}
}
