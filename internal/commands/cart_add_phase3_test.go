package commands

import (
	"errors"
	"testing"
	"time"

	"github.com/wydrox/martmart-cli/internal/catalog"
	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

func TestCartAddSearchWithMemoryFrisco(t *testing.T) {
	t.Run("cache miss live search ingest add success", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		now := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
		cartSearchNow = func() time.Time { return now }
		var ingested, successCalled bool
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) { return nil, nil }
		friscoSearchAndPick = func(_ *session.Session, uid, phrase, categoryID string) (any, string, error) {
			if uid != "u1" || phrase != "milk" || categoryID != "cat" {
				t.Fatalf("unexpected search args")
			}
			return map[string]any{"products": []any{}}, "p1", nil
		}
		cartSearchIngest = func(provider, queryText string, payload any) { ingested = true }
		friscoCartAdd = func(_ *session.Session, uid, productID string, quantity int) (any, error) {
			if uid != "u1" || productID != "p1" || quantity != 2 {
				t.Fatalf("unexpected add args")
			}
			return map[string]any{"ok": true}, nil
		}
		catalogUpsertQuerySuccessBestEffort = func(input catalog.QuerySuccessInput) error {
			successCalled = true
			if !input.LiveSearchUsed || input.LastSelectedProductID != "p1" || input.FallbackUsed {
				t.Fatalf("unexpected success input: %+v", input)
			}
			return nil
		}

		result, err := cartAddSearchWithMemoryFrisco(&session.Session{}, "u1", "milk", "cat", 2)
		if err != nil {
			t.Fatalf("cartAddSearchWithMemoryFrisco: %v", err)
		}
		if result == nil || !ingested || !successCalled {
			t.Fatalf("expected success path with ingest and query write")
		}
	})

	t.Run("fresh query cached add only", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		now := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
		live := now.Add(-24 * time.Hour)
		var searchCalls, ingestCalls int
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "cached"}, nil
		}
		friscoSearchAndPick = func(_ *session.Session, uid, phrase, categoryID string) (any, string, error) {
			searchCalls++
			return nil, "", nil
		}
		cartSearchIngest = func(provider, queryText string, payload any) { ingestCalls++ }
		friscoCartAdd = func(_ *session.Session, uid, productID string, quantity int) (any, error) {
			return map[string]any{"id": "ok"}, nil
		}
		catalogUpsertQuerySuccessBestEffort = func(input catalog.QuerySuccessInput) error {
			if !input.PreserveSelection || input.LiveSearchUsed {
				t.Fatalf("expected cache-hit update: %+v", input)
			}
			return nil
		}

		if _, err := cartAddSearchWithMemoryFrisco(&session.Session{}, "u1", "milk", "cat", 1); err != nil {
			t.Fatal(err)
		}
		if searchCalls != 0 || ingestCalls != 0 {
			t.Fatalf("expected cached path only, search=%d ingest=%d", searchCalls, ingestCalls)
		}
	})

	t.Run("stale query forces live search", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		now := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
		cartSearchNow = func() time.Time { return now }
		live := now.Add(-8 * 24 * time.Hour)
		var searchCalls int
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "cached"}, nil
		}
		friscoSearchAndPick = func(_ *session.Session, uid, phrase, categoryID string) (any, string, error) {
			searchCalls++
			return map[string]any{}, "new", nil
		}
		friscoCartAdd = func(_ *session.Session, uid, productID string, quantity int) (any, error) {
			return map[string]any{}, nil
		}
		if _, err := cartAddSearchWithMemoryFrisco(&session.Session{}, "u1", "milk", "cat", 1); err != nil {
			t.Fatal(err)
		}
		if searchCalls != 1 {
			t.Fatalf("searchCalls=%d want 1", searchCalls)
		}
	})

	t.Run("cached add business failure falls back once", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		now := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
		live := now.Add(-time.Hour)
		var addCalls, searchCalls int
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "cached"}, nil
		}
		friscoCartAdd = func(_ *session.Session, uid, productID string, quantity int) (any, error) {
			addCalls++
			if addCalls == 1 {
				return nil, &httpclient.ErrorDetails{Status: 404, Reason: "Not Found"}
			}
			if productID != "fresh" {
				t.Fatalf("expected fallback product, got %s", productID)
			}
			return map[string]any{"ok": true}, nil
		}
		friscoSearchAndPick = func(_ *session.Session, uid, phrase, categoryID string) (any, string, error) {
			searchCalls++
			return map[string]any{}, "fresh", nil
		}
		catalogUpsertQuerySuccessBestEffort = func(input catalog.QuerySuccessInput) error {
			if !input.FallbackUsed || !input.LiveSearchUsed || input.LastSelectedProductID != "fresh" {
				t.Fatalf("unexpected fallback success input: %+v", input)
			}
			return nil
		}
		if _, err := cartAddSearchWithMemoryFrisco(&session.Session{}, "u1", "milk", "cat", 1); err != nil {
			t.Fatal(err)
		}
		if addCalls != 2 || searchCalls != 1 {
			t.Fatalf("expected one fallback: add=%d search=%d", addCalls, searchCalls)
		}
	})

	t.Run("cached add auth no fallback", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		live := time.Now().UTC().Add(-time.Hour)
		var searchCalls, errorWrites int
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "cached"}, nil
		}
		friscoCartAdd = func(_ *session.Session, uid, productID string, quantity int) (any, error) {
			return nil, &httpclient.ErrorDetails{Status: 401, Reason: "Unauthorized"}
		}
		friscoSearchAndPick = func(_ *session.Session, uid, phrase, categoryID string) (any, string, error) {
			searchCalls++
			return nil, "", nil
		}
		catalogUpsertQueryErrorBestEffort = func(provider, queryText, queryNorm, errorCode string, fallbackUsed bool, now time.Time) error {
			errorWrites++
			return nil
		}
		if _, err := cartAddSearchWithMemoryFrisco(&session.Session{}, "u1", "milk", "cat", 1); err == nil {
			t.Fatal("expected error")
		}
		if searchCalls != 0 || errorWrites != 1 {
			t.Fatalf("expected no fallback and one error write, search=%d errwrites=%d", searchCalls, errorWrites)
		}
	})

	t.Run("fallback add failure does not overwrite selection", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		live := time.Now().UTC().Add(-time.Hour)
		var successWrites, errorWrites int
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "cached"}, nil
		}
		call := 0
		friscoCartAdd = func(_ *session.Session, uid, productID string, quantity int) (any, error) {
			call++
			if call == 1 {
				return nil, &httpclient.ErrorDetails{Status: 404, Reason: "Not Found"}
			}
			return nil, &httpclient.ErrorDetails{Status: 422, Reason: "Unprocessable Entity"}
		}
		friscoSearchAndPick = func(_ *session.Session, uid, phrase, categoryID string) (any, string, error) {
			return map[string]any{}, "fresh", nil
		}
		catalogUpsertQuerySuccessBestEffort = func(input catalog.QuerySuccessInput) error { successWrites++; return nil }
		catalogUpsertQueryErrorBestEffort = func(provider, queryText, queryNorm, errorCode string, fallbackUsed bool, now time.Time) error {
			errorWrites++
			return nil
		}
		if _, err := cartAddSearchWithMemoryFrisco(&session.Session{}, "u1", "milk", "cat", 1); err == nil {
			t.Fatal("expected error")
		}
		if successWrites != 0 || errorWrites != 1 {
			t.Fatalf("expected error write only, success=%d error=%d", successWrites, errorWrites)
		}
	})
}

