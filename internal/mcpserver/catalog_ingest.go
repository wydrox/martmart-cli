package mcpserver

import "github.com/wydrox/martmart-cli/internal/catalog"

var (
	mcpCatalogIngestSearch = catalog.IngestSearch
	mcpCatalogIngestGet    = catalog.IngestGet
	mcpCatalogIngestCart   = catalog.IngestCart
	mcpCatalogIngestOrder  = catalog.IngestOrder
)

func mcpIngestSearchBestEffort(provider, queryText string, payload any) {
	defer mcpSwallowCatalogPanic()
	_ = mcpCatalogIngestSearch(provider, queryText, payload)
}

func mcpIngestGetBestEffort(provider string, payload any) {
	defer mcpSwallowCatalogPanic()
	_ = mcpCatalogIngestGet(provider, payload)
}

func mcpIngestCartBestEffort(provider string, payload any) {
	defer mcpSwallowCatalogPanic()
	_ = mcpCatalogIngestCart(provider, payload)
}

func mcpIngestOrderBestEffort(provider string, payload any) {
	defer mcpSwallowCatalogPanic()
	_ = mcpCatalogIngestOrder(provider, payload)
}

func mcpSwallowCatalogPanic() {
	_ = recover()
}
