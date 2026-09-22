# keepgoing — task queue for Cursor agent

**Phase 12: hotspot fallback that actually works.** Log from 2026-09-17 11:51–12:04: offline 13 min, daemon alternated "bouncing en0 radio" / "joining hotspot Oh yeah" 8 times, never connected. Root causes verified on this Mac:
1. `networksetup -setairportnetwork` prints `Could not find network …` / `Failed to join …` and **exits 0** → we logged success.
2. macOS 14+ hides SSIDs from processes without Location Services (`system_profiler` shows `<redacted>`); a launchd daemon can't scan or reliably join by name. Only an app with Location permission can (CoreWLAN).
3. iPhone Personal Hotspot only broadcasts while its settings page is open or when a same-Apple-ID Mac requests it over Bluetooth (Instant Hotspot). Instant Hotspot has no public API; macOS does it itself when **Wi-Fi → Ask to join hotspots = Automatically**. Bouncing the radio every 45 s interrupts that negotiation.

## Hard rules
No sudo/pmset -a/visudo. Build `./app/build.sh`; bundle binary stays `keepgoing-cli`. Deploy per usual; then `keepgoing lid status` → `lid_mode=true`; `pgrep -x KeepGoing`. **Never bounce Wi-Fi or join a network while testing** — use `-wifi-dry-run` on any foreground daemon and the new `wifi test` command only when Elijah runs it. Strings per `~/.cursor/skills/native-mac-polish/SKILL.md`. Commit per task, push, no history rewriting.

## Tasks

### 1. Stop lying about joins (Go, `internal/wifi`)
Capture `networksetup` stdout+stderr; treat output containing `Could not find network`, `Failed to join`, `Error` as failure. Log `[wifi] join "SSID" failed: <first line>`. After a join attempt, poll reachability for up to 25 s and log `[wifi] join "SSID" ok` only when the watcher goes online. `/_status` gets `wifi_last_action`, `wifi_last_error`, `wifi_last_action_at`.

### 2. Recovery schedule that respects Instant Hotspot
New escalation (replace the alternate-every-45 s loop):
- t+20 s offline: bounce radio **once**.
- t+30 s → t+120 s: do nothing; macOS auto-join (Instant Hotspot) window. Log `[wifi] waiting for macOS to auto-join (Ask to join hotspots = Automatically)`.
- t+120 s: ask the app to join the configured hotspot (task 3). If the app isn't running, fall back to `networksetup` (now with real error detection).
- t+240 s and every 4 min after: repeat the app join; bounce the radio again only every 10 min.
- Reset on online. All timings constants at the top of the file, unit-tested with a fake clock.

### 3. Join through the app with Location permission (Swift + Go)
- `Info.plist`: `NSLocationUsageDescription` = "KeepGoing needs Location access to see Wi-Fi networks and join your hotspot when you're offline." `build.sh` links `CoreWLAN` and `CoreLocation`.
- App: `CLLocationManager` request-when-in-use on first need (when a hotspot is configured and the daemon requests a join, or when `wifi test` is run). Expose a poll: daemon `/_status.wifi_request` = `{"join":"SSID","id":N}`; app polls every 3 s (it already polls status), and on a new id: `CWWiFiClient.shared().interface()`, `scanForNetworks(withName: ssid)` → if empty, POST `/_wifi` `{"id":N,"ok":false,"error":"not visible"}`; else `associate(to:network,password:)` (password from Keychain via the CLI: `keepgoing-cli hotspot password` prints it, new subcommand, stdout only) → POST `{"id":N,"ok":true}` or the CWError text.
- Daemon logs the result, sets `wifi_last_error`.

### 4. `keepgoing wifi test` (no disconnect)
Runs through the app: asks it to scan (not join) and prints: Location permission state (`authorized` / `denied` → tells how to fix in System Settings → Privacy → Location Services), whether `hotspot_ssid` is currently visible (and its RSSI), and the exact text a join would use. Add `--join` to actually attempt the join (prints the real error). Exit 1 if not visible.

