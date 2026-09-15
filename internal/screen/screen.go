// Package screen turns the display off after user idle time while agents run.
package screen

// ShouldSleep reports whether the daemon should call displaysleepnow on this tick.
// threshold is screen_off_after in seconds; 0 disables the feature.
func ShouldSleep(agentsRunning bool, idle, threshold float64, fired bool) bool {
	if !agentsRunning || threshold <= 0 || fired {
		return false
	}
	return idle >= threshold
}
