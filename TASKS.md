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

## Phase 5.1 — visual fixes from review (do now, in order)

Reviewed at 1280 px. Copy is right (180 words). Visuals aren't there yet.

### F. Demo: the lid must actually close
- Build each laptop as HTML: `.laptop > .lid + .base`. `.lid` has `transform-origin: bottom center`; parent has `perspective: 900px`. Open = `rotateX(0)`, closed = `rotateX(-88deg)` with `transition: transform 700ms cubic-bezier(.4,0,.2,1)`. The lid folds down onto the base — a viewer must see it shut.
- Draw a MacBook, not two grey rectangles: lid = dark rounded rect with 6 px bezel and a small notch; base = light grey slab with a keyboard hint (a faint 2-row dotted pattern) and trackpad outline. ~260 px wide.
- Right laptop, closed state: 2 px amber glow line along the hinge + a floating ⚡ badge (inline SVG bolt, 20 px, amber circle) above the closed lid with a soft 1.2 s pulse. Left: nothing, slab goes flat grey.
- Sequence after click: lids close (0–700 ms) → both phones show "…" typing (900 ms) → left: "No reply" grey bubble + caption *macOS sleeps in 67 s.* (2400 ms); right: "Done — 3 tests fixed" green bubble + ⚡ badge + caption *Still running.* (2400 ms). Reverse on "Open the lid". `prefers-reduced-motion`: jump to end states.

### G. Demo: real phone frames
- Each side: a phone (rounded 40 px, 1.5 px border, 170×340) standing to the left of the laptop on desktop, above it on mobile. Inside: a chat with the user bubble right-aligned (blue-ish `#2f6fed`, white text) and agent replies left-aligned (grey; the success reply green `#1f7a4d`/white). Bubbles `max-width: 78%`, padding 8 px 12 px, radius 16 px — iMessage proportions, never full-width bars.
- Phone status bar: time "9:41" + a 3-bar signal glyph (SVG, currentColor). Remove the "Without KeepGoing / With KeepGoing" labels from above; put them as 12 px captions under each laptop instead.

### H. Hero menu mock → looks like a real macOS dark menu
- Top strip: 22 px, dark translucent (`#2b2b2b` @ 92%), right-aligned glyphs: our ⚡ (amber, with a highlighted 24 px rounded backdrop = "menu open"), then a wifi glyph, battery glyph, "Mon 14:42". Left: blank (no app name).
- Dropdown: 260 px, `#1e1e1e`, 6 px radius, 13 px system font, rows 22 px, separators 1 px at 14% white with 4 px margins, disabled rows 55% white, checkmark ✓ as inline SVG. `Lid: safe to close` in amber. Include the full shipped menu through `Quit KeepGoing ⌘Q`. Drop shadow 0 8 px 24 px rgba(0,0,0,.45).
- Same component reused for the "Rejoins Wi-Fi" card, showing `Network: offline — recovering` / `Hotspot: iPhone`, icon `wifi.slash`.

### I. Small things
- Install step 3: replace the ⚡ emoji with the inline SVG bolt (skill: no emoji icons).
- "What it does" icon card: active icon amber, the other three `--muted`; label each under the icon in 11 px: `running` `lid will sleep` `idle` `offline`.
- Hero: add a 12 px line under the buttons: `macOS 13+ · free, MIT · 3 MB`.

### J. Deploy, commit, push (no history rewrite), report deviations.

## Phase 5.2 — last nits (small, do all, one commit is fine)

Reviewed live at 1280 px after the click: this is now a real demo. Three fixes:

### K. Demo proportions
Phone is taller than the laptop and overlaps the hinge. Desktop: phone 150×300, laptop 320 wide, 32 px gap, both bottom-aligned; nothing overlaps. Closed lid at `rotateX(-82deg)` so a sliver of the lid top stays visible (at -88° it reads as a line). Mobile (≤ 640 px): phone above laptop, centred, phone 140×280.

### L. Menu mock checkmark row
`✓ Keep awake with lid closed` — the text is pushed to the right edge. Use a fixed 18 px check column on every action row (empty on unchecked rows) so all labels share one left edge, like a real NSMenu.

### M. Verify at 390 px for real
Open the deployed page in Safari/Chrome at 390 px wide (responsive mode) and confirm: no horizontal scroll, demo stacks, hero menu mock scales (max-width 100%). Fix anything found. Then deploy, commit, push, report.
