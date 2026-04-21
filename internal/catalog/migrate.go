package catalog

import (
	"context"
	"database/sql"
	"fmt"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS products (
  id INTEGER PRIMARY KEY,
  provider TEXT NOT NULL CHECK (provider IN ('frisco', 'delio')),
  external_id TEXT NOT NULL,
  slug TEXT,
  name TEXT NOT NULL,
  brand TEXT,
  description TEXT,
  measure_value REAL,
  measure_unit TEXT,
  measure_text TEXT,
  image_url TEXT,
  current_currency TEXT DEFAULT 'PLN',
  current_price_minor INTEGER,
  current_regular_price_minor INTEGER,
  current_promo_price_minor INTEGER,
  current_unit_price_minor INTEGER,
  current_available INTEGER,
  current_snapshot_at TEXT,
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  last_source TEXT NOT NULL,
  search_blob TEXT NOT NULL,
  raw_last_json TEXT,
  UNIQUE(provider, external_id)
);
CREATE INDEX IF NOT EXISTS idx_products_provider_name ON products(provider, name);
CREATE INDEX IF NOT EXISTS idx_products_provider_last_seen ON products(provider, last_seen_at DESC);`,
	`CREATE TABLE IF NOT EXISTS product_snapshots (
  id INTEGER PRIMARY KEY,
  product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  seen_at TEXT NOT NULL,
  source TEXT NOT NULL CHECK (source IN ('search', 'get', 'cart', 'order')),
  query_text TEXT,
  currency TEXT NOT NULL DEFAULT 'PLN',
  price_minor INTEGER,
  regular_price_minor INTEGER,
  promo_price_minor INTEGER,
  unit_price_minor INTEGER,
  available INTEGER,
  change_hash TEXT NOT NULL,
  raw_offer_json TEXT
);
CREATE INDEX IF NOT EXISTS idx_product_snapshots_product_seen ON product_snapshots(product_id, seen_at DESC);`,
	`CREATE TABLE IF NOT EXISTS queries (
  id INTEGER PRIMARY KEY,
  provider TEXT NOT NULL CHECK (provider IN ('frisco', 'delio')),
  query_text TEXT NOT NULL,
  query_norm TEXT NOT NULL,
  ttl_days INTEGER NOT NULL DEFAULT 7,
  last_live_search_at TEXT,
  last_selected_product_id TEXT,
  last_selected_at TEXT,
  last_used_at TEXT NOT NULL,
  success_count INTEGER NOT NULL DEFAULT 0,
  fallback_count INTEGER NOT NULL DEFAULT 0,
  last_error_code TEXT,
  last_error_at TEXT,
  UNIQUE(provider, query_norm)
);`,
}

func runMigrations(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version;`).Scan(&version); err != nil {
		return fmt.Errorf("read sqlite user_version: %w", err)
	}
	for i := version; i < len(migrations); i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d;`, i+1)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("set user_version %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}
	return nil
}
