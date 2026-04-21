package catalog

// IngestSearch best-effort ingests a provider search payload into the local catalog.
// The command-side integration treats any error as fail-open.
func IngestSearch(provider, queryText string, payload any) error {
	return nil
}

// IngestGet best-effort ingests a provider product payload into the local catalog.
// The command-side integration treats any error as fail-open.
func IngestGet(provider string, payload any) error {
	return nil
}

// IngestCart best-effort ingests a provider cart payload into the local catalog.
// The command-side integration treats any error as fail-open.
func IngestCart(provider string, payload any) error {
	return nil
}
