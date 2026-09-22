// Package proxy is a holding reverse proxy: when the network is down it parks
// requests instead of failing them, so the agent never burns a retry.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elijah/keepgoing/internal/agentsignal"
	"github.com/elijah/keepgoing/internal/netwatch"
	"github.com/elijah/keepgoing/internal/night"
	"github.com/elijah/keepgoing/internal/wifi"
)

// Route maps a local path prefix to an upstream origin.
type Route struct {
	Prefix   string // e.g. "/anthropic"
	Upstream string // e.g. "https://api.anthropic.com"
}

// DefaultRoutes covers Claude Code, Codex (API key) and Codex (ChatGPT login).
var DefaultRoutes = []Route{
	{Prefix: "/anthropic", Upstream: "https://api.anthropic.com"},
	{Prefix: "/openai", Upstream: "https://api.openai.com"},
	{Prefix: "/chatgpt", Upstream: "https://chatgpt.com"},
}

// Stats are exposed on /_status.
type Stats struct {
	Requests     int64 `json:"requests"`
	Held         int64 `json:"held"`
	HeldSeconds  int64 `json:"held_seconds"`
	Retried      int64 `json:"retried_before_response"`
	PartialLost  int64 `json:"partial_streams_lost"`
	InFlight     int64 `json:"in_flight"`
	LastActivity int64 `json:"last_activity_unix"`
}

// Server is the holding proxy.
type Server struct {
	routes  []Route
	net     *netwatch.Watcher
	client  *http.Client
	holdMax time.Duration
	stats   Stats
	maxBody int64

	// Extra, if set, is merged into /_status (daemon adds agents/awake/wifi).
	Extra func() map[string]any

	thermalMu      sync.RWMutex
	thermalState   string
	thermalSince   time.Time
	cpuC           *float64
	daemonThermal  bool

	AgentSignals *agentsignal.Store
	WiFiBridge   *wifi.AppBridge
	NightBridge  *night.Bridge
}

// New builds a Server. holdMax bounds how long a request may be parked.
func New(routes []Route, nw *netwatch.Watcher, holdMax time.Duration) *Server {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 10 * time.Minute, // long thinking turns
		ExpectContinueTimeout: 1 * time.Second,
		// SSE: never buffer
		DisableCompression: true,
	}
	return &Server{
		routes:  routes,
		net:     nw,
		client:  &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		holdMax: holdMax,
		maxBody: 64 << 20,
	}
}

// Handler returns the http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/_status", s.status)
	mux.HandleFunc("/_thermal", s.thermal)
	mux.HandleFunc("/_agent", s.agent)
	mux.HandleFunc("/_wifi", s.wifi)
	mux.HandleFunc("/_wifi/request", s.wifiRequest)
	mux.HandleFunc("/_wifi/result", s.wifiResult)
	mux.HandleFunc("/_night", s.night)
	mux.HandleFunc("/_force", s.force) // debug: /_force?offline=1|0|clear
	mux.HandleFunc("/", s.serve)
	return mux
}

// StatsSnapshot returns a copy of counters.
func (s *Server) StatsSnapshot() Stats {
	return Stats{
		Requests:     atomic.LoadInt64(&s.stats.Requests),
		Held:         atomic.LoadInt64(&s.stats.Held),
		HeldSeconds:  atomic.LoadInt64(&s.stats.HeldSeconds),
		Retried:      atomic.LoadInt64(&s.stats.Retried),
		PartialLost:  atomic.LoadInt64(&s.stats.PartialLost),
		InFlight:     atomic.LoadInt64(&s.stats.InFlight),
		LastActivity: atomic.LoadInt64(&s.stats.LastActivity),
	}
}

// SetThermal stores thermal readings from the menu bar app (ignored once daemon owns them).
func (s *Server) SetThermal(state string, cpuC *float64) {
	s.thermalMu.Lock()
	defer s.thermalMu.Unlock()
	if s.daemonThermal {
		return
	}
	if state != "" && state != s.thermalState {
		s.thermalState = state
		s.thermalSince = time.Now()
	}
	if cpuC != nil {
		v := *cpuC
		s.cpuC = &v
	}
}

