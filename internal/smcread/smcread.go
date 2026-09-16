// Package smcread runs the keepgoing-smc helper for CPU temperature.
package smcread

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Sample is one keepgoing-smc reading.
type Sample struct {
	CPUC    *float64
	Keys    map[string]float64
	Thermal string
}

var (
	locateOnce sync.Once
	located    string
	missingLog sync.Once
)

// Locate finds keepgoing-smc next to this binary, then ~/.local/bin.
func Locate() string {
	locateOnce.Do(func() {
		if exe, err := os.Executable(); err == nil {
			exe, _ = filepath.EvalSymlinks(exe)
			sibling := filepath.Join(filepath.Dir(exe), "keepgoing-smc")
			if st, err := os.Stat(sibling); err == nil && !st.IsDir() {
				located = sibling
				return
			}
		}
		home, _ := os.UserHomeDir()
		fallback := filepath.Join(home, ".local", "bin", "keepgoing-smc")
		if st, err := os.Stat(fallback); err == nil && !st.IsDir() {
			located = fallback
		}
	})
	return located
}

// Read runs keepgoing-smc and parses its JSON line. ok is false when the helper is missing.
func Read() (s Sample, ok bool, err error) {
	path := Locate()
	if path == "" {
		missingLog.Do(func() {
			fmt.Fprintf(os.Stderr, "[smc] keepgoing-smc not found (bundle or ~/.local/bin); skipping CPU °C\n")
		})
		return Sample{}, false, nil
	}
	out, err := exec.Command(path).Output()
	if err != nil {
		return Sample{}, true, fmt.Errorf("keepgoing-smc: %w", err)
	}
	var raw struct {
		CPUC    *float64           `json:"cpu_c"`
		Keys    map[string]float64 `json:"keys"`
		Thermal string             `json:"thermal"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return Sample{}, true, fmt.Errorf("keepgoing-smc json: %w", err)
	}
	return Sample{CPUC: raw.CPUC, Keys: raw.Keys, Thermal: raw.Thermal}, true, nil
}
