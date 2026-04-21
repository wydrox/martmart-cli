package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config stores shared CLI settings that apply across providers.
type Config struct {
	RateLimitRPS             float64 `json:"rate_limit_rps"`
	RateLimitBurst           int     `json:"rate_limit_burst"`
	OpenAIAPIKey             string  `json:"openai_api_key"`
	OpenAIModel              string  `json:"openai_model"`
	OpenAIVoice              string  `json:"openai_voice"`
	OpenAILanguage           string  `json:"openai_language"`
	OpenAITranscriptionModel string  `json:"openai_transcription_model"`
	OpenAIVoiceSpeed         float64 `json:"openai_voice_speed"`
	OpenAIInputDevice        int     `json:"openai_input_device"`
	OpenAIOutputDevice       int     `json:"openai_output_device"`
}

var (
	configFile       string
	legacyConfigFile string
)

func init() {
	home, _ := os.UserHomeDir()
	configFile = filepath.Join(home, ".martmart-cli", "config.json")
	legacyConfigFile = filepath.Join(home, ".frisco-cli", "config.json")
}

func defaultConfig() *Config {
	return &Config{
		RateLimitRPS:             0,
		RateLimitBurst:           1,
		OpenAIModel:              "gpt-realtime",
		OpenAIVoice:              "alloy",
		OpenAILanguage:           "pl",
		OpenAITranscriptionModel: "gpt-4o-transcribe",
		OpenAIVoiceSpeed:         1.0,
		OpenAIInputDevice:        -1,
		OpenAIOutputDevice:       -1,
	}
}

// Path returns the on-disk config path.
func Path() string {
	return configFile
}

// Load reads config.json or returns defaults when the file does not exist.
func Load() (*Config, error) {
	for _, path := range []string{configFile, legacyConfigFile} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		return Normalize(&cfg)
	}
	return defaultConfig(), nil
}

// Normalize applies defaults and validates persisted values.
func Normalize(cfg *Config) (*Config, error) {
	if cfg == nil {
		return defaultConfig(), nil
	}
	out := *cfg
	if out.RateLimitBurst < 1 {
		out.RateLimitBurst = 1
	}
	if out.RateLimitRPS < 0 {
		out.RateLimitRPS = 0
	}

	if strings.TrimSpace(out.OpenAIModel) == "" {
		out.OpenAIModel = "gpt-realtime"
	}
	if strings.TrimSpace(out.OpenAIVoice) == "" {
		out.OpenAIVoice = "alloy"
	}
	if strings.TrimSpace(out.OpenAILanguage) == "" {
		out.OpenAILanguage = "pl"
	}
	if strings.TrimSpace(out.OpenAITranscriptionModel) == "" {
		out.OpenAITranscriptionModel = "gpt-4o-transcribe"
	}
	if out.OpenAIVoiceSpeed <= 0 {
		out.OpenAIVoiceSpeed = 1.0
	}
	if out.OpenAIInputDevice < -1 {
		out.OpenAIInputDevice = -1
	}
	if out.OpenAIOutputDevice < -1 {
		out.OpenAIOutputDevice = -1
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
