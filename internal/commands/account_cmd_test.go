package commands

import (
	"testing"
)

func TestFormatStreet(t *testing.T) {
	cases := []struct {
		name string
		addr map[string]any
		want string
	}{
		{"nil", nil, "—"},
		{"no street", map[string]any{}, "—"},
		{"street only", map[string]any{"street": "Marszalkowska"}, "Marszalkowska"},
		{"street + building", map[string]any{"street": "Polna", "buildingNumber": "12"}, "Polna 12"},
		{"full", map[string]any{"street": "Polna", "buildingNumber": "12", "apartmentNumber": "5"}, "Polna 12/5"},
		{"street + apt no building", map[string]any{"street": "Polna", "apartmentNumber": "5"}, "Polna"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatStreet(tc.addr); got != tc.want {
				t.Errorf("formatStreet() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToFloat(t *testing.T) {
	cases := []struct {
		val  any
		want float64
	}{
		{float64(1.5), 1.5},
		{int(2), 2.0},
		{int64(3), 3.0},
		{"skip", 0},
		{nil, 0},
	}
	for _, tc := range cases {
		if got := toFloat(tc.val); got != tc.want {
			t.Errorf("toFloat(%v) = %f, want %f", tc.val, got, tc.want)
		}
	}
}

func TestToSlice(t *testing.T) {
	got := toSlice([]any{1, 2, 3})
	if len(got) != 3 {
		t.Errorf("expected 3 items, got %d", len(got))
	}
	got = toSlice("not a slice")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	got = toSlice(nil)
	if got != nil {
		t.Errorf("expected nil for nil, got %v", got)
	}
}
