package commands

import (
	"errors"
	"reflect"
	"testing"
)

func TestIngestSearchBestEffort_ForwardsArgs(t *testing.T) {
	orig := catalogIngestSearch
	defer func() { catalogIngestSearch = orig }()

	payload := map[string]any{"products": []any{"x"}}
	called := false
	catalogIngestSearch = func(provider, queryText string, gotPayload any) error {
		called = true
		if provider != "frisco" {
			t.Fatalf("provider = %q, want frisco", provider)
		}
		if queryText != "milk" {
			t.Fatalf("queryText = %q, want milk", queryText)
		}
		if !reflect.DeepEqual(gotPayload, payload) {
			t.Fatalf("payload mismatch: got %#v want %#v", gotPayload, payload)
		}
		return nil
	}

	ingestSearchBestEffort("frisco", "milk", payload)
	if !called {
		t.Fatal("expected ingest to be called")
	}
}

func TestIngestBestEffort_SwallowsErrors(t *testing.T) {
	origSearch := catalogIngestSearch
	origGet := catalogIngestGet
	origCart := catalogIngestCart
	defer func() {
		catalogIngestSearch = origSearch
		catalogIngestGet = origGet
		catalogIngestCart = origCart
	}()

	catalogIngestSearch = func(string, string, any) error { return errors.New("boom") }
	catalogIngestGet = func(string, any) error { return errors.New("boom") }
	catalogIngestCart = func(string, any) error { return errors.New("boom") }

	ingestSearchBestEffort("frisco", "milk", nil)
	ingestGetBestEffort("delio", nil)
	ingestCartBestEffort("frisco", nil)
}

func TestIngestBestEffort_SwallowsPanics(t *testing.T) {
	origSearch := catalogIngestSearch
	origGet := catalogIngestGet
	origCart := catalogIngestCart
	defer func() {
		catalogIngestSearch = origSearch
		catalogIngestGet = origGet
		catalogIngestCart = origCart
	}()

	catalogIngestSearch = func(string, string, any) error { panic("boom") }
	catalogIngestGet = func(string, any) error { panic("boom") }
	catalogIngestCart = func(string, any) error { panic("boom") }

	ingestSearchBestEffort("frisco", "milk", nil)
	ingestGetBestEffort("delio", nil)
	ingestCartBestEffort("frisco", nil)
}
