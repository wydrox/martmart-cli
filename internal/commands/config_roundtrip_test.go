package commands

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wydrox/martmart-cli/internal/config"
)

// configFieldFlagMap lists every exported field on config.Config together with
// the CLI flag that must exist on `martmart config set` to mutate it. Adding a
// new field to config.Config without updating this map (or the CLI/TUI wiring
// it points at) will fail TestConfigFieldsCovered, TestConfigStructHasCLISetterFlag,
// or TestConfigStructIsReferencedByTUI.
var configFieldFlagMap = map[string]string{
	"DefaultProvider":          "default-provider",
	"RateLimitRPS":             "rate-limit-rps",
	"RateLimitBurst":           "rate-limit-burst",
	"OpenAIAPIKey":             "openai-api-key",
	"OpenAIModel":              "openai-model",
	"OpenAIVoice":              "openai-voice",
	"OpenAILanguage":           "openai-language",
	"OpenAITranscriptionModel": "openai-transcription-model",
	"OpenAIVoiceSpeed":         "openai-voice-speed",
	"OpenAIInputDevice":        "openai-input-device",
	"OpenAIOutputDevice":       "openai-output-device",
}

// tuiConfigSourcePath is relative to the commands package directory; `go test`
// runs with CWD set to the package under test.
const tuiConfigSourcePath = "../tui/config.go"

func configStructFieldNames(t *testing.T) []string {
	t.Helper()
	typ := reflect.TypeOf(config.Config{})
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		names = append(names, f.Name)
	}
	return names
}

// TestConfigFieldsCovered enforces set equality between exported config.Config
// fields and configFieldFlagMap. Adding a field to Config without updating the
// map (or leaving stale map entries after a field removal) fails here first.
func TestConfigFieldsCovered(t *testing.T) {
	structFields := configStructFieldNames(t)
	mapFields := make([]string, 0, len(configFieldFlagMap))
	for k := range configFieldFlagMap {
		mapFields = append(mapFields, k)
	}
	sort.Strings(structFields)
	sort.Strings(mapFields)

	if !reflect.DeepEqual(structFields, mapFields) {
		t.Fatalf("config.Config exported fields and configFieldFlagMap are out of sync.\n  struct fields: %v\n  map fields:    %v\nUpdate configFieldFlagMap (and the CLI/TUI wiring) to match.", structFields, mapFields)
	}
}

// TestConfigStructHasCLISetterFlag verifies every config.Config field has a
// matching flag registered on `martmart config set`. Catches fields added to
// Config but not to newConfigSetCmd.
func TestConfigStructHasCLISetterFlag(t *testing.T) {
	cmd := newConfigSetCmd()
	for _, fieldName := range configStructFieldNames(t) {
		flagName, ok := configFieldFlagMap[fieldName]
		if !ok {
			// TestConfigFieldsCovered will flag this; skip here to keep the
			// failure focused on CLI wiring.
			continue
		}
		if cmd.Flag(flagName) == nil {
			t.Errorf("config.Config field %q has no corresponding --%s flag on `config set`", fieldName, flagName)
		}
	}
}

// TestConfigStructIsReferencedByTUI verifies every config.Config field is
// mentioned in internal/tui/config.go source, so fields can be edited through
// the interactive editor. This is a heuristic (substring match) but reliably
// catches fields added to Config but not plumbed into the TUI editor.
func TestConfigStructIsReferencedByTUI(t *testing.T) {
	raw, err := os.ReadFile(tuiConfigSourcePath)
	if err != nil {
		t.Fatalf("reading %s: %v", tuiConfigSourcePath, err)
	}
	src := string(raw)
	for _, fieldName := range configStructFieldNames(t) {
		if !strings.Contains(src, fieldName) {
			t.Errorf("config.Config field %q is not referenced in %s; TUI editor is missing this field", fieldName, tuiConfigSourcePath)
		}
	}
}
