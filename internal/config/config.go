// Package config persists the small amount of user configuration keepgoing
// needs (hotspot SSID; the password lives in the macOS Keychain).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is the on-disk config.
type Config struct {
	HotspotSSID string `json:"hotspot_ssid,omitempty"`
	Listen      string `json:"listen,omitempty"`
	Connect     string `json:"connect,omitempty"`
	AlwaysAwake    bool `json:"always_awake,omitempty"`
	LidMode        bool `json:"lid_mode,omitempty"`         // keep running with lid closed (needs sudoers rule)
	ScreenOffAfter int  `json:"screen_off_after,omitempty"` // seconds idle before display off; 0 = disabled
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
	return c, json.Unmarshal(b, &c)
}

// Save writes the config with 0600.
func Save(c Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, b, 0o600)
}
