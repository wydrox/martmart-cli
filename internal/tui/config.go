package tui

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/wydrox/martmart-cli/internal/config"
)

const (
	configActionSave   = "save"
	configActionCancel = "cancel"
)

type configFieldKind string

const (
	configKindString configFieldKind = "string"
	configKindFloat  configFieldKind = "float"
	configKindInt    configFieldKind = "int"
)

type configEditField struct {
	Key       string
	Label     string
	Value     string
	Kind      configFieldKind
	Sensitive bool
	Help      string
}

type configEditModel struct {
	base          config.Config
	fields        []configEditField
	cursor        int
	editing       bool
	editIndex     int
	editBuffer    string
	message       string
	action        string
	savedConfig   config.Config
	changed       bool
	editingBuffer bool
}

// RunConfigEditor opens an interactive TUI for editing CLI config and returns the
// resulting config, a changed flag, and any validation error.
func RunConfigEditor(cfg *config.Config) (config.Config, bool, error) {
	base := config.Config{}
	if cfg != nil {
		base = *cfg
	}
	p := tea.NewProgram(initialConfigEditModel(base))
	res, err := p.Run()
	if err != nil {
		return config.Config{}, false, err
	}
	model, ok := res.(configEditModel)
	if !ok {
		return config.Config{}, false, errors.New("unexpected TUI model state")
	}
	if model.action != configActionSave {
		return base, false, nil
	}
	return model.savedConfig, model.changed, nil
}

func initialConfigEditModel(base config.Config) configEditModel {
	return configEditModel{
		base: base,
		fields: []configEditField{
			{
				Key:   "rate_limit_rps",
				Label: "Rate limit RPS",
				Help:  "0 = disabled",
				Kind:  configKindFloat,
				Value: fmt.Sprintf("%.3g", base.RateLimitRPS),
			},
			{
				Key:   "rate_limit_burst",
				Label: "Rate limit burst",
				Kind:  configKindInt,
				Value: fmt.Sprintf("%d", base.RateLimitBurst),
			},
			{
				Key:       "openai_api_key",
				Label:     "OpenAI API key",
				Help:      "Leave empty to keep current value",
				Kind:      configKindString,
				Sensitive: true,
				Value:     base.OpenAIAPIKey,
			},
			{
				Key:   "openai_model",
				Label: "OpenAI model",
				Kind:  configKindString,
				Value: base.OpenAIModel,
			},
			{
				Key:   "openai_voice",
				Label: "OpenAI voice",
				Kind:  configKindString,
				Value: base.OpenAIVoice,
				Help:  "Example: alloy, ash, verse...",
			},
			{
				Key:   "openai_language",
				Label: "OpenAI language",
				Kind:  configKindString,
				Value: base.OpenAILanguage,
			},
			{
				Key:   "openai_transcription_model",
				Label: "Transcription model",
				Kind:  configKindString,
				Value: base.OpenAITranscriptionModel,
			},
			{
				Key:   "openai_voice_speed",
				Label: "Voice speed",
				Help:  "Must be > 0",
				Kind:  configKindFloat,
				Value: fmt.Sprintf("%.3g", base.OpenAIVoiceSpeed),
			},
			{
				Key:   "openai_input_device",
				Label: "Input device index",
				Kind:  configKindInt,
				Help:  "-1 = default",
				Value: fmt.Sprintf("%d", base.OpenAIInputDevice),
			},
			{
				Key:   "openai_output_device",
				Label: "Output device index",
				Kind:  configKindInt,
				Help:  "-1 = default",
				Value: fmt.Sprintf("%d", base.OpenAIOutputDevice),
			},
		},
		cursor:    0,
		editIndex: -1,
	}
}

func (m configEditModel) Init() tea.Cmd {
	return nil
}

func (m configEditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editing {
			return m.updateEditing(msg)
		}
		return m.updateNavigation(msg)
	}
	return m, nil
}

