package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Config is the on-disk config.
type Config struct {
	HotspotSSID    string `json:"hotspot_ssid,omitempty"`
	Listen         string `json:"listen,omitempty"`
	Connect        string `json:"connect,omitempty"`
	AlwaysAwake    bool   `json:"always_awake,omitempty"`
	LidMode        bool   `json:"lid_mode,omitempty"`         // keep running with lid closed (needs sudoers rule)
	CmuxStatus     *bool  `json:"cmux_status,omitempty"`      // poll cmux for working/idle; default true when cmux exists
	ScreenOffAfter int    `json:"screen_off_after,omitempty"` // seconds idle before display off; 0 = disabled
	IdleSleepAfter int    `json:"idle_sleep_after,omitempty"` // minutes all-idle before sleep allowed; 0 = off
}

// Loaded is a config read from disk plus which keys were present in JSON.
type Loaded struct {
	Config     Config
	HasLidMode bool
}

// CmuxOn reports whether cmux status polling is enabled (default true when cmux exists).
func (c Config) CmuxOn() bool {
	if c.CmuxStatus != nil {
		return *c.CmuxStatus
	}
	_, err := exec.LookPath("cmux")
	return err == nil
}

// Path returns ~/.config/keepgoing/config.json.
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "keepgoing", "config.json")
}

func statePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "keepgoing", "state.json")
}

func daemonLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Logs", "keepgoing", "daemon.log")
}

// Load reads the config; a missing file yields the zero value.
func Load() (Config, error) {
	loaded, err := LoadDetailed()
	return loaded.Config, err
}

// LoadDetailed reads config and records whether lid_mode was present in JSON.
func LoadDetailed() (Loaded, error) {
	doc, err := loadDocument()
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{Config: doc.cfg, HasLidMode: doc.has("lid_mode")}, nil
}

type document struct {
	cfg    Config
	raw    map[string]json.RawMessage
	keys   map[string]bool
	exists bool
}

func (d document) has(key string) bool {
	return d.keys[key]
}

func knownKeys() []string {
	return []string{
		"hotspot_ssid", "listen", "connect", "always_awake", "lid_mode",
		"cmux_status", "screen_off_after", "idle_sleep_after",
	}
}

func loadDocument() (document, error) {
	p := Path()
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return document{raw: map[string]json.RawMessage{}}, nil
	}
	if err != nil {
		return document{}, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return document{}, fmt.Errorf("config file is empty")
	}
	raw, keys, err := parseObject(b)
	if err != nil {
		return document{}, err
	}
	var cfg Config
	if len(raw) > 0 {
		merged, err := json.Marshal(raw)
		if err != nil {
			return document{}, err
		}
		if err := json.Unmarshal(merged, &cfg); err != nil {
			return document{}, err
		}
	}
	return document{cfg: cfg, raw: raw, keys: keys, exists: true}, nil
}

func parseObject(b []byte) (map[string]json.RawMessage, map[string]bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err == nil {
		return raw, keySet(raw), nil
	}
	salvaged := salvageObject(string(b))
	if len(salvaged) == 0 {
		return nil, nil, fmt.Errorf("config unreadable: invalid JSON")
	}
	return salvaged, keySet(salvaged), nil
}

func keySet(raw map[string]json.RawMessage) map[string]bool {
	keys := make(map[string]bool, len(raw))
	for k := range raw {
		keys[k] = true
	}
	return keys
}

var salvagePatterns = []struct {
	key string
	re  *regexp.Regexp
}{
	{"lid_mode", regexp.MustCompile(`"lid_mode"\s*:\s*(true|false)`)},
	{"hotspot_ssid", regexp.MustCompile(`"hotspot_ssid"\s*:\s*"(?:([^"\\]*(?:\\.[^"\\]*)*)|([^"]*))`)},
	{"always_awake", regexp.MustCompile(`"always_awake"\s*:\s*(true|false)`)},
	{"screen_off_after", regexp.MustCompile(`"screen_off_after"\s*:\s*(-?\d+)`)},
	{"idle_sleep_after", regexp.MustCompile(`"idle_sleep_after"\s*:\s*(-?\d+)`)},
	{"listen", regexp.MustCompile(`"listen"\s*:\s*"(?:([^"\\]*(?:\\.[^"\\]*)*)|([^"]*))`)},
	{"connect", regexp.MustCompile(`"connect"\s*:\s*"(?:([^"\\]*(?:\\.[^"\\]*)*)|([^"]*))`)},
}

