# keepgoing — task queue for Cursor agent

**Phase 12: stop losing lid mode, stop losing the app.** Verified 2026-09-16 00:55: at 17:50:01 the daemon logged "lid-closed sleep re-enabled" and restarted with lid=false in the same minute the hotspot SSID was written — `config.json` ended up `{"hotspot_ssid": "Oh yeah"}`, `lid_mode` gone. Cool mode rides on lid mode, so temperature protection was silently OFF for 7 hours. Separately, `launchctl print gui/$UID/com.elijah.keepgoing.app` shows `runs = 7, last exit code = 0, state = not running`: the app exits cleanly and `KeepAlive {SuccessfulExit=false}` never relaunches a clean exit.

## Hard rules
No sudo/pmset -a/visudo. Build `./app/build.sh`; bundle binary stays `keepgoing-cli`. Deploy per usual. After deploy: `keepgoing lid status` → `lid_mode=true`, `pgrep -x KeepGoing` → running, `keepgoing status` → `cool_mode=true`. Do not change the hotspot value; it must survive.

## Tasks

### 1. One config writer, read-modify-write
Find every place that writes `~/.config/keepgoing/config.json` (grep `config.json`, `WriteFile`, `json.Marshal` in `internal/` and `app/`). Replace all with a single `config.Update(func(c *Config))` that: loads the current file (tolerating missing/partial JSON), applies the mutation, writes atomically (temp file + rename), and never drops unknown keys (unmarshal into `map[string]any` alongside the struct, or keep a `Extra map[string]json.RawMessage` with `json:"-"` merge). The hotspot-set path and the lid enable/disable path must both go through it.
Regression test: write `{"lid_mode":true}`, call the hotspot setter, assert `lid_mode` still true; and the reverse. Also a test for a partially-written/corrupt file (keeps what parses).

### 2. Daemon never *infers* lid=false from a missing key
On daemon start, if `config.json` lacks `lid_mode` but the previous daemon state (a `~/.config/keepgoing/state.json` or the last daemon.log line "up: … lid=true") had lid=true, log `[config] lid_mode missing — restoring true from previous state` and restore it. If truly first run, default false as today.

### 3. App must come back after any exit
In the app launchd plist switch to `KeepAlive true` (unconditional) plus `ThrottleInterval 10`. Add a "Quit KeepGoing (stop autostart)" menu item that runs `launchctl bootout gui/$UID/com.elijah.keepgoing.app` before exiting, so a deliberate quit stays quit. Find why the app exits 0 — check `app/main.swift` for `exit(0)` / `NSApp.terminate` paths (duplicate-instance check? daemon-not-found?) and log the reason to `~/Library/Logs/keepgoing/app.log` before exiting.

### 4. Thermal watchdog notification
If `thermal` ≠ nominal for > 5 min, or `cpu_c` > 90 °C for > 2 min, post a macOS user notification once per episode ("KeepGoing: Mac is hot — 94 °C, throttling") and log it. Uses existing daemon readings; no new sensor code.

### 5. Build, `go test ./...`, deploy, verify
After deploy: set hotspot via CLI to the same value it has now, then `keepgoing lid status` must still say `lid_mode=true`; `osascript -e 'tell app "KeepGoing" to quit'` then within 15 s `pgrep -x KeepGoing` must show it back; `keepgoing status` shows `cool_mode=true`. Commit with the verification output in the message. Update CHANGELOG.
