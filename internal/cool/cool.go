// Package cool moves agent processes to efficiency cores and toggles Low Power
// Mode while the lid is closed.
package cool

import (
	"fmt"
	"os/exec"

	"github.com/elijah/keepgoing/internal/procwatch"
)

// Manager tracks backgrounded agent PIDs and whether it enabled Low Power Mode.
type Manager struct {
	pids           map[int]bool
	daemonLowPower bool
}

// New returns an empty Manager.
func New() *Manager {
	return &Manager{pids: map[int]bool{}}
}

// PIDCount is how many PIDs are currently backgrounded.
func (m *Manager) PIDCount() int {
	return len(m.pids)
}

// DaemonLowPower reports whether this manager turned on Low Power Mode.
func (m *Manager) DaemonLowPower() bool {
	return m.daemonLowPower
}

// LidClosed applies cool measures when the lid just shut with agents running.
func (m *Manager) LidClosed(agents []procwatch.Proc, setLowPower func(bool) error) {
	for _, p := range agents {
		m.background(p.PID)
	}
	if len(agents) == 0 {
		return
	}
	if err := setLowPower(true); err != nil {
		return
	}
	m.daemonLowPower = true
}

// LidOpened restores task policy and Low Power Mode if this manager changed them.
func (m *Manager) LidOpened(setLowPower func(bool) error) {
	m.clearBackground()
	if m.daemonLowPower {
		_ = setLowPower(false)
		m.daemonLowPower = false
	}
}

// Shutdown is the same as LidOpened for cleanup on daemon exit.
func (m *Manager) Shutdown(setLowPower func(bool) error) {
	m.LidOpened(setLowPower)
}

// Tick backgrounds any new agent PIDs while the lid is shut and cool mode is on.
func (m *Manager) Tick(agents []procwatch.Proc, lidClosed bool, coolOn bool) {
	if !lidClosed || !coolOn {
		return
	}
	for _, p := range agents {
		m.background(p.PID)
	}
}

func (m *Manager) background(pid int) {
	if m.pids[pid] {
		return
	}
	if err := exec.Command("taskpolicy", "-b", "-p", fmt.Sprint(pid)).Run(); err != nil {
		return
	}
	m.pids[pid] = true
}

func (m *Manager) clearBackground() {
	for pid := range m.pids {
		_ = exec.Command("taskpolicy", "-B", "-p", fmt.Sprint(pid)).Run()
	}
	m.pids = map[int]bool{}
}
