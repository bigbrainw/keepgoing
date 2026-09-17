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

// Recovery schedule (offline duration thresholds).
const (
	bounceDelay        = 20 * time.Second  // t+20s: bounce radio once
	autoJoinWaitStart  = 30 * time.Second  // t+30s: start Instant Hotspot window
	firstJoinDelay     = 120 * time.Second // t+120s: first join attempt
	secondJoinDelay    = 240 * time.Second // t+240s: second join attempt
	repeatJoinInterval = 4 * time.Minute   // every 4 min after t+240s
	radioBounceInterval = 10 * time.Minute // bounce radio again only every 10 min
	joinVerifyTimeout  = 25 * time.Second  // poll reachability after join
	appJoinTimeout     = 30 * time.Second  // wait for app before networksetup fallback
)

// Keeper drives recovery actions with a schedule that respects Instant Hotspot.
type Keeper struct {
	dev      string
	ssid     string
	dryRun   bool
	bridge   *AppBridge
	onlineFn func() bool
	now      func() time.Time

	// per-offline-episode state
	bouncedOnce     bool
	waitLogged      bool
	joinAttempts    int
	lastJoinTry     time.Time
	lastBounce      time.Time
	pendingJoinID   int
	pendingJoinAt   time.Time
}

// New builds a Keeper. ssid may be empty (bounce only). bridge may be nil.
func New(ssid string, dryRun bool, bridge *AppBridge) *Keeper {
	return &Keeper{
		dev:    device(),
		ssid:   ssid,
		dryRun: dryRun,
		bridge: bridge,
		now:    time.Now,
	}
}

// SetOnlineChecker sets the reachability probe used to verify joins.
func (k *Keeper) SetOnlineChecker(fn func() bool) { k.onlineFn = fn }

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

// LastAction returns the last Wi-Fi recovery action and error for /_status.
func (k *Keeper) LastAction() (action, err string, at time.Time) {
	if k.bridge != nil {
		action, err, at, _ = k.bridge.Status()
	}
	return action, err, at
}

// Tick is called by the daemon on every probe. offlineSince is zero when online.
func (k *Keeper) Tick(ctx context.Context, online bool, offlineSince time.Time) {
	if online {
		k.reset()
		return
	}
	if offlineSince.IsZero() {
		return
	}
	off := k.now().Sub(offlineSince)

	if k.pendingJoinID > 0 {
		k.pollAppJoin(ctx)
		return
	}

	if !k.bouncedOnce && off >= bounceDelay {
		k.bounce(ctx)
		k.bouncedOnce = true
		k.lastBounce = k.now()
		return
	}

	if off >= autoJoinWaitStart && off < firstJoinDelay {
		if !k.waitLogged {
			log.Printf("[wifi] waiting for macOS to auto-join (Ask to join hotspots = Automatically)")
			k.waitLogged = true
		}
		return
	}

	if k.shouldTryJoin(off) {
		k.startJoin(ctx)
		return
	}

	if k.bouncedOnce && k.lastBounce.IsZero() == false &&
		k.now().Sub(k.lastBounce) >= radioBounceInterval {
		k.bounce(ctx)
		k.lastBounce = k.now()
	}
}

func (k *Keeper) reset() {
	k.bouncedOnce = false
	k.waitLogged = false
	k.joinAttempts = 0
	k.lastJoinTry = time.Time{}
	k.lastBounce = time.Time{}
	k.pendingJoinID = 0
	k.pendingJoinAt = time.Time{}
}

func nextJoinThreshold(attempts int) time.Duration {
	if attempts == 0 {
		return firstJoinDelay
	}
	return secondJoinDelay + time.Duration(attempts-1)*repeatJoinInterval
}

func (k *Keeper) shouldTryJoin(off time.Duration) bool {
	if k.ssid == "" {
		return false
	}
	return off >= nextJoinThreshold(k.joinAttempts)
}

