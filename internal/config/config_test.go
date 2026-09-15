package config

import (
	"os"
	"path/filepath"
	"testing"
)

func testConfigPath(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(home, ".config", "keepgoing", "config.json")
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
