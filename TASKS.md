# keepgoing — task queue for Cursor agent

**Phase 10: working vs idle agents.** Elijah: "are we able to know which is actually running, not just idle?"

## Hard rules
Same as Phase 9: no sudo/pmset -a/visudo; build `./app/build.sh`; deploy per the usual rule then `keepgoing lid status` → `lid_mode=true`; strings per `~/.cursor/skills/native-mac-polish/SKILL.md`; commit per task, push, no history rewriting. Do not close the lid.

## Tasks

### 1. Universal signal: CPU-time delta (internal/procwatch)
`ps -axo pid=,cputime=,args=` → parse `cputime` (macOS format `mm:ss.cc`, can be `hh:mm:ss`). Keep the previous sample per PID; `Proc` gains `CPUPct float64` (delta cputime / wall delta ×100) and `Working bool` (= CPUPct ≥ 2.0 **or** any signal from tasks 2–3 within the last 60 s). Children count too: a `claude` whose child (`zsh`, `node`, `go test`…) is burning is working — sum cputime over the PID's descendant tree (one `ps -axo pid=,ppid=,cputime=` pass, build the tree in Go).
`Summary()` becomes `claude 2 working · 12 idle` (per agent kind, only kinds present). `/_status.agent_procs[]` gets `cpu_pct`, `working`, `working_since`.

### 2. Exact signal: Claude Code hooks (opt-in)
`keepgoing hooks install|uninstall|status`: merge into `~/.claude/settings.json` (create if missing; never clobber other hooks; JSON-preserving edit; back up to `settings.json.bak.<ts>` first):
- `UserPromptSubmit` and `PreToolUse` → `curl -s -m 1 -X POST http://127.0.0.1:7777/_agent -d '{"tool":"claude","pid":'$PPID',"state":"working"}'`
- `Stop` and `Notification` → same with `"state":"idle"`.
Daemon endpoint `/_agent` (POST, loopback only) records `{pid,tool,state,ts}`; procwatch consults it (task 1). Print what was written.

### 3. Exact signal: Codex notify (opt-in)
`keepgoing hooks install` also handles Codex: if `~/.codex/config.toml` exists, add/append to `notify = [...]` a small script `~/.config/keepgoing/codex-notify.sh` that POSTs `state=idle` on `turn-ended` (Codex passes the event JSON as argv[1]; parse `type`). Preserve any existing notify entry (Elijah has one) — the array may hold one command only in some versions; if so, wrap: our script calls the previous one with the same args afterwards. Show the resulting TOML line.

### 4. cmux (opt-in, only if `cmux` on PATH)
Every tick, if `cmux` exists: `cmux sessions list` → for each `claude|cursor` row with `pid_exists=yes`, map surface→workspace, then `cmux workspace status --workspace <uuid>` → `working|idle`; feed into the same signal store as tasks 2–3 (tool + pid where available, else by cwd match). Cap the cost: at most one `sessions list` per 10 s. Gate behind config `cmux_status` (default true when the binary exists).

### 5. Thermal log + menu
`thermal.csv` gains `working` (count) after `agents`. `keepgoing thermal` shows it. Menu: `Agents: claude 2 working · 12 idle` (one line, truncate kinds if > 40 chars).

### 6. Optional policy: idle sleep
Config `idle_sleep_after` (minutes, 0 = off, **default 0**). When > 0 and every agent has been idle that long, release the awake assertion + lid override (same path as no-agents), log `[daemon] all agents idle for Nm, allowing sleep`. Re-arm the moment any agent is working again. Menu: checkbox **Allow sleep when agents are idle 30 min** (writes 30/0). README: one paragraph, with the remote-control caveat: an idle Mac asleep can't receive your phone's next message.

### 7. Build, `go test ./...` (add tests for cputime parsing and the descendant sum), deploy, verify `keepgoing status` shows working/idle counts that make sense (this session's Claude should read working while it types), report with the first `keepgoing thermal` lines showing the new column.

## Phase 10.1 — cool mode is not optional (Elijah)
- Remove the **Run cooler with lid closed** checkbox from the menu and the `cool_mode` config key. Cool mode is always active whenever lid mode is on: lid closes → Low Power Mode + agents to efficiency cores; opens → restored. `keepgoing cool on|off` goes away; `keepgoing cool status` stays (read-only). `/_status.cool_mode` stays, always `true` when `lid_mode` is true.
- README table row + site FAQ sentence: state it as behaviour, not an option.
- Deploy, `keepgoing lid status`, commit, push, report.
