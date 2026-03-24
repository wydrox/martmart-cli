package commands

import (
	"testing"
)

func TestPackBonusScore(t *testing.T) {
	cases := []struct {
		grammage, preferSize, minWant float64
	}{
		{0, 0, 0},         // zero grammage
		{0.01, 0, 0},      // below 0.05
		{5.0, 0, 0},       // above 3.0
		{0.5, 0, 0.99},    // default range 0.1-1.5 → 1.0
		{1.0, 0, 0.99},    // default range
		{2.0, 0, 0},       // 1.5-3.0: decay
		{0.5, 0.5, 0.99},  // exact preferred size → ratio=1
		{0.25, 0.5, 0.49}, // half preferred → ratio=0.5
		{1.0, 0.5, 0.49},  // double preferred → ratio=0.5
	}
	for _, tc := range cases {
		got := packBonusScore(tc.grammage, tc.preferSize)
		if got < tc.minWant {
			t.Errorf("packBonusScore(%f, %f) = %f, want >= %f",
				tc.grammage, tc.preferSize, got, tc.minWant)
		}
	}
}

func TestHasBulkKeyword(t *testing.T) {
	if !hasBulkKeyword("Woda Zestaw 6x1.5L") {
		t.Error("expected true for 'zestaw'")
	}
	if !hasBulkKeyword("Coca-Cola Multipack") {
		t.Error("expected true for 'multipack'")
	}
	if hasBulkKeyword("Mleko UHT 1L") {
		t.Error("expected false for regular product")
	}
}

func TestScorePick(t *testing.T) {
	c := pickCandidate{
		name:         "Mleko UHT 2% Mlekovita",
		price:        3.49,
		grammage:     1.0,
		unit:         "Kilogram",
		pricePerUnit: 3.49,
	}
	scorePick(&c, "mleko", 0)
	if c.finalScore <= 0 {
		t.Errorf("expected positive score, got %f", c.finalScore)
	}
	if c.matchScore <= 0 {
		t.Errorf("expected positive match score, got %f", c.matchScore)
	}
}

func TestScorePick_BulkPenalty(t *testing.T) {
	c := pickCandidate{
		name:         "Woda Zestaw 6x1.5L",
		price:        15.0,
		grammage:     1.5,
		unit:         "Kilogram",
		pricePerUnit: 10.0,
	}
	scorePick(&c, "woda", 0)
	if c.bulkPenalty != 1.0 {
		t.Errorf("expected bulk penalty 1.0, got %f", c.bulkPenalty)
	}
}

func TestExtractPickCandidates(t *testing.T) {
	result := map[string]any{
		"products": []any{
			map[string]any{
				"productId": "100",
				"product": map[string]any{
					"name":          map[string]any{"pl": "Mleko UHT 2%"},
					"isAvailable":   true,
					"isStocked":     true,
					"unitOfMeasure": "Kilogram",
					"grammage":      1.0,
					"price":         map[string]any{"price": 3.49},
				},
			},
			map[string]any{
				"productId": "200",
				"product": map[string]any{
					"name":        map[string]any{"pl": "Unavailable product"},
					"isAvailable": false,
				},
			},
		},
	}
	candidates := extractPickCandidates(result, "mleko", 0)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 available candidate, got %d", len(candidates))
	}
	if candidates[0].productID != "100" {
		t.Errorf("expected product 100, got %s", candidates[0].productID)
	}
}

func TestExtractPickCandidates_Nil(t *testing.T) {
	got := extractPickCandidates(nil, "test", 0)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
