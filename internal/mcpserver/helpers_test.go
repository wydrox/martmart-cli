package mcpserver

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wydrox/martmart-cli/internal/session"
)

func TestOrExtractOrdersList_Array(t *testing.T) {
	data := []any{
		map[string]any{"id": "1"},
		map[string]any{"id": "2"},
	}
	got := orExtractOrdersList(data)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestOrExtractOrdersList_MapWithItems(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"id": "A"},
		},
	}
	got := orExtractOrdersList(data)
	if len(got) != 1 || got[0]["id"] != "A" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestOrExtractOrdersList_Nil(t *testing.T) {
	if got := orExtractOrdersList(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestOrExtractOrderDatetime(t *testing.T) {
	order := map[string]any{"placedAt": "2024-06-01"}
	if got := orExtractOrderDatetime(order); got != "2024-06-01" {
		t.Errorf("got %q", got)
	}
	if got := orExtractOrderDatetime(map[string]any{}); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestOrExtractOrderTotal(t *testing.T) {
	order := map[string]any{"total": 99.99}
	got := orExtractOrderTotal(order)
	if got == nil || *got != 99.99 {
		t.Errorf("expected 99.99, got %v", got)
	}
}

func TestOrExtractOrderTotal_Nil(t *testing.T) {
	got := orExtractOrderTotal(map[string]any{})
	if got != nil {
		t.Errorf("expected nil")
	}
}

func TestOrAddNumber(t *testing.T) {
	var c []float64
	orAddNumber(float64(1.0), &c)
	orAddNumber(int(2), &c)
	orAddNumber(int64(3), &c)
	orAddNumber("skip", &c)
	if len(c) != 3 {
		t.Fatalf("expected 3, got %d", len(c))
	}
}

func TestOrTruthy(t *testing.T) {
	if !orTruthy(true) {
		t.Error("expected true")
	}
	if orTruthy(false) {
		t.Error("expected false")
	}
	if orTruthy("true") {
		t.Error("string should not be truthy")
	}
	if orTruthy(nil) {
		t.Error("nil should not be truthy")
	}
}

func TestOrNonEmptyStr(t *testing.T) {
	s, ok := orNonEmptyStr("hello")
	if !ok || s != "hello" {
		t.Errorf("got %q, %v", s, ok)
	}
	_, ok = orNonEmptyStr(nil)
	if ok {
		t.Error("nil should be empty")
	}
	s, ok = orNonEmptyStr(42)
	if !ok || s != "42" {
		t.Errorf("got %q, %v", s, ok)
	}
}

func TestOrDateAndHHMMFromISO(t *testing.T) {
	cases := []struct {
		input    string
		wantDate string
		wantTime string
	}{
		{"2024-03-15T14:30:00Z", "2024-03-15", "14:30"},
		{"2024-03-15T09:00:00+02:00", "2024-03-15", "09:00"},
		{"no-t-here", "no-t-here", ""},
		{"2024-01-01T9", "2024-01-01", "9"},
	}
	for _, tc := range cases {
		d, h := orDateAndHHMMFromISO(tc.input)
		if d != tc.wantDate || h != tc.wantTime {
			t.Errorf("orDateAndHHMMFromISO(%q) = (%q, %q), want (%q, %q)",
				tc.input, d, h, tc.wantDate, tc.wantTime)
		}
	}
}

func TestParseCalendarDate(t *testing.T) {
	y, m, d, err := parseCalendarDate("2024-3-15")
	if err != nil {
		t.Fatal(err)
	}
	if y != 2024 || m != 3 || d != 15 {
		t.Errorf("got %d-%d-%d", y, m, d)
	}

	_, _, _, err = parseCalendarDate("invalid")
	if err == nil {
		t.Error("expected error")
	}

	_, _, _, err = parseCalendarDate("2024-xx-01")
	if err == nil {
		t.Error("expected error for invalid month")
	}
}

func TestOrExtractDeliveryWindows(t *testing.T) {
	data := map[string]any{
		"days": []any{
			map[string]any{
				"slots": []any{
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
				},
			},
		},
	}
	got := orExtractDeliveryWindows(data)
	if len(got) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(got))
	}
}

func TestOrExtractDeliveryWindows_Deduplicates(t *testing.T) {
	slot := map[string]any{
		"startsAt":       "2024-03-15T10:00:00Z",
		"endsAt":         "2024-03-15T12:00:00Z",
		"deliveryMethod": "Van",
		"warehouse":      "WAR1",
	}
	data := []any{slot, slot}
	got := orExtractDeliveryWindows(data)
	if len(got) != 1 {
		t.Fatalf("expected dedup to 1, got %d", len(got))
	}
}

func TestOrExtractReservableWindows(t *testing.T) {
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
			// missing deliveryMethod and warehouse — should be excluded
		},
	}
	got := orExtractReservableWindows(data)
	if len(got) != 1 {
		t.Fatalf("expected 1 reservable window, got %d", len(got))
	}
}

