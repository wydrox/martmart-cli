package upmenu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMenuHTML(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "menu_wola.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	menu, err := ParseMenuHTML(string(data))
	if err != nil {
		t.Fatalf("ParseMenuHTML: %v", err)
	}
	if len(menu.Categories) == 0 {
		t.Fatal("expected categories")
	}
	if len(menu.Products) == 0 {
		t.Fatal("expected products")
	}
	first := menu.Products[0]
	if first.Name != "Truflowe Love Burger" {
		t.Fatalf("unexpected first product name: %q", first.Name)
	}
	if first.ProductPriceID != "6cc16155-1669-11f1-9141-525400080621" {
		t.Fatalf("unexpected first product price id: %q", first.ProductPriceID)
	}
	if first.BasePrice == nil || *first.BasePrice != 39.5 {
		t.Fatalf("unexpected first product price: %+v", first.BasePrice)
	}
}
