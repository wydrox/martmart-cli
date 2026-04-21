package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/wydrox/martmart-cli/internal/shared"
)

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func normalizeSource(source string) string {
	return strings.ToLower(strings.TrimSpace(source))
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asList(v any) []any {
	list, _ := v.([]any)
	return list
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func asFloat64(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(strings.ReplaceAll(t, ",", "."))
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func boolPtr(v bool) *bool { return &v }

func int64Ptr(v int64) *int64 { return &v }

func stringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := asString(m[key]); value != "" {
			return value
		}
	}
	return ""
}

func localizedStringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := shared.LocalizedString(m[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func marshalJSON(v any) []byte {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func normalizeSearchBlob(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		part = strings.Join(strings.Fields(part), " ")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, " ")
}

func boolToInt(v *bool) any {
	if v == nil {
		return nil
	}
	if *v {
		return 1
	}
	return 0
}

func computeChangeHash(rec ProductRecord) string {
	parts := []string{
		firstNonEmpty(rec.Currency, "PLN"),
		nullableIntString(rec.PriceMinor),
		nullableIntString(rec.RegularPriceMinor),
		nullableIntString(rec.PromoPriceMinor),
		nullableIntString(rec.UnitPriceMinor),
		nullableBoolString(rec.Available),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func nullableIntString(v *int64) string {
	if v == nil {
		return "null"
	}
	return strconv.FormatInt(*v, 10)
}

func nullableBoolString(v *bool) string {
	if v == nil {
		return "null"
	}
	if *v {
		return "1"
	}
	return "0"
}

func offerStatePresent(rec ProductRecord) bool {
	return rec.PriceMinor != nil || rec.RegularPriceMinor != nil || rec.PromoPriceMinor != nil || rec.UnitPriceMinor != nil || rec.Available != nil
}

func defaultSeenAt(seenAt time.Time) time.Time {
	if seenAt.IsZero() {
		return time.Now().UTC()
	}
	return seenAt.UTC()
}

func roundMinorUnits(value float64) *int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return int64Ptr(int64(math.Round(value * 100)))
}

func measureText(value float64, unit string) string {
	unit = strings.TrimSpace(unit)
	if value <= 0 && unit == "" {
		return ""
	}
	if unit == "" {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	if value <= 0 {
		return unit
	}
	return fmt.Sprintf("%s %s", strconv.FormatFloat(value, 'f', -1, 64), unit)
}

func parseDelioMeasure(attributes map[string]any) (float64, string, string) {
	if attributes == nil {
		return 0, "", ""
	}
	unit := asString(attributes["contain_unit"])
	raw := firstNonEmpty(asString(attributes["net_contain"]), asString(attributes["netContain"]))
	if raw == "" && unit == "" {
		return 0, "", ""
	}
	if raw != "" {
		parts := strings.Fields(strings.ReplaceAll(raw, ",", "."))
		if len(parts) >= 1 {
			if value, err := strconv.ParseFloat(parts[0], 64); err == nil {
				parsedUnit := unit
				if parsedUnit == "" && len(parts) > 1 {
					parsedUnit = parts[1]
				}
				return value, normalizeMeasureUnit(parsedUnit), measureText(value, normalizeMeasureUnit(parsedUnit))
			}
		}
	}
	if value, ok := asFloat64(attributes["net_contain"]); ok {
		normalizedUnit := normalizeMeasureUnit(unit)
		return value, normalizedUnit, measureText(value, normalizedUnit)
	}
	return 0, normalizeMeasureUnit(unit), strings.TrimSpace(raw)
}

func normalizeMeasureUnit(unit string) string {
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch unit {
	case "kilogram", "kilograms":
		return "kg"
	case "gram", "grams":
		return "g"
	case "litre", "liter", "litres", "liters":
		return "l"
	case "millilitre", "milliliter", "millilitres", "milliliters":
		return "ml"
	default:
		return unit
	}
}
