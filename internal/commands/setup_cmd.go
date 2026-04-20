package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure MCP server for AI clients (Claude, Cursor).",
	}
	cmd.AddCommand(newSetupAutoCmd(), newSetupClaudeCodeCmd(), newSetupClaudeDesktopCmd(), newSetupCursorCmd())
	return cmd
}

func newSetupAutoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auto",
		Short: "Auto-detect and configure all supported MCP clients.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			bin, err := martmartBinaryPath()
			if err != nil {
				return err
			}
			configured := 0

			if _, err := exec.LookPath("claude"); err == nil {
				if err := runClaudeCodeSetup(cmd, bin); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Claude Code: %v\n", err)
				} else {
					configured++
				}
			}

			if p := claudeDesktopConfigPath(); p != "" && dirExists(filepath.Dir(p)) {
				if err := writeMCPConfig(cmd, "Claude Desktop", p, bin); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Claude Desktop: %v\n", err)
				} else {
					configured++
				}
			}

			if p := cursorConfigPath(); p != "" && dirExists(filepath.Dir(p)) {
				if err := writeMCPConfig(cmd, "Cursor", p, bin); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Cursor: %v\n", err)
				} else {
					configured++
				}
			}

			if configured == 0 {
				return fmt.Errorf("no MCP clients detected. Use a specific subcommand: setup claude-code, setup claude-desktop, setup cursor")
			}
			return nil
		},
	}
}

func newSetupClaudeCodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claude-code",
		Short: "Configure MartMart MCP server for Claude Code.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			bin, err := martmartBinaryPath()
			if err != nil {
				return err
			}
			return runClaudeCodeSetup(cmd, bin)
		},
	}
}

func newSetupClaudeDesktopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claude-desktop",
		Short: "Configure MartMart MCP server for Claude Desktop.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			bin, err := martmartBinaryPath()
			if err != nil {
				return err
			}
			p := claudeDesktopConfigPath()
			if p == "" {
				return fmt.Errorf("claude Desktop config path not known for %s", runtime.GOOS)
			}
			return writeMCPConfig(cmd, "Claude Desktop", p, bin)
		},
	}
}

func newSetupCursorCmd() *cobra.Command {
	var global bool
	c := &cobra.Command{
		Use:   "cursor",
		Short: "Configure MartMart MCP server for Cursor.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			bin, err := martmartBinaryPath()
			if err != nil {
				return err
			}
			var p string
			if global {
				p = cursorConfigPath()
				if p == "" {
					return fmt.Errorf("cursor global config path not known for %s", runtime.GOOS)
				}
			} else {
				p = filepath.Join(".cursor", "mcp.json")
			}
			label := "Cursor"
			if global {
				label = "Cursor (global)"
			}
			return writeMCPConfig(cmd, label, p, bin)
		},
	}
	c.Flags().BoolVar(&global, "global", false, "Write to global ~/.cursor/mcp.json instead of project-level .cursor/mcp.json.")
	return c
}

func martmartBinaryPath() (string, error) {
	p, err := exec.LookPath("martmart")
	if err != nil {
		p, err = os.Executable()
		if err != nil {
			return "", fmt.Errorf("cannot determine martmart binary path: %w", err)
		}
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return p, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func claudeDesktopConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "Claude", "claude_desktop_config.json")
		}
		return ""
	default:
		return ""
	}
}

func cursorConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".cursor", "mcp.json")
}

func runClaudeCodeSetup(cmd *cobra.Command, bin string) error {
	c := exec.Command("claude", "mcp", "add", "martmart", "--", bin, "mcp")
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		return fmt.Errorf("claude mcp add failed: %w", err)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Claude Code: configured.")
	return nil
}

// writeMCPConfig reads an existing JSON config (or creates a new one), adds/updates
// the MartMart MCP server entry, and writes it back.
func writeMCPConfig(cmd *cobra.Command, label, path, bin string) error {
	config := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("cannot parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	servers, _ := config["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["martmart"] = map[string]any{
		"command": bin,
		"args":    []string{"mcp"},
	}
	config["mcpServers"] = servers

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: configured (%s).\n", label, path)
	return nil
}