func salvageObject(s string) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for _, p := range salvagePatterns {
		m := p.re.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		switch p.key {
		case "lid_mode", "always_awake":
			out[p.key] = json.RawMessage(m[1])
		case "screen_off_after", "idle_sleep_after":
			out[p.key] = json.RawMessage(m[1])
		default:
			val := m[1]
			if val == "" && len(m) > 2 {
				val = m[2]
			}
			out[p.key] = json.RawMessage(`"` + strings.ReplaceAll(val, `"`, `\"`) + `"`)
		}
	}
	return out
}

// Update loads the current file, applies mutate, and writes atomically without dropping unknown keys.
func Update(mutate func(c *Config) error) error {
	doc, err := loadDocument()
	if err != nil {
		return err
	}
	if err := mutate(&doc.cfg); err != nil {
		return err
	}
	return doc.write()
}

// Save writes the config with 0600 via Update.
func Save(c Config) error {
	return Update(func(dst *Config) error {
		*dst = c
		return nil
	})
}

func (d *document) mergedMap() (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	for k, v := range d.raw {
		out[k] = v
	}
	cfgBytes, err := json.Marshal(d.cfg)
	if err != nil {
		return nil, err
	}
	var cfgMap map[string]json.RawMessage
	if err := json.Unmarshal(cfgBytes, &cfgMap); err != nil {
		return nil, err
	}
	for k, v := range cfgMap {
		out[k] = v
	}
	for _, k := range knownKeys() {
		if _, ok := cfgMap[k]; !ok {
			delete(out, k)
		}
	}
	return out, nil
}

func (d *document) write() error {
	merged, err := d.mergedMap()
	if err != nil {
		return err
	}
	if d.exists {
		existingTrim := strings.TrimSpace(string(mustMarshal(d.raw)))
		newTrim := strings.TrimSpace(string(mustMarshal(merged)))
		if existingTrim != "" && existingTrim != "{}" && newTrim == "{}" {
			return fmt.Errorf("refusing to overwrite config with empty settings")
		}
	}
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return err
	}
	fmt.Fprintf(os.Stderr, "[config] wrote %s (%s)\n", p, mapKeys(merged))
	return nil
}

func mustMarshal(m map[string]json.RawMessage) []byte {
	b, _ := json.Marshal(m)
	return b
}

func mapKeys(m map[string]json.RawMessage) string {
	if len(m) == 0 {
		return "none"
	}
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ", ")
}

// SaveState records the last known daemon lid mode for recovery.
func SaveState(lidMode bool) error {
	p := statePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(map[string]bool{"lid_mode": lidMode}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, p)
}

// PreviousLidMode returns the last known lid mode from state.json or daemon.log.
func PreviousLidMode() *bool {
	if b, err := os.ReadFile(statePath()); err == nil {
		var m map[string]bool
		if json.Unmarshal(b, &m) == nil {
			if v, ok := m["lid_mode"]; ok {
				return &v
			}
		}
	}
	if v := lidModeFromDaemonLog(); v != nil {
		return v
	}
	return nil
}

var daemonUpLine = regexp.MustCompile(`\[daemon\] up:.*\blid=(true|false)\b`)

func lidModeFromDaemonLog() *bool {
	b, err := os.ReadFile(daemonLogPath())
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		m := daemonUpLine.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		v := m[1] == "true"
		return &v
	}
	return nil
}

// RestoreMissingLidMode restores lid_mode=true when the key was dropped but prior state had it on.
func RestoreMissingLidMode(loaded Loaded) (Config, bool) {
	if loaded.HasLidMode {
		return loaded.Config, false
	}
	prev := PreviousLidMode()
	if prev == nil || !*prev {
		return loaded.Config, false
	}
	cfg := loaded.Config
	cfg.LidMode = true
	if err := Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[config] restore lid_mode: %v\n", err)
		return loaded.Config, false
	}
	fmt.Fprintf(os.Stderr, "[config] lid_mode missing — restoring true from previous state\n")
	return cfg, true
}
