package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const codexMarker = "keepgoing/codex-notify.sh"

// CodexConfigPath returns ~/.codex/config.toml.
func CodexConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

// CodexNotifyScriptPath returns ~/.config/keepgoing/codex-notify.sh.
func CodexNotifyScriptPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "keepgoing", "codex-notify.sh")
}

func codexNotifyScript(prevCmd string) string {
	return fmt.Sprintf(`#!/bin/sh
# keepgoing: report Codex turn state to the daemon.
if [ -n "$1" ]; then
  type=$(printf '%%s' "$1" | /usr/bin/python3 -c 'import sys,json; print(json.load(sys.stdin).get("type",""))' 2>/dev/null)
  case "$type" in
    turn-ended)
      /usr/bin/curl -s -m 1 -X POST http://127.0.0.1:7777/_agent -d '{"tool":"codex","pid":'$PPID',"state":"idle"}'
      ;;
    turn-started)
      /usr/bin/curl -s -m 1 -X POST http://127.0.0.1:7777/_agent -d '{"tool":"codex","pid":'$PPID',"state":"working"}'
      ;;
  esac
fi
if [ -n "%s" ] && [ -x "%s" ]; then
  exec "%s" "$@"
fi
`, prevCmd, prevCmd, prevCmd)
}

// InstallCodex adds the keepgoing notify wrapper to Codex config.
func InstallCodex() error {
	cfgPath := CodexConfigPath()
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	backup := cfgPath + ".bak." + time.Now().Format("20060102T150405")
	if err := os.WriteFile(backup, b, 0o600); err != nil {
		return err
	}
	fmt.Println("backed up to", backup)

	prevCmd := parseNotifyCommand(string(b))
	scriptPath := CodexNotifyScriptPath()
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(scriptPath, []byte(codexNotifyScript(prevCmd)), 0o755); err != nil {
		return err
	}
	fmt.Println("wrote", scriptPath)

	newLine := fmt.Sprintf("notify = [\"%s\", \"turn-ended\"]", scriptPath)
	out := replaceNotifyLine(string(b), newLine)
	if err := os.WriteFile(cfgPath, []byte(out), 0o600); err != nil {
		return err
	}
	fmt.Println("wrote", cfgPath)
	fmt.Println(" ", newLine)
	if prevCmd != "" {
		fmt.Println("  chains previous notify:", prevCmd)
	}
	return nil
}

// UninstallCodex restores a prior notify command if we wrapped one.
func UninstallCodex() error {
	cfgPath := CodexConfigPath()
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	if !strings.Contains(string(b), codexMarker) {
		return nil
	}
	_ = os.Remove(CodexNotifyScriptPath())
	// cannot reliably restore prev without parsing script; leave notify removed or user restores from backup
	out := strings.ReplaceAll(string(b), fmt.Sprintf("notify = [\"%s\", \"turn-ended\"]", CodexNotifyScriptPath()), "")
	return os.WriteFile(cfgPath, []byte(out), 0o600)
}

// CodexStatus reports whether keepgoing notify is configured.
func CodexStatus() bool {
	b, err := os.ReadFile(CodexConfigPath())
	if err != nil {
		return false
	}
	return strings.Contains(string(b), codexMarker)
}

var notifyLine = regexp.MustCompile(`(?m)^notify\s*=\s*\[.*\]\s*$`)

func parseNotifyCommand(toml string) string {
	m := notifyLine.FindString(toml)
	if m == "" {
		return ""
	}
	inner := strings.TrimPrefix(m, "notify = ")
	inner = strings.TrimSpace(strings.Trim(inner, "[]"))
	parts := splitNotifyParts(inner)
	if len(parts) == 0 || strings.Contains(parts[0], codexMarker) {
		return ""
	}
	return parts[0]
}

func replaceNotifyLine(toml, newLine string) string {
	if notifyLine.MatchString(toml) {
		return notifyLine.ReplaceAllString(toml, newLine)
	}
	return strings.TrimRight(toml, "\n") + "\n\n" + newLine + "\n"
}

func splitNotifyParts(inner string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if c == ',' && !inQuote {
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		parts = append(parts, s)
	}
	return parts
}
