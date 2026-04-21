package catalog

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenPathRunsMigrations(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	defer db.Close()

	for _, table := range []string{"products", "product_snapshots", "queries"} {
		var name string
		if err := db.sql.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}

	var version int
	if err := db.sql.QueryRow(`PRAGMA user_version;`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != len(migrations) {
		t.Fatalf("user_version=%d want %d", version, len(migrations))
	}
}

func TestUpsertProductsPreservesRicherFieldsAndDedupesSnapshots(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	firstSeen := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	secondSeen := firstSeen.Add(30 * time.Minute)

	price := int64(349)
	available := true
	if err := db.UpsertProducts(ctx, []ProductRecord{{
		Provider:     "frisco",
		ExternalID:   "123",
		Name:         "Mleko UHT 3.2%",
		Brand:        "Mlekovita",
		Description:  "Pełne mleko UHT",
		MeasureValue: 1,
		MeasureUnit:  "l",
		MeasureText:  "1 l",
		ImageURL:     "https://img.example/mleko.jpg",
		Currency:     "PLN",
		PriceMinor:   &price,
		Available:    &available,
		Source:       SourceSearch,
		SeenAt:       firstSeen,
		RawJSON:      []byte(`{"ok":true}`),
	}}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	if err := db.UpsertProducts(ctx, []ProductRecord{{
		Provider:   "frisco",
		ExternalID: "123",
		Name:       "Mleko UHT 3.2%",
		Brand:      "",
		Source:     SourceCart,
		SeenAt:     secondSeen,
	}}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var description, imageURL, lastSource string
	var snapshotCount int
	if err := db.sql.QueryRow(`SELECT description, image_url, last_source FROM products WHERE provider='frisco' AND external_id='123'`).Scan(&description, &imageURL, &lastSource); err != nil {
		t.Fatalf("read product: %v", err)
	}
	if description != "Pełne mleko UHT" {
		t.Fatalf("description=%q want preserved value", description)
	}
	if imageURL != "https://img.example/mleko.jpg" {
		t.Fatalf("image_url=%q want preserved value", imageURL)
	}
	if lastSource != SourceCart {
		t.Fatalf("last_source=%q want %q", lastSource, SourceCart)
	}
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM product_snapshots`).Scan(&snapshotCount); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if snapshotCount != 1 {
		t.Fatalf("snapshot count=%d want 1", snapshotCount)
	}

	newPrice := int64(399)
	if err := db.UpsertProducts(ctx, []ProductRecord{{
		Provider:   "frisco",
		ExternalID: "123",
		Name:       "Mleko UHT 3.2%",
		Currency:   "PLN",
		PriceMinor: &newPrice,
		Available:  &available,
		Source:     SourceGet,
		SeenAt:     secondSeen.Add(30 * time.Minute),
	}}); err != nil {
		t.Fatalf("third upsert: %v", err)
	}
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM product_snapshots`).Scan(&snapshotCount); err != nil {
		t.Fatalf("count snapshots after change: %v", err)
	}
	if snapshotCount != 2 {
		t.Fatalf("snapshot count after change=%d want 2", snapshotCount)
	}
}

func TestNormalizeFriscoPayloads(t *testing.T) {
	t.Parallel()
	seenAt := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	searchPayload := map[string]any{
		"products": []any{
			map[string]any{
				"productId": "100",
				"product": map[string]any{
					"name":          "Masło ekstra",
					"brand":         "Łaciate",
					"description":   map[string]any{"pl": "Śmietankowe"},
					"price":         map[string]any{"price": 8.99},
					"regularPrice":  map[string]any{"price": 9.49},
					"pricePerUnit":  map[string]any{"price": 44.95},
					"grammage":      0.2,
					"unitOfMeasure": "Kilogram",
					"isAvailable":   true,
					"imageUrl":      "https://img.example/butter.jpg",
				},
			},
		},
	}
	records, err := NormalizeFriscoSearch(searchPayload, seenAt)
	if err != nil {
		t.Fatalf("NormalizeFriscoSearch: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records)=%d want 1", len(records))
	}
	rec := records[0]
	if rec.ExternalID != "100" || rec.Name != "Masło ekstra" || rec.Brand != "Łaciate" {
		t.Fatalf("unexpected record identity: %+v", rec)
	}
	if rec.MeasureUnit != "kg" || rec.MeasureText != "0.2 kg" {
		t.Fatalf("unexpected measure: unit=%q text=%q", rec.MeasureUnit, rec.MeasureText)
	}
	if rec.PriceMinor == nil || *rec.PriceMinor != 899 {
		t.Fatalf("price_minor=%v want 899", rec.PriceMinor)
	}
	if rec.RegularPriceMinor == nil || *rec.RegularPriceMinor != 949 {
		t.Fatalf("regular_price_minor=%v want 949", rec.RegularPriceMinor)
	}
	if rec.UnitPriceMinor == nil || *rec.UnitPriceMinor != 4495 {
		t.Fatalf("unit_price_minor=%v want 4495", rec.UnitPriceMinor)
	}
	if rec.Available == nil || !*rec.Available {
		t.Fatalf("available=%v want true", rec.Available)
	}

	cartPayload := map[string]any{
		"products": []any{
			map[string]any{
				"productId": "100",
				"price":     map[string]any{"price": 8.99},
				"product": map[string]any{
					"name":        "Masło ekstra",
					"brand":       "Łaciate",
					"isAvailable": true,
				},
			},
		},
	}
	cartRecords, err := NormalizeFriscoCart(cartPayload, seenAt)
	if err != nil {
		t.Fatalf("NormalizeFriscoCart: %v", err)
	}
	if len(cartRecords) != 1 || cartRecords[0].PriceMinor == nil || *cartRecords[0].PriceMinor != 899 {
		t.Fatalf("unexpected cart record: %+v", cartRecords)
	}
}

func TestNormalizeDelioPayloads(t *testing.T) {
	t.Parallel()
	seenAt := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	searchPayload := map[string]any{
		"data": map[string]any{
			"productSearch": map[string]any{
				"results": []any{
					map[string]any{
						"sku":               "SKU-1",
						"name":              "Jogurt naturalny",
						"slug":              "jogurt-naturalny",
						"description":       "Gęsty jogurt",
						"imagesUrls":        []any{"https://img.example/yoghurt.jpg"},
						"availableQuantity": float64(7),
						"price": map[string]any{
							"value": map[string]any{"centAmount": float64(459), "currencyCode": "PLN"},
							"discounted": map[string]any{
								"value": map[string]any{"centAmount": float64(399), "currencyCode": "PLN"},
							},
						},
						"attributes": map[string]any{
							"bi_supplier_name": "Piątnica",
							"net_contain":      "180 g",
							"contain_unit":     "g",
						},
					},
				},
			},
		},
	}
	records, err := NormalizeDelioSearch(searchPayload, seenAt)
	if err != nil {
		t.Fatalf("NormalizeDelioSearch: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records)=%d want 1", len(records))
	}
	rec := records[0]
	if rec.ExternalID != "SKU-1" || rec.Brand != "Piątnica" || rec.Slug != "jogurt-naturalny" {
		t.Fatalf("unexpected record identity: %+v", rec)
	}
	if rec.PriceMinor == nil || *rec.PriceMinor != 399 {
		t.Fatalf("price_minor=%v want 399", rec.PriceMinor)
	}
	if rec.RegularPriceMinor == nil || *rec.RegularPriceMinor != 459 {
		t.Fatalf("regular_price_minor=%v want 459", rec.RegularPriceMinor)
	}
	if rec.PromoPriceMinor == nil || *rec.PromoPriceMinor != 399 {
		t.Fatalf("promo_price_minor=%v want 399", rec.PromoPriceMinor)
	}
	if rec.MeasureValue != 180 || rec.MeasureUnit != "g" || rec.MeasureText != "180 g" {
		t.Fatalf("unexpected measure: %+v", rec)
	}
	if rec.Available == nil || !*rec.Available {
		t.Fatalf("available=%v want true", rec.Available)
	}

	cartPayload := map[string]any{
		"data": map[string]any{
			"currentCart": map[string]any{
				"lineItems": []any{
					map[string]any{
						"quantity":   float64(2),
						"totalPrice": map[string]any{"centAmount": float64(798), "currencyCode": "PLN"},
						"product": map[string]any{
							"sku":               "SKU-1",
							"name":              "Jogurt naturalny",
							"availableQuantity": float64(7),
							"price": map[string]any{
								"value": map[string]any{"centAmount": float64(399), "currencyCode": "PLN"},
							},
						},
					},
				},
			},
		},
	}
	cartRecords, err := NormalizeDelioCart(cartPayload, seenAt)
	if err != nil {
		t.Fatalf("NormalizeDelioCart: %v", err)
	}
	if len(cartRecords) != 1 || cartRecords[0].PriceMinor == nil || *cartRecords[0].PriceMinor != 399 {
		t.Fatalf("unexpected cart record: %+v", cartRecords)
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.db")
	db, err := OpenPath(path)
	if err != nil {
		t.Fatalf("OpenPath(%q): %v", path, err)
	}
	return db
}
