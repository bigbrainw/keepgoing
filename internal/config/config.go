package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	ScreenOffAfter int    `json:"screen_off_after,omitempty"` // seconds idle before display off; 0 = disabled
}

// Path returns ~/.config/keepgoing/config.json.
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "keepgoing", "config.json")
}

// Load reads the config; a missing file yields the zero value.
func Load() (Config, error) {
	var c Config
	b, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return c, fmt.Errorf("config file is empty")
	}
	return c, json.Unmarshal(b, &c)
}

// Save writes the config with 0600.
func Save(c Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if existing, err := os.ReadFile(p); err == nil {
		existingTrim := strings.TrimSpace(string(existing))
		newTrim := strings.TrimSpace(string(b))
		if existingTrim != "" && existingTrim != "{}" && newTrim == "{}" {
			return fmt.Errorf("refusing to overwrite config with empty settings")
		}
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[config] wrote %s (%s)\n", p, keys(c))
	return nil
}

func keys(c Config) string {
	raw, err := json.Marshal(c)
	if err != nil {
		return "?"
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return "none"
	}
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ", ")
}
