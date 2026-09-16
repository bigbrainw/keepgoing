package thermolog

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const gapThreshold = 90 * time.Second

// GapInfo summarizes logging gaps longer than 90 seconds.
type GapInfo struct {
	Count      int
	Longest    time.Duration
	LongestAt  time.Time
}

// GapReport scans all CSV rows for consecutive gaps over 90 seconds.
func GapReport() (*GapInfo, error) {
	b, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var prev time.Time
	info := &GapInfo{}
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "ts_iso,") {
			continue
		}
		tsStr := ln
		if i := strings.Index(ln, ","); i > 0 {
			tsStr = ln[:i]
		}
		ts, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			continue
		}
		if !prev.IsZero() {
			gap := ts.Sub(prev)
			if gap > gapThreshold {
				info.Count++
				if gap > info.Longest {
					info.Longest = gap
					info.LongestAt = prev
				}
			}
		}
		prev = ts
	}
	if info.Count == 0 {
		return nil, nil
	}
	return info, nil
}

// FormatGapLine renders the gaps summary for keepgoing thermal.
func FormatGapLine(g *GapInfo) string {
	mins := int(g.Longest.Round(time.Minute).Minutes())
	if mins < 1 {
		mins = 1
	}
	return fmt.Sprintf("gaps: %d (longest %dm at %s)\n", g.Count, mins, g.LongestAt.Format("15:04"))
}
