# keepgoing — task queue for Cursor agent

Phases 1–3 DONE (repo public, v0.1.0 released, landing at https://keepgoing-pi.vercel.app).
**Phase 4: polish the Mac app.** `app/main.swift` only (plus `keepgoing-cli` if a status field is missing).

## Skill

Load and follow `~/.cursor/skills/native-mac-polish/SKILL.md` for every string and control. The goal: someone opening this menu should think "Apple-quality utility", not "AI made this". Bar = Amphetamine, iStat Menus, Bartender.

## Hard rules (same as before)

- `./app/build.sh`; bundle binary stays `keepgoing-cli`. No sudo/pmset/visudo, no Wi-Fi bounce; test daemons on ports 7790/7791 with `-no-wifi`.
- Deploy: `rm -rf ~/Applications/KeepGoing.app && cp -R dist/KeepGoing.app ~/Applications/ && launchctl kickstart -k gui/$(id -u)/com.elijah.keepgoing && open ~/Applications/KeepGoing.app`.
- Stock AppKit only. No new dependencies. No custom menu views.
- Commit after each task, Conventional Commits, subject ≤ 50 chars.

## Tasks

### 1. Menu structure + copy (skill §Text, §Menu bar apps)
Target menu, top to bottom:
```
KeepGoing 0.1.0                      (disabled)
──────────
Agents: claude 13 · codex 4          (disabled; "none" when empty)
Sleep: blocked                       (or "allowed — no agents")
Lid: safe to close                   (or "will sleep" / "setup needed")
Network: online                      (or "offline — recovering")
Hotspot: iPhone                      (or "not set")
──────────
☐ Keep awake with lid closed
☐ Always keep awake
  Set hotspot…
──────────
☐ Open at login
  Show log
  Restart daemon
──────────
  Quit KeepGoing                     ⌘Q
```
Drop "Daemon: running · N requests held" from the menu (developer noise). If the daemon is down, first item becomes `Daemon not running` and a `Start daemon` action appears in the actions group. Replace the `×` in agent counts with a space.

### 2. Status icon states
Exactly four template SF Symbols, 16 pt, `isTemplate = true`:
- agents running, lid safe → `bolt.fill`
- agents running, lid will sleep → `bolt`
- idle / no agents → `moon.zzz` (keep)
- offline → `wifi.slash` (takes priority)
- daemon down → `bolt.slash`
Remove any duplicate assignment paths; one `render()` decides the symbol.

### 3. Alerts (skill §Text)
Rewrite the four alerts to the skill's limits:
- Onboarding: messageText `KeepGoing keeps your Mac awake while agents run.` informativeText one sentence about lid mode needing an admin password once. Buttons `Set up lid mode` / `Not now`.
- Lid setup: messageText `Allow KeepGoing to override lid-closed sleep?` informativeText: the two exact pmset commands + "Asked once. Remove any time with `sudo rm /etc/sudoers.d/keepgoing`." Buttons `Install` / `Cancel`.
- Hotspot: messageText `Join this hotspot when Wi-Fi is lost`. Fields unchanged. Buttons `Save` / `Cancel`.
- Errors: messageText names the operation (`Couldn't save hotspot`), informativeText = the raw error string. No "Unknown error" unless the string is empty.

### 4. Flicker + resilience
- Only assign `title`/`image`/`state` when the value changed (compare before set).
- If `/_status` fails 3 polls in a row, show the daemon-down state; do not spam alerts.
- `refresh()` interval 3 s stays; also refresh immediately when the menu is about to open (`NSMenuDelegate.menuWillOpen`).

### 5. About + version
`About KeepGoing` item (app group, above Quit) → `NSApp.orderFrontStandardAboutPanel` with `Credits` = one line "Open source, MIT. github.com/bigbrainw/keepgoing". Info.plist gets `NSHumanReadableCopyright` already; keep it.

### 6. Build, deploy, verify, report
Build + deploy. Confirm with `keepgoing status` the daemon is up and the app is running (`pgrep -fl MacOS/KeepGoing`). Paste the final menu as text in your report. List every user-visible string you changed (old → new).
