package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func IngestSearch(provider, queryText string, payload any) error {
	records, err := normalizeBySource(provider, SourceSearch, payload, time.Now().UTC())
	if err != nil {
		return err
	}
	return upsertNormalized(records)
}

func IngestGet(provider string, payload any) error {
	records, err := normalizeBySource(provider, SourceGet, payload, time.Now().UTC())
	if err != nil {
		return err
	}
	return upsertNormalized(records)
}

func IngestCart(provider string, payload any) error {
	records, err := normalizeBySource(provider, SourceCart, payload, time.Now().UTC())
	if err != nil {
		return err
	}
	return upsertNormalized(records)
}

func normalizeBySource(provider, source string, payload any, seenAt time.Time) ([]ProductRecord, error) {
	switch normalizeProvider(provider) {
	case "frisco":
		switch source {
		case SourceSearch:
			return NormalizeFriscoSearch(payload, seenAt)
		case SourceGet:
			return NormalizeFriscoGet(payload, seenAt)
		case SourceCart:
			return NormalizeFriscoCart(payload, seenAt)
		}
	case "delio":
		switch source {
		case SourceSearch:
			return NormalizeDelioSearch(payload, seenAt)
		case SourceGet:
			return NormalizeDelioGet(payload, seenAt)
		case SourceCart:
			return NormalizeDelioCart(payload, seenAt)
		}
	}
	return nil, fmt.Errorf("unsupported catalog provider/source: %s/%s", provider, source)
}

func upsertNormalized(records []ProductRecord) error {
	if len(records) == 0 {
		return nil
	}
	db, err := Open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.UpsertProducts(context.Background(), records)
}

func (db *DB) UpsertProducts(ctx context.Context, records []ProductRecord) error {
	if db == nil || db.sql == nil {
		return fmt.Errorf("catalog db is nil")
	}
	if len(records) == 0 {
		return nil
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catalog upsert: %w", err)
	}
	for _, rec := range records {
		if err := upsertProductTx(ctx, tx, rec); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit catalog upsert: %w", err)
	}
	return nil
}

func upsertProductTx(ctx context.Context, tx *sql.Tx, rec ProductRecord) error {
	if normalizeProvider(rec.Provider) == "" {
		return fmt.Errorf("product %q missing provider", rec.ExternalID)
	}
	if rec.ExternalID == "" {
		return fmt.Errorf("catalog product missing external id")
	}
	if rec.Name == "" {
		rec.Name = rec.ExternalID
	}
	rec.Provider = normalizeProvider(rec.Provider)
	rec.Source = normalizeSource(rec.Source)
	rec.SeenAt = defaultSeenAt(rec.SeenAt)
	if rec.Currency == "" {
		rec.Currency = "PLN"
	}
	if rec.SearchBlob == "" {
		rec.SearchBlob = normalizeSearchBlob(rec.Name, rec.Brand, rec.Description, rec.Slug, rec.MeasureText)
	}
	seenAt := rec.SeenAt.Format(timeLayout)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO products (
  provider, external_id, slug, name, brand, description, measure_value, measure_unit, measure_text,
  image_url, current_currency, current_price_minor, current_regular_price_minor, current_promo_price_minor,
  current_unit_price_minor, current_available, current_snapshot_at, first_seen_at, last_seen_at,
  last_source, search_blob, raw_last_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(provider, external_id) DO UPDATE SET
  slug = CASE WHEN excluded.slug <> '' THEN excluded.slug ELSE products.slug END,
  name = CASE WHEN excluded.name <> '' THEN excluded.name ELSE products.name END,
  brand = CASE WHEN excluded.brand <> '' THEN excluded.brand ELSE products.brand END,
  description = CASE WHEN excluded.description <> '' THEN excluded.description ELSE products.description END,
  measure_value = CASE WHEN excluded.measure_value > 0 THEN excluded.measure_value ELSE products.measure_value END,
  measure_unit = CASE WHEN excluded.measure_unit <> '' THEN excluded.measure_unit ELSE products.measure_unit END,
  measure_text = CASE WHEN excluded.measure_text <> '' THEN excluded.measure_text ELSE products.measure_text END,
  image_url = CASE WHEN excluded.image_url <> '' THEN excluded.image_url ELSE products.image_url END,
  current_currency = CASE WHEN excluded.current_currency <> '' THEN excluded.current_currency ELSE products.current_currency END,
  current_price_minor = COALESCE(excluded.current_price_minor, products.current_price_minor),
  current_regular_price_minor = COALESCE(excluded.current_regular_price_minor, products.current_regular_price_minor),
  current_promo_price_minor = COALESCE(excluded.current_promo_price_minor, products.current_promo_price_minor),
  current_unit_price_minor = COALESCE(excluded.current_unit_price_minor, products.current_unit_price_minor),
  current_available = COALESCE(excluded.current_available, products.current_available),
  current_snapshot_at = CASE WHEN excluded.current_snapshot_at <> '' THEN excluded.current_snapshot_at ELSE products.current_snapshot_at END,
  last_seen_at = excluded.last_seen_at,
  last_source = excluded.last_source,
  search_blob = CASE WHEN excluded.search_blob <> '' THEN excluded.search_blob ELSE products.search_blob END,
  raw_last_json = COALESCE(excluded.raw_last_json, products.raw_last_json)
`,
		rec.Provider,
		rec.ExternalID,
		rec.Slug,
		rec.Name,
		rec.Brand,
		rec.Description,
		nullableMeasureValue(rec.MeasureValue),
		rec.MeasureUnit,
		rec.MeasureText,
		rec.ImageURL,
		rec.Currency,
		rec.PriceMinor,
		rec.RegularPriceMinor,
		rec.PromoPriceMinor,
		rec.UnitPriceMinor,
		boolToInt(rec.Available),
		seenAt,
		seenAt,
		seenAt,
		rec.Source,
		rec.SearchBlob,
		nullableString(rec.RawJSON),
	); err != nil {
		return fmt.Errorf("upsert product %s/%s: %w", rec.Provider, rec.ExternalID, err)
	}
	var productID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM products WHERE provider = ? AND external_id = ?`, rec.Provider, rec.ExternalID).Scan(&productID); err != nil {
		return fmt.Errorf("lookup product id %s/%s: %w", rec.Provider, rec.ExternalID, err)
	}
	if !offerStatePresent(rec) {
		return nil
	}
	changeHash := computeChangeHash(rec)
	var latestHash string
	err := tx.QueryRowContext(ctx, `SELECT change_hash FROM product_snapshots WHERE product_id = ? ORDER BY seen_at DESC, id DESC LIMIT 1`, productID).Scan(&latestHash)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("lookup latest snapshot %s/%s: %w", rec.Provider, rec.ExternalID, err)
	}
	if latestHash == changeHash {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO product_snapshots (
  product_id, seen_at, source, currency, price_minor, regular_price_minor, promo_price_minor,
  unit_price_minor, available, change_hash, raw_offer_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		productID,
		seenAt,
		rec.Source,
		rec.Currency,
		rec.PriceMinor,
		rec.RegularPriceMinor,
		rec.PromoPriceMinor,
		rec.UnitPriceMinor,
		boolToInt(rec.Available),
		changeHash,
		nullableString(rec.RawJSON),
	); err != nil {
		return fmt.Errorf("insert snapshot %s/%s: %w", rec.Provider, rec.ExternalID, err)
	}
	return nil
}

const timeLayout = "2006-01-02T15:04:05Z07:00"

func nullableString(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func nullableMeasureValue(v float64) any {
	if v <= 0 {
		return nil
	}
	return v
}
