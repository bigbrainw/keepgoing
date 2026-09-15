// Package procwatch counts running coding-agent processes so the daemon can
// hold a sleep assertion only while there is something to keep alive.
package procwatch

import (
	"strings"
	"time"
)

// Proc is one detected agent process.
type Proc struct {
	PID          int       `json:"PID"`
	Agent        string    `json:"Agent"`
	Path         string    `json:"Path"`
	CPUPct       float64   `json:"cpu_pct"`
	Working      bool      `json:"working"`
	WorkingSince time.Time `json:"working_since"`
}

// AnyWorking reports whether any proc is working.
func AnyWorking(p []Proc) bool {
	for _, x := range p {
		if x.Working {
			return true
		}
	}
	return false
}

// Summary returns working/idle counts per kind, e.g. "claude 2 working · 12 idle".
func Summary(p []Proc) string {
	if len(p) == 0 {
		return "none"
	}
	kinds := map[string]struct{ working, idle int }{}
	order := []string{}
	for _, x := range p {
		k := displayKind(x.Agent)
		if _, ok := kinds[k]; !ok {
			order = append(order, k)
		}
		c := kinds[k]
		if x.Working {
			c.working++
		} else {
			c.idle++
		}
		kinds[k] = c
	}
	var parts []string
	for _, k := range order {
		c := kinds[k]
		parts = append(parts, k+" "+itoa(c.working)+" working · "+itoa(c.idle)+" idle")
	}
	return strings.Join(parts, " · ")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
