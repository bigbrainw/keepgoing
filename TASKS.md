# keepgoing — task queue for Cursor agent

Goal of the product: any Mac user closes the lid and their Claude Code / Codex
agents keep running, reachable from the phone via each tool's remote control.
KeepGoing = menu bar app + launchd daemon that keeps the Mac awake, keeps Wi-Fi
up, and (new) overrides lid-closed sleep while agents are alive.

Read `README.md` first. Layout:

```
main.go                 CLI: daemon | install | uninstall | status | hotspot | lid | run | proxy | env
internal/procwatch      detects claude / codex / codex app-server / cursor processes
internal/awake          caffeinate assertion
internal/wifi           bounce radio, force-join hotspot (Keychain password)
internal/lid            NEW: pmset disablesleep via a two-command sudoers rule
internal/proxy, tunnel  optional holding proxy (token safety)
internal/runner, agents optional `run` wrapper
app/main.swift          AppKit menu bar UI (talks to daemon over http://127.0.0.1:7777/_status)
app/build.sh            builds dist/KeepGoing.app (Go daemon bundled as Contents/MacOS/keepgoing-cli)
```

## Hard rules

- Go 1.21 + macOS 26: build with `go build -ldflags=-linkmode=external` and `codesign -s - -f` (internal linker omits LC_UUID; unsigned arm64 gets SIGKILL). `app/build.sh` already does this.
- The Go binary inside the .app MUST stay named `keepgoing-cli` (APFS is case-insensitive; `keepgoing` collides with the `KeepGoing` Swift executable).
- Do NOT run `sudo`, `pmset`, `visudo`, or write to `/etc` yourself. Elijah runs `keepgoing lid enable` and types the password. Your job is the code path.
- Do NOT bounce Wi-Fi while testing (`-wifi-dry-run` or `-no-wifi` on any foreground daemon you start). Use a separate port (`-listen 127.0.0.1:7790 -connect 127.0.0.1:7791`) so you don't collide with the live launchd daemon on 7777/7778.
- Verify each step with `go vet ./... && ./app/build.sh` and, where relevant, `keepgoing status` (JSON).
- Deploy = `rm -rf ~/Applications/KeepGoing.app && cp -R dist/KeepGoing.app ~/Applications/ && launchctl kickstart -k gui/$(id -u)/com.elijah.keepgoing && open ~/Applications/KeepGoing.app`. The launchd plist points at the bundled `keepgoing-cli`; `~/.local/bin/keepgoing` symlinks to it.
- Commit after each numbered task (repo is not git yet — task 0 fixes that). Conventional Commits, subject ≤ 50 chars.

## Tasks (in order)

### 0. git init
`git init`, commit current tree as `feat: keepgoing daemon, menu bar app, holding proxy`. `.gitignore` already excludes `keepgoing`, `dist/`, `*.log`.

### 1. Finish wiring lid mode (build is currently BROKEN — half-edited)
Already done: `internal/lid/lid.go`, `LidMode` in `internal/config`, daemon integration in `main.go` (`lidOK`, `setLid`, status fields `lid_mode` / `lid_ready` / `sleep_disabled`), and `cmdLid()` at the bottom of `main.go`.
Missing in `main.go`:
- import `"github.com/elijah/keepgoing/internal/lid"`
- `case "lid": os.Exit(cmdLid(fs.Args(), saved))` in the subcommand switch (next to `hotspot`)
- usage line: `keepgoing lid enable|disable|status   keep running with the lid closed (one-time admin password)`
Then `gofmt -w . && go vet ./... && go run . lid status` → prints `lid_mode=false sudoers_ready=false sleep_disabled_now=false`.

### 2. Daemon reconcile + uninstall safety
- On daemon start, if `lidOK` and no agents → `lid.Set(false)` so a crash never leaves the Mac stuck in disablesleep.
- `keepgoing uninstall` → `lid.Set(false)` best-effort before bootout; print the `sudo rm /etc/sudoers.d/keepgoing` hint.

### 3. Menu bar: "Keep running with lid closed" toggle (app/main.swift)
- New checkbox item under "Always keep awake". State from `/_status.lid_mode`.
- Status line: `Lid: safe to close` when `lid_ready && sleep_disabled`, `Lid: will sleep` otherwise; `Lid: setup needed` when `lid_mode && !lid_ready`.
- Enabling when `lid_ready == false`: show an NSAlert explaining the one-time admin prompt and the two exact commands the rule allows, then run the install script with elevation via `NSAppleScript` `do shell script <script> with administrator privileges`. Get the script text from the CLI so there is one source of truth: add `keepgoing lid install-script` (prints `lid.InstallScript`) and call it via the bundled `keepgoing-cli`. After success write `lid_mode: true` to `~/.config/keepgoing/config.json` (same pattern as `toggleAlways`) and `kickDaemon`.
- Disabling: write `lid_mode: false`, run `keepgoing-cli lid disable`.
- Add a one-line warning in the alert: closed laptop under load gets warm — surface, not bag; prefer plugged in.

### 4. README
Section "Close the lid": what happens, the sudoers rule verbatim, how to remove it, the heat/battery caveat, and that lid mode auto-releases 5 min after the last agent exits.

### 5. Build + deploy + smoke
`./app/build.sh`, deploy per the rule above, `keepgoing status` shows `lid_mode`, `lid_ready`, `sleep_disabled` keys. Menu shows the new items. Leave `keepgoing lid enable` for Elijah to run.

### 6. (after Elijah enables) Log check
`tail ~/Library/Logs/keepgoing/daemon.log` should show `[lid] lid-closed sleep disabled while agents run`; `pmset -g | grep SleepDisabled` → 1 while agents run.

## Backlog (do not start unless told)
- Push notify (ntfy.sh) when offline > 3 min or hotspot join fails.
- Signed + notarised .app / dmg (currently ad-hoc signed).
- Cursor CLI proxy adapter; Linux support (`systemd-inhibit`, `nmcli`).
- brew formula.
