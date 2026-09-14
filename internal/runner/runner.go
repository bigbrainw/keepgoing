// Package runner spawns the agent, watches it, and resumes it after crashes.
package runner

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/elijah/keepgoing/internal/agents"
	"github.com/elijah/keepgoing/internal/netwatch"
)

// Options configure a run.
type Options struct {
	Argv        []string
	Kind        agents.Kind
	ProxyBase   string
	TunnelURL   string
	MaxRestarts int
	Backoff     time.Duration
	Net         *netwatch.Watcher
}

// Run executes the agent until it exits cleanly, the user interrupts, or the
// restart budget is exhausted. Returns the final exit code.
func Run(ctx context.Context, o Options) int {
	argv := agents.Argv(o.Kind, o.Argv, o.ProxyBase, o.TunnelURL)
	env := append(os.Environ(), agents.Env(o.Kind, o.ProxyBase, o.TunnelURL)...)
	restarts := 0

	for {
		log.Printf("[run] exec: %s", strings.Join(argv, " "))
		code, interrupted := runOnce(ctx, argv, env)
		if interrupted {
			log.Printf("[run] interrupted by user, not resuming")
			return 130
		}
		if code == 0 {
			log.Printf("[run] agent finished cleanly")
			return 0
		}
		if restarts >= o.MaxRestarts {
			log.Printf("[run] exit %d; restart budget (%d) exhausted", code, o.MaxRestarts)
			return code
		}
		resume := agents.ResumeArgv(o.Kind, o.Argv, o.ProxyBase, o.TunnelURL)
		if resume == nil {
			log.Printf("[run] exit %d; unknown agent, cannot resume", code)
			return code
		}
		restarts++
		// wait for network before resuming: a resume while offline just crashes again.
		if !o.Net.Online() {
			log.Printf("[run] exit %d; waiting for network before resume %d/%d", code, restarts, o.MaxRestarts)
			if !o.Net.WaitOnline(ctx) {
				return code
			}
		}
		log.Printf("[run] exit %d; resuming in %s (%d/%d)", code, o.Backoff, restarts, o.MaxRestarts)
		select {
		case <-ctx.Done():
			return code
		case <-time.After(o.Backoff):
		}
		argv = resume
	}
}

// runOnce spawns argv with inherited stdio. Returns (exit code, user interrupted).
func runOnce(ctx context.Context, argv, env []string) (int, bool) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// own process group so Ctrl-C reaches the child via the terminal, and we
	// can still forward signals explicitly.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: false}
	if err := cmd.Start(); err != nil {
		log.Printf("[run] start failed: %v", err)
		return 127, false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		_ = cmd.Process.Signal(syscall.SIGINT)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
		return 130, true
	case err := <-done:
		if err == nil {
			return 0, false
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				sig := ws.Signal()
				if sig == syscall.SIGINT || sig == syscall.SIGTERM {
					return 128 + int(sig), true
				}
				return 128 + int(sig), false
			}
			return ee.ExitCode(), false
		}
		return 1, false
	}
}
