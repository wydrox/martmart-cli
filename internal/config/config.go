package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/wydrox/martmart-cli/internal/session"
)

// Config stores shared CLI settings that apply across providers.
type Config struct {
	DefaultProvider string  `json:"default_provider"`
	RateLimitRPS    float64 `json:"rate_limit_rps"`
	RateLimitBurst  int     `json:"rate_limit_burst"`
}

var configFile string

func init() {
	home, _ := os.UserHomeDir()
	configFile = filepath.Join(home, ".frisco-cli", "config.json")
}

func defaultConfig() *Config {
	return &Config{
		DefaultProvider: session.ProviderFrisco,
		RateLimitRPS:    0,
		RateLimitBurst:  1,
	}
}

// Path returns the on-disk config path.
func Path() string {
	return configFile
}

// Load reads config.json or returns defaults when the file does not exist.
func Load() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return Normalize(&cfg)
}

// Normalize applies defaults and validates persisted values.
func Normalize(cfg *Config) (*Config, error) {
	if cfg == nil {
		return defaultConfig(), nil
	}
	out := *cfg
	out.DefaultProvider = session.NormalizeProvider(strings.TrimSpace(out.DefaultProvider))
	if out.DefaultProvider == "" {
		out.DefaultProvider = session.ProviderFrisco
	}
	if err := session.ValidateProvider(out.DefaultProvider); err != nil {
		return nil, err
	}
	if out.RateLimitBurst < 1 {
		out.RateLimitBurst = 1
	}
	if out.RateLimitRPS < 0 {
		out.RateLimitRPS = 0
	}
	return &out, nil
}

// Save persists the config file with 0600 permissions.
func Save(cfg *Config) error {
	norm, err := Normalize(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configFile), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(norm, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0o600)
}
