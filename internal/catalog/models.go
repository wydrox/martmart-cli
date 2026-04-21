package catalog

import "time"

const (
	SourceSearch = "search"
	SourceGet    = "get"
	SourceCart   = "cart"
	SourceOrder  = "order"

	DefaultQueryTTLDays = 7
)

type ProductRecord struct {
	Provider          string
	ExternalID        string
	Slug              string
	Name              string
	Brand             string
	Description       string
	MeasureValue      float64
	MeasureUnit       string
	MeasureText       string
	ImageURL          string
	Currency          string
	PriceMinor        *int64
	RegularPriceMinor *int64
	PromoPriceMinor   *int64
	UnitPriceMinor    *int64
	Available         *bool
	Source            string
	SeenAt            time.Time
	SearchBlob        string
	RawJSON           []byte
}

type QueryRecord struct {
	Provider              string
	QueryText             string
	QueryNorm             string
	TTLDays               int
	LastLiveSearchAt      *time.Time
	LastSelectedProductID string
	LastSelectedAt        *time.Time
	LastUsedAt            time.Time
	SuccessCount          int
	FallbackCount         int
	LastErrorCode         string
	LastErrorAt           *time.Time
}

type SnapshotRecord struct {
	SeenAt            time.Time
	Source            string
	QueryText         string
	Currency          string
	PriceMinor        *int64
	RegularPriceMinor *int64
	PromoPriceMinor   *int64
	UnitPriceMinor    *int64
	Available         *bool
	ChangeHash        string
	RawOfferJSON      []byte
}
