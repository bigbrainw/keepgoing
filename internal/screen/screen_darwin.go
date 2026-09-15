//go:build darwin

package screen

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// IdleSeconds returns HID user idle time in seconds.
func IdleSeconds() (float64, error) {
	out, err := exec.Command("ioreg", "-c", "IOHIDSystem").Output()
	if err != nil {
		return 0, fmt.Errorf("ioreg: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "HIDIdleTime") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			break
		}
		ns, err := strconv.ParseUint(fields[len(fields)-1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse HIDIdleTime: %w", err)
		}
		return float64(ns) / 1e9, nil
	}
	return 0, fmt.Errorf("HIDIdleTime not found")
}

// SleepNow turns the display off immediately.
func SleepNow() error {
	out, err := exec.Command("pmset", "displaysleepnow").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pmset displaysleepnow: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
