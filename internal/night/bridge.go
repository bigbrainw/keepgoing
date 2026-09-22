package night

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Bridge coordinates overnight prompts with the menu bar app.
type Bridge struct {
	mu      sync.Mutex
	nextID  int
	pending bool
	pendingID int
	onAnswer func(answer string) error
}

// NewBridge builds a Bridge.
func NewBridge() *Bridge {
	return &Bridge{}
}

// SetOnAnswer registers a callback when the user answers via the app or CLI.
func (b *Bridge) SetOnAnswer(fn func(answer string) error) {
	b.mu.Lock()
	b.onAnswer = fn
	b.mu.Unlock()
}

// BeginAsk starts a new overnight prompt. Returns the request id for /_status.
func (b *Bridge) BeginAsk() int {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.pending = true
	b.pendingID = id
	b.mu.Unlock()
	return id
}

// ClearAsk clears a pending prompt (answered or timed out).
func (b *Bridge) ClearAsk() {
	b.mu.Lock()
	b.pending = false
	b.mu.Unlock()
}

// Pending reports whether a prompt is waiting and its id.
func (b *Bridge) Pending() (id int, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.pending {
		return 0, false
	}
	return b.pendingID, true
}

// HandlePOST handles POST /_night {"answer":"yes|no"}.
func (b *Bridge) HandlePOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Answer != "yes" && body.Answer != "no" {
		http.Error(w, "answer must be yes or no", http.StatusBadRequest)
		return
	}
	b.mu.Lock()
	fn := b.onAnswer
	b.pending = false
	b.mu.Unlock()
	if fn != nil {
		if err := fn(body.Answer); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// StatusRequest is exposed on /_status when a prompt is pending.
type StatusRequest struct {
	ID int `json:"id"`
}

// StatusRequestField returns night_request for /_status, or nil.
func (b *Bridge) StatusRequestField() *StatusRequest {
	id, ok := b.Pending()
	if !ok {
		return nil
	}
	return &StatusRequest{ID: id}
}

// AnswerAt formats answer timestamp for status JSON.
func AnswerAt(at int64) string {
	if at == 0 {
		return ""
	}
	return time.Unix(at, 0).Format(time.RFC3339)
}