// SetThermalDaemon stores readings from keepgoing-smc; always wins over the app.
func (s *Server) SetThermalDaemon(state string, cpuC *float64) {
	s.thermalMu.Lock()
	defer s.thermalMu.Unlock()
	s.daemonThermal = true
	if state != "" && state != s.thermalState {
		s.thermalState = state
		s.thermalSince = time.Now()
	}
	if cpuC != nil {
		v := *cpuC
		s.cpuC = &v
	}
}

// ThermalSnapshot returns the stored thermal state, CPU °C, and when state last changed.
func (s *Server) ThermalSnapshot() (state string, since time.Time, cpuC *float64) {
	s.thermalMu.RLock()
	state, since = s.thermalState, s.thermalSince
	if s.cpuC != nil {
		v := *s.cpuC
		cpuC = &v
	}
	s.thermalMu.RUnlock()
	return state, since, cpuC
}

func loopbackOnly(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host == "127.0.0.1" || host == "::1"
}

func (s *Server) wifi(w http.ResponseWriter, r *http.Request) {
	if s.WiFiBridge == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	s.WiFiBridge.HandlePOST(w, r)
}

func (s *Server) wifiRequest(w http.ResponseWriter, r *http.Request) {
	if s.WiFiBridge == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	s.WiFiBridge.HandleRequestPOST(w, r)
}

func (s *Server) wifiResult(w http.ResponseWriter, r *http.Request) {
	if s.WiFiBridge == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	s.WiFiBridge.HandleResultGET(w, r)
}

func (s *Server) night(w http.ResponseWriter, r *http.Request) {
	if s.NightBridge == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	s.NightBridge.HandlePOST(w, r)
}