func TestCartAddSearchWithMemoryDelio(t *testing.T) {
	t.Run("fresh cached sku success", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		live := time.Now().UTC().Add(-time.Hour)
		var searchCalls int
		resolved := &delio.Coordinates{Lat: 52.2, Long: 21.0}
		delioResolveCoords = func(_ *session.Session, coords *delio.Coordinates) (*delio.Coordinates, error) { return resolved, nil }
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "sku-cached"}, nil
		}
		delioSearchAndPick = func(_ *session.Session, phrase string, coords *delio.Coordinates) (any, string, error) {
			searchCalls++
			return nil, "", nil
		}
		delioCurrentCart = func(_ *session.Session) (any, error) {
			return map[string]any{"data": map[string]any{"currentCart": map[string]any{"id": "cart-1"}}}, nil
		}
		delioExtractCurrentCart = delio.ExtractCurrentCart
		delioUpdateCurrentCart = func(_ *session.Session, cartID string, actions []map[string]any) (any, error) {
			return map[string]any{"data": map[string]any{"updateCart": map[string]any{"id": cartID}}}, nil
		}
		delioValidateCartUpdate = delio.ExtractUpdatedCart
		if _, err := cartAddSearchWithMemoryDelio(&session.Session{}, "milk", nil, 1); err != nil {
			t.Fatal(err)
		}
		if searchCalls != 0 {
			t.Fatalf("expected cached path only, searchCalls=%d", searchCalls)
		}
	})

	t.Run("graphql error falls back once", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		live := time.Now().UTC().Add(-time.Hour)
		resolved := &delio.Coordinates{Lat: 52.2, Long: 21.0}
		delioResolveCoords = func(_ *session.Session, coords *delio.Coordinates) (*delio.Coordinates, error) { return resolved, nil }
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "sku-cached"}, nil
		}
		var searchCalls, updateCalls, ingestCalls int
		delioCurrentCart = func(_ *session.Session) (any, error) {
			return map[string]any{"data": map[string]any{"currentCart": map[string]any{"id": "cart-1"}}}, nil
		}
		delioExtractCurrentCart = delio.ExtractCurrentCart
		delioUpdateCurrentCart = func(_ *session.Session, cartID string, actions []map[string]any) (any, error) {
			updateCalls++
			if updateCalls == 1 {
				return map[string]any{"errors": []any{map[string]any{"message": "out of stock"}}}, nil
			}
			return map[string]any{"data": map[string]any{"updateCart": map[string]any{"id": cartID}}}, nil
		}
		delioValidateCartUpdate = delio.ExtractUpdatedCart
		delioSearchAndPick = func(_ *session.Session, phrase string, coords *delio.Coordinates) (any, string, error) {
			searchCalls++
			return map[string]any{}, "sku-fresh", nil
		}
		cartSearchIngest = func(provider, queryText string, payload any) { ingestCalls++ }
		if _, err := cartAddSearchWithMemoryDelio(&session.Session{}, "milk", nil, 1); err != nil {
			t.Fatal(err)
		}
		if searchCalls != 1 || updateCalls != 2 || ingestCalls != 1 {
			t.Fatalf("expected one fallback flow, search=%d update=%d ingest=%d", searchCalls, updateCalls, ingestCalls)
		}
	})

	t.Run("current cart failure does not fallback", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		live := time.Now().UTC().Add(-time.Hour)
		resolved := &delio.Coordinates{Lat: 52.2, Long: 21.0}
		delioResolveCoords = func(_ *session.Session, coords *delio.Coordinates) (*delio.Coordinates, error) { return resolved, nil }
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "sku-cached"}, nil
		}
		var searchCalls int
		delioCurrentCart = func(_ *session.Session) (any, error) { return nil, errors.New("current cart failed") }
		delioSearchAndPick = func(_ *session.Session, phrase string, coords *delio.Coordinates) (any, string, error) {
			searchCalls++
			return nil, "", nil
		}
		if _, err := cartAddSearchWithMemoryDelio(&session.Session{}, "milk", nil, 1); err == nil {
			t.Fatal("expected error")
		}
		if searchCalls != 0 {
			t.Fatalf("expected no fallback, searchCalls=%d", searchCalls)
		}
	})

	t.Run("stale query forces live search", func(t *testing.T) {
		reset := stubCartSearchGlobals()
		defer reset()
		now := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
		cartSearchNow = func() time.Time { return now }
		live := now.Add(-8 * 24 * time.Hour)
		resolved := &delio.Coordinates{Lat: 52.2, Long: 21.0}
		var searchCalls, ingestCalls int
		delioResolveCoords = func(_ *session.Session, coords *delio.Coordinates) (*delio.Coordinates, error) { return resolved, nil }
		catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
			return &catalog.QueryRecord{TTLDays: 7, LastLiveSearchAt: &live, LastSelectedProductID: "sku-cached"}, nil
		}
		delioSearchAndPick = func(_ *session.Session, phrase string, coords *delio.Coordinates) (any, string, error) {
			searchCalls++
			return map[string]any{}, "sku-fresh", nil
		}
		cartSearchIngest = func(provider, queryText string, payload any) { ingestCalls++ }
		delioCurrentCart = func(_ *session.Session) (any, error) {
			return map[string]any{"data": map[string]any{"currentCart": map[string]any{"id": "cart-1"}}}, nil
		}
		delioExtractCurrentCart = delio.ExtractCurrentCart
		delioUpdateCurrentCart = func(_ *session.Session, cartID string, actions []map[string]any) (any, error) {
			return map[string]any{"data": map[string]any{"updateCart": map[string]any{"id": cartID}}}, nil
		}
		delioValidateCartUpdate = delio.ExtractUpdatedCart
		if _, err := cartAddSearchWithMemoryDelio(&session.Session{}, "milk", nil, 1); err != nil {
			t.Fatal(err)
		}
		if searchCalls != 1 || ingestCalls != 1 {
			t.Fatalf("expected stale live search, search=%d ingest=%d", searchCalls, ingestCalls)
		}
	})
}

