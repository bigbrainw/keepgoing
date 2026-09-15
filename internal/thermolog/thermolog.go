// Package thermolog appends lid/thermal/CPU samples to a CSV log.
package thermolog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const header = "ts_iso,lid_closed,thermal_state,cpu_c,low_power,cool_pids,agents,on_battery\n"
const maxBytes = 5 * 1024 * 1024

// Row is one CSV sample.
type Row struct {
	LidClosed   bool
	Thermal     string
	CPUC        *float64
	LowPower    bool
	CoolPIDs    int
	Agents      string
	OnBattery   bool
}

// Path returns ~/Library/Logs/keepgoing/thermal.csv.
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Logs", "keepgoing", "thermal.csv")
}

// Append writes one row, creating the file and header if needed.
func Append(r Row) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := rotateIfNeeded(p); err != nil {
		return err
	}
	needHeader := true
	if st, err := os.Stat(p); err == nil && st.Size() > 0 {
		needHeader = false
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if needHeader {
		if _, err := f.WriteString(header); err != nil {
			return err
		}
	}
	cpu := ""
	if r.CPUC != nil {
		cpu = fmt.Sprintf("%.1f", *r.CPUC)
	}
	agents := strings.ReplaceAll(r.Agents, ",", ";")
	line := fmt.Sprintf("%s,%t,%s,%s,%t,%d,%s,%t\n",
		time.Now().Format(time.RFC3339),
		r.LidClosed,
		r.Thermal,
		cpu,
		r.LowPower,
		r.CoolPIDs,
		agents,
		r.OnBattery,
	)
	_, err = f.WriteString(line)
	return err
}

func rotateIfNeeded(p string) error {
	st, err := os.Stat(p)
	if err != nil || st.Size() < maxBytes {
		return nil
	}
	backup := p + ".1"
	_ = os.Remove(backup)
	return os.Rename(p, backup)
}

// LastLines returns up to n most recent data lines (no header).
func LastLines(n int) ([]string, error) {
	b, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var lines []string
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "ts_iso,") {
			continue
		}
		lines = append(lines, ln)
	}
	if len(lines) <= n {
		return lines, nil
	}
	return lines[len(lines)-n:], nil
}

// FormatTable renders CSV lines as a fixed-width table.
func FormatTable(lines []string) string {
	if len(lines) == 0 {
		return "(no samples yet)\n"
	}
	cols := [][]string{{"time"}, {"lid"}, {"thermal"}, {"°C"}, {"lp"}, {"cool"}, {"agents"}, {"batt"}}
	for _, ln := range lines {
		p := strings.Split(ln, ",")
		if len(p) < 8 {
			continue
		}
		ts := p[0]
		if len(ts) > 19 {
			ts = ts[:19]
		}
		cols[0] = append(cols[0], ts)
		cols[1] = append(cols[1], p[1])
		cols[2] = append(cols[2], p[2])
		cols[3] = append(cols[3], p[3])
		cols[4] = append(cols[4], p[4])
		cols[5] = append(cols[5], p[5])
		ag := p[6]
		if len(ag) > 18 {
			ag = ag[:15] + "..."
		}
		cols[6] = append(cols[6], ag)
		cols[7] = append(cols[7], p[7])
	}
	widths := make([]int, len(cols))
	for i, col := range cols {
		for _, cell := range col {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	var out strings.Builder
	for row := 0; row < len(cols[0]); row++ {
		for col := 0; col < len(cols); col++ {
			if col > 0 {
				out.WriteByte(' ')
			}
			cell := cols[col][row]
			if col < len(cols)-1 {
				fmt.Fprintf(&out, "%-*s", widths[col], cell)
			} else {
				out.WriteString(cell)
			}
		}
		out.WriteByte('\n')
	}
	return out.String()
}
