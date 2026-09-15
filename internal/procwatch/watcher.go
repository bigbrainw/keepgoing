package procwatch

import (
	"bufio"
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elijah/keepgoing/internal/agentsignal"
)

// Agent names by executable basename.
var names = map[string]string{
	"claude":       "claude",
	"codex":        "codex",
	"cursor-agent": "cursor",
	"agent":        "cursor",
}

// isAppServer reports whether a codex invocation is `codex … app-server …`.
func isAppServer(args []string) bool {
	for _, a := range args {
		if a == "app-server" {
			return true
		}
	}
	return false
}

const workingCPUPct = 2.0
const signalWindow = 60 * time.Second

// Watcher tracks CPU deltas and consults opt-in agent signals.
type Watcher struct {
	signals      *agentsignal.Store
	prevCPU      map[int]float64
	workingSince map[int]time.Time
	lastAt       time.Time
}

// NewWatcher creates a Watcher. signals may be nil (CPU only).
func NewWatcher(signals *agentsignal.Store) *Watcher {
	return &Watcher{
		signals:      signals,
		prevCPU:      map[int]float64{},
		workingSince: map[int]time.Time{},
	}
}

// Scan lists agents with working/idle classification.
func (w *Watcher) Scan() ([]Proc, error) {
	now := time.Now()
	wall := 5.0
	if !w.lastAt.IsZero() {
		wall = now.Sub(w.lastAt).Seconds()
		if wall < 0.5 {
			wall = 0.5
		}
	}
	w.lastAt = now

	tree, agents, err := readProcessTree()
	if err != nil {
		return nil, err
	}

	var procs []Proc
	nextPrev := map[int]float64{}
	for _, a := range agents {
		total := SubtreeCPU(a.PID, tree.children, tree.cpu)
		nextPrev[a.PID] = total
		pct := 0.0
		if prev, ok := w.prevCPU[a.PID]; ok && wall > 0 {
			pct = (total - prev) / wall * 100
			if pct < 0 {
				pct = 0
			}
		}
		working := pct >= workingCPUPct
		if w.signals != nil && w.signals.Working(a.PID, signalWindow) {
			working = true
		}
		if w.signals != nil && w.signals.WorkingByTool(displayKind(a.Agent), signalWindow) {
			working = true
		}
		var since time.Time
		if working {
			if t, ok := w.workingSince[a.PID]; ok {
				since = t
			} else {
				since = now
				w.workingSince[a.PID] = since
			}
		} else {
			delete(w.workingSince, a.PID)
		}
		procs = append(procs, Proc{
			PID:          a.PID,
			Agent:        a.Agent,
			Path:         a.Path,
			CPUPct:       pct,
			Working:      working,
			WorkingSince: since,
		})
	}
	w.prevCPU = nextPrev
	return procs, nil
}

type procTree struct {
	cpu      map[int]float64
	ppid     map[int]int
	children map[int][]int
}

type agentStub struct {
	PID   int
	Agent string
	Path  string
}

func readProcessTree() (*procTree, []agentStub, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,cputime=,args=").Output()
	if err != nil {
		return nil, nil, err
	}
	tree := &procTree{
		cpu:      map[int]float64{},
		ppid:     map[int]int{},
		children: map[int][]int{},
	}
	var agents []agentStub
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		pid, rest, ok := fieldsInt(line)
		if !ok {
			continue
		}
		ppid, rest, ok := fieldsInt(rest)
		if !ok {
			continue
		}
		cpuStr, args, ok := fieldsToken(rest)
		if !ok {
			continue
		}
		tree.cpu[pid] = ParseCPUTime(cpuStr)
		tree.ppid[pid] = ppid
		tree.children[ppid] = append(tree.children[ppid], pid)

		f := strings.Fields(args)
		if len(f) < 1 {
			continue
		}
		exe := f[0]
		agent, ok := names[filepath.Base(exe)]
		if !ok {
			continue
		}
		if agent == "codex" && isAppServer(f[1:]) {
			agent = "codex-app"
		} else if strings.Contains(exe, ".app/Contents/") {
			continue
		}
		agents = append(agents, agentStub{PID: pid, Agent: agent, Path: exe})
	}
	return tree, agents, nil
}

func fieldsInt(s string) (int, string, bool) {
	s = strings.TrimLeft(s, " ")
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		return 0, "", false
	}
	n := 0
	for _, c := range s[:i] {
		if c < '0' || c > '9' {
			return 0, "", false
		}
		n = n*10 + int(c-'0')
	}
	return n, strings.TrimLeft(s[i+1:], " "), true
}

func fieldsToken(s string) (string, string, bool) {
	s = strings.TrimLeft(s, " ")
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		return "", "", false
	}
	return s[:i], strings.TrimLeft(s[i+1:], " "), true
}

func displayKind(agent string) string {
	if agent == "codex-app" {
		return "codex"
	}
	return agent
}

// WorkingCount returns how many procs are working.
func WorkingCount(p []Proc) int {
	n := 0
	for _, x := range p {
		if x.Working {
			n++
		}
	}
	return n
}
