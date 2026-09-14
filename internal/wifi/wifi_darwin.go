//go:build darwin

// Package wifi keeps the Wi-Fi link up on macOS: bounce the radio when the
// network is gone, and force-join a configured hotspot if that doesn't help.
package wifi

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

const keychainService = "keepgoing-hotspot"

// Keeper drives recovery actions with backoff.
type Keeper struct {
	dev         string
	ssid        string
	dryRun      bool
	offlineFor  time.Duration // how long offline before acting
	lastAction  time.Time
	stage       int
	minInterval time.Duration
}

// New builds a Keeper. ssid may be empty (bounce only).
func New(ssid string, dryRun bool) *Keeper {
	return &Keeper{
		dev:         device(),
		ssid:        ssid,
		dryRun:      dryRun,
		offlineFor:  20 * time.Second,
		minInterval: 45 * time.Second,
	}
}

// Device returns the Wi-Fi interface name.
func (k *Keeper) Device() string { return k.dev }

// SSID returns the currently associated network, or "".
func (k *Keeper) SSID() string {
	out, _ := exec.Command("ipconfig", "getsummary", k.dev).Output()
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.Contains(ln, " SSID :") {
			return strings.TrimSpace(strings.SplitN(ln, ":", 2)[1])
		}
	}
	return ""
}

// Tick is called by the daemon on every probe. offlineSince is zero when
// online. Actions escalate: bounce radio → join hotspot → bounce again…
func (k *Keeper) Tick(ctx context.Context, online bool, offlineSince time.Time) {
	if online {
		k.stage = 0
		return
	}
	if time.Since(offlineSince) < k.offlineFor || time.Since(k.lastAction) < k.minInterval {
		return
	}
	k.lastAction = time.Now()
	switch {
	case k.stage%2 == 0:
		k.bounce(ctx)
	case k.ssid != "":
		k.joinHotspot(ctx)
	default:
		k.bounce(ctx)
	}
	k.stage++
}

func (k *Keeper) bounce(ctx context.Context) {
	log.Printf("[wifi] offline: bouncing %s radio", k.dev)
	k.run(ctx, "networksetup", "-setairportpower", k.dev, "off")
	time.Sleep(2 * time.Second)
	k.run(ctx, "networksetup", "-setairportpower", k.dev, "on")
}

func (k *Keeper) joinHotspot(ctx context.Context) {
	pw, err := Password(k.ssid)
	if err != nil {
		log.Printf("[wifi] hotspot %q: no password in keychain (%v); run `keepgoing hotspot set`", k.ssid, err)
		return
	}
	log.Printf("[wifi] still offline: joining hotspot %q", k.ssid)
	k.run(ctx, "networksetup", "-setairportnetwork", k.dev, k.ssid, pw)
}

func (k *Keeper) run(ctx context.Context, name string, args ...string) {
	if k.dryRun {
		shown := args
		if name == "networksetup" && len(args) == 4 && args[0] == "-setairportnetwork" {
			shown = append(append([]string{}, args[:3]...), "<password>")
		}
		log.Printf("[wifi] dry-run: %s %s", name, strings.Join(shown, " "))
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, name, args...).CombinedOutput()
	if err != nil {
		log.Printf("[wifi] %s failed: %v %s", name, err, strings.TrimSpace(string(out)))
	}
}

// Password reads the hotspot password from the login keychain.
func Password(ssid string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", keychainService, "-a", ssid, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain lookup failed")
	}
	return strings.TrimSpace(string(out)), nil
}

// SetPassword stores the hotspot password in the login keychain.
func SetPassword(ssid, pw string) error {
	out, err := exec.Command("security", "add-generic-password", "-U", "-s", keychainService, "-a", ssid, "-w", pw).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func device() string {
	out, err := exec.Command("networksetup", "-listallhardwareports").Output()
	if err != nil {
		return "en0"
	}
	lines := strings.Split(string(out), "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "Wi-Fi") && i+1 < len(lines) {
			f := strings.Fields(lines[i+1])
			if len(f) == 2 {
				return f[1]
			}
		}
	}
	return "en0"
}
