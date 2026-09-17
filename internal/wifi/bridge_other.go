//go:build !darwin

package wifi

import (
	"net/http"
	"time"
)

// AppRequest is a no-op off macOS.
type AppRequest struct {
	ID   int    `json:"id"`
	Join string `json:"join,omitempty"`
	Scan string `json:"scan,omitempty"`
}

// AppResult is a no-op off macOS.
type AppResult struct {
	ID      int    `json:"id"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Visible  bool   `json:"visible,omitempty"`
	RSSI     int    `json:"rssi,omitempty"`
	Location string `json:"location,omitempty"`
}

// AppBridge is a no-op off macOS.
type AppBridge struct{}

func NewAppBridge() *AppBridge { return &AppBridge{} }
func (b *AppBridge) Status() (string, string, time.Time, *AppRequest) { return "", "", time.Time{}, nil }
func (b *AppBridge) RequestJoin(string) int { return 0 }
func (b *AppBridge) RequestScan(string) int { return 0 }
func (b *AppBridge) Result(int) (AppResult, bool) { return AppResult{}, false }
func (b *AppBridge) WaitResult(int, time.Duration) (AppResult, bool) { return AppResult{}, false }
func (b *AppBridge) HandlePOST(http.ResponseWriter, *http.Request) {}
func (b *AppBridge) HandleRequestPOST(http.ResponseWriter, *http.Request) {}
func (b *AppBridge) HandleResultGET(http.ResponseWriter, *http.Request) {}
