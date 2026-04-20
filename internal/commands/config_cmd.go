package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/config"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
	"github.com/wydrox/martmart-cli/internal/tui"
)

func newConfigCmd() *cobra.Command {
	setCmd := newConfigSetCmd()
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure shared CLI settings (interactive by default) or show them with `show`.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if isInteractive(cmd.InOrStdin()) {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				updated, changed, err := tui.RunConfigEditor(cfg)
				if err != nil {
					return err
				}
				if changed {
					if err := config.Save(&updated); err != nil {
						return err
					}
				}
			}
			return runConfigShow()
		},
	}
	cmd.AddCommand(newConfigShowCmd(), setCmd)
	return cmd
}

func isInteractive(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show saved config and currently active provider/rate limit.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runConfigShow()
		},
	}
}

func runConfigShow() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rps, burst := httpclient.CurrentRateLimit()
	return printJSON(map[string]any{
		"config_file":      config.Path(),
		"default_provider": cfg.DefaultProvider,
		"saved_rate_limit": map[string]any{"rps": cfg.RateLimitRPS, "burst": cfg.RateLimitBurst},
		"openai": map[string]any{
			"api_key_set":         cfg.OpenAIAPIKey != "",
			"model":               cfg.OpenAIModel,
			"voice":               cfg.OpenAIVoice,
			"language":            cfg.OpenAILanguage,
			"transcription_model": cfg.OpenAITranscriptionModel,
			"voice_speed":         cfg.OpenAIVoiceSpeed,
			"input_device":        cfg.OpenAIInputDevice,
			"output_device":       cfg.OpenAIOutputDevice,
		},
		"active_provider":     session.CurrentProvider(),
		"active_rate_limit":   map[string]any{"rps": rps, "burst": burst},
		"supported_providers": session.SupportedProviders(),
	})
}

func newConfigSetCmd() *cobra.Command {
	var (
		defaultProvider          string
		rateLimitRPS             float64
		rateLimitBurst           int
		openAIAPIKey             string
		openAIModel              string
		openAIVoice              string
		openAILanguage           string
		openAITranscriptionModel string
		openAIVoiceSpeed         float64
		openAIInputDevice        int
		openAIOutputDevice       int
	)

	c := &cobra.Command{
		Use:   "set",
		Short: "Persist default provider/rate limits and voice assistant settings.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			changed := false

			if cmd.Flags().Changed("default-provider") {
				provider := session.NormalizeProvider(defaultProvider)
				if err := session.ValidateProvider(provider); err != nil {
					return err
				}
				cfg.DefaultProvider = provider
				changed = true
			}
			if cmd.Flags().Changed("rate-limit-rps") {
				if rateLimitRPS < 0 {
					return fmt.Errorf("--rate-limit-rps must be >= 0")
				}
				cfg.RateLimitRPS = rateLimitRPS
				changed = true
			}
			if cmd.Flags().Changed("rate-limit-burst") {
				if rateLimitBurst < 1 {
					return fmt.Errorf("--rate-limit-burst must be >= 1")
				}
				cfg.RateLimitBurst = rateLimitBurst
				changed = true
			}

			if cmd.Flags().Changed("openai-api-key") {
				cfg.OpenAIAPIKey = strings.TrimSpace(openAIAPIKey)
				changed = true
			}
			if cmd.Flags().Changed("openai-model") {
				cfg.OpenAIModel = strings.TrimSpace(openAIModel)
				changed = true
			}
			if cmd.Flags().Changed("openai-voice") {
				cfg.OpenAIVoice = strings.TrimSpace(openAIVoice)
				changed = true
			}
			if cmd.Flags().Changed("openai-language") {
				cfg.OpenAILanguage = strings.TrimSpace(openAILanguage)
				changed = true
			}
			if cmd.Flags().Changed("openai-transcription-model") {
				cfg.OpenAITranscriptionModel = strings.TrimSpace(openAITranscriptionModel)
				changed = true
			}
			if cmd.Flags().Changed("openai-voice-speed") {
				if openAIVoiceSpeed <= 0 {
					return fmt.Errorf("--openai-voice-speed must be > 0")
				}
				cfg.OpenAIVoiceSpeed = openAIVoiceSpeed
				changed = true
			}
			if cmd.Flags().Changed("openai-input-device") {
				cfg.OpenAIInputDevice = openAIInputDevice
				changed = true
			}
			if cmd.Flags().Changed("openai-output-device") {
				cfg.OpenAIOutputDevice = openAIOutputDevice
				changed = true
			}

			if !changed {
				if !isInteractive(cmd.InOrStdin()) {
					return errors.New("nothing to save: set at least one flag")
				}
				updated, uiChanged, err := tui.RunConfigEditor(cfg)
				if err != nil {
					return err
				}
				if uiChanged {
					cfg = &updated
					changed = true
				}
			}

			if !changed {
				return errors.New("nothing to save: set at least one value")
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			return runConfigShow()
		},
	}

	c.Flags().StringVar(&defaultProvider, "default-provider", "", "Default provider: frisco or delio.")
	c.Flags().Float64Var(&rateLimitRPS, "rate-limit-rps", 0, "Saved request rate in requests/second (0 = disabled).")
	c.Flags().IntVar(&rateLimitBurst, "rate-limit-burst", 1, "Saved request burst size.")
	c.Flags().StringVar(&openAIAPIKey, "openai-api-key", "", "OpenAI API key used by voice assistant.")
	c.Flags().StringVar(&openAIModel, "openai-model", "", "OpenAI Realtime model (default: gpt-realtime).")
	c.Flags().StringVar(&openAIVoice, "openai-voice", "", "OpenAI voice profile name for responses.")
	c.Flags().StringVar(&openAILanguage, "openai-language", "", "Default speech language code.")
	c.Flags().StringVar(&openAITranscriptionModel, "openai-transcription-model", "", "Transcription model.")
	c.Flags().Float64Var(&openAIVoiceSpeed, "openai-voice-speed", 0, "Speech playback speed (must be > 0).")
	c.Flags().IntVar(&openAIInputDevice, "openai-input-device", -1, "Default PyAudio input device index (-1 = default).")
	c.Flags().IntVar(&openAIOutputDevice, "openai-output-device", -1, "Default PyAudio output device index (-1 = default).")
	return c
}
