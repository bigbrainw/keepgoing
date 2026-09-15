//go:build !darwin

package screen

import "errors"

var errUnsupported = errors.New("screen control unsupported on this platform")

// IdleSeconds returns HID user idle time in seconds.
func IdleSeconds() (float64, error) { return 0, errUnsupported }

// SleepNow turns the display off immediately.
func SleepNow() error { return errUnsupported }
