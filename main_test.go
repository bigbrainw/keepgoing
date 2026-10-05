package main

import (
	"reflect"
	"testing"

	"github.com/elijah/keepgoing/internal/night"
)

func TestFormatNightStatusEnabled(t *testing.T) {
	got := formatNightStatus(night.ModeOff, "23:00", "07:00", nil)
	want := "night_mode=off ask_at=23:00 until=07:00\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatNightStatusDisabled(t *testing.T) {
	got := formatNightStatus(night.ModeOff, "", "07:00", nil)
	want := "night_mode=off\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestInstallAgents(t *testing.T) {
	tests := []struct {
		name       string
		daemonOnly bool
		daemonCode int
		appBin     string
		appCode    int
		wantCode   int
		wantCalls  []string
	}{
		{
			name:       "daemon only skips app discovery and installation",
			daemonOnly: true,
			appBin:     "/Applications/KeepGoing.app/Contents/MacOS/KeepGoing",
			wantCalls:  []string{"daemon"},
		},
		{
			name:       "daemon only preserves daemon failure",
			daemonOnly: true,
			daemonCode: 7,
			wantCode:   7,
			wantCalls:  []string{"daemon"},
		},
		{
			name:      "default installs available app after daemon",
			appBin:    "/Applications/KeepGoing.app/Contents/MacOS/KeepGoing",
			wantCalls: []string{"daemon", "find app", "app"},
		},
		{
			name:      "default succeeds without an app",
			wantCalls: []string{"daemon", "find app"},
		},
		{
			name:       "default daemon failure prevents app actions",
			daemonCode: 7,
			appBin:     "/Applications/KeepGoing.app/Contents/MacOS/KeepGoing",
			wantCode:   7,
			wantCalls:  []string{"daemon"},
		},
		{
			name:      "default preserves app installation failure",
			appBin:    "/Applications/KeepGoing.app/Contents/MacOS/KeepGoing",
			appCode:   9,
			wantCode:  9,
			wantCalls: []string{"daemon", "find app", "app"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			code := installAgents(tt.daemonOnly,
				func() int {
					calls = append(calls, "daemon")
					return tt.daemonCode
				},
				func() string {
					calls = append(calls, "find app")
					return tt.appBin
				},
				func() int {
					calls = append(calls, "app")
					return tt.appCode
				},
			)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Errorf("actions = %v, want %v", calls, tt.wantCalls)
			}
		})
	}
}
