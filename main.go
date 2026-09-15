// keepgoing keeps your Mac awake and online while coding agents (Claude Code,
// Codex, Cursor) run — so remote control from your phone keeps working.
//
//	keepgoing daemon            what `install` runs: awake + wifi + proxy
//	keepgoing install           register with launchd (runs at login, restarts)
//	keepgoing status            one-line state
//	keepgoing hotspot set SSID  store hotspot password in Keychain
//	keepgoing run -- <agent>    optional: wrap one agent with resume-on-crash
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/elijah/keepgoing/internal/agents"
	"github.com/elijah/keepgoing/internal/awake"
	"github.com/elijah/keepgoing/internal/config"
	"github.com/elijah/keepgoing/internal/lid"
	"github.com/elijah/keepgoing/internal/netwatch"
	"github.com/elijah/keepgoing/internal/procwatch"
	"github.com/elijah/keepgoing/internal/proxy"
	"github.com/elijah/keepgoing/internal/runner"
	"github.com/elijah/keepgoing/internal/screen"
	"github.com/elijah/keepgoing/internal/tunnel"
	"github.com/elijah/keepgoing/internal/wifi"
)

const label = "com.elijah.keepgoing"

var version = "dev"

type cfg struct {
	listen      string
	connect     string
	holdMax     time.Duration
	probeEvery  time.Duration
	maxRestarts int
	backoff     time.Duration
	agent       string
	noAwake     bool
	always      bool
	idleGrace   time.Duration
	wifiDry     bool
	noWifi      bool
}

func main() {
	log.SetFlags(log.Ltime)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	sub, rest := os.Args[1], os.Args[2:]
	saved, _ := config.Load()

	fs := flag.NewFlagSet(sub, flag.ExitOnError)
	c := cfg{}
	fs.StringVar(&c.listen, "listen", or(saved.Listen, "127.0.0.1:7777"), "HTTP holding proxy address (Claude Code)")
	fs.StringVar(&c.connect, "connect", or(saved.Connect, "127.0.0.1:7778"), "CONNECT tunnel address (Codex)")
	fs.DurationVar(&c.holdMax, "hold-max", 45*time.Minute, "max time to park a request while offline")
	fs.DurationVar(&c.probeEvery, "probe", 3*time.Second, "network probe interval")
	fs.IntVar(&c.maxRestarts, "max-restarts", 20, "run: resume attempts after crashes")
	fs.DurationVar(&c.backoff, "backoff", 5*time.Second, "run: delay before resume")
	fs.StringVar(&c.agent, "agent", "", "run: claude|codex (default: detect)")
	fs.BoolVar(&c.noAwake, "no-awake", false, "never inhibit sleep")
	fs.BoolVar(&c.always, "always", saved.AlwaysAwake, "daemon: inhibit sleep even with no agent running")
	fs.DurationVar(&c.idleGrace, "idle-grace", 5*time.Minute, "daemon: keep awake this long after last agent exits")
	fs.BoolVar(&c.wifiDry, "wifi-dry-run", false, "daemon: log wifi actions instead of doing them")
	fs.BoolVar(&c.noWifi, "no-wifi", false, "daemon: disable wifi recovery")

	var cmd []string
	for i, a := range rest {
		if a == "--" {
			cmd = rest[i+1:]
			rest = rest[:i]
			break
		}
	}
	_ = fs.Parse(rest)
	base := "http://" + c.listen
	tun := "http://" + c.connect

	switch sub {
	case "daemon":
		os.Exit(cmdDaemon(c, saved))
	case "install":
		os.Exit(cmdInstall())
	case "uninstall":
		os.Exit(cmdUninstall())
	case "status":
		os.Exit(cmdStatus(base))
	case "version":
		fmt.Println(version)
		os.Exit(0)
	case "hotspot":
		os.Exit(cmdHotspot(fs.Args(), saved))
	case "lid":
		os.Exit(cmdLid(fs.Args(), saved))
	case "screen":
		os.Exit(cmdScreen(fs.Args(), saved))
	case "run":
		if len(cmd) == 0 {
			fmt.Fprintln(os.Stderr, "keepgoing run: missing command after --")
			os.Exit(2)
		}
		os.Exit(cmdRun(c, base, tun, cmd))
	case "proxy":
		os.Exit(cmdProxy(c, base, tun))
	case "env":
		k := agents.Kind(c.agent)
		if k == agents.Unknown {
			k = agents.Claude
		}
		for _, e := range agents.Env(k, base, tun) {
			fmt.Println("export " + e)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func usage() {
	fmt.Fprint(os.Stderr, `keepgoing — keep the Mac awake + online while your coding agents run

  keepgoing install                 run at login via launchd (awake + wifi + proxy)
  keepgoing uninstall
  keepgoing status                  agents, awake, network, wifi, proxy stats
  keepgoing version                 print release version
  keepgoing hotspot set <SSID>      store hotspot password in Keychain; auto-join when offline
  keepgoing lid enable|disable|status|install-script   keep running with the lid closed (one-time admin password)
  keepgoing screen off-after <seconds|0> | status   turn display off after idle while agents run
  keepgoing daemon [flags]          foreground daemon (what install runs)
  keepgoing env [-agent claude|codex]   exports to route an agent through the holding proxy
  keepgoing run [flags] -- claude -p "task" --dangerously-skip-permissions
                                    optional: wrap one agent, resume it on crash

daemon flags: -always -idle-grace 5m -no-wifi -wifi-dry-run -no-awake -listen -connect -hold-max -probe
`)
}

// ---- shared core: netwatch + proxy + tunnel -------------------------------

type core struct {
	nw   *netwatch.Watcher
	px   *proxy.Server
	stop func()
}

func startCore(ctx context.Context, c cfg) (*core, error) {
	targets := []string{"api.anthropic.com:443", "api.openai.com:443", "chatgpt.com:443"}
	nw := netwatch.New(targets, c.probeEvery, nil)
	go nw.Run(ctx)

	px := proxy.New(proxy.DefaultRoutes, nw, c.holdMax)
	ln, err := net.Listen("tcp", c.listen)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w (already running? `keepgoing status`)", c.listen, err)
	}
	srv := &http.Server{Handler: px.Handler()}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[proxy] serve: %v", err)
		}
	}()
	log.Printf("[proxy] listening on %s (hold-max %s)", c.listen, c.holdMax)

	tn := tunnel.New(nw, c.holdMax, []string{"chatgpt.com", "api.openai.com", "api.anthropic.com"})
	tln, err := net.Listen("tcp", c.connect)
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("listen %s: %w", c.connect, err)
	}
	go tn.Serve(ctx, tln)
	log.Printf("[tunnel] CONNECT proxy on %s", c.connect)

	stop := func() {
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}
	return &core{nw: nw, px: px, stop: stop}, nil
}

func signalCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
		<-ch
		os.Exit(130)
	}()
	return ctx, cancel
}

// ---- daemon ----------------------------------------------------------------

func cmdDaemon(c cfg, saved config.Config) int {
	ctx, cancel := signalCtx()
	defer cancel()
	co, err := startCore(ctx, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "keepgoing:", err)
		return 1
	}
	defer co.stop()

	var wk *wifi.Keeper
	if !c.noWifi {
		wk = wifi.New(saved.HotspotSSID, c.wifiDry)
		log.Printf("[wifi] keeper on %s (hotspot=%q dry-run=%v)", wk.Device(), saved.HotspotSSID, c.wifiDry)
	}

	var holder *awake.Holder
	var lastSeen time.Time
	var procs []procwatch.Proc
	awakeSince := time.Time{}
	var screenFired bool
	var lastIdle float64

	// lid mode: flip pmset disablesleep together with the awake assertion.
	lidOK := saved.LidMode && lid.Available()
	if saved.LidMode && !lidOK {
		log.Printf("[lid] lid mode set but sudoers rule missing — run `keepgoing lid enable`")
	}
	setLid := func(disable bool) {
		if !lidOK {
			return
		}
		if err := lid.Set(disable); err != nil {
			log.Printf("[lid] %v", err)
		} else if disable {
			log.Printf("[lid] lid-closed sleep disabled while agents run")
		} else {
			log.Printf("[lid] lid-closed sleep re-enabled")
		}
	}
	if lidOK {
		// crash recovery: never leave disablesleep stuck on with no agents
		setLid(false)
	}

	co.px.Extra = func() map[string]any {
		m := map[string]any{
			"version":        version,
			"agents":         procwatch.Summary(procs),
			"agent_procs":    procs,
			"awake":          holder != nil,
			"always_awake":   c.always,
			"lid_mode":       saved.LidMode,
			"lid_ready":      lidOK,
			"sleep_disabled": lid.SleepDisabled(),
			"screen_off_after": saved.ScreenOffAfter,
			"idle_seconds":     lastIdle,
		}
		if holder != nil {
			m["awake_since"] = awakeSince.Format(time.RFC3339)
		}
		if wk != nil {
			m["wifi_device"] = wk.Device()
			m["wifi_ssid"] = wk.SSID()
			m["hotspot"] = saved.HotspotSSID
		}
		return m
	}

	release := func() {
		if holder != nil {
			holder.Release()
			holder = nil
		}
		setLid(false)
	}
	defer release()

	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	log.Printf("[daemon] up: always=%v idle-grace=%s lid=%v", c.always, c.idleGrace, lidOK)
	prev := ""
	for {
		ps, err := procwatch.Scan()
		if err != nil {
			log.Printf("[daemon] scan: %v", err)
		}
		procs = ps
		sum := procwatch.Summary(ps)
		if sum != prev {
			log.Printf("[daemon] agents: %s", sum)
			prev = sum
		}
		if len(ps) > 0 {
			lastSeen = time.Now()
		}
		want := !c.noAwake && (c.always || (!lastSeen.IsZero() && time.Since(lastSeen) < c.idleGrace))
		switch {
		case want && holder == nil:
			holder = awake.Hold()
			awakeSince = time.Now()
			setLid(true)
		case want && holder != nil && lidOK && !lid.SleepDisabled():
			setLid(true) // someone flipped it back; reassert while agents run
		case !want && holder != nil:
			log.Printf("[daemon] no agents for %s, releasing sleep inhibit", c.idleGrace)
			release()
		}
		if idle, err := screen.IdleSeconds(); err != nil {
			log.Printf("[screen] idle: %v", err)
		} else {
			lastIdle = idle
			threshold := float64(saved.ScreenOffAfter)
			if idle < threshold {
				screenFired = false
			} else if screen.ShouldSleep(holder != nil, idle, threshold, screenFired) {
				if err := screen.SleepNow(); err != nil {
					log.Printf("[screen] displaysleepnow: %v", err)
				} else {
					log.Printf("[screen] display off after %.0fs idle", idle)
					screenFired = true
				}
			}
		}
		if wk != nil {
			var since time.Time
			if !co.nw.Online() {
				since = co.nw.Since()
			}
			wk.Tick(ctx, co.nw.Online(), since)
		}
		select {
		case <-ctx.Done():
			log.Printf("[daemon] shutting down")
			return 0
		case <-tick.C:
		}
	}
}

