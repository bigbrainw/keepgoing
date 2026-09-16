package smcread

import (
	"fmt"
	"os/exec"
)

// AppRunning reports whether the KeepGoing menu bar app is alive.
func AppRunning() bool {
	return exec.Command("pgrep", "-x", "KeepGoing").Run() == nil
}

// NotifyThermal posts a user notification when the app is not running.
func NotifyThermal(state string) {
	if AppRunning() {
		return
	}
	msg := "Mac is running hot with the lid closed. Open it or move it off soft surfaces."
	script := fmt.Sprintf(`display notification %q with title "KeepGoing"`, msg)
	_ = exec.Command("osascript", "-e", script).Run()
}
