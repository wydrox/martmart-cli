package commands

import "github.com/wydrox/martmart-cli/internal/catalog"

var (
	catalogIngestSearch = catalog.IngestSearch
	catalogIngestGet    = catalog.IngestGet
	catalogIngestCart   = catalog.IngestCart
)

func ingestSearchBestEffort(provider, queryText string, payload any) {
	defer swallowCatalogPanic()
	_ = catalogIngestSearch(provider, queryText, payload)
}

func ingestGetBestEffort(provider string, payload any) {
	defer swallowCatalogPanic()
	_ = catalogIngestGet(provider, payload)
}

func ingestCartBestEffort(provider string, payload any) {
	defer swallowCatalogPanic()
	_ = catalogIngestCart(provider, payload)
}

func swallowCatalogPanic() {
	_ = recover()
}
