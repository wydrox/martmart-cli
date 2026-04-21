package upmenu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestaurantInfo(t *testing.T) {
	ts := newFixtureServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	info, err := client.RestaurantInfo(context.Background())
	if err != nil {
		t.Fatalf("RestaurantInfo: %v", err)
	}
	if info.Name != "Dobra Buła Wola" {
		t.Fatalf("name=%q