func (s *Server) agent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if !loopbackOnly(r) {
		http.Error(w, "loopback only", http.StatusForbidden)
		return
	}
	if s.AgentSignals == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		PID   int    `json:"pid"`
		Tool  string `json:"tool"`
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Tool == "" || (body.State != "working" && body.State != "idle") {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	s.AgentSignals.Record(body.PID, body.Tool, body.State)
	log.Printf("[agent] %s pid=%d %s", body.Tool, body.PID, body.State)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) thermal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		State string   `json:"state"`
		CPUC  *float64 `json:"cpu_c"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	switch body.State {
	case "nominal", "fair", "serious", "critical":
		s.SetThermal(body.State, body.CPUC)
	default:
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m := map[string]any{
		"online":       s.net.Online(),
		"state_since":  s.net.Since().Format(time.RFC3339),
		"stats":        s.StatsSnapshot(),
		"routes":       s.routes,
		"hold_max_sec": int(s.holdMax.Seconds()),
	}
	if s.Extra != nil {
		for k, v := range s.Extra() {
			m[k] = v
		}
	}
	if state, since, cpuC := s.ThermalSnapshot(); state != "" {
		m["thermal"] = state
		if !since.IsZero() {
			m["thermal_since"] = since.Format(time.RFC3339)
		}
		if cpuC != nil {
			m["cpu_c"] = *cpuC
		}
	}
	_ = json.NewEncoder(w).Encode(m)
}

func (s *Server) force(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("offline") {
	case "1":
		v := false
		s.net.Force(&v)
	case "0":
		v := true
		s.net.Force(&v)
	default:
		s.net.Force(nil)
	}
	log.Printf("[net] force=%q", r.URL.Query().Get("offline"))
	s.status(w, r)
}

func (s *Server) match(path string) (Route, string, bool) {
	for _, rt := range s.routes {
		if path == rt.Prefix || strings.HasPrefix(path, rt.Prefix+"/") {
			return rt, strings.TrimPrefix(path, rt.Prefix), true
		}
	}
	return Route{}, "", false
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	rt, rest, ok := s.match(r.URL.Path)
	if !ok {
		http.Error(w, `{"error":"keepgoing: unknown route prefix; use /anthropic, /openai or /chatgpt"}`, http.StatusNotFound)
		return
	}
	atomic.AddInt64(&s.stats.Requests, 1)
	atomic.AddInt64(&s.stats.InFlight, 1)
	defer atomic.AddInt64(&s.stats.InFlight, -1)
	atomic.StoreInt64(&s.stats.LastActivity, time.Now().Unix())

	body, err := io.ReadAll(io.LimitReader(r.Body, s.maxBody+1))
	if err != nil || int64(len(body)) > s.maxBody {
		http.Error(w, `{"error":"keepgoing: request body too large or unreadable"}`, http.StatusRequestEntityTooLarge)
		return
	}
	if rest == "" {
		rest = "/"
	}
	target := rt.Upstream + rest
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	up, err := url.Parse(target)
	if err != nil {
		http.Error(w, `{"error":"keepgoing: bad target"}`, http.StatusBadGateway)
		return
	}

	start := time.Now()
	ctx := r.Context()
	holdCtx, cancel := context.WithTimeout(ctx, s.holdMax)
	defer cancel()

	var resp *http.Response
	attempt := 0
	heldTotal := time.Duration(0)
	for {
		// 1. park while offline. Nothing has been sent upstream, so this costs zero tokens.
		if !s.net.Online() {
			if attempt == 0 {
				atomic.AddInt64(&s.stats.Held, 1)
			}
			h0 := time.Now()
			log.Printf("[proxy] hold %s %s (offline)", r.Method, rt.Prefix+rest)
			if !s.net.WaitOnline(holdCtx) {
				heldTotal += time.Since(h0)
				atomic.AddInt64(&s.stats.HeldSeconds, int64(heldTotal.Seconds()))
				s.fail(w, ctx, "held past hold-max, network still down", heldTotal)
				return
			}
			heldTotal += time.Since(h0)
			log.Printf("[proxy] release after %s", heldTotal.Round(time.Second))
		}
		attempt++

		// 2. forward.
		req, _ := http.NewRequestWithContext(ctx, r.Method, up.String(), bytes.NewReader(body))
		copyHeaders(req.Header, r.Header)
		req.Host = up.Host
		req.ContentLength = int64(len(body))

		resp, err = s.client.Do(req)
		if err == nil {
			break
		}
		// 3. failed before any response byte: safe to retry. The upstream never
		//    started generating, so nothing was billed.
		if ctx.Err() != nil {
			log.Printf("[proxy] client gone during dial: %v", ctx.Err())
			return
		}
		atomic.AddInt64(&s.stats.Retried, 1)
		log.Printf("[proxy] upstream dial failed (attempt %d): %v — re-checking network", attempt, err)
		s.net.MarkOffline()
		select {
		case <-holdCtx.Done():
			atomic.AddInt64(&s.stats.HeldSeconds, int64(heldTotal.Seconds()))
			s.fail(w, ctx, "upstream unreachable past hold-max", heldTotal)
			return
		case <-time.After(2 * time.Second):
		}
	}
	defer resp.Body.Close()
	if heldTotal > 0 {
		atomic.AddInt64(&s.stats.HeldSeconds, int64(heldTotal.Seconds()))
	}

	// 4. stream response back. From here on a drop is NOT retryable: tokens
	//    were generated. We surface the truncation instead of silently re-sending.
	copyHeaders(w.Header(), resp.Header)
	w.Header().Set("X-Keepgoing-Held-Ms", fmt.Sprint(heldTotal.Milliseconds()))
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32<<10)
	var written int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				log.Printf("[proxy] client closed mid-stream after %d bytes", written)
				return
			}
			written += int64(n)
			if flusher != nil {
				flusher.Flush()
			}
		}
		if rerr != nil {
			if !errors.Is(rerr, io.EOF) {
				atomic.AddInt64(&s.stats.PartialLost, 1)
				log.Printf("[proxy] upstream stream cut after %d bytes: %v (agent will see truncated response)", written, rerr)
				// hint the network watcher; the next request will hold rather than fail
				s.net.MarkOffline()
			}
			break
		}
	}
	log.Printf("[proxy] %s %s → %d in %s (held %s, %d bytes)",
		r.Method, rt.Prefix+rest, resp.StatusCode, time.Since(start).Round(time.Millisecond),
		heldTotal.Round(time.Second), written)
}

func (s *Server) fail(w http.ResponseWriter, ctx context.Context, msg string, held time.Duration) {
	if ctx.Err() != nil {
		return // client already gone
	}
	log.Printf("[proxy] giving up: %s (held %s)", msg, held.Round(time.Second))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "30")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]string{"type": "keepgoing_offline", "message": "keepgoing: " + msg},
	})
}

var hopByHop = map[string]bool{
	"Connection": true, "Keep-Alive": true, "Proxy-Authenticate": true,
	"Proxy-Authorization": true, "Te": true, "Trailer": true,
	"Transfer-Encoding": true, "Upgrade": true, "Accept-Encoding": true,
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		if hopByHop[k] {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
