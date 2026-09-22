//go:build !darwin

package power

func OnBattery() bool { return false }

// BatteryPercent is unavailable off macOS.
func BatteryPercent() (pct int, ok bool) { return 0, false }

// ParseBatteryOutput extracts battery percent from pmset output (testing helper).
func ParseBatteryOutput(string) (pct int, ok bool) { return 0, false }
