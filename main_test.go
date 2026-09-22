package main

import (
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
