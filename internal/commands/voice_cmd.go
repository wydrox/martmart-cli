package commands

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/config"
	"github.com/wydrox/martmart-cli/internal/session"
)

//go:embed voice_assets/*
var voiceAssets embed.FS

const (
	voiceScriptAsset       = "voice_assets/pipecat_shopper.py"
	voiceRequirementsAsset = "voice_assets/requirements.txt"
)

type voiceRuntimePaths struct {
	RootDir          string
	ScriptPath       string
	RequirementsPath string
	VenvDir          string
	PythonPath       string
}

func newVoiceCmd() *cobra.Command {
	runCmd := newVoiceRunCmd()
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Run a local Pipecat + OpenAI voice shopping assistant.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmd.RunE(cmd, args)
		},
	}
	cmd.Flags().AddFlagSet(runCmd.Flags())
	cmd.AddCommand(newVoiceSetupCmd(), runCmd)
	return cmd
}

func newVoiceSetupCmd() *cobra.Command {
	var (
		pythonPath string
		reinstall  bool
	)
	c := &cobra.Command{
		Use:   "setup",
		Short: "Install the local Python runtime for the voice assistant.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := resolveVoiceRuntimePaths()
			if err != nil {
				return err
			}
			if err := writeVoiceAssets(paths); err != nil {
				return err
			}
			python, err := resolveVoicePython(pythonPath)
			if err != nil {
				return err
			}
			if err := installVoiceRuntime(cmd, paths, python, reinstall); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Voice runtime ready in %s\n", paths.RootDir)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Set OpenAI key with: martmart config set --openai-api-key <key>")
			return nil
		},
	}
	c.Flags().StringVar(&pythonPath, "python", "", "Python interpreter to use for setup (defaults to python3.12/python3).")
	c.Flags().BoolVar(&reinstall, "reinstall", false, "Remove and recreate the voice virtualenv before installing dependencies.")
	return c
}

func newVoiceRunCmd() *cobra.Command {
	var (
		pythonPath         string
		model              string
		voice              string
		language           string
		transcriptionModel string
		voiceSpeed         float64
		inputDevice        int
		outputDevice       int
		debug              bool
		showLogs           bool
	)
	c := &cobra.Command{
		Use:   "run",
		Short: "Start the local voice shopping assistant.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			apiKey := strings.TrimSpace(cfg.OpenAIAPIKey)
			if apiKey == "" {
				apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
			}
			if apiKey == "" {
				return errors.New("openai api key is not configured; use `martmart config set --openai-api-key` or set OPENAI_API_KEY")
			}

			resolvedModel := strings.TrimSpace(model)
			if resolvedModel == "" {
				resolvedModel = strings.TrimSpace(cfg.OpenAIModel)
			}
			if resolvedModel == "" {
				resolvedModel = "gpt-realtime"
			}

			resolvedVoice := strings.TrimSpace(voice)
			if resolvedVoice == "" {
				resolvedVoice = strings.TrimSpace(cfg.OpenAIVoice)
			}
			if resolvedVoice == "" {
				resolvedVoice = "alloy"
			}

			resolvedLanguage := strings.TrimSpace(language)
			if resolvedLanguage == "" {
				resolvedLanguage = strings.TrimSpace(cfg.OpenAILanguage)
			}
			if resolvedLanguage == "" {
				resolvedLanguage = "pl"
			}

			resolvedTranscriptionModel := strings.TrimSpace(transcriptionModel)
			if resolvedTranscriptionModel == "" {
				resolvedTranscriptionModel = strings.TrimSpace(cfg.OpenAITranscriptionModel)
			}
			if resolvedTranscriptionModel == "" {
				resolvedTranscriptionModel = "gpt-4o-transcribe"
			}

			resolvedVoiceSpeed := voiceSpeed
			if !cmd.Flags().Changed("voice-speed") {
				resolvedVoiceSpeed = cfg.OpenAIVoiceSpeed
			}
			if resolvedVoiceSpeed <= 0 {
				resolvedVoiceSpeed = 1.0
			}
			if !cmd.Flags().Changed("input-device") {
				inputDevice = cfg.OpenAIInputDevice
			}
			if !cmd.Flags().Changed("output-device") {
				outputDevice = cfg.OpenAIOutputDevice
			}

			paths, err := resolveVoiceRuntimePaths()
			if err != nil {
				return err
			}
			if err := writeVoiceAssets(paths); err != nil {
				return err
			}
			if _, err := os.Stat(paths.PythonPath); err != nil {
				python, rerr := resolveVoicePython(pythonPath)
				if rerr != nil {
					return fmt.Errorf("voice runtime not installed and no suitable python found: %w", rerr)
				}
				if err := installVoiceRuntime(cmd, paths, python, false); err != nil {
					return err
				}
			}

			bin, err := martmartBinaryPath()
			if err != nil {
				return err
			}
			provider := session.CurrentProvider()
			mcpArgs := []string{"--provider", provider, "mcp"}

			args := []string{
				paths.ScriptPath,
				"--martmart-binary", bin,
				"--model", resolvedModel,
				"--voice", resolvedVoice,
				"--transcription-model", resolvedTranscriptionModel,
				"--voice-speed", strconv.FormatFloat(resolvedVoiceSpeed, 'f', -1, 64),
				"--language", resolvedLanguage,
			}
			if inputDevice >= 0 {
				args = append(args, "--input-device", strconv.Itoa(inputDevice))
			}
			if outputDevice >= 0 {
				args = append(args, "--output-device", strconv.Itoa(outputDevice))
			}
			if debug {
				args = append(args, "--debug")
			}
			if showLogs {
				args = append(args, "--show-logs")
			}
			args = append(args, "--")
			args = append(args, mcpArgs...)

			proc := exec.Command(paths.PythonPath, args...)
			proc.Stdin = cmd.InOrStdin()
			proc.Stdout = cmd.OutOrStdout()
			proc.Stderr = cmd.ErrOrStderr()
			proc.Env = append(os.Environ(), "PYTHONUNBUFFERED=1", "OPENAI_API_KEY="+apiKey)
			return proc.Run()
		},
	}
	c.Flags().StringVar(&pythonPath, "python", "", "Python interpreter to use when auto-installing the voice runtime.")
	c.Flags().StringVar(&model, "model", "", "Realtime OpenAI model for the voice assistant.")
	c.Flags().StringVar(&voice, "voice", "", "OpenAI voice name for spoken responses.")
	c.Flags().StringVar(&language, "language", "", "Primary language code for speech transcription.")
	c.Flags().StringVar(&transcriptionModel, "transcription-model", "", "Transcription model used for input ASR.")
	c.Flags().Float64Var(&voiceSpeed, "voice-speed", 0, "Speech playback speed for assistant responses.")
	c.Flags().IntVar(&inputDevice, "input-device", -2, "PyAudio input device index (-1 = default).")
	c.Flags().IntVar(&outputDevice, "output-device", -2, "PyAudio output device index (-1 = default).")
	c.Flags().BoolVar(&debug, "debug", false, "Enable verbose Pipecat logging and low-level debug traces.")
	c.Flags().BoolVar(&showLogs, "show-logs", false, "Show voice session logs, especially MCP tool calls and results.")
	return c
}

