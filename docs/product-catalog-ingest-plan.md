# Product catalog ingest plan

Status: proposal for the next MartMart CLI iteration.

## Summary

Add a local SQLite catalog at `~/.martmart-cli/catalog.db` and ingest product data from successful live command responses:

- `products search`
- `products get`
- `cart show`
- optionally `orders products` (Frisco first)

The catalog is meant to reduce repeated live searches without introducing background crawlers.

Core rule:

- every successful live search ingests **all returned products** into `products`
- every successful product/cart/order response can append price/availability history to `product_snapshots`
- `queries` remembers which product was last selected for a user search phrase and when that phrase was last refreshed

This design intentionally avoids background `refresh_jobs`. Refresh happens only when the user triggers a flow, especially `cart add --search ...`.

## Goals

- Reduce repeated provider requests for common items.
- Grow a reusable local product catalog over time.
- Keep price / availability history.
- Remember which product was actually chosen for a past query.
- Re-run old queries after a TTL so new or cheaper products are not missed.
- Avoid extra validation calls before `cart add`.

## Non-goals

- No background worker.
- No periodic silent crawling.
- No checkout finalization.
- No extra "pre-validate" API call before `cart add`.
- No aggressive retry loops.

## User-facing behavior

### First time: `cart add --search "mleko 3.2 1l"`

1. Run live provider search.
2. Ingest **all search results** into `products`.
3. Append search snapshots for returned products.
4. Pick the best candidate.
5. Call `cart add` for the selected product.
6. Save query memory:
   - provider
   - original query text
   - normalized query text
   - last selected product id
   - last live search timestamp
   - last selected timestamp

### Next time, query still fresh

If the same query is used again before the TTL expires:

1. Look up `queries`.
2. Try `last_selected_product_id` directly in `cart add`.
3. If it succeeds, stop.
4. If it fails with a **business/product** error, run one fallback live search, ingest all results, pick again, and retry once.

### Next time, query is stale (example: after 7 days)

1. Look up `queries`.
2. If `last_live_search_at` is older than the TTL, run a fresh live search first.
3. Ingest **all search results** into `products`.
4. Append search snapshots.
5. Pick the best current candidate.
6. Add it to cart.
7. Update the query row with the newly selected product.

This is the mechanism that lets MartMart discover a new product like `456` even if the previous chosen product was `123`.

## Why `queries` exists

`products` answers:

> What products have we seen?

`queries` answers:

> When the user searches for `"mleko 3.2 1l"`, what did we choose last time, and when did we last refresh that search?

Without `queries`, the app would only remember product ids. That is not enough to discover newly introduced equivalents.

## SQLite location

Use one shared DB file:

- `~/.martmart-cli/catalog.db`

Recommended package:

- `internal/catalog`

Recommended path resolution:

- `filepath.Join(session.StorageDir(), "catalog.db")`

Recommended driver:

- `modernc.org/sqlite` (no CGO requirement)

## Initial schema

### `products`

Current/stable product data for fast lookup.

```sql
CREATE TABLE IF NOT EXISTS products (
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

CREATE INDEX IF NOT EXISTS idx_products_provider_name
  ON products(provider, name);

CREATE INDEX IF NOT EXISTS idx_products_provider_last_seen
  ON products(provider, last_seen_at DESC);
```

Notes:

- Frisco: `external_id = productId`
- Delio: `external_id = sku`
- keep money in integer minor units (`grosze`), not floats
- `slug` is mainly useful for Delio

### `product_snapshots`

Append-only history of observed offer state.

```sql
CREATE TABLE IF NOT EXISTS product_snapshots (
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

CREATE INDEX IF NOT EXISTS idx_product_snapshots_product_seen
  ON product_snapshots(product_id, seen_at DESC);
```

Insert a new snapshot only when the offer state changes compared with the latest known snapshot for that product.

### `queries`

Remember the last chosen product for a user phrase.

```sql
CREATE TABLE IF NOT EXISTS queries (
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
);
```

`queries` is intentionally simple in v1. It stores the last selected product id as provider-native text:

- Frisco `productId`
- Delio `sku`

## Ingest sources

### 1. `products search`

Commands:

- `internal/commands/products_cmd.go:newProductsSearchCmd`
- `internal/commands/delio_helpers.go:extractDelioSearchResults`

Behavior:

- after every successful live search, ingest **all returned results**
- source = `search`
- when the search came from a user add-to-cart flow, update `queries` only after the selected product is successfully added to cart

### 2. `products get`

Command:

- `internal/commands/products_get_cmd.go:newProductsGetCmd`

Behavior:

- ingest the returned product(s)
- source = `get`
- do not update `queries` here, because `get` is product-centric, not query-centric

### 3. `cart show`

Command:

- `internal/commands/cart_cmd.go:newCartShowCmd`

Behavior:

- ingest all products currently present in cart
- source = `cart`
- this is useful because cart data confirms that a specific product is still cart-usable and often includes price information
- do not update `queries` here

### 4. `orders products` (optional)

Commands:

- `internal/commands/orders_cmd.go:newOrdersProductsCmd`
- `internal/commands/orders_cmd.go:extractOrderProducts`

Behavior:

