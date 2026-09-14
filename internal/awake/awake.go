// Package awake holds a system sleep-inhibit assertion while an agent works.
package awake

import (
	"log"
	"os"
	"os/exec"
	"runtime"
)

// Holder wraps a child process that inhibits sleep.
type Holder struct{ cmd *exec.Cmd }

// Hold starts the inhibitor. No-op on unsupported platforms.
// On macOS this blocks idle/display/disk sleep. Lid-closed sleep on battery is
// not preventable from userspace without `pmset disablesleep 1`; see README.
func Hold() *Holder {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// -d display, -i idle, -m disk, -s AC-system, -w wait for our pid
		cmd = exec.Command("caffeinate", "-dims", "-w", itoa(os.Getpid()))
	case "linux":
		if _, err := exec.LookPath("systemd-inhibit"); err == nil {
			cmd = exec.Command("systemd-inhibit", "--what=idle:sleep",
				"--who=keepgoing", "--why=coding agent running", "sleep", "infinity")
		}
	}
	if cmd == nil {
		return &Holder{}
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[awake] could not start inhibitor: %v", err)
		return &Holder{}
	}
	log.Printf("[awake] sleep inhibited (pid %d)", cmd.Process.Pid)
	return &Holder{cmd: cmd}
}

// Release stops the inhibitor.
func (h *Holder) Release() {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return
	}
	_ = h.cmd.Process.Kill()
	_, _ = h.cmd.Process.Wait()
	log.Printf("[awake] sleep inhibit released")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
