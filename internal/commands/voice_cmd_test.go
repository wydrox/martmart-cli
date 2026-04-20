package commands

import (
	"path/filepath"
	"testing"
)

func TestRootIncludesVoiceCommand(t *testing.T) {
	cmd := NewRootCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "voice" {
			return
		}
	}
	t.Fatalf("expected root command to include 'voice' subcommand")
}

func TestResolveVoiceRuntimePathsUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	paths, err := resolveVoiceRuntimePaths()
	if err != nil {
		t.Fatalf("resolveVoiceRuntimePaths returned error: %v", err)
	}

	expectedRoot := filepath.Join(home, ".martmart-cli", "voice")
	if paths.RootDir != expectedRoot {
		t.Fatalf("RootDir = %q, expected %q", paths.RootDir, expectedRoot)
	}
	if base := filepath.Base(paths.PythonPath); base != "python" && base != "python.exe" {
		t.Fatalf("unexpected python binary name in %q", paths.PythonPath)
	}
}

func TestVoiceRootExposesRunFlags(t *testing.T) {
	cmd := newVoiceCmd()
	if cmd.Flags().Lookup("debug") == nil {
		t.Fatalf("expected voice root command to expose --debug")
	}
	if cmd.Flags().Lookup("show-logs") == nil {
		t.Fatalf("expected voice root command to expose --show-logs")
	}
}
