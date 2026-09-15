# keepgoing — task queue for Cursor agent

**Phase 9: run cool with the lid closed.** Elijah: "way too hot when I close the lid." Go daemon + Swift app.

## Hard rules
- Never run `sudo`, `pmset -a …`, `visudo`, or write `/etc` yourself. You may run `taskpolicy` and `pmset -g` (read-only). Elijah runs `keepgoing lid enable` to install the extended rule.
- `./app/build.sh`; bundle binary stays `keepgoing-cli`. Test daemons on ports 7790/7791 `-no-wifi -no-awake`.
- Deploy: `rm -rf ~/Applications/KeepGoing.app && cp -R dist/KeepGoing.app ~/Applications/ && launchctl kickstart -k gui/$(id -u)/com.elijah.keepgoing && open ~/Applications/KeepGoing.app`. Then `keepgoing lid status` must print `lid_mode=true`.
- `~/.cursor/skills/native-mac-polish/SKILL.md` for every string. Commit per task, push, no history rewriting.

## Facts (verified on this Mac)
- Lid state: `ioreg -r -k AppleClamshellState -d 4 | grep AppleClamshellState` → `Yes`/`No`.
- Low Power Mode: `pmset -g | grep lowpowermode` (0/1). Setting it needs root: `/usr/bin/pmset -a lowpowermode 1|0`.
- Efficiency cores without root: `taskpolicy -b -p <pid>` (background QoS), undo `taskpolicy -B -p <pid>`. Works on the user's own processes.
- Thermal: `ProcessInfo.processInfo.thermalState` in Swift (nominal/fair/serious/critical) + `NSProcessInfo.thermalStateDidChangeNotification`. Go can't read it directly; the app reports it to the daemon.

## Tasks

### 1. Extend the sudoers rule (internal/lid)
`Sudoers` gains two commands: `/usr/bin/pmset -a lowpowermode 1, /usr/bin/pmset -a lowpowermode 0` (same `%admin … NOPASSWD:` line, comma-separated, exact args). `lid.Available()` must check all four (`sudo -n -l` each). `keepgoing lid enable` reinstalls when any is missing (prints that it's an upgrade of the rule). `lid.SetLowPower(bool)` mirrors `Set()`. Update the grep-verification line in `InstallScript`, the README rule text, and the site FAQ sentence ("two `pmset` commands" → "four").

### 2. Lid watcher in the daemon
`internal/lid.Closed() bool` via ioreg. Daemon tick (5 s) tracks `lidClosed` transitions and logs `[lid] closed` / `[lid] opened`. `/_status` gets `lid_closed`.

### 3. Cool mode (config `cool_mode`, default **true** when lid mode is on)
On lid-closed transition while agents are running:
- `lid.SetLowPower(true)` (if the rule allows; log if not).
- For every agent PID from procwatch (claude, codex, codex-app, cursor): `taskpolicy -b -p PID`; remember the set.
On lid-opened transition: `SetLowPower(false)` **only if the daemon turned it on** (remember prior state), `taskpolicy -B -p` for every remembered PID still alive. Also on daemon shutdown. New agent processes that appear while closed get `-b` too.
`/_status`: `cool_mode`, `low_power`, `cool_pids` (count). CLI: `keepgoing cool on|off|status`.

### 4. Thermal readout
Swift app: observe `thermalStateDidChangeNotification` + poll every 10 s; `POST http://127.0.0.1:7777/_thermal` body `{"state":"nominal|fair|serious|critical"}`. Daemon stores it (`/_status.thermal`, plus `thermal_since`). Menu status group gets one line `Thermal: nominal` (hide when nominal? no — always show, it's the point). At `serious`/`critical`: daemon logs, and the app posts a `UNUserNotificationCenter` notification once per episode: title `KeepGoing`, body `Mac is running hot with the lid closed. Open it or move it off soft surfaces.` (request notification permission on first need, once).

### 5. Menu
Under "Keep awake with lid closed": checkbox **Run cooler with lid closed** (= `cool_mode`), enabled only when lid mode is on. Tooltip-free; the README explains.

### 6. README + site
README daemon table: one row "Heat — lid closed → Low Power Mode + agents moved to efficiency cores; restored when the lid opens. Thermal state shown in the menu; notification at serious." Site FAQ "Hot in a bag?" answer becomes: *With the lid closed it drops into Low Power Mode and moves agents to the efficiency cores, and warns you if it still gets hot. Keep it on a hard surface.* Word budget: replace, don't add.

### 7. Build, `go test ./...`, deploy (rule above), `keepgoing lid status`, `keepgoing cool status`. Do NOT close the lid to test. Report; tell Elijah he needs to run `keepgoing lid enable` once for the extended rule.
