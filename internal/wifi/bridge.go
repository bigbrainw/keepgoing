//go:build darwin

package wifi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// AppRequest is exposed on /_status as wifi_request for the menu bar app.
type AppRequest struct {
	ID   int    `json:"id"`
	Join string `json:"join,omitempty"`
	Scan string `json:"scan,omitempty"`
}

// AppResult is posted by the app to /_wifi.
type AppResult struct {
	ID       int    `json:"id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Visible  bool   `json:"visible,omitempty"`
	RSSI     int    `json:"rssi,omitempty"`
	Location string `json:"location,omitempty"`
}

// AppBridge coordinates Wi-Fi scan/join requests that need Location permission.
type AppBridge struct {
	mu           sync.Mutex
	nextID       int
	pending      *AppRequest
	results      map[int]AppResult
	lastAction   string
	lastError    string
	lastActionAt time.Time
}

// NewAppBridge builds an AppBridge.
func NewAppBridge() *AppBridge {
	return &AppBridge{results: make(map[int]AppResult)}
}

// Status returns fields for /_status.
func (b *AppBridge) Status() (action, err string, at time.Time, req *AppRequest) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var r *AppRequest
	if b.pending != nil {
		cp := *b.pending
		r = &cp
	}
	return b.lastAction, b.lastError, b.lastActionAt, r
}

func (b *AppBridge) setStatus(action, err string) {
	b.mu.Lock()
	b.lastAction = action
	b.lastError = err
	b.lastActionAt = time.Now()
	b.mu.Unlock()
}

// RequestJoin asks the app to join ssid. Returns the request id.
func (b *AppBridge) RequestJoin(ssid string) int {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.pending = &AppRequest{ID: id, Join: ssid}
	b.mu.Unlock()
	b.setStatus("app_join", "")
	return id
}

// RequestScan asks the app to scan for ssid without joining.
func (b *AppBridge) RequestScan(ssid string) int {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.pending = &AppRequest{ID: id, Scan: ssid}
	b.mu.Unlock()
	b.setStatus("app_scan", "")
	return id
}

// Result returns a completed result for id, if any.
func (b *AppBridge) Result(id int) (AppResult, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.results[id]
	return r, ok
}

// WaitResult blocks until a result arrives or timeout.
func (b *AppBridge) WaitResult(id int, timeout time.Duration) (AppResult, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r, ok := b.Result(id); ok {
			return r, true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return AppResult{}, false
}

func (b *AppBridge) complete(r AppResult) {
	b.mu.Lock()
	action := "app_join"
	if b.pending != nil && b.pending.Scan != "" {
		action = "app_scan"
	}
	b.results[r.ID] = r
	if b.pending != nil && b.pending.ID == r.ID {
		b.pending = nil
	}
	b.mu.Unlock()
	if r.OK {
		b.setStatus(action, "")
	} else {
		b.setStatus(action, r.Error)
	}
}

// HandlePOST handles POST /_wifi from the menu bar app.
func (b *AppBridge) HandlePOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body AppResult
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.ID == 0 {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	b.complete(body)
	w.WriteHeader(http.StatusNoContent)
}

// HandleResultGET handles GET /_wifi/result?id=N.
func (b *AppBridge) HandleResultGET(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	res, ok := b.Result(id)
	if !ok {
		http.Error(w, "pending", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// HandleRequestPOST handles POST /_wifi/request from the CLI (scan or join test).
func (b *AppBridge) HandleRequestPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Scan string `json:"scan"`
		Join string `json:"join"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	var id int
	switch {
	case body.Scan != "":
		id = b.RequestScan(body.Scan)
	case body.Join != "":
		id = b.RequestJoin(body.Join)
	default:
		http.Error(w, "scan or join required", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"id": id})
}