- ingest purchased products when enough structured data is present
- source = `order`
- lower-fidelity than live search/cart responses, so use it mainly for:
  - product id
  - name
  - quantity / price history
  - grammage / unit if available
- do not let order ingest overwrite richer product metadata with poorer order payloads

## Normalization rules

Create provider-specific normalizers that convert raw Frisco/Delio payloads into one internal shape before DB writes.

Suggested internal struct:

```go
type ProductRecord struct {
    Provider           string
    ExternalID         string
    Slug               string
    Name               string
    Brand              string
    Description        string
    MeasureValue       float64
    MeasureUnit        string
    MeasureText        string
    ImageURL           string
    Currency           string
    PriceMinor         *int64
    RegularPriceMinor  *int64
    PromoPriceMinor    *int64
    UnitPriceMinor     *int64
    Available          *bool
    Source             string
    SeenAt             time.Time
    SearchBlob         string
    RawJSON            []byte
}
```

Rules:

- prefer normalized values from provider payloads, not formatted table strings
- keep raw JSON for debugging / future re-normalization
- `search_blob` should include normalized name + brand + description + slug
- price and availability updates write both:
  - the `products.current_*` fields
  - a new `product_snapshots` row when changed

## Query freshness rules

Default TTL in v1:

- `7 days`

Initial implementation should keep this as a package constant. It can move to config later if needed.

Decision rules for `cart add --search`:

1. If there is no `queries` row:
   - do live search
   - ingest all results
   - pick
   - add to cart
   - upsert `queries`

2. If query exists and is fresh:
   - try `last_selected_product_id`
   - if success: update `last_used_at`, increment `success_count`
   - if business/product failure: do one fallback live search, ingest all results, pick, retry once, update `queries`

3. If query exists and is stale:
   - do live search first
   - ingest all results
   - pick current best candidate
   - add to cart
   - update `queries`

## Error / monitoring policy

The intent is to keep provider-visible errors low.

Rules:

- do **not** add a separate validation request before `cart add`
- let `cart add` be the validation step
- do at most **one fallback search** after a product/business failure
- do **not** fallback-search on:
  - `401 Unauthorized`
  - `403 Forbidden`
  - `429 Too Many Requests`
  - generic network errors
  - obvious provider outage / `5xx`
- record the failure in `queries.last_error_code` and return it to the caller

This keeps the flow quieter than `search -> validate -> add` on every request.

## Proposed package layout

```text
internal/catalog/
  db.go            # open DB, PRAGMAs, migrations
  migrate.go       # schema creation / versioning
  models.go        # ProductRecord, QueryRecord, snapshot input structs
  normalize.go     # shared helpers
  normalize_frisco.go
  normalize_delio.go
  ingest.go        # UpsertProducts, IngestSearchResults, IngestCart, IngestOrder
  queries.go       # GetQuery, UpsertQuery, IsStale
```

## Code touch points

### New package

- `internal/catalog/*`

### Dependency

- `go.mod` -> add SQLite driver (`modernc.org/sqlite`)

### Hook ingest into successful commands

- `internal/commands/products_cmd.go`
- `internal/commands/products_get_cmd.go`
- `internal/commands/cart_cmd.go`
- `internal/commands/orders_cmd.go` (optional stage)

### Future query-aware cart flow

- `internal/commands/cart_cmd.go:newCartAddCmd`

## Phased implementation plan

### Phase 1 — catalog foundation

- add SQLite dependency
- add `internal/catalog`
- create migrations for `products`, `product_snapshots`, `queries`
- add Frisco + Delio normalizers
- add snapshot dedupe via `change_hash`

### Phase 2 — passive ingest hooks

- ingest `products search`
- ingest `products get`
- ingest `cart show`
- keep command output unchanged
- fail open: command success must not depend on DB ingest success at first

### Phase 3 — query memory in `cart add --search`

- lookup `queries` before live search
- use 7-day TTL logic
- fallback to one live search on product/business failure
- update `queries` only after successful cart add

### Phase 4 — optional order ingest

- add low-risk order ingest for Frisco `orders products`
- use conservative field precedence so order payloads do not degrade richer product rows

### Phase 5 — docs / UX follow-up

- update README local data layout with `catalog.db`
- document query TTL behavior
- optionally add a future `catalog` debug command (`catalog stats`, `catalog query`, `catalog product-history`)

## Testing plan

### Unit tests

- normalizers for Frisco search/get/cart payloads
- normalizers for Delio search/get/cart payloads
- `queries` TTL checks
- snapshot dedupe rules
- product upsert field precedence

### Integration-ish tests

- `products search` ingests all results
- `products get` ingests one product
- `cart show` ingests all cart lines
- stale query triggers live search
- fresh query uses cached selected product first
- failed cached add triggers one fallback search only

## Open decisions

- whether `orders products` should be enabled immediately or behind a feature flag
- whether query TTL should stay hard-coded at 7 days in v1 or be exposed in config later
- whether to add FTS5 search later for local catalog exploration

## Recommended v1 defaults

- catalog enabled by default
- query TTL = `7 days`
- no background refresh jobs
- no extra pre-validation request
- one fallback live search max
- order ingest postponed until search/get/cart ingest is stable