func TestWrapShippingAddressPayload(t *testing.T) {
	// Already wrapped
	input := map[string]any{"shippingAddress": map[string]any{"city": "Warsaw"}}
	got := wrapShippingAddressPayload(input)
	if _, ok := got["shippingAddress"].(map[string]any); !ok {
		t.Error("expected shippingAddress preserved")
	}

	// Needs wrapping
	input2 := map[string]any{"city": "Warsaw"}
	got2 := wrapShippingAddressPayload(input2)
	inner, ok := got2["shippingAddress"].(map[string]any)
	if !ok {
		t.Fatal("expected wrapping")
	}
	if inner["city"] != "Warsaw" {
		t.Error("unexpected inner")
	}
}

func TestMcpASAStringField(t *testing.T) {
	s, ok := mcpASAStringField("hello")
	if !ok || s != "hello" {
		t.Errorf("got %q, %v", s, ok)
	}
	_, ok = mcpASAStringField(nil)
	if ok {
		t.Error("nil should be empty")
	}
	_, ok = mcpASAStringField("  ")
	if ok {
		t.Error("whitespace should be empty")
	}
}

func TestMcpASATokenSaved(t *testing.T) {
	if mcpASATokenSaved(nil) {
		t.Error("nil session should be false")
	}
	if mcpASATokenSaved(&session.Session{}) {
		t.Error("nil token should be false")
	}
	if !mcpASATokenSaved(&session.Session{Token: "abc"}) {
		t.Error("non-empty token should be true")
	}
	if mcpASATokenSaved(&session.Session{Token: ""}) {
		t.Error("empty string token should be false")
	}
}

func TestMcpASAHeaderKeysSorted(t *testing.T) {
	got := mcpASAHeaderKeysSorted(map[string]string{"C": "3", "A": "1", "B": "2"})
	if len(got) != 3 || got[0] != "A" || got[1] != "B" || got[2] != "C" {
		t.Errorf("unexpected: %v", got)
	}
	got = mcpASAHeaderKeysSorted(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestMcpCPWrapFriscoResult(t *testing.T) {
	data := map[string]any{"key": "value"}
	res, out, err := mcpCPWrapFriscoResult(data)
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Error("expected OK=true")
	}
	if res == nil || len(res.Content) == 0 {
		t.Error("expected non-empty content")
	}
}

func TestNew_RegistersTools(t *testing.T) {
	srv := New()
	if srv == nil {
		t.Fatal("expected non-nil mcp.Server from New()")
	}
}

func TestMcpCPWrapFriscoResult_LargePayload(t *testing.T) {
	// Build a string well over 8000 runes to trigger truncation.
	large := strings.Repeat("x", 9000)
	data := map[string]any{"payload": large}

	res, _, err := mcpCPWrapFriscoResult(data)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.Content) == 0 {
		t.Fatal("expected non-empty content")
	}

	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", res.Content[0])
	}
	if !strings.Contains(tc.Text, "[truncated]") {
		t.Errorf("expected '[truncated]' suffix in content, got length %d", len(tc.Text))
	}
}
