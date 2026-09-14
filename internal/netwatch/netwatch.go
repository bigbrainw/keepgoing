// Package netwatch tracks whether upstream LLM APIs are reachable and lets
// callers block until they are.
package netwatch

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
)

// Watcher polls a set of host:port targets and exposes an online flag.
type Watcher struct {
	targets  []string
	interval time.Duration
	timeout  time.Duration

	mu     sync.Mutex
	online bool
	since  time.Time
	waitCh chan struct{} // closed when we transition to online
	onChg  func(online bool)
	forced *bool // debug override; nil = follow probes
}

// New builds a Watcher. targets are "host:port" strings; any one reachable
// counts as online.
func New(targets []string, interval time.Duration, onChange func(bool)) *Watcher {
	return &Watcher{
		targets:  targets,
		interval: interval,
		timeout:  3 * time.Second,
		waitCh:   make(chan struct{}),
		onChg:    onChange,
	}
}

// Run polls until ctx is done. Call in a goroutine.
func (w *Watcher) Run(ctx context.Context) {
	w.set(w.probe())
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.set(w.probe())
		}
	}
}

// Online reports current state.
func (w *Watcher) Online() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.online
}

// Since reports when the current state began.
func (w *Watcher) Since() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.since
}

// Force overrides probe results (debug/testing). Pass nil to clear.
func (w *Watcher) Force(v *bool) {
	w.mu.Lock()
	w.forced = v
	w.mu.Unlock()
	w.set(w.probe())
}

// MarkOffline lets a caller report a failed upstream dial so the next
// WaitOnline blocks until a probe succeeds again.
func (w *Watcher) MarkOffline() { w.set(false) }

// WaitOnline blocks until online or ctx is done. Returns false on ctx done.
func (w *Watcher) WaitOnline(ctx context.Context) bool {
	for {
		w.mu.Lock()
		if w.online {
			w.mu.Unlock()
			return true
		}
		ch := w.waitCh
		w.mu.Unlock()
		select {
		case <-ctx.Done():
			return false
		case <-ch:
		}
	}
}

func (w *Watcher) set(online bool) {
	w.mu.Lock()
	changed := online != w.online
	w.online = online
	if changed {
		w.since = time.Now()
		if online {
			close(w.waitCh)
			w.waitCh = make(chan struct{})
		}
	}
	w.mu.Unlock()
	if changed {
		if online {
			log.Printf("[net] online")
		} else {
			log.Printf("[net] offline — holding requests")
		}
		if w.onChg != nil {
			w.onChg(online)
		}
	}
}

func (w *Watcher) probe() bool {
	w.mu.Lock()
	f := w.forced
	w.mu.Unlock()
	if f != nil {
		return *f
	}
	for _, t := range w.targets {
		c, err := net.DialTimeout("tcp", t, w.timeout)
		if err == nil {
			c.Close()
			return true
		}
	}
	return false
}
