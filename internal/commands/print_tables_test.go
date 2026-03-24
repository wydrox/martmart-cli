package commands

import (
	"strings"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

func makeCartPayload(products []any) map[string]any {
	return map[string]any{
		"products": products,
		"total": map[string]any{
			"_total": "45.99",
		},
	}
}

func makeCartProduct(pid, name string, qty int, price, grammage, unit string) map[string]any {
	return map[string]any{
		"productId": pid,
		"quantity":  qty,
		"price":     price,
		"product": map[string]any{
			"name":          name,
			"price":         price,
			"grammage":      grammage,
			"unitOfMeasure": unit,
		},
	}
}

// ---- printCartSummary -------------------------------------------------------

func TestPrintCartSummary(t *testing.T) {
	payload := makeCartPayload([]any{
		makeCartProduct("prod-001", "Mleko UHT 3.2%", 2, "3.49", "1 l", "Litre"),
		makeCartProduct("prod-002", "Chleb pszenny", 1, "5.99", "500 g", "Piece"),
	})

	out := captureStdout(func() {
		if err := printCartSummary(payload, cartShowOpts{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Mleko UHT 3.2%") {
		t.Errorf("expected product name 'Mleko UHT 3.2%%' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Chleb pszenny") {
		t.Errorf("expected product name 'Chleb pszenny' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Cart total") {
		t.Errorf("expected 'Cart total' in output, got:\n%s", out)
	}
}

func TestPrintCartSummary_SortByTotal(t *testing.T) {
	payload := makeCartPayload([]any{
		makeCartProduct("prod-001", "Masło 82%", 1, "8.99", "200 g", "Piece"),
		makeCartProduct("prod-002", "Jajka L", 3, "2.49", "10 szt", "Piece"),
	})

	out := captureStdout(func() {
		if err := printCartSummary(payload, cartShowOpts{sortBy: "total"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Masło 82%") {
		t.Errorf("expected 'Masło 82%%' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Jajka L") {
		t.Errorf("expected 'Jajka L' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Cart total") {
		t.Errorf("expected 'Cart total' in output, got:\n%s", out)
	}
}

func TestPrintCartSummary_TopN(t *testing.T) {
	payload := makeCartPayload([]any{
		makeCartProduct("prod-001", "Pomidory cherry", 1, "4.99", "250 g", "Piece"),
		makeCartProduct("prod-002", "Ogórki gruntowe", 1, "3.49", "500 g", "Piece"),
		makeCartProduct("prod-003", "Papryka czerwona", 1, "6.99", "300 g", "Piece"),
	})

	out := captureStdout(func() {
		if err := printCartSummary(payload, cartShowOpts{top: 1}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Only the first product line should appear; the other two should not
	// (tabwriter header line + exactly 1 data line).
	lines := []string{}
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// Skip header and summary lines.
		if strings.HasPrefix(l, "NAME") || strings.HasPrefix(l, "Cart total") {
			continue
		}
		lines = append(lines, l)
	}
	if len(lines) != 1 {
		t.Errorf("expected exactly 1 product line with top=1, got %d lines:\n%s", len(lines), out)
	}
}

func TestPrintCartSummary_NotMap(t *testing.T) {
	err := printCartSummary("not a map", cartShowOpts{})
	if err == nil {
		t.Error("expected error for non-map input, got nil")
	}
}

// ---- printOrderProductsTable ------------------------------------------------

func TestPrintOrderProductsTable(t *testing.T) {
	products := []orderProduct{
		{
			ProductID: "op-001",
			Name:      "Łosoś atlantycki",
			Quantity:  1,
			Price:     24.99,
			Total:     24.99,
			Grammage:  300,
			Unit:      "gram",
		},
		{
			ProductID: "op-002",
			Name:      "Brokuł",
			Quantity:  2,
			Price:     3.49,
			Total:     6.98,
			Grammage:  0,
			Unit:      "Piece",
		},
	}

	out := captureStdout(func() {
		printOrderProductsTable(products, "")
	})

	if !strings.Contains(out, "NAME") {
		t.Errorf("expected 'NAME' header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Łosoś atlantycki") {
		t.Errorf("expected product name 'Łosoś atlantycki' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Brokuł") {
		t.Errorf("expected product name 'Brokuł' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Order total") {
		t.Errorf("expected 'Order total' in output, got:\n%s", out)
	}
}

// ---- printProfileTable ------------------------------------------------------

func TestPrintProfileTable(t *testing.T) {
	profile := map[string]any{
		"fullName": map[string]any{
			"firstName": "Anna",
			"lastName":  "Kowalska",
		},
		"email":        "anna.kowalska@example.com",
		"phoneNumber":  "+48600123456",
		"postcode":     "00-001",
		"language":     "pl",
		"profileType":  "standard",
		"isAdult":      true,
		"registeredAt": "2021-06-15T10:00:00Z",
	}

	out := captureStdout(func() {
		if err := printProfileTable(profile); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Anna Kowalska", "anna.kowalska@example.com", "+48600123456", "00-001", "2021-06-15"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in profile output, got:\n%s", want, out)
		}
	}
}

// ---- printAddressesTable ----------------------------------------------------

func TestPrintAddressesTable(t *testing.T) {
	addresses := []any{
		map[string]any{
			"id": "addr-uuid-001",
			"shippingAddress": map[string]any{
				"recipient":       "Anna Kowalska",
				"street":          "Marszałkowska",
				"buildingNumber":  "10",
				"apartmentNumber": "5",
				"city":            "Warszawa",
				"postcode":        "00-001",
				"phoneNumber":     "+48600123456",
			},
		},
	}

	out := captureStdout(func() {
		if err := printAddressesTable(addresses); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"id", "recipient", "street", "city", "postcode", "phone"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected header %q in addresses output, got:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "Anna Kowalska") {
		t.Errorf("expected recipient 'Anna Kowalska' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Warszawa") {
		t.Errorf("expected city 'Warszawa' in output, got:\n%s", out)
	}
}

// ---- printSlotsTable --------------------------------------------------------

func TestPrintSlotsTable(t *testing.T) {
	// printSlotsTable takes []map[string]any where each entry has "date" (string)
	// and "slots" ([]map[string]any) with startsAt/endsAt/deliveryMethod/warehouse.
	days := []map[string]any{
		{
			"date": "2026-03-25",
			"slots": []map[string]any{
				{
					"startsAt":       "2026-03-25T06:00:00",
					"endsAt":         "2026-03-25T07:00:00",
					"deliveryMethod": "Van",
					"warehouse":      "WRO1",
				},
				{
					"startsAt":       "2026-03-25T08:00:00",
					"endsAt":         "2026-03-25T09:00:00",
					"deliveryMethod": "Van",
					"warehouse":      "WRO1",
				},
			},
		},
		{
			"date":  "2026-03-26",
			"slots": []map[string]any{},
		},
	}

	out := captureStdout(func() {
		if err := printSlotsTable(days); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "2026-03-25") {
		t.Errorf("expected date '2026-03-25' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2026-03-26") {
		t.Errorf("expected date '2026-03-26' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "from") {
		t.Errorf("expected 'from' column header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Van") {
		t.Errorf("expected delivery method 'Van' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "WRO1") {
		t.Errorf("expected warehouse 'WRO1' in output, got:\n%s", out)
	}
}

// ---- printConsentsTable -----------------------------------------------------

func TestPrintConsentsTable(t *testing.T) {
	consents := map[string]any{
		"marketing_email": true,
		"analytics":       false,
		"sms_promo":       true,
	}

	out := captureStdout(func() {
		if err := printConsentsTable(consents); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "key") || !strings.Contains(out, "enabled") {
		t.Errorf("expected header with 'key' and 'enabled', got:\n%s", out)
	}
	for _, k := range []string{"analytics", "marketing_email", "sms_promo"} {
		if !strings.Contains(out, k) {
			t.Errorf("expected consent key %q in output, got:\n%s", k, out)
		}
	}
}

// ---- printPaymentsTable -----------------------------------------------------

func TestPrintPaymentsTable(t *testing.T) {
	payload := map[string]any{
		"pageIndex":  float64(1),
		"pageCount":  float64(3),
		"totalCount": float64(50),
		"items": []any{
			map[string]any{
				"createdAt":       "2024-06-15T10:30:00Z",
				"status":          "Completed",
				"channelName":     "Online",
				"creditCardBrand": "Visa",
				"orderId":         "ORD-001",
			},
			map[string]any{
				"createdAt":       "2024-07-01T08:00:00Z",
				"status":          "Pending",
				"channelName":     "Mobile",
				"creditCardBrand": "Mastercard",
				"orderId":         "ORD-002",
			},
		},
	}

	out := captureStdout(func() {
		if err := printPaymentsTable(payload); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Page 1/3") {
		t.Errorf("expected 'Page 1/3' in output, got:\n%s", out)
	}
	for _, want := range []string{"date", "status", "channel", "card", "orderId", "Completed", "Visa", "ORD-001", "2024-06-15"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestPrintPaymentsTable_NotMap(t *testing.T) {
	out := captureStdout(func() {
		_ = printPaymentsTable("not a map")
	})
	// Should fall back to printJSON (JSON output of the string)
	if !strings.Contains(out, "not a map") {
		t.Errorf("expected fallback output, got:\n%s", out)
	}
}

// ---- printPointsHistoryTable ------------------------------------------------

func TestPrintPointsHistoryTable(t *testing.T) {
	payload := map[string]any{
		"items": []any{
			map[string]any{
				"createdAt":        "2024-05-10T12:00:00Z",
				"operation":        "Purchase",
				"membershipPoints": 150,
				"orderId":          "ORD-100",
			},
			map[string]any{
				"createdAt":        "2024-05-15T09:00:00Z",
				"operation":        "Redeem",
				"membershipPoints": -50,
				"orderId":          "ORD-101",
			},
		},
	}

	out := captureStdout(func() {
		if err := printPointsHistoryTable(payload); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"date", "operation", "points", "orderId", "Purchase", "Redeem", "2024-05-10", "ORD-100"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestPrintPointsHistoryTable_NotMap(t *testing.T) {
	out := captureStdout(func() {
		_ = printPointsHistoryTable(42)
	})
	if !strings.Contains(out, "42") {
		t.Errorf("expected fallback output, got:\n%s", out)
	}
}

// ---- printProductSearchTable ------------------------------------------------

func TestPrintProductSearchTable(t *testing.T) {
	payload := map[string]any{
		"pageIndex":  float64(1),
		"pageCount":  float64(5),
		"totalCount": float64(123),
		"products": []any{
			map[string]any{
				"productId": "pid-001",
				"product": map[string]any{
					"name":          "Masło ekstra",
					"brand":         "Łaciate",
					"price":         map[string]any{"price": 8.99},
					"grammage":      0.2,
					"unitOfMeasure": "Kilogram",
					"isAvailable":   true,
				},
			},
			map[string]any{
				"productId": "pid-002",
				"product": map[string]any{
					"name":          "Mleko UHT",
					"brand":         "Mlekovita",
					"price":         map[string]any{"price": 3.49},
					"grammage":      1.0,
					"unitOfMeasure": "Litre",
					"isAvailable":   false,
				},
			},
		},
	}

	out := captureStdout(func() {
		if err := printProductSearchTable(payload); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Page 1/5") {
		t.Errorf("expected 'Page 1/5' in output, got:\n%s", out)
	}
	for _, want := range []string{"id", "name", "brand", "price", "grammage", "unit", "pid-001", "Masło ekstra", "Łaciate", "8.99"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestPrintProductSearchTable_NoProducts(t *testing.T) {
	payload := map[string]any{
		"pageIndex":  float64(0),
		"pageCount":  float64(0),
		"totalCount": float64(0),
		"products":   []any{},
	}

	out := captureStdout(func() {
		if err := printProductSearchTable(payload); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "No products found") {
		t.Errorf("expected 'No products found' in output, got:\n%s", out)
	}
}

// ---- printCartBatchDryRun ---------------------------------------------------

func TestPrintCartBatchDryRun(t *testing.T) {
	lines := []cartBatchLine{
		{productID: "abc-001", quantity: 3},
		{productID: "xyz-999", quantity: 1},
	}

	out := captureStdout(func() {
		if err := printCartBatchDryRun(lines); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "PRODUCT ID") || !strings.Contains(out, "QTY") {
		t.Errorf("expected header with 'PRODUCT ID' and 'QTY', got:\n%s", out)
	}
	if !strings.Contains(out, "abc-001") {
		t.Errorf("expected 'abc-001' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "xyz-999") {
		t.Errorf("expected 'xyz-999' in output, got:\n%s", out)
	}
}
