package mcpserver

import (
	"errors"
	"testing"
)

func TestMCPCatalogIngestBestEffortSwallowsErrorsAndPanics(t *testing.T) {
	oldSearch := mcpCatalogIngestSearch
	oldGet := mcpCatalogIngestGet
	oldCart := mcpCatalogIngestCart
	oldOrder := mcpCatalogIngestOrder
	t.Cleanup(func() {
		mcpCatalogIngestSearch = oldSearch
		mcpCatalogIngestGet = oldGet
		mcpCatalogIngestCart = oldCart
		mcpCatalogIngestOrder = oldOrder
	})

	calls := 0
	mcpCatalogIngestSearch = func(provider, queryText string, payload any) error {
		calls++
		if provider != "frisco" || queryText != "milk" || payload == nil {
			t.Fatalf("unexpected search ingest args: %q %q %#v", provider, queryText, payload)
		}
		return errors.New("ignored")
	}
	mcpCatalogIngestGet = func(string, any) error { calls++; panic("ignored") }
	mcpCatalogIngestCart = func(string, any) error { calls++; return errors.New("ignored") }
	mcpCatalogIngestOrder = func(string, any) error { calls++; panic("ignored") }

	mcpIngestSearchBestEffort("frisco", "milk", map[string]any{"ok": true})
	mcpIngestGetBestEffort("frisco", nil)
	mcpIngestCartBestEffort("frisco", nil)
	mcpIngestOrderBestEffort("frisco", nil)
	if calls != 4 {
		t.Fatalf("calls = %d, want 4", calls)
	}
}
