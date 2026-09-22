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
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/elijah/keepgoing/internal/agentsignal"
	"github.com/elijah/keepgoing/internal/config"
	"github.com/elijah/keepgoing/internal/cmux"
	"github.com/elijah/keepgoing/internal/cool"
	"github.com/elijah/keepgoing/internal/hooks"
	"github.com/elijah/keepgoing/internal/lid"
	"github.com/elijah/keepgoing/internal/netwatch"
	"github.com/elijah/keepgoing/internal/night"
	"github.com/elijah/keepgoing/internal/power"
	"github.com/elijah/keepgoing/internal/procwatch"
	"github.com/elijah/keepgoing/internal/proxy"
	"github.com/elijah/keepgoing/internal/runner"
	"github.com/elijah/keepgoing/internal/screen"
	"github.com/elijah/keepgoing/internal/smcread"
	"github.com/elijah/keepgoing/internal/thermolog"
	"github.com/elijah/keepgoing/internal/tunnel"
	"github.com/elijah/keepgoing/internal/wifi"
)

const label = "com.elijah.keepgoing"
const appLabel = "com.elijah.keepgoing.app"

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
	saved, cfgErr := config.Load()
	if cfgErr != nil {
		if sub != "daemon" {
			fmt.Fprintf(os.Stderr, "keepgoing: config unreadable: %v\n", cfgErr)
			os.Exit(1)
		}
		log.Printf("[config] unreadable: %v (read-only)", cfgErr)
	}

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
	case "wifi":
		os.Exit(cmdWiFi(fs.Args(), saved))
	case "lid":
		os.Exit(cmdLid(fs.Args(), saved))
	case "cool":
		os.Exit(cmdCool(fs.Args(), saved))
	case "thermal":
		os.Exit(cmdThermal(fs.Args()))
	case "hooks":
		os.Exit(cmdHooks(fs.Args()))
	case "screen":
		os.Exit(cmdScreen(fs.Args(), saved))
	case "app":
		os.Exit(cmdApp(fs.Args()))
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
  keepgoing wifi test [--join]      scan for configured hotspot via the menu bar app
  keepgoing lid enable|disable|status|install-script   keep running with the lid closed (one-time admin password)
  keepgoing cool status          lid-closed cooling (Low Power Mode + efficiency cores)
  keepgoing thermal [--csv]      last 20 lid/thermal/CPU samples (or CSV path)
  keepgoing hooks install|uninstall|status   opt-in Claude/Codex working-idle hooks
  keepgoing screen off-after <seconds|0> | status   turn display off after idle while agents run
  keepgoing app login on|off|status   menu bar app at login (launchd KeepAlive)
  keepgoing daemon [flags]          foreground daemon (what install runs)
  keepgoing env [-agent claude|codex]   exports to route an agent through the holding proxy
  keepgoing run [flags] -- claude -p "task" --dangerously-skip-permissions
                                    optional: wrap one agent, resume it on crash

