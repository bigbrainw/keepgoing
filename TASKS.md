# keepgoing — task queue for Cursor agent

**Phase 6: screen off while agents run.** Daemon + one menu item. Go + Swift.

## Hard rules
- No sudo/pmset with -a, no visudo, no Wi-Fi bounce. `pmset displaysleepnow` is allowed (no root) but do NOT run it interactively while testing — Elijah is using the Mac. Test the logic with the threshold set to a huge value or by unit-testing the decision function.
- Build `./app/build.sh`; bundle binary stays `keepgoing-cli`. Test daemons on ports 7790/7791 `-no-wifi`.
- Deploy: `rm -rf ~/Applications/KeepGoing.app && cp -R dist/KeepGoing.app ~/Applications/ && launchctl kickstart -k gui/$(id -u)/com.elijah.keepgoing && open ~/Applications/KeepGoing.app`.
- Follow `~/.cursor/skills/native-mac-polish/SKILL.md` for strings. Commit per task, push, no history rewrite.

## Facts (verified on this Mac)
- `pmset -g` currently prints `displaysleep 0 (display sleep prevented by caffeinate)` — because `internal/awake/awake.go` runs `caffeinate -dims`. `-d` blocks display sleep. We only need system sleep blocked.
- User idle time: `ioreg -c IOHIDSystem | awk '/HIDIdleTime/ {print $NF; exit}'` → nanoseconds.
- `pmset displaysleepnow` turns the display off immediately without root; any key/mouse wakes it. `IODisplayWrangler` power state is NOT readable on macOS 26, so don't try to read display state — use idle time + a one-shot guard.

## Tasks

### 1. Stop blocking display sleep
`awake.go`: `caffeinate -ims` (drop `-d`). Keep `-w <pid>`. Verify after deploy: `pmset -g | grep displaysleep` no longer says "prevented by caffeinate".

### 2. Screen-off-when-idle in the daemon
- New package `internal/screen` with `IdleSeconds() (float64, error)` (parse HIDIdleTime) and `SleepNow() error` (`pmset displaysleepnow`).
- Config: `ScreenOffAfter int` seconds in `internal/config` (`screen_off_after`, 0 = disabled; default written by the app when enabled = 120).
- Daemon loop (every 5 s tick): if `holder != nil` (agents running) and `ScreenOffAfter > 0` and `idle >= ScreenOffAfter` and `!firedThisIdlePeriod` → `SleepNow()`, log `[screen] display off after Ns idle`, set `firedThisIdlePeriod = true`. When `idle < ScreenOffAfter` → reset the flag. Never fire more than once per idle period; never fire when no agents.
- `/_status` gets `screen_off_after` and `idle_seconds`.
- CLI: `keepgoing screen off-after <seconds|0>` writes config + kicks daemon; `keepgoing screen status`.
- Put the decision in a pure function `ShouldSleep(agentsRunning bool, idle, threshold float64, fired bool) bool` with a `go test` covering: below threshold, at threshold, already fired, no agents, disabled.

### 3. Menu item
Under "Always keep awake": checkbox **Turn off screen when idle** (state = `screen_off_after > 0`). Toggling writes `screen_off_after` 120 / 0 via the CLI and restarts the daemon. Status line group gets nothing new (keep the menu short).

### 4. README
One paragraph in the daemon table: "Screen: while agents run and you haven't touched the Mac for 2 min, the display sleeps (`pmset displaysleepnow`). The system stays awake. Off by default; menu → Turn off screen when idle."

### 5. Build, `go test ./...`, deploy, verify `pmset -g` line, report. Do not enable the option yourself; Elijah toggles it.
