package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout to a pipe for the duration of f, then
// returns all bytes written by f as a string.
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// TestPrintJSON_TableMode verifies that printJSON delegates to printPretty when
// outputFormat is "table", rendering key-value pairs in a tabwriter layout.
func TestPrintJSON_TableMode(t *testing.T) {
	outputFormat = "table"
	defer func() { outputFormat = "table" }()

	input := map[string]any{
		"name":  "banana",
		"price": 2.99,
	}

	out := captureStdout(func() {
		if err := printJSON(input); err != nil {
			t.Fatalf("printJSON returned error: %v", err)
		}
	})

	if !strings.Contains(out, "name") {
		t.Errorf("expected output to contain 'name', got: %q", out)
	}
	if !strings.Contains(out, "banana") {
		t.Errorf("expected output to contain 'banana', got: %q", out)
	}
	if !strings.Contains(out, "price") {
		t.Errorf("expected output to contain 'price', got: %q", out)
	}
}

// TestPrintJSON_JSONMode verifies that printJSON emits valid, indented JSON
// when outputFormat is "json".
func TestPrintJSON_JSONMode(t *testing.T) {
	outputFormat = "json"
	defer func() { outputFormat = "table" }()

	input := map[string]any{
		"id":     42,
		"status": "delivered",
	}

	out := captureStdout(func() {
		if err := printJSON(input); err != nil {
			t.Fatalf("printJSON returned error: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, out)
	}
	if parsed["id"] == nil {
		t.Errorf("expected 'id' key in JSON output, got: %q", out)
	}
	if parsed["status"] != "delivered" {
		t.Errorf("expected status='delivered', got: %v", parsed["status"])
	}
}

// TestPrintPretty_Map verifies that printPretty with a scalar map emits a
// tabwriter-formatted key-value table.
func TestPrintPretty_Map(t *testing.T) {
	outputFormat = "table"
	defer func() { outputFormat = "table" }()

	m := map[string]any{
		"city":    "Warsaw",
		"country": "Poland",
	}

	out := captureStdout(func() {
		if err := printPretty(m); err != nil {
			t.Fatalf("printPretty returned error: %v", err)
		}
	})

	if !strings.Contains(out, "city") {
		t.Errorf("expected 'city' in output, got: %q", out)
	}
	if !strings.Contains(out, "Warsaw") {
		t.Errorf("expected 'Warsaw' in output, got: %q", out)
	}
	if !strings.Contains(out, "country") {
		t.Errorf("expected 'country' in output, got: %q", out)
	}
	if !strings.Contains(out, "Poland") {
		t.Errorf("expected 'Poland' in output, got: %q", out)
	}
}

// TestPrintPretty_List verifies that printPretty with a []any of maps renders a
// tabwriter table including column headers inferred from the map keys.
func TestPrintPretty_List(t *testing.T) {
	outputFormat = "table"
	defer func() { outputFormat = "table" }()

	list := []any{
		map[string]any{"id": "1", "name": "apple", "price": 1.5},
		map[string]any{"id": "2", "name": "pear", "price": 2.0},
	}

	out := captureStdout(func() {
		if err := printPretty(list); err != nil {
			t.Fatalf("printPretty returned error: %v", err)
		}
	})

	// Column headers should appear in output.
	if !strings.Contains(out, "id") {
		t.Errorf("expected column header 'id' in output, got: %q", out)
	}
	if !strings.Contains(out, "name") {
		t.Errorf("expected column header 'name' in output, got: %q", out)
	}
	// Row values should appear.
	if !strings.Contains(out, "apple") {
		t.Errorf("expected 'apple' in output, got: %q", out)
	}
	if !strings.Contains(out, "pear") {
		t.Errorf("expected 'pear' in output, got: %q", out)
	}
}

// TestPrintPrettyList_ScalarItems verifies that a []any of plain strings is
// rendered as a bulleted list (one "- value" line per item).
func TestPrintPrettyList_ScalarItems(t *testing.T) {
	outputFormat = "table"
	defer func() { outputFormat = "table" }()

	list := []any{"milk", "eggs", "bread"}

	out := captureStdout(func() {
		if err := printPrettyList(list, ""); err != nil {
			t.Fatalf("printPrettyList returned error: %v", err)
		}
	})

	for _, item := range []string{"milk", "eggs", "bread"} {
		if !strings.Contains(out, "- "+item) {
			t.Errorf("expected '- %s' in output, got: %q", item, out)
		}
	}
}

// TestPrintPrettyMap_WithProductsList verifies that a map containing a
// "products" key with a non-empty []any of maps renders the list as a table.
func TestPrintPrettyMap_WithProductsList(t *testing.T) {
	outputFormat = "table"
	defer func() { outputFormat = "table" }()

	m := map[string]any{
		"total": 3,
		"products": []any{
			map[string]any{"id": "p1", "name": "yogurt", "price": 3.49},
			map[string]any{"id": "p2", "name": "butter", "price": 5.99},
		},
	}

	out := captureStdout(func() {
		if err := printPrettyMap(m); err != nil {
			t.Fatalf("printPrettyMap returned error: %v", err)
		}
	})

	if !strings.Contains(out, "yogurt") {
		t.Errorf("expected 'yogurt' in output, got: %q", out)
	}
	if !strings.Contains(out, "butter") {
		t.Errorf("expected 'butter' in output, got: %q", out)
	}
	// The scalar "total" field should also appear.
	if !strings.Contains(out, "total") {
		t.Errorf("expected 'total' in output, got: %q", out)
	}
}

// TestPrintPrettySlots verifies that a days/slots payload is rendered with the
// date header and the "from\tto\tmethod\twarehouse" table columns.
func TestPrintPrettySlots(t *testing.T) {
	outputFormat = "table"
	defer func() { outputFormat = "table" }()

	payload := map[string]any{
		"days": []any{
			map[string]any{
				"date": "2026-03-28",
				"slots": []any{
					map[string]any{
						"startsAt":       "2026-03-28T09:00:00Z",
						"endsAt":         "2026-03-28T11:00:00Z",
						"deliveryMethod": "standard",
						"warehouse":      "WA1",
					},
					map[string]any{
						"startsAt":       "2026-03-28T13:00:00Z",
						"endsAt":         "2026-03-28T15:00:00Z",
						"deliveryMethod": "express",
						"warehouse":      "WA2",
					},
				},
			},
		},
	}

	out := captureStdout(func() {
		if err := printPrettySlots(payload); err != nil {
			t.Fatalf("printPrettySlots returned error: %v", err)
		}
	})

	if !strings.Contains(out, "2026-03-28") {
		t.Errorf("expected date '2026-03-28' in output, got: %q", out)
	}
	if !strings.Contains(out, "from") {
		t.Errorf("expected column header 'from' in output, got: %q", out)
	}
	if !strings.Contains(out, "to") {
		t.Errorf("expected column header 'to' in output, got: %q", out)
	}
	if !strings.Contains(out, "09:00") {
		t.Errorf("expected slot time '09:00' in output, got: %q", out)
	}
	if !strings.Contains(out, "standard") {
		t.Errorf("expected delivery method 'standard' in output, got: %q", out)
	}
	if !strings.Contains(out, "express") {
		t.Errorf("expected delivery method 'express' in output, got: %q", out)
	}
}

// TestLoadJSONFile writes a temporary JSON file and verifies that loadJSONFile
// parses it correctly into a map[string]any.
func TestLoadJSONFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	content := `{"key": "value", "count": 7}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	v, err := loadJSONFile(path)
	if err != nil {
		t.Fatalf("loadJSONFile returned error: %v", err)
	}

	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", v)
	}
	if m["key"] != "value" {
		t.Errorf("expected key='value', got: %v", m["key"])
	}
	// JSON numbers unmarshal as float64 by default.
	if m["count"] != float64(7) {
		t.Errorf("expected count=7, got: %v", m["count"])
	}
}

// TestLoadJSONFile_NotExist verifies that loadJSONFile returns a non-nil error
// when the target file does not exist.
func TestLoadJSONFile_NotExist(t *testing.T) {
	_, err := loadJSONFile("/nonexistent/path/that/does/not/exist.json")
	if err == nil {
		t.Fatal("expected an error for missing file, got nil")
	}
}