func stubCartSearchGlobals() func() {
	oldNow := cartSearchNow
	oldGet := catalogGetQueryBestEffort
	oldSuccess := catalogUpsertQuerySuccessBestEffort
	oldError := catalogUpsertQueryErrorBestEffort
	oldIngest := cartSearchIngest
	oldFriscoSearch := friscoSearchAndPick
	oldFriscoAdd := friscoCartAdd
	oldDelioResolve := delioResolveCoords
	oldDelioSearch := delioSearchAndPick
	oldDelioCurrent := delioCurrentCart
	oldDelioExtract := delioExtractCurrentCart
	oldDelioUpdate := delioUpdateCurrentCart
	oldDelioValidate := delioValidateCartUpdate

	cartSearchNow = func() time.Time { return time.Now().UTC() }
	catalogGetQueryBestEffort = oldGet
	catalogUpsertQuerySuccessBestEffort = func(input catalog.QuerySuccessInput) error { return nil }
	catalogUpsertQueryErrorBestEffort = func(provider, queryText, queryNorm, errorCode string, fallbackUsed bool, now time.Time) error {
		return nil
	}
	cartSearchIngest = func(provider, queryText string, payload any) {}
	friscoSearchAndPick = oldFriscoSearch
	friscoCartAdd = oldFriscoAdd
	delioResolveCoords = oldDelioResolve
	delioSearchAndPick = oldDelioSearch
	delioCurrentCart = oldDelioCurrent
	delioExtractCurrentCart = oldDelioExtract
	delioUpdateCurrentCart = oldDelioUpdate
	delioValidateCartUpdate = oldDelioValidate

	return func() {
		cartSearchNow = oldNow
		catalogGetQueryBestEffort = oldGet
		catalogUpsertQuerySuccessBestEffort = oldSuccess
		catalogUpsertQueryErrorBestEffort = oldError
		cartSearchIngest = oldIngest
		friscoSearchAndPick = oldFriscoSearch
		friscoCartAdd = oldFriscoAdd
		delioResolveCoords = oldDelioResolve
		delioSearchAndPick = oldDelioSearch
		delioCurrentCart = oldDelioCurrent
		delioExtractCurrentCart = oldDelioExtract
		delioUpdateCurrentCart = oldDelioUpdate
		delioValidateCartUpdate = oldDelioValidate
	}
}
