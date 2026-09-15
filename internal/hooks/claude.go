package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const claudeMarker = "127.0.0.1:7777/_agent"

// ClaudeSettingsPath returns ~/.claude/settings.json.
func ClaudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func claudeWorkingCmd() string {
	return `curl -s -m 1 -X POST http://127.0.0.1:7777/_agent -d '{"tool":"claude","pid":'$PPID',"state":"working"}'`
}

func claudeIdleCmd() string {
	return `curl -s -m 1 -X POST http://127.0.0.1:7777/_agent -d '{"tool":"claude","pid":'$PPID',"state":"idle"}'`
}

// InstallClaude merges keepgoing hooks into Claude Code settings.
func InstallClaude() error {
	path := ClaudeSettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var settings map[string]any
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		backup := path + ".bak." + time.Now().Format("20060102T150405")
		if err := os.WriteFile(backup, b, 0o600); err != nil {
			return err
		}
		fmt.Println("backed up to", backup)
	} else {
		settings = map[string]any{}
	}
	hooks := hookMap(settings)
	mergeEvent(hooks, "UserPromptSubmit", claudeWorkingCmd())
	mergeEvent(hooks, "PreToolUse", claudeWorkingCmd())
	mergeEvent(hooks, "Stop", claudeIdleCmd())
	mergeEvent(hooks, "Notification", claudeIdleCmd())
	settings["hooks"] = hooks
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Println("wrote", path)
	fmt.Println("  UserPromptSubmit, PreToolUse → working")
	fmt.Println("  Stop, Notification → idle")
	return nil
}

// UninstallClaude removes keepgoing hooks from Claude settings.
func UninstallClaude() error {
	path := ClaudeSettingsPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return err
	}
	hooks := hookMap(settings)
	for k, v := range hooks {
		hooks[k] = filterEntries(v)
	}
	settings["hooks"] = hooks
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o600)
}

// ClaudeStatus reports whether keepgoing hooks are present.
func ClaudeStatus() bool {
	path := ClaudeSettingsPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), claudeMarker)
}

func hookMap(settings map[string]any) map[string]any {
	h, _ := settings["hooks"].(map[string]any)
	if h == nil {
		h = map[string]any{}
	}
	return h
}

func mergeEvent(hooks map[string]any, event, command string) {
	if hasKeepgoing(hooks[event]) {
		return
	}
	var entries []any
	if raw, ok := hooks[event].([]any); ok {
		entries = raw
	}
	entries = append(entries, map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	})
	hooks[event] = entries
}

func hasKeepgoing(v any) bool {
	b, _ := json.Marshal(v)
	return strings.Contains(string(b), claudeMarker)
}

func filterEntries(v any) any {
	entries, ok := v.([]any)
	if !ok {
		return v
	}
	var out []any
	for _, e := range entries {
		if hasKeepgoing(e) {
			continue
		}
		out = append(out, e)
	}
	return out
}
