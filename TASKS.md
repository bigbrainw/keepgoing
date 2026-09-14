# keepgoing — task queue for Cursor agent

**Phase 5: rebuild the landing page. Fewer words, more pictures, a live demo.**
Live site: https://keepgoing-pi.vercel.app (`site/`). Elijah's verdict on the current page: "too many words, needs to be super simple, more images, live demo."

## Skills
Read both before starting: `~/.cursor/skills/landing-page-polish/SKILL.md` and `~/.cursor/skills/native-mac-polish/SKILL.md` (the second one for any recreated Mac UI — real strings only).

## Hard rules
- Static HTML/CSS + small vanilla JS (demo + copy buttons, ≤ 120 lines total). No frameworks, no webfonts, no CDNs, no external images.
- Total page words (excluding code blocks and FAQ) **≤ 180**. Count them. If over, cut.
- No sudo/pmset/wifi. Do not rewrite git history (no amend/rebase of pushed commits). Commit per task, push after each.
- Deploy: `cd site && vercel --prod --yes`.
- Every Mac UI string shown must match the shipped app exactly (see menu below).

## The shipped menu (recreate pixel-faithfully; these are the real strings)
```
KeepGoing 0.1.0
──────────
Agents: claude 13 · codex 4
Sleep: blocked
Lid: safe to close
Network: online
Hotspot: iPhone
──────────
✓ Keep awake with lid closed
  Always keep awake
  Set hotspot…
──────────
  Open at login
  Show log
  Restart daemon
──────────
  About KeepGoing
  Quit KeepGoing              ⌘Q
```
Icon states (SF Symbol names): `bolt.fill` agents + lid safe · `bolt` agents, lid will sleep · `moon.zzz` idle · `wifi.slash` offline.

## Page — exactly these sections, in order

### 1. Hero (≤ 25 words)
H1: **Don't wait for the prompt to finish. Close the lid.**
One line: *Your agent finishes the job while you travel. KeepGoing keeps a Mac awake and online for Claude Code and Codex — and reachable from your phone — with the laptop shut.*
(Keep "Close the lid. Your agent keeps going." as the `<title>` and OG title.)
Buttons: **Download for macOS** (→ https://github.com/bigbrainw/keepgoing/releases/latest) · GitHub.
Right side (desktop) / below (mobile): **real screenshot** of the open menu (task A). Fallback: CSS recreation of the menu above, dark, with a macOS menu bar strip on top.

### 2. Live demo (the centrepiece)
An interactive SVG/CSS scene, no words above it except a small label "Try it".
- Two laptops side by side: **Without KeepGoing** · **With KeepGoing**. Above each, a phone showing a chat bubble "Run the tests and fix what breaks".
- One button under the scene: **Close the lid**. On click: both lids animate shut (CSS transform, ~600 ms).
  - Left: screen goes dark, phone bubble gets "…" then a grey "No reply". Small caption fades in: *macOS sleeps in 67 s.*
  - Right: lid closes but a thin light stays on the hinge, ⚡ appears in a tiny menu bar, phone bubble gets a green reply "Done — 3 tests fixed". Caption: *Still running.*
- Button becomes **Open the lid** → reverses. `prefers-reduced-motion` → instant states, no animation.
- Everything drawn inline (SVG paths for laptop + phone, ≤ 8 KB). No images.

### 3. Three pictures, one line each (h2 "What it does", ≤ 30 words total)
Row of three real screenshots (task A) or faithful CSS recreations:
1. Menu bar icon states (four icons in a row) — caption: *Watches your agents.*
2. The lid setup alert ("Allow KeepGoing to override lid-closed sleep?") — caption: *Asks for your password once.*
3. Menu showing `Network: offline — recovering` + `Hotspot: iPhone` — caption: *Rejoins Wi-Fi or your hotspot.*

### 4. Install (≤ 40 words)
Three steps, each one line + one command with copy button:
1. Download, drag to Applications.
2. `xattr -d com.apple.quarantine /Applications/KeepGoing.app` — *not notarised yet.*
3. Click ⚡ → **Keep awake with lid closed**.

### 5. Proof (no heading, one mono block, keep as is)
```
$ ioreg -r -k AppleClamshellState -d 4 | grep State
"AppleClamshellState" = Yes        # lid closed
$ pmset -g | grep SleepDisabled
SleepDisabled 1                    # still running, on battery
```
Caption (≤ 15 words): *MacBook Pro, macOS 26, on battery, replied from a phone through Claude Code.*

### 6. FAQ — 4 items, one sentence each
- Hot in a bag? — Warm under load; keep it on a surface, plug in if you can.
- Battery? — 2–5 h awake; it re-enables sleep 5 min after agents stop.
- My keys / code? — Never touched; it only watches process names, power, and network.
- Why a password? — Only root can override lid sleep; the rule allows two `pmset` commands and nothing else.

### 7. Footer
MIT · GitHub · Issues · v0.1.0

## Tasks (in order)

### A. Real screenshots (try first, 10 min cap)
The app is running. Try `screencapture -x -R <region> site/img/menu.png` after opening the menu via AppleScript (`tell application "System Events" to click menu bar item 1 of menu bar 2 of application process "KeepGoing"`). If Screen Recording permission is denied for your terminal, stop and use CSS recreations; note it in the report. Save PNGs at 2× into `site/img/`, ≤ 200 KB each, with width/height attributes in HTML.

### B. Rebuild `site/index.html` + `site/style.css` per the section list. Dark and light. Word count ≤ 180 — put the count in your commit message.

### C. Live demo (section 2) as `site/demo.js` + inline SVG. Test: click works, reduced-motion works, no console errors, works at 390 px.

### D. Favicon (inline SVG bolt), `theme-color` both schemes, OG/Twitter meta with `site/img/og.png` (1200×630) rendered by a small Swift or `sips` script in `site/make-og.sh` from the hero text — no external services.

### E. Deploy, then screenshot the deployed page at 390 and 1280 (`npx playwright` is NOT allowed — use `screencapture` of a browser window or skip). Report: URL, word count, which images are real vs CSS, anything you deviated from.
