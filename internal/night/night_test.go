package night

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
	return Settings{AskAt: "23:00", Until: "07:00"}
}

func TestDecideBeforeAsk(t *testing.T) {
	loc := mustLoc(t)
	mode, _ := Decide(at(t, loc, 2026, 9, 22, 22, 0), cfg(), nil)
	if mode != ModeOff {
		t.Fatalf("before 23:00: got %q want off", mode)
	}
}

func TestDecideAskNoAnswer(t *testing.T) {
	loc := mustLoc(t)
	mode, _ := Decide(at(t, loc, 2026, 9, 22, 23, 0), cfg(), nil)
	if mode != ModeAsk {
		t.Fatalf("23:00 no answer: got %q want ask", mode)
	}
}

func TestDecideAskTimeout(t *testing.T) {
	loc := mustLoc(t)
	mode, reason := Decide(at(t, loc, 2026, 9, 22, 23, 10), cfg(), nil)
	if mode != ModeSleep {
		t.Fatalf("23:10 no answer: got %q want sleep", mode)
	}
	if reason == "" {
		t.Fatal("expected reason")
	}
}

func TestDecideAnswerYes(t *testing.T) {
	loc := mustLoc(t)
	ans := &Answer{Date: "2026-09-22", Answer: "yes", At: at(t, loc, 2026, 9, 22, 23, 1).Unix()}
	mode, _ := Decide(at(t, loc, 2026, 9, 22, 23, 30), cfg(), ans)
	if mode != ModeRun {
		t.Fatalf("yes answer: got %q want run", mode)
	}
	// still run at 06:00 next morning
	mode, _ = Decide(at(t, loc, 2026, 9, 23, 6, 0), cfg(), ans)
	if mode != ModeRun {
		t.Fatalf("yes before until: got %q want run", mode)
	}
}

func TestDecideAnswerNo(t *testing.T) {
	loc := mustLoc(t)
	ans := &Answer{Date: "2026-09-22", Answer: "no", At: at(t, loc, 2026, 9, 22, 23, 2).Unix()}
	mode, _ := Decide(at(t, loc, 2026, 9, 22, 23, 5), cfg(), ans)
	if mode != ModeSleep {
		t.Fatalf("no answer: got %q want sleep", mode)
	}
}

func TestDecideNextDayResets(t *testing.T) {
	loc := mustLoc(t)
	old := &Answer{Date: "2026-09-21", Answer: "yes", At: at(t, loc, 2026, 9, 21, 23, 1).Unix()}
	mode, _ := Decide(at(t, loc, 2026, 9, 22, 23, 0), cfg(), old)
	if mode != ModeAsk {
		t.Fatalf("new night: got %q want ask (prior answer ignored)", mode)
	}
	mode, _ = Decide(at(t, loc, 2026, 9, 22, 10, 0), cfg(), old)
	if mode != ModeOff {
		t.Fatalf("midday: got %q want off", mode)
	}
}

func TestDecideFeatureOff(t *testing.T) {
	loc := mustLoc(t)
	mode, _ := Decide(at(t, loc, 2026, 9, 22, 23, 0), Settings{AskAt: ""}, nil)
	if mode != ModeOff {
		t.Fatalf("empty ask_at: got %q want off", mode)
	}
}

func TestUntilAt(t *testing.T) {
	loc := mustLoc(t)
	end := UntilAt(at(t, loc, 2026, 9, 22, 23, 30), cfg())
	want := at(t, loc, 2026, 9, 23, 7, 0)
	if !end.Equal(want) {
		t.Fatalf("until_at = %v want %v", end, want)
	}
}
