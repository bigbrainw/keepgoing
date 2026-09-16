# keepgoing — task queue for Cursor agent

**Phase 11: temperature logging must not depend on the menu bar app.** Today the app died silently and the thermal log lost °C/thermal for an hour.

## Hard rules
No sudo/pmset -a/visudo. Build `./app/build.sh`; bundle binary stays `keepgoing-cli`. Deploy per usual, then `keepgoing lid status` → `lid_mode=true` and `pgrep -x KeepGoing` must show the app running. Commit per task, push, no history rewriting. Don't close the lid.

## Tasks

### 1. `keepgoing-smc` helper (Swift CLI, ~80 lines)
Move the SMC/IOKit temperature code from `app/main.swift` into `app/smc/main.swift`, compiled by `build.sh` to `Contents/MacOS/keepgoing-smc` (ad-hoc signed like the rest). Output one JSON line: `{"cpu_c":63.4,"keys":{"Tp09":..},"thermal":"nominal"}` — thermal via `ProcessInfo.processInfo.thermalState`. Exit 0 always; `cpu_c` null if no key reads. Keep the app using the same source file (symlink or shared file), not a copy.

### 2. Daemon owns the readings
Every 10 s the daemon runs `keepgoing-smc` (path: next to its own executable; fall back to `~/.local/bin/keepgoing-smc`; if missing, log once and skip). Store `cpu_c`, `thermal`, `thermal_since`; the `/_thermal` POST from the app becomes optional (accepted, but daemon values win). `thermal.csv` rows always carry °C now. Notification at serious/critical: daemon posts via `osascript -e 'display notification …'` when the app is not running, app's UNUserNotification when it is (daemon checks `pgrep -x KeepGoing`).

### 3. Keep the app alive
`keepgoing install` also writes `~/Library/LaunchAgents/com.elijah.keepgoing.app.plist`: `ProgramArguments` = bundled `KeepGoing` executable, `RunAtLoad true`, `KeepAlive { SuccessfulExit = false }` (restart only on crash, so Quit still quits), `ProcessType Interactive`, `LimitLoadToSessionType Aqua`. `uninstall` removes it. Deploy script step `open ~/Applications/KeepGoing.app` becomes `launchctl kickstart -k gui/$UID/com.elijah.keepgoing.app`. The app's "Open at login" checkbox now reflects/controls this plist instead of SMAppService (simpler, one mechanism).

### 4. Gap detector
`keepgoing thermal` prints a line `gaps: N (longest Xm at HH:MM)` when consecutive rows are > 90 s apart, so a dead logger is visible.

### 5. Build, `go test ./...`, deploy, verify: `keepgoing status` shows `cpu_c` with the app quit (`osascript -e 'tell app "KeepGoing" to quit'`, check, then `launchctl kickstart -k …app` to bring it back). Report the numbers.