func resolveVoiceRuntimePaths() (voiceRuntimePaths, error) {
	home, err := os.UserHomeDir()
	if strings.TrimSpace(home) == "" {
		if err == nil {
			return voiceRuntimePaths{}, fmt.Errorf("cannot determine user home directory")
		}
		return voiceRuntimePaths{}, fmt.Errorf("cannot determine user home directory: %w", err)
	}
	root := filepath.Join(home, ".martmart-cli", "voice")
	venv := filepath.Join(root, "venv")
	pythonPath := filepath.Join(venv, "bin", "python")
	if runtime.GOOS == "windows" {
		pythonPath = filepath.Join(venv, "Scripts", "python.exe")
	}
	return voiceRuntimePaths{
		RootDir:          root,
		ScriptPath:       filepath.Join(root, "pipecat_shopper.py"),
		RequirementsPath: filepath.Join(root, "requirements.txt"),
		VenvDir:          venv,
		PythonPath:       pythonPath,
	}, nil
}

func writeVoiceAssets(paths voiceRuntimePaths) error {
	if err := os.MkdirAll(paths.RootDir, 0o700); err != nil {
		return err
	}
	if err := writeVoiceAsset(paths.ScriptPath, voiceScriptAsset, 0o600); err != nil {
		return err
	}
	if err := writeVoiceAsset(paths.RequirementsPath, voiceRequirementsAsset, 0o600); err != nil {
		return err
	}
	return nil
}

func writeVoiceAsset(dstPath, assetPath string, mode fs.FileMode) error {
	data, err := voiceAssets.ReadFile(assetPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, mode)
}

func resolveVoicePython(explicit string) (string, error) {
	candidates := []string{}
	if s := strings.TrimSpace(explicit); s != "" {
		candidates = append(candidates, s)
	}
	candidates = append(candidates, "python3.12", "python3.11", "python3.10", "python3", "python")

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok || strings.TrimSpace(candidate) == "" {
			continue
		}
		seen[candidate] = struct{}{}
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		if ok, err := pythonMeetsVersion(path, 3, 10); err == nil && ok {
			return path, nil
		}
	}
	return "", errors.New("could not find Python >= 3.10 (tried python3.12, python3.11, python3.10, python3, python)")
}

func pythonMeetsVersion(path string, wantMajor, wantMinor int) (bool, error) {
	cmd := exec.Command(path, "-c", "import sys; print(f'{sys.version_info[0]}.{sys.version_info[1]}')")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	version := strings.TrimSpace(string(out))
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false, fmt.Errorf("unexpected python version output: %q", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, err
	}
	if major > wantMajor {
		return true, nil
	}
	if major == wantMajor && minor >= wantMinor {
		return true, nil
	}
	return false, nil
}

func installVoiceRuntime(cmd *cobra.Command, paths voiceRuntimePaths, python string, reinstall bool) error {
	if reinstall {
		if err := os.RemoveAll(paths.VenvDir); err != nil {
			return err
		}
	}
	if _, err := os.Stat(paths.PythonPath); err != nil {
		if err := runStreamingCommand(cmd, python, "-m", "venv", paths.VenvDir); err != nil {
			return fmt.Errorf("create voice virtualenv: %w", err)
		}
	}
	if err := runStreamingCommand(cmd, paths.PythonPath, "-m", "pip", "install", "--upgrade", "pip"); err != nil {
		return fmt.Errorf("upgrade pip in voice virtualenv: %w", err)
	}
	if err := runStreamingCommand(cmd, paths.PythonPath, "-m", "pip", "install", "-r", paths.RequirementsPath); err != nil {
		msg := "install Python dependencies for voice runtime"
		if runtime.GOOS == "darwin" {
			msg += " (on macOS, install portaudio first with `brew install portaudio` if PyAudio fails)"
		}
		return fmt.Errorf("%s: %w", msg, err)
	}
	return nil
}

func runStreamingCommand(cmd *cobra.Command, name string, args ...string) error {
	proc := exec.Command(name, args...)
	proc.Stdin = cmd.InOrStdin()
	proc.Stdout = cmd.OutOrStdout()
	proc.Stderr = cmd.ErrOrStderr()
	return proc.Run()
}
