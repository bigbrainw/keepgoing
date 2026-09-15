package power

import (
	"os/exec"
	"strings"
)

// OnBattery reports whether the Mac is running on battery power.
func OnBattery() bool {
	out, err := exec.Command("pmset", "-g", "batt").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Battery Power")
}
