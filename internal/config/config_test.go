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
	if err := os.WriteFile(legacy, []byte(`{"rate_limit_rps":2,"rate_limit_burst":3}`), 0o600); err != nil {
		t.Fatalf("WriteFile legacy config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
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

	if err := Save(&Config{RateLimitRPS: 1.5, RateLimitBurst: 2}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(current); err != nil {
		t.Fatalf("current config missing: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy config should not be written, stat err=%v", err)
	}
}
