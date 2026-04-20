package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setTempConfigPaths(t *testing.T, current, legacy string) {
	t.Helper()
	origCurrent := configFile
	origLegacy := legacyConfigFile
	configFile = current
	legacyConfigFile = legacy
	t.Cleanup(func() {
		configFile = origCurrent
		legacyConfigFile = origLegacy
	})
}

func TestLoad_FallsBackToLegacyConfig(t *testing.T) {
	base := t.TempDir()
	current := filepath.Join(base, "martmart-cli", "config.json")
	legacy := filepath.Join(base, "frisco-cli", "config.json")
	setTempConfigPaths(t, current, legacy)

	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatalf("MkdirAll legacy: %v", err)
	}
	if err := os.WriteFile(legacy, []byte(`{"default_provider":"delio","rate_limit_rps":2,"rate_limit_burst":3}`), 0o600); err != nil {
		t.Fatalf("WriteFile legacy config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultProvider != "delio" {
		t.Fatalf("DefaultProvider: got %q, want delio", cfg.DefaultProvider)
	}
	if cfg.RateLimitRPS != 2 {
		t.Fatalf("RateLimitRPS: got %v, want 2", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 3 {
		t.Fatalf("RateLimitBurst: got %d, want 3", cfg.RateLimitBurst)
	}
}

func TestSave_WritesCurrentConfigPath(t *testing.T) {
	base := t.TempDir()
	current := filepath.Join(base, "martmart-cli", "config.json")
	legacy := filepath.Join(base, "frisco-cli", "config.json")
	setTempConfigPaths(t, current, legacy)

	if err := Save(&Config{DefaultProvider: "frisco", RateLimitRPS: 1.5, RateLimitBurst: 2}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(current); err != nil {
		t.Fatalf("current config missing: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy config should not be written, stat err=%v", err)
	}
}

func TestSave_EnforcesFileMode0600(t *testing.T) {
	base := t.TempDir()
	current := filepath.Join(base, "martmart-cli", "config.json")
	legacy := filepath.Join(base, "frisco-cli", "config.json")
	setTempConfigPaths(t, current, legacy)

	// Fresh write creates the file with 0600.
	if err := Save(&Config{DefaultProvider: "frisco", RateLimitRPS: 1, RateLimitBurst: 1}); err != nil {
		t.Fatalf("Save (fresh): %v", err)
	}
	fi, err := os.Stat(current)
	if err != nil {
		t.Fatalf("stat after fresh Save: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("fresh file mode: got %o, want 600", got)
	}

	// Pre-existing file with wider permissions must be narrowed back to 0600.
	if err := os.Chmod(current, 0o644); err != nil {
		t.Fatalf("Chmod 0644: %v", err)
	}
	if err := Save(&Config{DefaultProvider: "frisco", RateLimitRPS: 2, RateLimitBurst: 2}); err != nil {
		t.Fatalf("Save (overwrite): %v", err)
	}
	fi, err = os.Stat(current)
	if err != nil {
		t.Fatalf("stat after overwrite Save: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("overwrite file mode: got %o, want 600", got)
	}
}