### 5. Guidance in the product
- `keepgoing hotspot set` and the app's hotspot sheet: after saving, show one line: *iPhone: Settings → Personal Hotspot → Allow Others to Join. Mac: System Settings → Wi-Fi → Ask to join hotspots → Automatically. The name must match your iPhone's name exactly.* App version has a button **Open Wi-Fi Settings** (`x-apple.systempreferences:com.apple.wifi-settings-extension`).
- README Wi-Fi row rewritten to describe the new schedule and the two settings. Site FAQ: no change needed unless the word budget allows one clause.

### 6. Build, `go test ./...`, deploy, verify: `keepgoing wifi test` with the iPhone hotspot page OPEN (ask Elijah to open it) must print `visible` for "Oh yeah"; with it closed, `not visible`. Report both outputs verbatim. Don't run `--join`.

## Phase 13 — overnight prompt + battery guard (Elijah 2026-09-22)
"Temperature still high, battery drops fast. Around 11 at night it should ask whether to run through the midnight; if no answer, default off."

### Hard rules
Unchanged: no sudo/pmset -a/visudo yourself; `./app/build.sh`; bundle binary stays `keepgoing-cli`; deploy then `keepgoing lid status` → `lid_mode=true` and `pgrep -x KeepGoing`; strings per `~/.cursor/skills/native-mac-polish/SKILL.md`; commit per task, push, no history rewriting. Don't close the lid. **Don't kill BTLEServer or touch Bluetooth** (separate, unrelated system bug).

### 1. Night window state (Go, `internal/night`)
Config: `night_ask_at` (string "23:00", empty = feature off, **default "23:00"**), `night_until` (string "07:00"), and runtime state in `state.json`: `night_answer` = `{"date":"2026-09-22","answer":"yes|no","at":ts}`.
Pure function `Decide(now time.Time, cfg, answer) (mode: run|sleep|ask, reason string)` — unit-tested with a fake clock across: before 23:00, 23:00 with no answer, 23:10 with no answer (→ sleep), answer yes (→ run until `night_until` next morning), answer no, next day resets. Nothing in this package touches the system.

### 2. Daemon behaviour
- At `night_ask_at`, if any agent is running: ask (task 3), log `[night] asking whether to run overnight`.
- No answer within **10 min** → treat as **no**: release the awake assertion, `lid.Set(false)`, `lid.SetLowPower(false)`, log `[night] no answer, allowing sleep until HH:MM`. Keep the daemon itself alive (Wi-Fi + logging continue); just stop forcing wakefulness. Re-arm normally at `night_until`.
- Answer yes → normal behaviour until `night_until`, then ask again only the next night.
- `/_status`: `night_mode` = `run|sleep|ask|off`, `night_answer`, `night_until_at`.

### 3. The prompt (Swift app)
`UNUserNotificationCenter` with two actions: **Keep running** / **Let it sleep** (identifier-based, `UNNotificationCategory`). Title `KeepGoing`, body `Keep your Mac awake overnight? Agents are still running. No answer in 10 minutes means sleep.` Tapping an action POSTs `/_night` `{"answer":"yes|no"}`. If notification permission is denied, fall back to a menu-bar flash: set the status item to the `moon.zzz` symbol + menu line `Overnight: asking — choose below`, and add two temporary menu items. Request notification permission on first need only.

### 4. Battery guard (independent of night)
Config `battery_floor` (percent, default **25**, 0 = off). On battery and at/below the floor: release assertion + lid override + low power off, log `[battery] 24% ≤ floor, allowing sleep`, and notify once. Re-arm when charging or above floor + 5. `/_status.battery_pct`, `battery_floor`. Read battery with `pmset -g batt` (already shell-free? if not, parse it) — no new deps.

### 5. Menu + CLI
Status group gains one line: `Overnight: on until 7:00` / `off tonight` / `asks at 23:00`. Actions group gains **Run overnight tonight** (checkbox reflecting tonight's answer; toggling writes it). CLI `keepgoing night status|yes|no|ask-at HH:MM|off` and `keepgoing battery floor <pct|0>`.

### 6. README + site
README: new row "Overnight — at 23:00 KeepGoing asks whether to stay awake; no answer means sleep. Battery below 25% also releases everything." Site FAQ "Battery?" answer updated in one clause. Keep the word budget.

### 7. Build, `go test -ldflags=-linkmode=external ./...`, deploy, verify. Test the decision logic with the fake clock (don't wait for 23:00). Report `keepgoing night status` and `keepgoing status | grep night`.
