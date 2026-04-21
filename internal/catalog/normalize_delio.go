package catalog

import (
	"time"

	"github.com/wydrox/martmart-cli/internal/delio"
)

func NormalizeDelioSearch(payload any, seenAt time.Time) ([]ProductRecord, error) {
	search, err := delio.ExtractProductSearch(payload)
	if err != nil {
		return nil, err
	}
	items := asList(search["results"])
	seenAt = defaultSeenAt(seenAt)
	out := make([]ProductRecord, 0, len(items))
	for _, item := range items {
		product := asMap(item)
		rec, ok := delioRecordFromProduct(product, SourceSearch, seenAt, item)
		if ok {
			out = append(out, rec)
		}
	}
	return out, nil
}

func NormalizeDelioGet(payload any, seenAt time.Time) ([]ProductRecord, error) {
	product, err := delio.ExtractProduct(payload)
	if err != nil {
		return nil, err
	}
	seenAt = defaultSeenAt(seenAt)
	rec, ok := delioRecordFromProduct(product, SourceGet, seenAt, product)
	if !ok {
		return nil, nil
	}
	return []ProductRecord{rec}, nil
}

func NormalizeDelioCart(payload any, seenAt time.Time) ([]ProductRecord, error) {
	cart, err := delio.ExtractCurrentCart(payload)
	if err != nil {
		return nil, err
	}
	items := asList(cart["lineItems"])
	seenAt = defaultSeenAt(seenAt)
	out := make([]ProductRecord, 0, len(items))
	for _, item := range items {
		line := asMap(item)
		product := asMap(line["product"])
		rec, ok := delioRecordFromProduct(product, SourceCart, seenAt, line)
		if !ok {
			continue
		}
		if rec.PriceMinor == nil {
			rec.PriceMinor = delioMoneyMinor(line["totalPrice"])
		}
		out = append(out, rec)
	}
	return out, nil
}

func delioRecordFromProduct(product map[string]any, source string, seenAt time.Time, raw any) (ProductRecord, bool) {
	externalID := firstNonEmpty(asString(product["sku"]), asString(product["id"]))
	if externalID == "" {
		return ProductRecord{}, false
	}
	name := firstNonEmpty(asString(product["name"]), externalID)
	description := firstNonEmpty(asString(product["description"]), asString(product["metaDescription"]), asString(product["metaTitle"]))
	attributes := asMap(product["attributes"])
	measureValue, measureUnit, measureTextValue := parseDelioMeasure(attributes)
	available := delioAvailable(product)
	rec := ProductRecord{
		Provider:          "delio",
		ExternalID:        externalID,
		Slug:              asString(product["slug"]),
		Name:              name,
		Brand:             firstNonEmpty(asString(attributes["bi_supplier_name"]), asString(product["brand"])),
		Description:       description,
		MeasureValue:      measureValue,
		MeasureUnit:       measureUnit,
		MeasureText:       measureTextValue,
		ImageURL:          delioImageURL(product),
		Currency:          firstNonEmpty(delioCurrency(product["price"]), "PLN"),
		PriceMinor:        delioCurrentPriceMinor(product),
		RegularPriceMinor: delioRegularPriceMinor(product),
		PromoPriceMinor:   delioPromoPriceMinor(product),
		Available:         available,
		Source:            source,
		SeenAt:            seenAt,
		RawJSON:           marshalJSON(raw),
	}
	rec.SearchBlob = normalizeSearchBlob(rec.Name, rec.Brand, rec.Description, rec.Slug, rec.MeasureText)
	return rec, true
}

func delioImageURL(product map[string]any) string {
	for _, item := range asList(product["imagesUrls"]) {
		if value := asString(item); value != "" {
			return value
		}
	}
	return ""
}

func delioAvailable(product map[string]any) *bool {
	if quantity, ok := asFloat64(product["availableQuantity"]); ok {
		return boolPtr(quantity > 0)
	}
	return nil
}

func delioCurrentPriceMinor(product map[string]any) *int64 {
	if minor := delioPromoPriceMinor(product); minor != nil {
		return minor
	}
	return delioRegularPriceMinor(product)
}

func delioRegularPriceMinor(product map[string]any) *int64 {
	return delioMoneyMinor(asMap(asMap(product["price"])["value"]))
}

func delioPromoPriceMinor(product map[string]any) *int64 {
	return delioMoneyMinor(asMap(asMap(product["price"])["discounted"])["value"])
}

func delioCurrency(price any) string {
	candidates := []map[string]any{
		asMap(asMap(price)["value"]),
		asMap(asMap(asMap(price)["discounted"])["value"]),
	}
	for _, candidate := range candidates {
		if currency := asString(candidate["currencyCode"]); currency != "" {
			return currency
		}
	}
	return ""
}

func delioMoneyMinor(v any) *int64 {
	m := asMap(v)
	if amount, ok := asFloat64(m["centAmount"]); ok {
		return int64Ptr(int64(amount))
	}
	if amount, ok := asFloat64(v); ok {
		return int64Ptr(int64(amount))
	}
	return nil
}