func (m configEditModel) View() string {
	var b strings.Builder
	b.WriteString("MartMart config (TUI)\n")
	b.WriteString("\nUse ↑/↓ to choose, Enter to edit, S to save, Q to cancel.\n\n")

	for i, f := range m.fields {
		prefix := "  "
		if m.cursor == i {
			prefix = "> "
		}
		val := displayConfigFieldValue(f, m.cursor == i && m.editing, m.editBuffer)
		if f.Help != "" {
			_, _ = fmt.Fprintf(&b, "%s%s: %s  // %s\n", prefix, f.Label, val, f.Help)
			continue
		}
		_, _ = fmt.Fprintf(&b, "%s%s: %s\n", prefix, f.Label, val)
	}

	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(m.message)
		b.WriteByte('\n')
	}
	if m.editing {
		b.WriteString("\nEditing: type value and press Enter to save, Esc to cancel\n")
	}
	return b.String()
}

func (m configEditModel) saveIfValid() (tea.Model, tea.Cmd) {
	updated, err := configFromFields(m.base, m.fields)
	if err != nil {
		m.message = err.Error()
		return m, nil
	}
	m.savedConfig = updated
	m.changed = !configEquals(m.base, updated)
	m.message = ""
	m.action = configActionSave
	return m, tea.Quit
}

func (m configEditModel) startEditing(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.fields) {
		return m, nil
	}
	m.editing = true
	m.editIndex = idx
	m.editBuffer = m.fields[idx].Value
	if m.fields[idx].Sensitive {
		m.editBuffer = ""
	}
	m.message = ""
	return m, nil
}

func (m configEditModel) finishEditing() (tea.Model, tea.Cmd) {
	if m.editIndex < 0 || m.editIndex >= len(m.fields) {
		m.editing = false
		m.editIndex = -1
		m.editBuffer = ""
		return m, nil
	}

	field := m.fields[m.editIndex]
	incoming := strings.TrimSpace(m.editBuffer)

	if field.Sensitive {
		if incoming != "" {
			field.Value = incoming
			m.fields[m.editIndex] = field
		}
	} else {
		field.Value = incoming
		m.fields[m.editIndex] = field
	}

	m.editing = false
	m.editIndex = -1
	m.editBuffer = ""
	m.message = ""
	return m, nil
}

func (m configEditModel) cancelEditing() (tea.Model, tea.Cmd) {
	m.editing = false
	m.editIndex = -1
	m.editBuffer = ""
	m.message = ""
	return m, nil
}

func (m configEditModel) updateNavigation(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyCtrlC:
		m.action = configActionCancel
		return m, tea.Quit
	case tea.KeyRunes:
		switch strings.ToLower(string(key.Runes)) {
		case "q", "Q":
			m.action = configActionCancel
			return m, tea.Quit
		case "s", "S":
			return m.saveIfValid()
		}
	case tea.KeyDown, tea.KeyCtrlN, tea.KeyTab:
		if m.cursor < len(m.fields)-1 {
			m.cursor++
		}
	case tea.KeyUp, tea.KeyCtrlP, tea.KeyShiftTab:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyEnter, tea.KeyRight, tea.KeyCtrlE:
		return m.startEditing(m.cursor)
	}
	return m, nil
}

func (m configEditModel) updateEditing(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		return m.cancelEditing()
	case tea.KeyCtrlC:
		m.action = configActionCancel
		return m, tea.Quit
	case tea.KeyEnter:
		return m.finishEditing()
	case tea.KeyBackspace:
		if len(m.editBuffer) > 0 {
			m.editBuffer = m.editBuffer[:len(m.editBuffer)-1]
		}
	case tea.KeyDelete:
		if len(m.editBuffer) > 0 {
			m.editBuffer = m.editBuffer[:len(m.editBuffer)-1]
		}
	case tea.KeyCtrlU:
		m.editBuffer = ""
	default:
		if key.Type == tea.KeyRunes {
			for _, r := range key.Runes {
				if r == '\n' || r == '\r' {
					continue
				}
				if r == '\x7f' {
					continue
				}
				m.editBuffer += string(r)
			}
		}
	}
	return m, nil
}

