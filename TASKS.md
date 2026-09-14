# keepgoing — task queue for Cursor agent

Phases 1–2.1 DONE (lid mode proven on real hardware; v0.1.0 draft release exists).
**Phase 3: landing page.** One static page, deployed to Vercel.

## Hard rules

- Plain static HTML + CSS in `site/index.html` (+ `site/style.css` if you like). **No framework, no build step, no JS beyond a tiny copy-to-clipboard**. No external fonts/CDNs — system font stack. Must look right at 390 px and 1440 px, light **and** dark (`prefers-color-scheme`).
- Never run `sudo`, `pmset`, `visudo`, or bounce Wi-Fi. Don't touch the Go/Swift code in this phase.
- Deploy: `cd site && vercel --prod --yes` (CLI is logged in as `bigbrainw`; project name `keepgoing`). Report the URL.
- Commit after each task. Conventional Commits, subject ≤ 50 chars.
- Use the copy below **verbatim** (fix typos only). Don't invent features we don't have. Don't add testimonials, logos, or fake stats.

## Design brief

Feel: developer tool, calm, dark-first. One accent colour (amber/gold, like the ⚡ bolt). Big type, lots of air, thin 1 px borders, monospace for commands. Visual anchor in the hero: a CSS-drawn "menu bar" strip showing `⚡ KeepGoing` with a dropdown mock listing the real menu items (Agents / Sleep: blocked / Network: online / Lid: safe to close). No stock images. Section max-width ~880 px.

## Page structure + copy

### Nav
`⚡ KeepGoing` — links: How it works · Install · FAQ · GitHub (`https://github.com/bigbrainw/keepgoing`)

### Hero
**H1:** Close the lid. Your agent keeps going.
**Sub:** KeepGoing is a tiny macOS menu bar app that keeps your Mac awake and online while Claude Code, Codex, or Cursor run — so the remote control on your phone still works when the laptop is shut in your bag.
**Buttons:** `Download for macOS` (→ `https://github.com/bigbrainw/keepgoing/releases/latest`) · `View on GitHub`
**Small print under buttons:** Apple Silicon & Intel · macOS 13+ · free & open source (MIT) · 3 MB

### The problem (short, 3 bullets, no heading larger than h2 "Why")
- You start a long agent run, close the lid, walk away. macOS sleeps. The run freezes.
- Your phone's remote control goes quiet — the Mac it talks to is asleep.
- Hop from office Wi-Fi to a hotspot and every in-flight request dies with it.

### How it works (h2, 3 numbered cards)
1. **Sees your agents.** Every 5 seconds it looks for `claude`, `codex` (including the app-server that ChatGPT / Codex desktop use), and `cursor-agent`.
2. **Keeps the Mac awake — lid open or closed.** Holds a sleep assertion while an agent is alive. With lid mode on, closing the lid no longer sleeps the Mac. Releases 5 minutes after the last agent exits, so your battery is normal the rest of the time.
3. **Keeps you online.** If the network is gone for 20 s it bounces the Wi-Fi radio; still gone, it joins your iPhone hotspot (password from Keychain). Optional holding proxy parks API calls while offline instead of failing them.

### Proof strip (h2 "Tested on real hardware")
Monospace block, exactly this:
```
$ keepgoing lid status
lid_mode=true  sudoers_ready=true  sleep_disabled_now=true

$ ioreg -r -k AppleClamshellState -d 4 | grep State
"AppleClamshellState" = Yes        # lid physically closed
$ pmset -g | grep SleepDisabled
SleepDisabled 1                    # still running, on battery
```
Caption: MacBook Pro, macOS 26.3, lid closed on battery — replied to a phone message through Claude Code remote control. With lid mode off, the same test slept in 67 seconds.

### Install (h2)
Steps as a numbered list with copy buttons on commands:
1. Download `KeepGoing-<version>.dmg` from Releases, drag to Applications.
2. Not yet notarised — first launch: `xattr -d com.apple.quarantine /Applications/KeepGoing.app` (or right-click → Open).
3. Click ⚡ → **Keep running with lid closed** → enter your admin password once. Optional: **Set hotspot…**.
Note box: "The one-time admin prompt installs a sudoers rule allowing exactly two commands: `pmset -a disablesleep 1` and `pmset -a disablesleep 0`. Nothing else. Remove with `sudo rm /etc/sudoers.d/keepgoing`."
CLI alternative, small: `keepgoing install · keepgoing status · keepgoing lid enable · keepgoing hotspot set "<iPhone>"`

### FAQ (h2, `<details>` accordions)
- **Will my laptop get hot in a bag?** A closed laptop under agent load gets warm. Put it on a surface, prefer plugged in. Lid mode turns itself off 5 minutes after the last agent exits.
- **Battery?** Awake on battery ≈ 2–5 h depending on load. Plug in for travel.
- **Does it need my API keys or read my code?** No. It watches process names, power state, and network reachability. It never talks to Anthropic/OpenAI itself. The optional proxy is a plain passthrough on localhost.
- **Why the admin password?** macOS only lets root override lid-closed sleep. See the note above — the rule is two exact commands.
- **What about Windows / Linux?** macOS only today. Linux (systemd-inhibit + nmcli) is on the list.
- **Does it launch agents for me?** No. Start them however you already do; KeepGoing just keeps the machine and the network up.

### Footer
`MIT · Built by Elijah · GitHub · Report an issue` (issues → `https://github.com/bigbrainw/keepgoing/issues`)

## Tasks (in order)

### 1. Build `site/index.html` per brief. Verify: opens locally, no console errors, both colour schemes, 390 px width has no horizontal scroll.
### 2. Copy-to-clipboard on every `<pre>`/code command (≤ 15 lines of JS, inline).
### 3. `site/vercel.json` with `{"cleanUrls": true}`; deploy `cd site && vercel --prod --yes`. Commit. Report the URL + a one-line list of anything you had to deviate from in the brief.
