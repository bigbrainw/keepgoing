//go:build !darwin

package power

func OnBattery() bool { return false }