// ---- status ----------------------------------------------------------------

func cmdStatus(base string) int {
	resp, err := http.Get(base + "/_status")
	if err != nil {
		fmt.Println("keepgoing: daemon not running (" + err.Error() + ")")
		fmt.Println("start:  keepgoing install   or   keepgoing daemon")
		return 1
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Println(string(b))
	return 0
}

// ---- lid -------------------------------------------------------------------

func kickDaemon() {
	_ = exec.Command("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", os.Getuid(), label)).Run()
}

func cmdLid(args []string, saved config.Config) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: keepgoing lid enable|disable|status|install-script")
		return 2
	}
	switch args[0] {
	case "install-script":
		fmt.Print(lid.InstallScript)
		return 0
	case "enable":
		if !lid.Available() {
			fmt.Println("One-time setup: a sudoers rule so the daemon can run exactly these two commands without a password:")
			fmt.Println("  /usr/bin/pmset -a disablesleep 1")
			fmt.Println("  /usr/bin/pmset -a disablesleep 0")
			fmt.Println("Written to " + lid.SudoersPath + " after visudo validation. Admin password required.")
			cmd := exec.Command("sudo", "sh", "-c", lid.InstallScript)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "install failed:", err)
				return 1
			}
		}
		saved.LidMode = true
		if err := config.Save(saved); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		kickDaemon()
		fmt.Println("lid mode ON: while agents run, closing the lid no longer sleeps the Mac. Off again 5 min after the last agent exits.")
		fmt.Println("Heads-up: a closed laptop under load gets warm; keep it on a surface, not in a bag, and prefer plugged in.")
		return 0
	case "disable":
		saved.LidMode = false
		if err := config.Save(saved); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_ = lid.Set(false)
		kickDaemon()
		fmt.Println("lid mode OFF. Sudoers rule kept; remove with: sudo rm " + lid.SudoersPath)
		return 0
	case "status":
		fmt.Printf("lid_mode=%v sudoers_ready=%v sleep_disabled_now=%v\n", saved.LidMode, lid.Available(), lid.SleepDisabled())
		return 0
	}
	fmt.Fprintln(os.Stderr, "unknown lid subcommand:", args[0])
	return 2
}

// ---- screen ---------------------------------------------------------------

func cmdScreen(args []string, saved config.Config) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: keepgoing screen off-after <seconds|0> | status")
		return 2
	}
	switch args[0] {
	case "off-after":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: keepgoing screen off-after <seconds|0>")
			return 2
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			fmt.Fprintln(os.Stderr, "seconds must be a non-negative integer")
			return 2
		}
		saved.ScreenOffAfter = n
		if err := config.Save(saved); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		kickDaemon()
		if n == 0 {
			fmt.Println("screen off when idle: disabled")
		} else {
			fmt.Printf("screen off when idle: %ds\n", n)
		}
		return 0
	case "status":
		fmt.Printf("screen_off_after=%d\n", saved.ScreenOffAfter)
		return 0
	}
	fmt.Fprintln(os.Stderr, "unknown screen subcommand:", args[0])
	return 2
}

// ---- hotspot ---------------------------------------------------------------