daemon flags: -always -idle-grace 5m -no-wifi -wifi-dry-run -no-awake -listen -connect -hold-max -probe
`)
}

// ---- shared core: netwatch + proxy + tunnel -------------------------------

type core struct {
	nw      *netwatch.Watcher
	px      *proxy.Server
	signals *agentsignal.Store
	stop    func()
}

func startCore(ctx context.Context, c cfg) (*core, error) {
	targets := []string{"api.anthropic.com:443", "api.openai.com:443", "chatgpt.com:443"}
	nw := netwatch.New(targets, c.probeEvery, nil)
	go nw.Run(ctx)

	signals := agentsignal.New()
	bridge := wifi.NewAppBridge()
	nightBridge := night.NewBridge()
	px := proxy.New(proxy.DefaultRoutes, nw, c.holdMax)
	px.AgentSignals = signals
	px.WiFiBridge = bridge
	px.NightBridge = nightBridge
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
	return &core{nw: nw, px: px, signals: signals, stop: stop}, nil
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
	var loaded config.Loaded
	if l, err := config.LoadDetailed(); err == nil {
		loaded = l
		if restored, ok := config.RestoreMissingLidMode(l); ok {
			saved = restored
		} else if !l.HasLidMode {
			saved = l.Config
		}
	}
	askAt, untilStr := saved.NightSettings(loaded)
	nightCfg := night.Settings{AskAt: askAt, Until: untilStr}
	st, _ := config.LoadState()
	var nightAns *night.Answer
	if st.NightAnswer != nil {
		nightAns = &night.Answer{
			Date:   st.NightAnswer.Date,
			Answer: st.NightAnswer.Answer,
			At:     st.NightAnswer.At,
		}
	}
	var nightMode string
	var nightAskSession string
	var nightSleepLogged bool
	batteryFloor := saved.BatteryFloorEffective(loaded)
	var batteryGuard bool
	var batteryNotified bool
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
		wk = wifi.New(saved.HotspotSSID, c.wifiDry, co.px.WiFiBridge)
		wk.SetOnlineChecker(co.nw.Online)
		log.Printf("[wifi] keeper on %s (hotspot=%q dry-run=%v)", wk.Device(), saved.HotspotSSID, c.wifiDry)
	}

	var holder *awake.Holder
	var lastSeen time.Time
	var procs []procwatch.Proc
	watch := procwatch.NewWatcher(co.signals)
	cmuxPoll := cmux.New(co.signals, saved.CmuxOn())
	awakeSince := time.Time{}
	var screenFired bool
	var lastIdle float64
	var thermalWarned string
	var thermalHotSince time.Time
	var cpuHotSince time.Time
	var thermalEpisodeNotified bool
	var lastThermalLogged string
	var lastLogAt time.Time
	var lastSMC time.Time
	var allIdleSince time.Time
	lidClosed := lid.Closed()
	coolMgr := cool.New()
	coolOn := saved.LidMode

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
	setLowPower := func(enable bool) error {
		if !lidOK {
			if enable {
				log.Printf("[cool] low power needs extended sudoers rule — run `keepgoing lid enable`")
			}
			return fmt.Errorf("sudoers rule missing")
		}
		if err := lid.SetLowPower(enable); err != nil {
			log.Printf("[cool] %v", err)
			return err
		}
		if enable {
			log.Printf("[cool] low power mode on")
		} else {
			log.Printf("[cool] low power mode off")
		}
		return nil
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
			"lid_closed":     lidClosed,
			"cool_mode":      coolOn,
			"low_power":      lid.LowPowerMode(),
			"cool_pids":      coolMgr.PIDCount(),
			"screen_off_after":   saved.ScreenOffAfter,
			"idle_sleep_after":   saved.IdleSleepAfter,
			"idle_seconds":       lastIdle,
		}
		if holder != nil {
			m["awake_since"] = awakeSince.Format(time.RFC3339)
		}
		if wk != nil {
			m["wifi_device"] = wk.Device()
			m["wifi_ssid"] = wk.SSID()
			m["hotspot"] = saved.HotspotSSID
			action, err, at := wk.LastAction()
			if action != "" {
				m["wifi_last_action"] = action
			}
			if err != "" {
				m["wifi_last_error"] = err
			}
			if !at.IsZero() {
				m["wifi_last_action_at"] = at.Format(time.RFC3339)
			}
		}
		if co.px.WiFiBridge != nil {
			_, _, _, req := co.px.WiFiBridge.Status()
			if req != nil {
				m["wifi_request"] = req
			}
		}
		if co.px.NightBridge != nil {
			if req := co.px.NightBridge.StatusRequestField(); req != nil {
				m["night_request"] = req
			}
		}
		m["night_mode"] = nightMode
		if nightAns != nil {
			m["night_answer"] = map[string]any{
				"date":   nightAns.Date,
				"answer": nightAns.Answer,
				"at":     night.AnswerAt(nightAns.At),
			}
		}
		if until := night.UntilAt(time.Now(), nightCfg); !until.IsZero() &&
			(nightMode == night.ModeRun || nightMode == night.ModeSleep) {
			m["night_until_at"] = until.Format(time.RFC3339)
		}
		m["battery_floor"] = batteryFloor
		if pct, ok := power.BatteryPercent(); ok {
			m["battery_pct"] = pct
		}
		if _, _, cpuC := co.px.ThermalSnapshot(); cpuC != nil {
			m["cpu_c"] = *cpuC
		}
		return m
	}

	appendThermalLog := func() {
		thermal, _, cpuC := co.px.ThermalSnapshot()
		if thermal == "" {
			thermal = "unknown"
		}
		if err := thermolog.Append(thermolog.Row{
			LidClosed: lidClosed,
			Thermal:   thermal,
			CPUC:      cpuC,
			LowPower:  lid.LowPowerMode(),
			CoolPIDs:  coolMgr.PIDCount(),
			Agents:    procwatch.Summary(procs),
			Working:   procwatch.WorkingCount(procs),
			OnBattery: power.OnBattery(),
		}); err != nil {
			log.Printf("[thermal] log: %v", err)
		}
		lastLogAt = time.Now()
		lastThermalLogged = thermal
	}

	release := func() {
		if holder != nil {
			holder.Release()
			holder = nil
		}
		setLid(false)
	}
	nightRelease := func(msg string) {
		if holder != nil || lidOK && lid.SleepDisabled() || lid.LowPowerMode() {
			setLowPower(false)
			release()
			if msg != "" {
				log.Printf("[night] %s", msg)
			}
		}
	}
	defer func() {
		coolMgr.Shutdown(setLowPower)
		release()
	}()

	readSMC := func() {
		sample, ok, err := smcread.Read()
		if !ok {
			return
		}
		if err != nil {
			log.Printf("[smc] %v", err)
			return
		}
		co.px.SetThermalDaemon(sample.Thermal, sample.CPUC)
	}
	readSMC()
	lastSMC = time.Now()

	co.px.NightBridge.SetOnAnswer(func(ans string) error {
		sessionDate := night.SessionDate(time.Now(), nightCfg)
		if sessionDate == "" {
			sessionDate = time.Now().Format("2006-01-02")
		}
		na := config.NightAnswer{Date: sessionDate, Answer: ans, At: time.Now().Unix()}
		if err := config.SaveNightAnswer(na); err != nil {
			return err
		}
		nightAns = &night.Answer{Date: na.Date, Answer: na.Answer, At: na.At}
		log.Printf("[night] answer %s for %s", ans, sessionDate)
		if ans == "no" {
			nightRelease(fmt.Sprintf("allowing sleep until %s", untilStr))
			nightSleepLogged = true
		} else {
			nightSleepLogged = false
		}
		return nil
	})

	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	log.Printf("[daemon] up: always=%v idle-grace=%s lid=%v", c.always, c.idleGrace, lidOK)
	if err := config.SaveLidMode(saved.LidMode); err != nil {
		log.Printf("[config] state: %v", err)
	}
	prev := ""
	for {
		cmuxPoll.Tick()
		ps, err := watch.Scan()
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
		idleSleep := saved.IdleSleepAfter > 0 && len(ps) > 0
		if idleSleep {
			if procwatch.AnyWorking(ps) {
				allIdleSince = time.Time{}
			} else if allIdleSince.IsZero() {
				allIdleSince = time.Now()
			}
		} else {
			allIdleSince = time.Time{}
		}
		idleSleepRelease := idleSleep && !allIdleSince.IsZero() &&
			time.Since(allIdleSince) >= time.Duration(saved.IdleSleepAfter)*time.Minute
		want := !c.noAwake && (c.always || (len(ps) > 0 && !idleSleepRelease && (!lastSeen.IsZero() && time.Since(lastSeen) < c.idleGrace)))

		now := time.Now()
		mode, reason := night.Decide(now, nightCfg, nightAns)
		nightMode = mode
		if mode == night.ModeSleep && len(ps) > 0 {
			want = false
			if !nightSleepLogged {
				if strings.Contains(reason, "no answer") {
					log.Printf("[night] no answer, allowing sleep until %s", untilStr)
				} else {
					log.Printf("[night] %s", reason)
				}
				nightSleepLogged = true
				co.px.NightBridge.ClearAsk()
			}
			if holder != nil || (lidOK && lid.SleepDisabled()) || lid.LowPowerMode() {
				setLowPower(false)
			}
		} else if mode != night.ModeSleep {
			nightSleepLogged = false
		}
		if mode == night.ModeAsk && len(ps) > 0 {
			sessionDate := night.SessionDate(now, nightCfg)
			if sessionDate != "" && nightAskSession != sessionDate {
				log.Printf("[night] asking whether to run overnight")
				co.px.NightBridge.BeginAsk()
				nightAskSession = sessionDate
			}
		}
		pct, hasPct := power.BatteryPercent()
		onBatt := power.OnBattery()
		if batteryFloor > 0 && hasPct && onBatt && pct <= batteryFloor {
			want = false
			if !batteryGuard {
				log.Printf("[battery] %d%% ≤ floor, allowing sleep", pct)
				if !batteryNotified {
					smcread.NotifyUser(fmt.Sprintf("Battery at %d%% — allowing sleep", pct))
					batteryNotified = true
				}
				batteryGuard = true
			}
			if holder != nil || (lidOK && lid.SleepDisabled()) || lid.LowPowerMode() {
				setLowPower(false)
			}
		} else if batteryGuard {
			rearm := !onBatt
			if hasPct && pct > batteryFloor+5 {
				rearm = true
			}
			if rearm {
				batteryGuard = false
				batteryNotified = false
				if hasPct {
					log.Printf("[battery] re-armed at %d%%", pct)
				} else {
					log.Printf("[battery] re-armed on AC power")
				}
			}
		}
		switch {
		case want && holder == nil:
			holder = awake.Hold()
			awakeSince = time.Now()
			setLid(true)
		case want && holder != nil && lidOK && !lid.SleepDisabled():
			setLid(true) // someone flipped it back; reassert while agents run
		case !want && holder != nil:
			if idleSleepRelease {
				log.Printf("[daemon] all agents idle for %dm, allowing sleep", saved.IdleSleepAfter)
			} else {
				log.Printf("[daemon] no agents for %s, releasing sleep inhibit", c.idleGrace)
			}
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
		thermalLogged := false
		nowClosed := lid.Closed()
		if nowClosed != lidClosed {
			if nowClosed {
				log.Printf("[lid] closed")
				if coolOn {
					coolMgr.LidClosed(ps, setLowPower)
				}
			} else {
				log.Printf("[lid] opened")
				if coolOn {
					coolMgr.LidOpened(setLowPower)
				}
			}
			lidClosed = nowClosed
			appendThermalLog()
			thermalLogged = true
		}
		if coolOn {
			coolMgr.Tick(ps, lidClosed)
		}
		thermal, _, cpuC := co.px.ThermalSnapshot()
		if !thermalLogged && thermal != "" && thermal != lastThermalLogged {
			appendThermalLog()
			thermalLogged = true
		}
		if thermal == "serious" || thermal == "critical" {
			if thermal != thermalWarned {
				log.Printf("[thermal] %s", thermal)
				smcread.NotifyThermal(thermal)
				thermalWarned = thermal
			}
		} else {
			thermalWarned = ""
		}
		thermalHot := thermal != "" && thermal != "nominal" && thermal != "unknown"
		if thermalHot {
			if thermalHotSince.IsZero() {
				thermalHotSince = time.Now()
			}
		} else {
			thermalHotSince = time.Time{}
		}
		cpuHot := cpuC != nil && *cpuC > 90
		if cpuHot {
			if cpuHotSince.IsZero() {
				cpuHotSince = time.Now()
			}
		} else {
			cpuHotSince = time.Time{}
		}
		thermalLong := thermalHot && !thermalHotSince.IsZero() && time.Since(thermalHotSince) >= 5*time.Minute
		cpuLong := cpuHot && !cpuHotSince.IsZero() && time.Since(cpuHotSince) >= 2*time.Minute
		if (thermalLong || cpuLong) && !thermalEpisodeNotified {
			msg := "Mac is hot"
			if cpuC != nil {
				msg = fmt.Sprintf("Mac is hot — %.0f °C, throttling", *cpuC)
			} else if thermal != "" {
				msg = fmt.Sprintf("Mac is hot — %s, throttling", thermal)
			}
			log.Printf("[thermal] notify: %s", msg)
			smcread.NotifyUser(msg)
			thermalEpisodeNotified = true
		}
		if !thermalHot && !cpuHot {
			thermalEpisodeNotified = false
		}
		if lastSMC.IsZero() || time.Since(lastSMC) >= 10*time.Second {
			readSMC()
			lastSMC = time.Now()
		}
		if !thermalLogged && (lastLogAt.IsZero() || time.Since(lastLogAt) >= 30*time.Second) {
			appendThermalLog()
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
			if lid.LegacyAvailable() {
				fmt.Println("Upgrading sudoers rule to add Low Power Mode commands.")
			}
			fmt.Println("One-time setup: a sudoers rule so the daemon can run exactly these four commands without a password:")
			fmt.Println("  /usr/bin/pmset -a disablesleep 1")
			fmt.Println("  /usr/bin/pmset -a disablesleep 0")
			fmt.Println("  /usr/bin/pmset -a lowpowermode 1")
			fmt.Println("  /usr/bin/pmset -a lowpowermode 0")
			fmt.Println("Written to " + lid.SudoersPath + " after visudo validation. Admin password required.")
			cmd := exec.Command("sudo", "sh", "-c", lid.InstallScript)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "install failed:", err)
				return 1
			}
		}
		if err := config.Update(func(c *config.Config) error {
			c.LidMode = true
			return nil
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		kickDaemon()
		fmt.Println("lid mode ON: while agents run, closing the lid no longer sleeps the Mac. Off again 5 min after the last agent exits.")
		fmt.Println("Heads-up: a closed laptop under load gets warm; keep it on a surface, not in a bag, and prefer plugged in.")
		return 0
	case "disable":
		if err := config.Update(func(c *config.Config) error {
			c.LidMode = false
			return nil
		}); err != nil {
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

// ---- cool ------------------------------------------------------------------

func cmdCool(args []string, saved config.Config) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: keepgoing cool status")
		return 2
	}
	switch args[0] {
	case "status":
		coolPIDs := 0
		if resp, err := http.Get("http://" + or(saved.Listen, "127.0.0.1:7777") + "/_status"); err == nil {
			defer resp.Body.Close()
			var m map[string]any
			if json.NewDecoder(resp.Body).Decode(&m) == nil {
				if n, ok := m["cool_pids"].(float64); ok {
					coolPIDs = int(n)
				}
			}
		}
		fmt.Printf("cool_mode=%v low_power=%v cool_pids=%d\n", saved.LidMode, lid.LowPowerMode(), coolPIDs)
		return 0
	}
	fmt.Fprintln(os.Stderr, "unknown cool subcommand:", args[0])
	return 2
}

// ---- hooks -----------------------------------------------------------------

func cmdHooks(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: keepgoing hooks install|uninstall|status")
		return 2
	}
	switch args[0] {
	case "install":
		if err := hooks.InstallClaude(); err != nil {
			fmt.Fprintln(os.Stderr, "claude:", err)
			return 1
		}
		if err := hooks.InstallCodex(); err != nil {
			fmt.Fprintln(os.Stderr, "codex:", err)
			return 1
		}
		return 0
	case "uninstall":
		if err := hooks.UninstallClaude(); err != nil {
			fmt.Fprintln(os.Stderr, "claude:", err)
			return 1
		}
		if err := hooks.UninstallCodex(); err != nil {
			fmt.Fprintln(os.Stderr, "codex:", err)
			return 1
		}
		fmt.Println("removed keepgoing hooks from", hooks.ClaudeSettingsPath())
		return 0
	case "status":
		fmt.Printf("claude_hooks=%v codex_notify=%v\n", hooks.ClaudeStatus(), hooks.CodexStatus())
		return 0
	}
	fmt.Fprintln(os.Stderr, "unknown hooks subcommand:", args[0])
	return 2
}

// ---- thermal ---------------------------------------------------------------

func cmdThermal(args []string) int {
	for _, a := range args {
		if a == "--csv" {
			fmt.Println(thermolog.Path())
			return 0
		}
	}
	lines, err := thermolog.LastLines(20)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Print(thermolog.FormatTable(lines))
	if g, err := thermolog.GapReport(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	} else if g != nil {
		fmt.Print(thermolog.FormatGapLine(g))
	}
	return 0
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
		if err := config.Update(func(c *config.Config) error {
			c.ScreenOffAfter = n
			return nil
		}); err != nil {
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

const hotspotGuidance = "iPhone: Settings → Personal Hotspot → Allow Others to Join. Mac: System Settings → Wi-Fi → Ask to join hotspots → Automatically. The name must match your iPhone's name exactly."

func cmdHotspot(args []string, saved config.Config) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: keepgoing hotspot set <SSID> | password")
		return 2
	}
	if args[0] == "password" {
		if saved.HotspotSSID == "" {
			fmt.Fprintln(os.Stderr, "no hotspot configured")
			return 1
		}
		pw, err := wifi.Password(saved.HotspotSSID)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Print(pw)
		return 0
	}
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
	if err := config.Update(func(c *config.Config) error {
		c.HotspotSSID = ssid
		return nil
	}); err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}
	fmt.Printf("hotspot %q saved (password in Keychain, SSID in %s)\n", ssid, config.Path())
	fmt.Println(hotspotGuidance)
	return 0
}

// ---- wifi ------------------------------------------------------------------

func cmdWiFi(args []string, saved config.Config) int {
	if len(args) < 1 || args[0] != "test" {
		fmt.Fprintln(os.Stderr, "usage: keepgoing wifi test [--join]")
		return 2
	}
	doJoin := false
	for _, a := range args[1:] {
		if a == "--join" {
			doJoin = true
		}
	}
	if saved.HotspotSSID == "" {
		fmt.Fprintln(os.Stderr, "no hotspot configured; run `keepgoing hotspot set <SSID>`")
		return 1
	}
	base := "http://" + or(saved.Listen, "127.0.0.1:7777")
	if _, err := http.Get(base + "/_status"); err != nil {
		fmt.Fprintln(os.Stderr, "daemon not running; start KeepGoing or run `keepgoing install`")
		return 1
	}
	body := map[string]string{"scan": saved.HotspotSSID}
	if doJoin {
		body = map[string]string{"join": saved.HotspotSSID}
	}
	reqBody, _ := json.Marshal(body)
	resp, err := http.Post(base+"/_wifi/request", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Fprintln(os.Stderr, "wifi request:", err)
		return 1
	}
	defer resp.Body.Close()
	var idResp struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&idResp); err != nil || idResp.ID == 0 {
		fmt.Fprintln(os.Stderr, "wifi request: bad response")
		return 1
	}
	deadline := time.Now().Add(45 * time.Second)
	var result wifi.AppResult
	for time.Now().Before(deadline) {
		st, err := http.Get(fmt.Sprintf("%s/_wifi/result?id=%d", base, idResp.ID))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if st.StatusCode == http.StatusNotFound {
			st.Body.Close()
			time.Sleep(500 * time.Millisecond)
			continue
		}
		_ = json.NewDecoder(st.Body).Decode(&result)
		st.Body.Close()
		if result.ID == idResp.ID {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if result.ID != idResp.ID {
		fmt.Fprintln(os.Stderr, "timed out waiting for KeepGoing app (is it running?)")
		return 1
	}
	loc := result.Location
	if loc == "" {
		loc = "unknown"
	}
	fmt.Printf("location: %s\n", loc)
	if loc == "denied" {
		fmt.Println("fix: System Settings → Privacy & Security → Location Services → KeepGoing → While Using")
	}
	if doJoin {
		if result.OK {
			fmt.Printf("join %q: ok\n", saved.HotspotSSID)
		} else {
			fmt.Printf("join %q: %s\n", saved.HotspotSSID, result.Error)
			return 1
		}
		return 0
	}
	if result.Visible {
		fmt.Printf("hotspot %q: visible (RSSI %d)\n", saved.HotspotSSID, result.RSSI)
		fmt.Printf("join would use: networksetup -setairportnetwork <device> %q <password>\n", saved.HotspotSSID)
		return 0
	}
	fmt.Printf("hotspot %q: not visible\n", saved.HotspotSSID)
	if result.Error != "" {
		fmt.Println(result.Error)
	}
	return 1
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

func appPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", appLabel+".plist")
}

func keepGoingAppBin() string {
	if exe, err := os.Executable(); err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		sibling := filepath.Join(filepath.Dir(exe), "KeepGoing")
		if st, err := os.Stat(sibling); err == nil && !st.IsDir() {
			return sibling
		}
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, "Applications", "KeepGoing.app", "Contents", "MacOS", "KeepGoing"),
		"/Applications/KeepGoing.app/Contents/MacOS/KeepGoing",
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func installAppAgent() int {
	appBin := keepGoingAppBin()
	if appBin == "" {
		fmt.Fprintln(os.Stderr, "KeepGoing.app not found; install the app bundle first")
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(appPlistPath()), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key><array><string>%s</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ThrottleInterval</key><integer>10</integer>
  <key>ProcessType</key><string>Interactive</string>
  <key>LimitLoadToSessionType</key><array><string>Aqua</string></array>
</dict></plist>
`, appLabel, appBin)
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), appLabel)).Run()
	if err := os.WriteFile(appPlistPath(), []byte(plist), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	out, err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), appPlistPath()).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "launchctl bootstrap app: %v %s\n", err, out)
		return 1
	}
	_ = exec.Command("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", os.Getuid(), appLabel)).Run()
	fmt.Printf("installed %s (KeepGoing menu bar)\n", appLabel)
	return 0
}

func uninstallAppAgent() int {
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), appLabel)).Run()
	_ = os.Remove(appPlistPath())
	fmt.Println("removed", appLabel)
	return 0
}

func cmdApp(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: keepgoing app login on|off|status")
		return 2
	}
	switch args[0] {
	case "login":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: keepgoing app login on|off|status")
			return 2
		}
		switch args[1] {
		case "on":
			return installAppAgent()
		case "off":
			return uninstallAppAgent()
		case "status":
			_, err := os.Stat(appPlistPath())
			fmt.Printf("app_login=%v\n", err == nil)
			return 0
		}
	}
	fmt.Fprintln(os.Stderr, "unknown app subcommand:", args[0])
	return 2
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
	if keepGoingAppBin() != "" {
		if code := installAppAgent(); code != 0 {
			return code
		}
	}
	return 0
}

func cmdUninstall() int {
	_ = lid.Set(false)
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), label)).Run()
	_ = os.Remove(plistPath())
	uninstallAppAgent()
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
