package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/login"
)

func TestWriteMCPConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "mcp.json")
	cmd := &cobra.Command{}
	cmd.SetOut(&discardWriter{})

	if err := writeMCPConfig(cmd, "Test", path, "/usr/bin/frisco"); err != nil {
		t.Fatalf("writeMCPConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not found")
	}
	frisco, ok := servers["frisco"].(map[string]any)
	if !ok {
		t.Fatal("frisco entry not found")
	}
	if frisco["command"] != "/usr/bin/frisco" {
		t.Errorf("command = %v, want /usr/bin/frisco", frisco["command"])
	}
}

func TestWriteMCPConfig_MergesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	existing := `{"mcpServers":{"other":{"command":"other-bin","args":["serve"]}}}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.SetOut(&discardWriter{})

	if err := writeMCPConfig(cmd, "Test", path, "/usr/bin/frisco"); err != nil {
		t.Fatalf("writeMCPConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	servers := config["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("existing 'other' entry was lost")
	}
	if _, ok := servers["frisco"]; !ok {
		t.Error("frisco entry not added")
	}
}

func TestChromeCandidates_ReturnsNonEmpty(t *testing.T) {
	paths := login.ChromeCandidates()
	if len(paths) == 0 {
		t.Error("ChromeCandidates returned empty list")
	}
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