func cmdHotspot(args []string, saved config.Config) int {
	if len(args) < 2 || args[0] != "set" {
		fmt.Fprintln(os.Stderr, "usage: keepgoing hotspot set <SSID>   (password read from prompt)")
		if saved.HotspotSSID != "" {
			fmt.Fprintf(os.Stderr, "current: %q\n", saved.HotspotSSID)
		}
		return 2
	}
	ssid := strings.Join(args[1:], " ")
	fmt.Fprintf(os.Stderr, "password for %q (hidden): ", ssid)
	pw, err := readSecret()
	fmt.Fprintln(os.Stderr)
	if err != nil || pw == "" {
		fmt.Fprintln(os.Stderr, "no password entered")
		return 1
	}
	if err := wifi.SetPassword(ssid, pw); err != nil {
		fmt.Fprintln(os.Stderr, "keychain:", err)
		return 1
	}
	saved.HotspotSSID = ssid
	if err := config.Save(saved); err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}
	fmt.Printf("hotspot %q saved (password in Keychain, SSID in %s)\n", ssid, config.Path())
	fmt.Println("tip: iPhone → Settings → Personal Hotspot → Allow Others to Join = on; Mac → Wi-Fi → Ask to join hotspots = Automatically")
	return 0
}

func readSecret() (string, error) {
	// disable echo via stty; fall back to plain read if not a tty
	cmd := exec.Command("stty", "-echo")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
	defer func() {
		cmd := exec.Command("stty", "echo")
		cmd.Stdin = os.Stdin
		_ = cmd.Run()
	}()
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// ---- launchd ---------------------------------------------------------------

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func cmdInstall() int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	exe, _ = filepath.EvalSymlinks(exe)
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, "Library", "Logs", "keepgoing")
	_ = os.MkdirAll(logDir, 0o755)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key><array><string>%s</string><string>daemon</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ProcessType</key><string>Interactive</string>
  <key>StandardOutPath</key><string>%s/daemon.log</string>
  <key>StandardErrorPath</key><string>%s/daemon.log</string>
  <key>EnvironmentVariables</key><dict><key>PATH</key><string>/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin</string></dict>
</dict></plist>
`, label, exe, logDir, logDir)
	if err := os.MkdirAll(filepath.Dir(plistPath()), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), label)).Run()
	if err := os.WriteFile(plistPath(), []byte(plist), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	out, err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), plistPath()).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "launchctl bootstrap: %v %s\n", err, out)
		return 1
	}
	fmt.Printf("installed %s\n  binary: %s\n  log:    %s/daemon.log\n  check:  keepgoing status\n", label, exe, logDir)
	return 0
}

func cmdUninstall() int {
	_ = lid.Set(false)
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), label)).Run()
	_ = os.Remove(plistPath())
	fmt.Println("uninstalled", label)
	fmt.Println("to remove the lid sudoers rule: sudo rm " + lid.SudoersPath)
	return 0
}

// ---- run / proxy (wrapper mode) --------------------------------------------

func cmdRun(c cfg, base, tun string, cmd []string) int {
	ctx, cancel := signalCtx()
	defer cancel()

	kind := agents.Kind(c.agent)
	if kind == agents.Unknown {
		kind = agents.Detect(cmd)
	}
	if kind == agents.Unknown {
		fmt.Fprintf(os.Stderr, "keepgoing: cannot detect agent from %q; pass -agent claude|codex\n", cmd[0])
		return 2
	}
	co, err := startCore(ctx, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "keepgoing:", err)
		return 1
	}
	defer co.stop()
	if !c.noAwake {
		defer awake.Hold().Release()
	}
	log.Printf("[run] agent=%s env=%s", kind, strings.Join(agents.Env(kind, base, tun), " "))
	return runner.Run(ctx, runner.Options{
		Argv:        cmd,
		Kind:        kind,
		ProxyBase:   base,
		TunnelURL:   tun,
		MaxRestarts: c.maxRestarts,
		Backoff:     c.backoff,
		Net:         co.nw,
	})
}

func cmdProxy(c cfg, base, tun string) int {
	ctx, cancel := signalCtx()
	defer cancel()
	co, err := startCore(ctx, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "keepgoing:", err)
		return 1
	}
	defer co.stop()
	if !c.noAwake {
		defer awake.Hold().Release()
	}
	fmt.Fprintf(os.Stderr, "\nwire an agent:\n  # claude\n")
	for _, e := range agents.Env(agents.Claude, base, tun) {
		fmt.Fprintf(os.Stderr, "  export %s\n", e)
	}
	fmt.Fprintf(os.Stderr, "  # codex\n")
	for _, e := range agents.Env(agents.Codex, base, tun) {
		fmt.Fprintf(os.Stderr, "  export %s\n", e)
	}
	fmt.Fprintln(os.Stderr)
	<-ctx.Done()
	return 0
}
