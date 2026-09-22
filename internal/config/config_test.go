package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConfigPath(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(home, ".config", "keepgoing", "config.json")
}

func readConfig(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHotspotSetPreservesLidMode(t *testing.T) {
	p := testConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"lid_mode":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Update(func(c *Config) error {
		c.HotspotSSID = "Oh yeah"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	b := readConfig(t)
	if !strings.Contains(b, `"lid_mode": true`) {
		t.Fatalf("lid_mode dropped: %s", b)
	}
	if !strings.Contains(b, `"hotspot_ssid": "Oh yeah"`) {
		t.Fatalf("hotspot missing: %s", b)
	}
}

func TestLidSetPreservesHotspot(t *testing.T) {
	p := testConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"hotspot_ssid":"Oh yeah"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Update(func(c *Config) error {
		c.LidMode = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	b := readConfig(t)
	if !strings.Contains(b, `"lid_mode": true`) {
		t.Fatalf("lid_mode missing: %s", b)
	}
	if !strings.Contains(b, `"hotspot_ssid": "Oh yeah"`) {
		t.Fatalf("hotspot dropped: %s", b)
	}
}

func TestUpdateKeepsUnknownKeys(t *testing.T) {
	p := testConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"lid_mode":true,"future_flag":"keep"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Update(func(c *Config) error {
		c.HotspotSSID = "ssid"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	b := readConfig(t)
	if !strings.Contains(b, `"future_flag": "keep"`) {
		t.Fatalf("unknown key dropped: %s", b)
	}
}

func TestSalvagePartialJSON(t *testing.T) {
	p := testConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"lid_mode":true,"hotspot_ssid":"Oh yeah`), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadDetailed()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Config.LidMode {
		t.Fatal("expected salvaged lid_mode=true")
	}
	if loaded.Config.HotspotSSID != "Oh yeah" {
		t.Fatalf("expected salvaged hotspot, got %q", loaded.Config.HotspotSSID)
	}

	if err := Update(func(c *Config) error {
		c.ScreenOffAfter = 120
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	b := readConfig(t)
	if !strings.Contains(b, `"lid_mode": true`) {
		t.Fatalf("lid_mode lost after update: %s", b)
	}
	if !strings.Contains(b, `"hotspot_ssid": "Oh yeah"`) {
		t.Fatalf("hotspot lost after update: %s", b)
	}
}

func TestSaveRefusesEmptyOverwrite(t *testing.T) {
	p := testConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"lid_mode":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Save(Config{}); err == nil {
		t.Fatal("Save({}) should refuse to overwrite non-empty config")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"lid_mode":true}` {
		t.Fatalf("config changed: %q", b)
	}
}

func TestSaveAllowsEmptyWhenMissing(t *testing.T) {
	testConfigPath(t)
	if err := Save(Config{}); err != nil {
		t.Fatalf("Save({}) on missing file: %v", err)
	}
}

func TestSaveAllowsEmptyWhenFileEmpty(t *testing.T) {
	p := testConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Save(Config{}); err != nil {
		t.Fatalf("Save({}) on empty file: %v", err)
	}
}

func TestRestoreMissingLidMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	state := filepath.Join(home, ".config", "keepgoing", "state.json")
	if err := os.MkdirAll(filepath.Dir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte(`{"lid_mode":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(home, ".config", "keepgoing", "config.json")
	if err := os.WriteFile(p, []byte(`{"hotspot_ssid":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadDetailed()
	if err != nil {
		t.Fatal(err)
	}
	cfg, restored := RestoreMissingLidMode(loaded)
	if !restored {
		t.Fatal("expected restore")
	}
	if !cfg.LidMode {
		t.Fatal("expected lid_mode true")
	}
	b := readConfig(t)
	if !strings.Contains(b, `"lid_mode": true`) {
		t.Fatalf("config not updated: %s", b)
	}
}

func TestNightSettingsDefaultWhenAbsent(t *testing.T) {
	_ = testConfigPath(t)
	loaded, err := LoadDetailed()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.HasNightAskAt {
		t.Fatal("expected night_ask_at absent in empty config")
	}
	askAt, until := loaded.Config.NightSettings(loaded)
	if askAt != "23:00" {
		t.Fatalf("askAt = %q want 23:00", askAt)
	}
	if until != "07:00" {
		t.Fatalf("until = %q want 07:00", until)
	}
}

func TestNightSettingsDefaultWhenKeyMissingFromConfig(t *testing.T) {
	p := testConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"lid_mode":true,"hotspot_ssid":"x"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDetailed()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.HasNightAskAt {
		t.Fatal("night_ask_at should be absent when not in JSON")
	}
	askAt, until := loaded.Config.NightSettings(loaded)
	if askAt != "23:00" || until != "07:00" {
		t.Fatalf("got ask_at=%q until=%q want 23:00 / 07:00", askAt, until)
	}
}
