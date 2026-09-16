//go:build !darwin

package smcread

func AppRunning() bool { return false }

func NotifyThermal(string) {}
