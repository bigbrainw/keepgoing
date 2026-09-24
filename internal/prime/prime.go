package prime

import (
	"fmt"
	"strings"
	"time"
)

const CatchUpWindow = 30 * time.Minute

// Settings holds session-window primer configuration.
type Settings struct {
	At         []string
	Agents     []string
	Message    string
	Model      string // claude model; default claude-haiku-4-5 when empty
	ModelCodex string // optional codex -m; empty uses codex default
}

// Due returns agent names that should run now — at most once per scheduled time
// per day, with a 30-minute catch-up window after each scheduled time.
func Due(now time.Time, cfg Settings, lastRun map[string]time.Time) []string {
	if len(cfg.At) == 0 || len(cfg.Agents) == 0 {
		return nil
	}
	today := dateOnly(now)
	var due []string
	seen := map[string]bool{}
	for _, slot := range cfg.At {
		slotTime, err := slotOnDay(today, slot)
		if err != nil {
			continue
		}
		if now.Before(slotTime) {
			continue
		}
		if now.Sub(slotTime) > CatchUpWindow {
			continue
		}
		for _, agent := range cfg.Agents {
			key := runKey(agent, slot)
			if ranToday(lastRun[key], today) {
				continue
			}
			if seen[agent] {
				continue
			}
			seen[agent] = true
			due = append(due, agent)
		}
	}
	return due
}

// NextAt returns the next scheduled primer time after now.
func NextAt(now time.Time, cfg Settings) time.Time {
	if len(cfg.At) == 0 {
		return time.Time{}
	}
	today := dateOnly(now)
	var next time.Time
	for _, slot := range cfg.At {
		t, err := slotOnDay(today, slot)
		if err != nil {
			continue
		}
		if !now.Before(t) {
			t, err = slotOnDay(today.Add(24*time.Hour), slot)
			if err != nil {
				continue
			}
		}
		if next.IsZero() || t.Before(next) {
			next = t
		}
	}
	return next
}

// LastPrimedAt returns the most recent successful run time on the calendar day of now.
func LastPrimedAt(now time.Time, last map[string]PrimeLast) time.Time {
	if len(last) == 0 {
		return time.Time{}
	}
	today := dateOnly(now)
	var best time.Time
	for _, e := range last {
		if !e.OK {
			continue
		}
		t := time.Unix(e.TS, 0).In(now.Location())
		if dateOnly(t) != today {
			continue
		}
		if best.IsZero() || t.After(best) {
			best = t
		}
	}
	return best
}

// PrimeLast is one agent's most recent primer run (for status/CLI).
type PrimeLast struct {
	TS   int64
	OK   bool
	Slot string
}

// MenuLine formats the menu status line.
func MenuLine(now time.Time, cfg Settings, last map[string]PrimeLast) string {
	if len(cfg.At) == 0 {
		return "Window: not scheduled"
	}
	if t := LastPrimedAt(now, last); !t.IsZero() {
		next := NextAt(now, cfg)
		if next.IsZero() {
			return fmt.Sprintf("Window: primed %s", t.Format("15:04"))
		}
		return fmt.Sprintf("Window: primed %s · next %s", t.Format("15:04"), next.Format("15:04"))
	}
	next := NextAt(now, cfg)
	if next.IsZero() {
		return "Window: not scheduled"
	}
	return fmt.Sprintf("Window: next %s", next.Format("15:04"))
}

func runKey(agent, slot string) string {
	return agent + ":" + slot
}

func ranToday(t time.Time, today time.Time) bool {
	if t.IsZero() {
		return false
	}
	return dateOnly(t) == today
}

func slotOnDay(day time.Time, slot string) (time.Time, error) {
	hour, min, err := parseClock(slot)
	if err != nil {
		return time.Time{}, err
	}
	y, mo, d := day.Date()
	return time.Date(y, mo, d, hour, min, 0, 0, day.Location()), nil
}

func parseClock(s string) (hour, min int, err error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("bad time %q", s)
	}
	if _, err := fmt.Sscanf(parts[0], "%d", &hour); err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("bad hour in %q", s)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &min); err != nil || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("bad minute in %q", s)
	}
	return hour, min, nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// LastRunFromPrimeLast builds the Due() lastRun map from persisted state.
func LastRunFromPrimeLast(last map[string]PrimeLast, cfg Settings) map[string]time.Time {
	out := map[string]time.Time{}
	if len(last) == 0 {
		return out
	}
	for agent, e := range last {
		slot := e.Slot
		if slot == "" && len(cfg.At) > 0 {
			slot = cfg.At[0]
		}
		if slot == "" {
			continue
		}
		out[runKey(agent, slot)] = time.Unix(e.TS, 0)
	}
	return out
}
