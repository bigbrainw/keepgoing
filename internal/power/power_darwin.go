package power

import (
	"os/exec"
	"regexp"
	"strings"
)

var batteryPctRE = regexp.MustCompile(`(\d+)%`)

// OnBattery reports whether the Mac is running on battery power.
func OnBattery() bool {
	out, err := exec.Command("pmset", "-g", "batt").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Battery Power")
}

// BatteryPercent reads the current battery level from pmset -g batt.
func BatteryPercent() (pct int, ok bool) {
	out, err := exec.Command("pmset", "-g", "batt").Output()
	if err != nil {
		return 0, false
	}
	return ParseBatteryOutput(string(out))
}

// ParseBatteryOutput extracts the first percentage from pmset -g batt output.
func ParseBatteryOutput(out string) (pct int, ok bool) {
	m := batteryPctRE.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	var n int
	for _, c := range m[1] {
		n = n*10 + int(c-'0')
	}
	return n, true
}
