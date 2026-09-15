// Package cmux polls cmux session/workspace status for working-idle hints.
package cmux

import (
	"os/exec"
	"strings"
	"time"

	"github.com/elijah/keepgoing/internal/agentsignal"
)

// Poller queries cmux at most once per 10 seconds.
type Poller struct {
	signals  *agentsignal.Store
	enabled  bool
	lastList time.Time
}

// New returns a Poller. enabled gates polling; binary must exist.
func New(signals *agentsignal.Store, enabled bool) *Poller {
	if enabled {
		if _, err := exec.LookPath("cmux"); err != nil {
			enabled = false
		}
	}
	return &Poller{signals: signals, enabled: enabled}
}

// Tick polls cmux and records working/idle signals.
func (p *Poller) Tick() {
	if !p.enabled || p.signals == nil {
		return
	}
	now := time.Now()
	if !p.lastList.IsZero() && now.Sub(p.lastList) < 10*time.Second {
		return
	}
	p.lastList = now
	out, err := exec.Command("cmux", "sessions", "list").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		tool, ws, ok := parseSession(line)
		if !ok {
			continue
		}
		st, err := exec.Command("cmux", "workspace", "status", "--workspace", ws).Output()
		if err != nil {
			continue
		}
		state := strings.TrimSpace(strings.Split(string(st), "\n")[0])
		if state != "working" && state != "idle" {
			continue
		}
		p.signals.Record(0, tool, state)
	}
}

func parseSession(line string) (tool, workspace string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.Contains(line, "pid_exists=yes") {
		return "", "", false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	tool = fields[0]
	if tool != "claude" && tool != "cursor" {
		return "", "", false
	}
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "workspace=") {
			workspace = strings.TrimPrefix(f, "workspace=")
		}
	}
	return tool, workspace, workspace != ""
}
