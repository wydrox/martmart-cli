package catalog

import (
	"time"

	"github.com/wydrox/martmart-cli/internal/shared"
)

func NormalizeFriscoSearch(payload any, seenAt time.Time) ([]ProductRecord, error) {
	return normalizeFriscoProducts(payload, SourceSearch, seenAt)
}

func NormalizeFriscoGet(payload any, seenAt time.Time) ([]ProductRecord, error) {
	return normalizeFriscoProducts(payload, SourceGet, seenAt)
}

func NormalizeFriscoCart(payload any, seenAt time.Time) ([]ProductRecord, error) {
	root := asMap(payload)
	items := asList(root["products"])
	seenAt = defaultSeenAt(seenAt)
	out := make([]ProductRecord, 0, len(items))
	for _, item := range items {
		entry := asMap(item)
		product := asMap(entry["product"])
		rec, ok := friscoRecordFromEntry(entry, product, SourceCart, seenAt)
		if !ok {
			continue
		}
		if rec.PriceMinor == nil {
			rec.PriceMinor = friscoMoneyMinor(entry["price"])
		}
		out = append(out, rec)
	}
	return out, nil
}

func normalizeFriscoProducts(payload any, source string, seenAt time.Time) ([]ProductRecord, error) {
	root := asMap(payload)
	items := asList(root["products"])
	seenAt = defaultSeenAt(seenAt)
	out := make([]ProductRecord, 0, len(items))
	for _, item := range items {
		entry := asMap(item)
		product := asMap(entry["product"])
		rec, ok := friscoRecordFromEntry(entry, product, source, seenAt)
		if ok {
			out = append(out, rec)
		}
	}
	return out, nil
}

func friscoRecordFromEntry(entry, product map[string]any, source string, seenAt time.Time) (ProductRecord, bool) {
	externalID := firstNonEmpty(asString(entry["productId"]), asString(product["productId"]), asString(entry["id"]))
	if externalID == "" {
		return ProductRecord{}, false
	}
	name := firstNonEmpty(shared.ProductNameFromMap(product), shared.ProductNameFromMap(entry))
	if name == "" {
		name = externalID
	}
	brand := stringFromMap(product, "brand")
	description := localizedStringFromMap(product, "description", "metaDescription", "productDescription")
	measureValue, _ := asFloat64(product["grammage"])
	measureUnit := normalizeMeasureUnit(asString(product["unitOfMeasure"]))
	measureTextValue := firstNonEmpty(asString(product["grammageText"]), measureText(measureValue, measureUnit))
	available := friscoAvailable(product)
	rec := ProductRecord{
		Provider:          "frisco",
		ExternalID:        externalID,
		Slug:              stringFromMap(product, "slug"),
		Name:              name,
		Brand:             brand,
		Description:       description,
		MeasureValue:      measureValue,
		MeasureUnit:       measureUnit,
		MeasureText:       measureTextValue,
		ImageURL:          friscoImageURL(product),
		Currency:          "PLN",
		PriceMinor:        friscoMoneyMinor(product["price"]),
		RegularPriceMinor: friscoMoneyMinor(product["regularPrice"]),
		PromoPriceMinor:   firstPriceMinor(product["promoPrice"], product["promotionalPrice"], product["discountedPrice"]),
		UnitPriceMinor:    friscoUnitPriceMinor(product),
		Available:         available,
		Source:            source,
		SeenAt:            seenAt,
		RawJSON:           marshalJSON(entry),
	}
	rec.SearchBlob = normalizeSearchBlob(rec.Name, rec.Brand, rec.Description, rec.Slug, rec.MeasureText)
	return rec, true
}

func friscoMoneyMinor(v any) *int64 {
	m := asMap(v)
	for _, key := range []string{"price", "gross", "amount", "value"} {
		if amount, ok := asFloat64(m[key]); ok {
			return roundMinorUnits(amount)
		}
	}
	if amount, ok := asFloat64(v); ok {
		return roundMinorUnits(amount)
	}
	return nil
}

func firstPriceMinor(values ...any) *int64 {
	for _, value := range values {
		if minor := friscoMoneyMinor(value); minor != nil {
			return minor
		}
	}
	return nil
}

func friscoUnitPriceMinor(product map[string]any) *int64 {
	if pricePerUnit := asMap(product["pricePerUnit"]); pricePerUnit != nil {
		if minor := friscoMoneyMinor(pricePerUnit); minor != nil {
			return minor
		}
	}
	if value, ok := asFloat64(product["pricePerKg"]); ok {
		return roundMinorUnits(value)
	}
	return nil
}

func friscoAvailable(product map[string]any) *bool {
	if value, ok := product["isAvailable"].(bool); ok {
		return boolPtr(value)
	}
	return nil
}

func friscoImageURL(product map[string]any) string {
	for _, key := range []string{"imageUrl", "imageURL", "mainImageUrl", "thumbnailUrl"} {
		if value := asString(product[key]); value != "" {
			return value
		}
	}
	if images := asList(product["images"]); len(images) > 0 {
		for _, image := range images {
			if value := asString(image); value != "" {
				return value
			}
			if m := asMap(image); m != nil {
				if value := firstNonEmpty(asString(m["url"]), asString(m["src"])); value != "" {
					return value
				}
			}
		}
	}
	return ""
}
