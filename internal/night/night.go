package night

import (
	"fmt"
	"strings"
	"time"
)

const (
	ModeOff   = "off"
	ModeRun   = "run"
	ModeSleep = "sleep"
	ModeAsk   = "ask"
)

// AskTimeout is how long to wait for an overnight answer before defaulting to sleep.
const AskTimeout = 10 * time.Minute

// Settings holds overnight prompt configuration.
type Settings struct {
	AskAt string // "23:00"; empty disables the feature
	Until string // "07:00"
}

// Answer is a user's overnight choice for one night session.
type Answer struct {
	Date   string // "2006-01-02"
	Answer string // "yes" | "no"
	At     int64  // unix seconds
}

// Decide returns the overnight mode and a short reason. Pure — no system calls.
func Decide(now time.Time, cfg Settings, answer *Answer) (mode, reason string) {
	if cfg.AskAt == "" {
		return ModeOff, "feature disabled"
	}
	askHour, askMin, err := parseClock(cfg.AskAt)
	if err != nil {
		return ModeOff, "invalid night_ask_at"
	}
	untilStr := cfg.Until
	if untilStr == "" {
		untilStr = "07:00"
	}
	untilHour, untilMin, err := parseClock(untilStr)
	if err != nil {
		return ModeOff, "invalid night_until"
	}

	sessionDate, inSession, sessionStart, _ := nightSession(now, askHour, askMin, untilHour, untilMin)
	if !inSession {
		return ModeOff, "outside overnight window"
	}

	if answer != nil && answer.Date == sessionDate {
		switch answer.Answer {
		case "yes":
			return ModeRun, fmt.Sprintf("running until %s", untilStr)
		case "no":
			return ModeSleep, fmt.Sprintf("sleep until %s", untilStr)
		}
	}

	if now.Before(sessionStart) {
		return ModeOff, "before ask time"
	}
	if now.Before(sessionStart.Add(AskTimeout)) {
		return ModeAsk, fmt.Sprintf("asking until %s", sessionStart.Add(AskTimeout).Format("15:04"))
	}
	return ModeSleep, fmt.Sprintf("no answer, sleep until %s", untilStr)
}

func parseClock(s string) (hour, min int, err error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("bad time %q", s)
	}
	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("bad hour in %q", s)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("bad minute in %q", s)
	}
	return h, m, nil
}

func combine(day time.Time, hour, min int) time.Time {
	loc := day.Location()
	y, mo, d := day.Date()
	return time.Date(y, mo, d, hour, min, 0, 0, loc)
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// nightSession returns the session date key and bounds for the overnight window containing now.
// Session for calendar date D starts at D askAt and ends at D+1 until.
func nightSession(now time.Time, askHour, askMin, untilHour, untilMin int) (sessionDate string, inSession bool, sessionStart, sessionEnd time.Time) {
	today := dateOnly(now)
	todayAsk := combine(today, askHour, askMin)
	todayUntil := combine(today, untilHour, untilMin)

	if !now.Before(todayAsk) {
		sessionStart = todayAsk
		sessionEnd = combine(today.Add(24*time.Hour), untilHour, untilMin)
		return today.Format("2006-01-02"), true, sessionStart, sessionEnd
	}
	if now.Before(todayUntil) {
		yesterday := today.Add(-24 * time.Hour)
		sessionStart = combine(yesterday, askHour, askMin)
		sessionEnd = todayUntil
		return yesterday.Format("2006-01-02"), true, sessionStart, sessionEnd
	}
	return "", false, time.Time{}, time.Time{}
}

// SessionDate returns the overnight session date key for now, or "" if outside the window.
func SessionDate(now time.Time, cfg Settings) string {
	if cfg.AskAt == "" {
		return ""
	}
	askHour, askMin, err := parseClock(cfg.AskAt)
	if err != nil {
		return ""
	}
	untilStr := cfg.Until
	if untilStr == "" {
		untilStr = "07:00"
	}
	untilHour, untilMin, err := parseClock(untilStr)
	if err != nil {
		return ""
	}
	sessionDate, inSession, _, _ := nightSession(now, askHour, askMin, untilHour, untilMin)
	if !inSession {
		return ""
	}
	return sessionDate
}

// UntilAt returns when the current overnight run/sleep period ends (for /_status).
func UntilAt(now time.Time, cfg Settings) time.Time {
	if cfg.AskAt == "" {
		return time.Time{}
	}
	askHour, askMin, err := parseClock(cfg.AskAt)
	if err != nil {
		return time.Time{}
	}
	untilStr := cfg.Until
	if untilStr == "" {
		untilStr = "07:00"
	}
	untilHour, untilMin, err := parseClock(untilStr)
	if err != nil {
		return time.Time{}
	}
	_, inSession, _, sessionEnd := nightSession(now, askHour, askMin, untilHour, untilMin)
	if !inSession {
		return time.Time{}
	}
	return sessionEnd
}

// NextAskAt returns the next night_ask_at time (for daytime /_status).
func NextAskAt(now time.Time, cfg Settings) time.Time {
	if cfg.AskAt == "" {
		return time.Time{}
	}
	askHour, askMin, err := parseClock(cfg.AskAt)
	if err != nil {
		return time.Time{}
	}
	today := dateOnly(now)
	ask := combine(today, askHour, askMin)
	if now.Before(ask) {
		return ask
	}
	return combine(today.Add(24*time.Hour), askHour, askMin)
}

// Status returns daemon / CLI fields: mode is always run|sleep|ask|off.
func Status(now time.Time, cfg Settings, answer *Answer) (mode string, untilAt time.Time, askAt, untilClock string) {
	askAt = cfg.AskAt
	untilClock = cfg.Until
	if untilClock == "" {
		untilClock = "07:00"
	}
	if askAt == "" {
		return ModeOff, time.Time{}, "", untilClock
	}
	mode, _ = Decide(now, cfg, answer)
	switch mode {
	case ModeRun, ModeSleep:
		untilAt = UntilAt(now, cfg)
	case ModeAsk:
		askHour, askMin, err := parseClock(cfg.AskAt)
		untilHour, untilMin, err2 := parseClock(untilClock)
		if err == nil && err2 == nil {
			_, _, sessionStart, _ := nightSession(now, askHour, askMin, untilHour, untilMin)
			if !sessionStart.IsZero() {
				untilAt = sessionStart.Add(AskTimeout)
			}
		}
	default:
		untilAt = NextAskAt(now, cfg)
	}
	return mode, untilAt, askAt, untilClock
}
