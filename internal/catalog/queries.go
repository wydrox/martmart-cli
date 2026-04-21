package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wydrox/martmart-cli/internal/delio"
)

const delioQueryCoordPrecision = 4

type QuerySuccessInput struct {
	Provider              string
	QueryText             string
	QueryNorm             string
	TTLDays               int
	LastSelectedProductID string
	Now                   time.Time
	LiveSearchUsed        bool
	FallbackUsed          bool
	PreserveSelection     bool
}

func BuildFriscoQueryNorm(phrase, categoryID string) string {
	base := normalizeSearchBlob(phrase)
	scope := strings.TrimSpace(categoryID)
	if scope == "" {
		return base + "|category:-"
	}
	return base + "|category:" + strings.ToLower(scope)
}

func BuildDelioQueryNorm(phrase string, coords *delio.Coordinates) string {
	base := normalizeSearchBlob(phrase)
	if coords == nil {
		return base + "|coords:-"
	}
	lat := strconvCoord(coords.Lat)
	lng := strconvCoord(coords.Long)
	return base + "|coords:" + lat + "," + lng
}

func strconvCoord(v float64) string {
	factor := math.Pow10(delioQueryCoordPrecision)
	rounded := fmt.Sprintf("%.*f", delioQueryCoordPrecision, math.Round(v*factor)/factor)
	trimmed := strings.TrimRight(strings.TrimRight(rounded, "0"), ".")
	if trimmed == "" || trimmed == "-0" {
		return "0"
	}
	return trimmed
}

func normalizeTTLDays(days int) int {
	if days <= 0 {
		return DefaultQueryTTLDays
	}
	return days
}

func IsFresh(rec *QueryRecord, now time.Time) bool {
	if rec == nil || rec.LastLiveSearchAt == nil {
		return false
	}
	now = defaultSeenAt(now)
	ttl := time.Duration(normalizeTTLDays(rec.TTLDays)) * 24 * time.Hour
	return now.Before(rec.LastLiveSearchAt.Add(ttl))
}

func IsStale(rec *QueryRecord, now time.Time) bool {
	return !IsFresh(rec, now)
}

func (db *DB) GetQuery(ctx context.Context, provider, queryNorm string) (*QueryRecord, error) {
	if db == nil || db.sql == nil {
		return nil, fmt.Errorf("catalog db is nil")
	}
	provider = normalizeProvider(provider)
	queryNorm = strings.TrimSpace(queryNorm)
	if provider == "" || queryNorm == "" {
		return nil, sql.ErrNoRows
	}
	var rec QueryRecord
	var lastLiveSearchAt sql.NullString
	var lastSelectedAt sql.NullString
	var lastUsedAt string
	var lastErrorCode sql.NullString
	var lastErrorAt sql.NullString
	if err := db.sql.QueryRowContext(ctx, `
SELECT provider, query_text, query_norm, ttl_days, last_live_search_at, last_selected_product_id,
       last_selected_at, last_used_at, success_count, fallback_count, last_error_code, last_error_at
FROM queries
WHERE provider = ? AND query_norm = ?
`, provider, queryNorm).Scan(
		&rec.Provider,
		&rec.QueryText,
		&rec.QueryNorm,
		&rec.TTLDays,
		&lastLiveSearchAt,
		&rec.LastSelectedProductID,
		&lastSelectedAt,
		&lastUsedAt,
		&rec.SuccessCount,
		&rec.FallbackCount,
		&lastErrorCode,
		&lastErrorAt,
	); err != nil {
		return nil, err
	}
	rec.TTLDays = normalizeTTLDays(rec.TTLDays)
	if ts, ok := parseQueryTime(lastLiveSearchAt.String); ok {
		rec.LastLiveSearchAt = &ts
	}
	if ts, ok := parseQueryTime(lastSelectedAt.String); ok {
		rec.LastSelectedAt = &ts
	}
	if ts, ok := parseQueryTime(lastUsedAt); ok {
		rec.LastUsedAt = ts
	}
	if lastErrorCode.Valid {
		rec.LastErrorCode = lastErrorCode.String
	}
	if ts, ok := parseQueryTime(lastErrorAt.String); ok {
		rec.LastErrorAt = &ts
	}
	return &rec, nil
}

func parseQueryTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(timeLayout, raw)
	if err != nil {
		return time.Time{}, false
	}
	return ts.UTC(), true
}

func (db *DB) UpsertQuerySuccess(ctx context.Context, input QuerySuccessInput) error {
	if db == nil || db.sql == nil {
		return fmt.Errorf("catalog db is nil")
	}
	provider := normalizeProvider(input.Provider)
	queryNorm := strings.TrimSpace(input.QueryNorm)
	if provider == "" || queryNorm == "" {
		return fmt.Errorf("query success missing provider/query_norm")
	}
	now := defaultSeenAt(input.Now)
	queryText := strings.TrimSpace(input.QueryText)
	ttlDays := normalizeTTLDays(input.TTLDays)
	lastUsedAt := now.Format(timeLayout)
	lastSelectedAt := any(nil)
	lastSelectedProductID := any(nil)
	if !input.PreserveSelection {
		lastSelectedAt = lastUsedAt
		lastSelectedProductID = strings.TrimSpace(input.LastSelectedProductID)
	}
	lastLiveSearchAt := any(nil)
	if input.LiveSearchUsed {
		lastLiveSearchAt = lastUsedAt
	}
	fallbackInc := 0
	if input.FallbackUsed {
		fallbackInc = 1
	}
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO queries (
  provider, query_text, query_norm, ttl_days, last_live_search_at, last_selected_product_id,
  last_selected_at, last_used_at, success_count, fallback_count, last_error_code, last_error_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, NULL, NULL)
ON CONFLICT(provider, query_norm) DO UPDATE SET
  query_text = excluded.query_text,
  ttl_days = excluded.ttl_days,
  last_selected_product_id = CASE WHEN ? THEN queries.last_selected_product_id ELSE excluded.last_selected_product_id END,
  last_selected_at = CASE WHEN ? THEN queries.last_selected_at ELSE excluded.last_selected_at END,
  last_live_search_at = CASE WHEN ? THEN excluded.last_live_search_at ELSE queries.last_live_search_at END,
  last_used_at = excluded.last_used_at,
  success_count = queries.success_count + 1,
  fallback_count = queries.fallback_count + ?,
  last_error_code = NULL,
  last_error_at = NULL
`,
		provider,
		queryText,
		queryNorm,
		ttlDays,
		lastLiveSearchAt,
		lastSelectedProductID,
		lastSelectedAt,
		lastUsedAt,
		fallbackInc,
		input.PreserveSelection,
		input.PreserveSelection,
		input.LiveSearchUsed,
		fallbackInc,
	)
	if err != nil {
		return fmt.Errorf("upsert query success %s/%s: %w", provider, queryNorm, err)
	}
	return nil
}

func (db *DB) UpsertQueryError(ctx context.Context, provider, queryText, queryNorm, errorCode string, now time.Time, fallbackUsed bool) error {
	if db == nil || db.sql == nil {
		return fmt.Errorf("catalog db is nil")
	}
	provider = normalizeProvider(provider)
	queryNorm = strings.TrimSpace(queryNorm)
	if provider == "" || queryNorm == "" {
		return fmt.Errorf("query error missing provider/query_norm")
	}
	now = defaultSeenAt(now)
	fallbackInc := 0
	if fallbackUsed {
		fallbackInc = 1
	}
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO queries (
  provider, query_text, query_norm, ttl_days, last_used_at, success_count, fallback_count, last_error_code, last_error_at
) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)
ON CONFLICT(provider, query_norm) DO UPDATE SET
  last_error_code = excluded.last_error_code,
  last_error_at = excluded.last_error_at,
  fallback_count = queries.fallback_count + ?
`, provider, strings.TrimSpace(queryText), queryNorm, DefaultQueryTTLDays, now.Format(timeLayout), fallbackInc, strings.TrimSpace(errorCode), now.Format(timeLayout), fallbackInc)
	if err != nil {
		return fmt.Errorf("upsert query error %s/%s: %w", provider, queryNorm, err)
	}
	return nil
}
