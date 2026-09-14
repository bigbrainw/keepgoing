// Package tunnel is an HTTP CONNECT proxy with the same hold-while-offline
// behaviour as the HTTP proxy. Used for agents that hard-code their model
// endpoint or speak WebSocket (Codex under ChatGPT login).
//
// Holding happens at CONNECT time: while offline the tunnel is not
// established, so the client sees a slow connect rather than a refused one.
// A tunnel that drops mid-stream cannot be resumed; the client reconnects
// and the new CONNECT is held until the network is back.
package tunnel

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elijah/keepgoing/internal/netwatch"
)

// Server is the CONNECT proxy.
type Server struct {
	net     *netwatch.Watcher
	holdMax time.Duration
	allow   map[string]bool // host allowlist; empty = any

	Tunnels int64
	Held    int64
	Active  int64
}

// New builds a Server. allowHosts restricts CONNECT targets (host only, no port).
func New(nw *netwatch.Watcher, holdMax time.Duration, allowHosts []string) *Server {
	s := &Server{net: nw, holdMax: holdMax, allow: map[string]bool{}}
	for _, h := range allowHosts {
		s.allow[h] = true
	}
	return s
}

// Serve accepts connections until ctx is done.
func (s *Server) Serve(ctx context.Context, ln net.Listener) {
	go func() { <-ctx.Done(); ln.Close() }()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[tunnel] accept: %v", err)
			continue
		}
		go s.handle(ctx, c)
	}
}

func (s *Server) handle(ctx context.Context, c net.Conn) {
	defer c.Close()
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		// plain HTTP through a forward proxy: not supported, agents use TLS.
		io.WriteString(c, "HTTP/1.1 405 Method Not Allowed\r\nContent-Length: 0\r\n\r\n")
		return
	}
	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
	}
	if len(s.allow) > 0 && !s.allow[host] {
		// pass through non-LLM traffic without holding (analytics, updates…)
		s.dialAndPipe(ctx, c, br, req.Host, false)
		return
	}
	s.dialAndPipe(ctx, c, br, req.Host, true)
}

func (s *Server) dialAndPipe(ctx context.Context, c net.Conn, br *bufio.Reader, target string, hold bool) {
	atomic.AddInt64(&s.Tunnels, 1)
	start := time.Now()
	holdCtx, cancel := context.WithTimeout(ctx, s.holdMax)
	defer cancel()

	var up net.Conn
	var err error
	for attempt := 1; ; attempt++ {
		if hold && !s.net.Online() {
			if attempt == 1 {
				atomic.AddInt64(&s.Held, 1)
			}
			log.Printf("[tunnel] hold CONNECT %s (offline)", target)
			if !s.net.WaitOnline(holdCtx) {
				io.WriteString(c, "HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\n\r\n")
				return
			}
			log.Printf("[tunnel] release CONNECT %s after %s", target, time.Since(start).Round(time.Second))
		}
		d := net.Dialer{Timeout: 15 * time.Second}
		up, err = d.DialContext(ctx, "tcp", target)
		if err == nil {
			break
		}
		if !hold || holdCtx.Err() != nil {
			log.Printf("[tunnel] dial %s failed: %v", target, err)
			io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
			return
		}
		log.Printf("[tunnel] dial %s failed (attempt %d): %v — re-checking network", target, attempt, err)
		s.net.MarkOffline()
		select {
		case <-holdCtx.Done():
		case <-time.After(2 * time.Second):
		}
	}
	defer up.Close()
	if _, err := io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	atomic.AddInt64(&s.Active, 1)
	defer atomic.AddInt64(&s.Active, -1)

	var wg sync.WaitGroup
	wg.Add(2)
	var down, upb int64
	go func() { defer wg.Done(); n, _ := io.Copy(up, br); upb = n; closeWrite(up) }()
	go func() { defer wg.Done(); n, _ := io.Copy(c, up); down = n; closeWrite(c) }()
	wg.Wait()
	if hold {
		log.Printf("[tunnel] %s closed after %s (↑%dB ↓%dB)", target, time.Since(start).Round(time.Second), upb, down)
	}
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}