func displayConfigFieldValue(field configEditField, editing bool, editBuffer string) string {
	if editing {
		if field.Sensitive {
			if editBuffer == "" {
				return "(empty)"
			}
			return strings.Repeat("*", len(editBuffer))
		}
		if editBuffer == "" {
			return "(empty)"
		}
		return editBuffer
	}
	if field.Sensitive {
		if field.Value == "" {
			return "(not set)"
		}
		return "(set)"
	}
	if strings.TrimSpace(field.Value) == "" {
		return "(empty)"
	}
	return field.Value
}

func configFromFields(base config.Config, fields []configEditField) (config.Config, error) {
	updated := base
	for _, f := range fields {
		switch f.Key {
		case "rate_limit_rps":
			v, err := parseFloatField(f.Value, base.RateLimitRPS)
			if err != nil {
				return config.Config{}, fmt.Errorf("rate_limit_rps: %w", err)
			}
			if v < 0 {
				return config.Config{}, fmt.Errorf("rate_limit_rps must be >= 0")
			}
			updated.RateLimitRPS = v
		case "rate_limit_burst":
			v, err := parseIntField(f.Value, base.RateLimitBurst)
			if err != nil {
				return config.Config{}, fmt.Errorf("rate_limit_burst: %w", err)
			}
			if v < 1 {
				return config.Config{}, fmt.Errorf("rate_limit_burst must be >= 1")
			}
			updated.RateLimitBurst = v
		case "openai_api_key":
			if strings.TrimSpace(f.Value) != "" {
				updated.OpenAIAPIKey = strings.TrimSpace(f.Value)
			}
		case "openai_model":
			updated.OpenAIModel = strings.TrimSpace(f.Value)
		case "openai_voice":
			updated.OpenAIVoice = strings.TrimSpace(f.Value)
		case "openai_language":
			updated.OpenAILanguage = strings.TrimSpace(f.Value)
		case "openai_transcription_model":
			updated.OpenAITranscriptionModel = strings.TrimSpace(f.Value)
		case "openai_voice_speed":
			v, err := parseFloatField(f.Value, base.OpenAIVoiceSpeed)
			if err != nil {
				return config.Config{}, fmt.Errorf("openai_voice_speed: %w", err)
			}
			if v <= 0 {
				return config.Config{}, fmt.Errorf("openai_voice_speed must be > 0")
			}
			updated.OpenAIVoiceSpeed = v
		case "openai_input_device":
			v, err := parseIntField(f.Value, base.OpenAIInputDevice)
			if err != nil {
				return config.Config{}, fmt.Errorf("openai_input_device: %w", err)
			}
			if v < -1 {
				return config.Config{}, fmt.Errorf("openai_input_device must be >= -1")
			}
			updated.OpenAIInputDevice = v
		case "openai_output_device":
			v, err := parseIntField(f.Value, base.OpenAIOutputDevice)
			if err != nil {
				return config.Config{}, fmt.Errorf("openai_output_device: %w", err)
			}
			if v < -1 {
				return config.Config{}, fmt.Errorf("openai_output_device must be >= -1")
			}
			updated.OpenAIOutputDevice = v
		default:
			return config.Config{}, fmt.Errorf("unknown config key %q", f.Key)
		}
	}
	return updated, nil
}

func parseFloatField(raw string, current float64) (float64, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return current, nil
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("must be numeric")
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("must be finite number")
	}
	return parsed, nil
}

func parseIntField(raw string, current int) (int, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return current, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("must be integer")
	}
	return parsed, nil
}

func configEquals(a, b config.Config) bool {
	return a.RateLimitRPS == b.RateLimitRPS &&
		a.RateLimitBurst == b.RateLimitBurst &&
		a.OpenAIAPIKey == b.OpenAIAPIKey &&
		a.OpenAIModel == b.OpenAIModel &&
		a.OpenAIVoice == b.OpenAIVoice &&
		a.OpenAILanguage == b.OpenAILanguage &&
		a.OpenAITranscriptionModel == b.OpenAITranscriptionModel &&
		a.OpenAIVoiceSpeed == b.OpenAIVoiceSpeed &&
		a.OpenAIInputDevice == b.OpenAIInputDevice &&
		a.OpenAIOutputDevice == b.OpenAIOutputDevice
}
