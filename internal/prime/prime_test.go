package prime

import (
	"testing"
	"time"
)

func mustLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func at(t *testing.T, loc *time.Location, y int, mo time.Month, d, h, m int) time.Time {
	t.Helper()
	return time.Date(y, mo, d, h, m, 0, 0, loc)
}

func cfg() Settings {
	return Settings{
		At:      []string{"07:00", "12:00"},
		Agents:  []string{"claude", "codex"},
		Message: "hi",
		Model:   "claude-haiku-4-5",
	}
}

func TestDueBeforeTime(t *testing.T) {
	loc := mustLoc(t)
	now := at(t, loc, 2026, 9, 23, 6, 30)
	if got := Due(now, cfg(), nil); len(got) != 0 {
		t.Fatalf("before 07:00: got %v want none", got)
	}
}

func TestDueAtTime(t *testing.T) {
	loc := mustLoc(t)
	now := at(t, loc, 2026, 9, 23, 7, 0)
	got := Due(now, cfg(), nil)
	if len(got) != 2 {
		t.Fatalf("at 07:00: got %v want both agents", got)
	}
}

func TestDueTwentyMinLate(t *testing.T) {
	loc := mustLoc(t)
	now := at(t, loc, 2026, 9, 23, 7, 20)
	got := Due(now, cfg(), nil)
	if len(got) != 2 {
		t.Fatalf("07:20 catch-up: got %v want both agents", got)
	}
}

func TestDueFortyMinLate(t *testing.T) {
	loc := mustLoc(t)
	now := at(t, loc, 2026, 9, 23, 7, 40)
	got := Due(now, cfg(), nil)
	if len(got) != 0 {
		t.Fatalf("07:40 past catch-up: got %v want none", got)
	}
}

func TestDueAlreadyRanToday(t *testing.T) {
	loc := mustLoc(t)
	now := at(t, loc, 2026, 9, 23, 7, 10)
	last := map[string]time.Time{
		runKey("claude", "07:00"): at(t, loc, 2026, 9, 23, 7, 2),
		runKey("codex", "07:00"):  at(t, loc, 2026, 9, 23, 7, 3),
	}
	got := Due(now, cfg(), last)
	if len(got) != 0 {
		t.Fatalf("already ran: got %v want none", got)
	}
}

func TestDueMultipleTimesPerDay(t *testing.T) {
	loc := mustLoc(t)
	last := map[string]time.Time{
		runKey("claude", "07:00"): at(t, loc, 2026, 9, 23, 7, 1),
		runKey("codex", "07:00"):  at(t, loc, 2026, 9, 23, 7, 2),
	}
	now := at(t, loc, 2026, 9, 23, 12, 5)
	got := Due(now, cfg(), last)
	if len(got) != 2 {
		t.Fatalf("12:05 second slot: got %v want both agents", got)
	}
}

func TestDueAgentListEmpty(t *testing.T) {
	loc := mustLoc(t)
	c := cfg()
	c.Agents = nil
	got := Due(at(t, loc, 2026, 9, 23, 7, 0), c, nil)
	if len(got) != 0 {
		t.Fatalf("empty agents: got %v want none", got)
	}
}

func TestDueAtListEmpty(t *testing.T) {
	loc := mustLoc(t)
	c := cfg()
	c.At = nil
	got := Due(at(t, loc, 2026, 9, 23, 7, 0), c, nil)
	if len(got) != 0 {
		t.Fatalf("empty at: got %v want none", got)
	}
}

func TestNextAt(t *testing.T) {
	loc := mustLoc(t)
	now := at(t, loc, 2026, 9, 23, 8, 0)
	next := NextAt(now, cfg())
	want := at(t, loc, 2026, 9, 23, 12, 0)
	if !next.Equal(want) {
		t.Fatalf("next = %v want %v", next, want)
	}
}

func TestNextAtTomorrow(t *testing.T) {
	loc := mustLoc(t)
	now := at(t, loc, 2026, 9, 23, 13, 0)
	next := NextAt(now, cfg())
	want := at(t, loc, 2026, 9, 24, 7, 0)
	if !next.Equal(want) {
		t.Fatalf("next = %v want %v", next, want)
	}
}

func TestMenuLine(t *testing.T) {
	loc := mustLoc(t)
	now := at(t, loc, 2026, 9, 23, 8, 0)
	last := map[string]PrimeLast{
		"claude": {TS: at(t, loc, 2026, 9, 23, 7, 2).Unix(), OK: true, Slot: "07:00"},
	}
	line := MenuLine(now, cfg(), last)
	want := "Window: primed 07:02 · next 12:00"
	if line != want {
		t.Fatalf("menu = %q want %q", line, want)
	}
}

func TestMenuLineNotScheduled(t *testing.T) {
	loc := mustLoc(t)
	line := MenuLine(at(t, loc, 2026, 9, 23, 8, 0), Settings{}, nil)
	if line != "Window: not scheduled" {
		t.Fatalf("menu = %q want not scheduled", line)
	}
}