func (k *Keeper) startJoin(ctx context.Context) {
	k.joinAttempts++
	k.lastJoinTry = k.now()
	if k.bridge != nil {
		log.Printf("[wifi] still offline: requesting app join %q", k.ssid)
		k.pendingJoinID = k.bridge.RequestJoin(k.ssid)
		k.pendingJoinAt = k.now()
		return
	}
	k.joinHotspot(ctx)
}

func (k *Keeper) pollAppJoin(ctx context.Context) {
	if k.bridge == nil {
		k.pendingJoinID = 0
		return
	}
	if res, ok := k.bridge.Result(k.pendingJoinID); ok {
		k.pendingJoinID = 0
		if res.OK {
			log.Printf("[wifi] app join %q ok", k.ssid)
			if k.waitOnline(ctx, joinVerifyTimeout) {
				log.Printf("[wifi] join %q ok", k.ssid)
			}
		} else {
			log.Printf("[wifi] app join %q failed: %s", k.ssid, firstLine(res.Error))
			log.Printf("[wifi] falling back to networksetup join %q", k.ssid)
			k.joinHotspot(ctx)
		}
		return
	}
	if k.now().Sub(k.pendingJoinAt) >= appJoinTimeout {
		log.Printf("[wifi] app join %q timed out; falling back to networksetup", k.ssid)
		k.pendingJoinID = 0
		k.joinHotspot(ctx)
	}
}

func (k *Keeper) bounce(ctx context.Context) {
	log.Printf("[wifi] offline: bouncing %s radio", k.dev)
	k.setAction("bounce", "")
	k.run(ctx, "networksetup", "-setairportpower", k.dev, "off")
	time.Sleep(2 * time.Second)
	k.run(ctx, "networksetup", "-setairportpower", k.dev, "on")
}

func (k *Keeper) joinHotspot(ctx context.Context) {
	pw, err := Password(k.ssid)
	if err != nil {
		msg := fmt.Sprintf("no password in keychain (%v); run `keepgoing hotspot set`", err)
		log.Printf("[wifi] hotspot %q: %s", k.ssid, msg)
		k.setAction("join", msg)
		return
	}
	log.Printf("[wifi] still offline: joining hotspot %q", k.ssid)
	k.setAction("join", "")
	out, runErr := k.run(ctx, "networksetup", "-setairportnetwork", k.dev, k.ssid, pw)
	if runErr != nil {
		line := firstLine(runErr.Error())
		if out != "" {
			line = firstLine(out)
		}
		log.Printf("[wifi] join %q failed: %s", k.ssid, line)
		k.setAction("join", line)
		return
	}
	if k.waitOnline(ctx, joinVerifyTimeout) {
		log.Printf("[wifi] join %q ok", k.ssid)
		k.setAction("join_ok", "")
	} else {
		k.setAction("join", "still offline after join")
	}
}

func (k *Keeper) waitOnline(ctx context.Context, timeout time.Duration) bool {
	if k.onlineFn == nil {
		return false
	}
	deadline := k.now().Add(timeout)
	for k.now().Before(deadline) {
		if k.onlineFn() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(1 * time.Second):
		}
	}
	return false
}

func (k *Keeper) setAction(action, err string) {
	if k.bridge != nil {
		k.bridge.setStatus(action, err)
	}
}

func (k *Keeper) run(ctx context.Context, name string, args ...string) (string, error) {
	if k.dryRun {
		shown := args
		if name == "networksetup" && len(args) == 4 && args[0] == "-setairportnetwork" {
			shown = append(append([]string{}, args[:3]...), "<password>")
		}
		log.Printf("[wifi] dry-run: %s %s", name, strings.Join(shown, " "))
		return "", nil
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, name, args...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s != "" {
			return s, fmt.Errorf("%s", firstLine(s))
		}
		return s, err
	}
	if joinOutputFailed(s) {
		return s, fmt.Errorf("%s", firstLine(s))
	}
	return s, nil
}

func joinOutputFailed(output string) bool {
	lower := strings.ToLower(output)
	for _, p := range []string{"could not find network", "failed to join", "error"} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
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
