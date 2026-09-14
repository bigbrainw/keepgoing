// Package procwatch counts running coding-agent processes so the daemon can
// hold a sleep assertion only while there is something to keep alive.
package procwatch

import (
	"bufio"
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
)

// Agent names by executable basename.
var names = map[string]string{
	"claude":       "claude",
	"codex":        "codex",
	"cursor-agent": "cursor",
	"agent":        "cursor", // cursor's CLI installs as `agent`
}

// Proc is one detected agent process.
type Proc struct {
	PID   int
	Agent string
	Path  string
}

// Scan lists agent processes: CLI sessions (claude, codex, cursor) and the
// Codex app-server that ChatGPT.app / the Codex desktop app run their threads
// in. The app-server is counted while it exists — phone remote control needs
// it reachable, so "ChatGPT.app open" means "keep the Mac awake". Renderer
// helpers, updaters and other bundle internals are ignored.
func Scan() ([]Proc, error) {
	out, err := exec.Command("ps", "-axo", "pid=,args=").Output()
	if err != nil {
		return nil, err
	}
	var procs []Proc
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		exe := f[1]
		agent, ok := names[filepath.Base(exe)]
		if !ok {
			continue
		}
		if agent == "codex" && isAppServer(f[2:]) {
			agent = "codex-app"
		} else if strings.Contains(exe, ".app/Contents/") {
			continue
		}
		pid := 0
		for _, c := range f[0] {
			pid = pid*10 + int(c-'0')
		}
		procs = append(procs, Proc{PID: pid, Agent: agent, Path: exe})
	}
	return procs, nil
}

// isAppServer reports whether a codex invocation is `codex … app-server …`
// (the long-lived host for desktop-app and remote-control threads).
func isAppServer(args []string) bool {
	for _, a := range args {
		if a == "app-server" {
			return true
		}
	}
	return false
}

// Summary returns counts by agent, e.g. "claude×3 codex×1".
func Summary(p []Proc) string {
	c := map[string]int{}
	for _, x := range p {
		c[x.Agent]++
	}
	var parts []string
	for _, k := range []string{"claude", "codex", "codex-app", "cursor"} {
		if c[k] > 0 {
			parts = append(parts, k+"×"+itoa(c[k]))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
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
