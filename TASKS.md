# keepgoing — task queue for Cursor agent

**Phase 8: launch prep.** Elijah launches after this. Two halves: the download must be current, and the site must be share-ready.

## Skills
`~/.agents/skills/frontend-design`, `~/.agents/skills/web-design-guidelines`, `~/.cursor/skills/landing-page-polish`. Keep the current design; this is polish, not redesign.

## Hard rules
- No sudo/pmset/visudo/Wi-Fi. No git history rewriting. Commit per task, push each.
- App build: `./app/build.sh`; bundle binary stays `keepgoing-cli`. Release: `scripts/release.sh` (ad-hoc signing; no cert yet).
- After any app deploy: `keepgoing lid status` must print `lid_mode=true`. If not, stop and report.
- Site deploy: `cd site && vercel --prod --yes`.
- Word budget on the page stays ≤ 180 (code/FAQ excluded).

## A. Release v0.1.1 (the build people will download)
1. `VERSION` → `0.1.1`. `CHANGELOG.md` (new): 0.1.1 — native menu copy, About panel, screen-off-when-idle, config hardening, caffeinate no longer blocks display sleep; 0.1.0 — first release.
2. `scripts/release.sh` → `dist/KeepGoing-0.1.1.dmg` + `.zip`. Print SHA256s.
3. `gh release create v0.1.1 dist/KeepGoing-0.1.1.dmg dist/KeepGoing-0.1.1.zip --title "KeepGoing 0.1.1" --notes-file <the 0.1.1 section>` — **published, not draft** (Elijah asked). Verify `https://github.com/bigbrainw/keepgoing/releases/latest` redirects to v0.1.1.
4. Deploy the 0.1.1 app to this Mac (rule above), check lid status.
5. Site: every "0.1.0" → "0.1.1" (footer `Version 0.1.1`, hero menu mock `KeepGoing 0.1.1`). Install step 1: link the dmg directly (`…/releases/download/v0.1.1/KeepGoing-0.1.1.dmg`) and keep the hero button on `releases/latest`.

## B. Site share-readiness
6. **OG image**: regenerate `site/img/og.png` (1200×630) in the *current* design — same tokens/type as the page: headline "Close the lid. Your agent keeps going." on `--paper`, the gold bolt, small "keepgoing-pi.vercel.app". Update `site/make-og.sh` to produce it. Confirm `<meta property="og:image">` is an absolute URL and add `og:image:width/height`, `twitter:card=summary_large_image`.
7. **Head**: `<link rel="canonical">`, `<meta name="theme-color">` for both schemes (verify present), JSON-LD `SoftwareApplication` (name, operatingSystem "macOS", applicationCategory "UtilitiesApplication", offers price 0, downloadUrl, softwareVersion 0.1.1, license MIT URL).
8. **Demo affordance**: at 1440 the "Close the lid" button sits below the fold of the stage; a visitor may not know the scene is interactive. Move the button into the stage header row, right of "Try it", so label and button are visible together; keep it full-width below the scene on mobile.
9. **Analytics (privacy-safe)**: add Vercel Web Analytics script tag (`<script defer src="/_vercel/insights/script.js"></script>`). Note in the report that Elijah must enable Analytics in the Vercel dashboard for it to record.
10. **README**: first screen = one line, the site link, the demo GIF or a static PNG of the demo scene (make it: `screencapture` if permitted, else render the demo with the CSS and skip), then Install. Keep the rest.
11. **Repo metadata**: `gh repo edit --description "Keep your Mac awake and online while Claude Code / Codex run — close the lid, control from your phone." --homepage https://keepgoing-pi.vercel.app --add-topic macos,menu-bar,claude-code,codex,ai-agents,swift,go`.
12. Lighthouse (`npx lighthouse https://keepgoing-pi.vercel.app --only-categories=performance,accessibility,best-practices,seo --quiet --chrome-flags="--headless"` is allowed): all four ≥ 95, fix what isn't.
13. Deploy, commit, push. Report: release URL, SHA256s, Lighthouse scores, OG image check (`curl -I` the og:image URL → 200), anything skipped.

## C. Donation link (do after B; URL from Elijah — if `BMC_URL` below is still a placeholder, do everything except the href and leave `TODO_BMC_URL` in place)
`BMC_URL = TODO_BMC_URL`
14. Site: footer gets a fourth link **Buy me a coffee** (plain text link like the others; no yellow BMC badge, no image). Also one small line under the FAQ, above the footer: `Free, MIT. If it saved your trip, buy me a coffee.` — the last three words are the link. Word budget: this adds 11; if the page is over 180, trim elsewhere.
15. Repo: `.github/FUNDING.yml` with `buy_me_a_coffee: <username>` (the part after `buymeacoffee.com/`) so GitHub shows the Sponsor button. README: one line under the install section `Support: buy me a coffee → <url>`.
16. App: About panel credits line becomes `Open source, MIT · github.com/bigbrainw/keepgoing · buymeacoffee.com/<username>` (About panel is the one place middle dots are fine — it's Apple's own convention there). Rebuild + deploy only if you're already shipping the app in this phase; otherwise leave for the next release.

## D. OG image v2 (before launch)
Current `site/img/og.png` is mostly empty: small headline top-left, tiny black bolt, system font. Remake with `site/make-og.sh`:
- 1200×630 @2x. Background `--paper` light. Headline "Close the lid." on line 1 and "Your agent keeps going." on line 2, IBM Plex Sans 600 (self-hosted font files already in `site/`), ~96 px, `--ink`, left-aligned at x=80, vertically centred as a block.
- Right third: the closed MacBook from the demo (render the demo scene's closed-lid state with the gold hinge glow and the ⚡ badge) — use `screencapture` of a local browser at the demo's closed state if permission allows, else draw it with the same CSS-to-SVG shapes in Swift/CoreGraphics. Bolt is `--bolt` gold, never black.
- Bottom-left small: `keepgoing-pi.vercel.app` in `--muted`. Nothing else.
- Verify by opening the PNG; no more than 25 % empty canvas. Deploy, commit, push.

## D2. OG image v3 — fix
v2 has the laptop lid drawn as a rotated rectangle overlapping the headline. Do not try to reproduce the 3D CSS transform in CoreGraphics. Draw the closed laptop flat, explicitly, in 1200×630 canvas coordinates (@2x output):
- Headline block: x=80, max width 640 px (wrap only at the line break given): line 1 "Close the lid." baseline y=250; line 2 "Your agent keeps going." baseline y=350. IBM Plex Sans 600, 84 px, `--ink`. Nothing may overlap x<760.
- Laptop, centred at x=990, resting on y=430:
  - base: rounded rect 360×22, corner 6, fill `#d8dfe8`, top edge at y=430; a 120×4 lighter notch centred on its front edge.
  - lid (closed, lying on base): rounded rect 344×12, corner 4, fill `#2a3340`, centred, sitting directly on the base (y=418–430).
  - hinge glow: 2 px line, `--bolt` gold (#c8941a), full lid width, at y=430, plus a 10 px soft gold shadow below it (alpha 0.35).
  - bolt badge: gold circle r=18 centred at (990, 380) with the white bolt path inside; a 30 px soft gold halo (alpha 0.25) around it.
- Bottom-left `keepgoing-pi.vercel.app`, 22 px, `--muted`, at (80, 560).
- Verify by opening the PNG: text and laptop must not touch. Deploy, commit, push.

## E. Demo phones = real agent apps (Elijah's request)
Replace the generic iMessage phone screens in the lid demo with recreations of the actual remote-control apps. Left phone = **Claude** app (Claude Code remote session). Right phone = **Codex** (ChatGPT app, Codex tab). No logos, no wordmark images — app name as text in the header is enough; do not copy brand colours beyond neutral dark/light.
- Copy (both phones): user message **Fix the failing tests**. Then typing "…". Left ends with a muted system line **No reply** ; right ends with **Done — 3 tests fixed** plus one tool chip above it: `Ran npm test`.
- Claude app screen: dark canvas (#1c1c1e), header row `‹  Claude Code · keepgoing` 13 px, user message as a soft rounded bubble right-aligned (#2c2c2e), assistant text left-aligned plain (no bubble) with a small 6 px dot avatar; typing indicator three dots.
- Codex app screen: dark canvas (#0d0d0d), header `‹  Codex · keepgoing/main`, user bubble right (#262626), assistant response as plain text with a tool chip (`Ran npm test`, 11 px, 1 px border, rounded 6) above it, then the reply line.
- Phone frame unchanged (rounded 40, 1.5 px border). Status bar keeps 9:41 + signal.
- Captions under laptops stay: *Without KeepGoing — macOS sleeps in 67 s.* / *With KeepGoing — Still running.* Word budget: "Fix the failing tests" is shorter than before; fine.
- Deploy, commit, push, screenshot the closed state at 1280 and report.

## F. Demo rebuilt around real photos (replaces section E — Elijah: the CSS phone screens are "very ugly")
Assets are already in `site/img/`: `lid-closing.{webp,jpg}` (1600×952, lid half-shut, held in hand) and `lid-closed.{webp,jpg}` (1600×1003, shut on a desk, plugged in). EXIF stripped. Use `<picture>` with webp + jpg fallback, explicit width/height, `loading="eager"` for the first, `lazy` for the second.

New demo = one photo stage, no CSS laptops, no CSS phone chat screens. Delete `.laptop`, `.phone` markup and CSS; keep `demo.js` only for the toggle logic.
- Stage: full content width, 16:10, rounded 16, overflow hidden. Shows `lid-closing` by default.
- Button **Close the lid** (same place: header row right of "Try it" on desktop, full-width below on mobile). Click → crossfade to `lid-closed` (600 ms), and an iOS-style notification banner slides down from the top of the photo (dark translucent, 14 px, rounded 18, 1 px border white/10%): left a 28 px gold-bolt circle, title **KeepGoing**, body **Still running — Claude Code finished: 3 tests fixed**, right "now". That banner is the *only* UI element; it must look like a real iOS banner (SF-like system font stack, 12 px title 600 / 13 px body).
- Two captions under the stage, left/right, mono 13 px: left **Lid closed, on battery** · right **SleepDisabled 1 — still running**. Button becomes **Open the lid** → reverse.
- `prefers-reduced-motion`: instant swap, no slide.
- Everything else on the page unchanged. Word budget unaffected (fewer words than before).
- Verify at 1280 and 390, both schemes, deploy, commit, push, screenshot the closed state.

## G. Demo = procedural 3D MacBook in Three.js (replaces F — Elijah: the photos look awkward)
Skill: `~/.agents/skills/threejs-webgl/SKILL.md` (plus the design skills already in use).

Build the laptop **procedurally** — no downloaded GLB, no logos, no Apple wordmark. A convincing space-grey notebook, not a toy:
- Geometry: base = RoundedBoxGeometry (3.0 × 0.10 × 2.0, radius 0.06, 4 segments) with a slightly inset keyboard well (dark plane, 0.5 mm below the deck) and a trackpad rectangle (outline via a thin darker plane). Lid = RoundedBoxGeometry (3.0 × 0.05 × 1.95, radius 0.06) pivoted at the rear hinge (group with origin at the hinge line), containing a screen plane inset 0.08 from the edges with a tiny notch cutout at the top edge (dark plane). Feet: four small cylinders.
- Materials: base + lid `MeshPhysicalMaterial` color #6e7077, metalness 0.85, roughness 0.45, clearcoat 0.15 (brushed aluminium, matte). Keyboard well #1c1c1e. Screen: `MeshBasicMaterial` with a CanvasTexture showing a dark terminal (three lines mono: `$ npm test`, `3 passing`, `Done — 3 tests fixed`) with faint emissive bloom (just a slightly lighter plane behind it, no post-processing).
- Lighting: `RoomEnvironment` via `PMREMGenerator` for reflections (from `three/examples/jsm/environments/RoomEnvironment.js`), one soft `DirectionalLight` (intensity 1.2, from upper-left, shadow map 1024 on a large `ShadowMaterial` ground), `renderer.toneMapping = ACESFilmicToneMapping`, `outputColorSpace = SRGBColorSpace`, pixel ratio ≤ 2.
- Camera: PerspectiveCamera fov 32, three-quarter view from front-left, slightly above, looking at the hinge. Subtle `OrbitControls` with rotation only, damping, no zoom/pan, auto-rotate off, limited polar angle so you can't see under it.
- **Interaction** (keep the existing button): "Close the lid" → tween the lid group's rotation.x from open (−100°) to closed (0°) over 900 ms with easeInOutCubic. When closed: screen texture off, a 2 px-wide emissive gold strip at the hinge (thin box, emissive #c8941a, emissiveIntensity 2) fades in, and a small floating gold bolt badge (sprite from an SVG canvas) rises 0.2 above the lid with a slow bob. "Open the lid" reverses. Keep the iOS notification banner from F but only show it 600 ms after the lid is fully closed.
- Loading: `<script type="importmap">` pinning `three` to `https://cdn.jsdelivr.net/npm/three@0.170.0/build/three.module.js` and `three/addons/` to `https://cdn.jsdelivr.net/npm/three@0.170.0/examples/jsm/`. Module script loads only when the demo section is within 200 px of the viewport (IntersectionObserver). Until then, and if WebGL is unavailable or `prefers-reduced-motion`, show a static fallback image: render the scene once headless-ish in your browser, `screencapture` it or use `renderer.domElement.toDataURL()` to save `site/img/laptop-open.png` and `laptop-closed.png` (1600 px wide), and swap them on click. Delete `lid-closing.*` and `lid-closed.*` and the photo stage markup.
- Canvas: full content width, aspect 16:10, transparent background over the existing stage surface; `ResizeObserver` for DPR-correct resizing.
- Perf: Lighthouse performance must stay ≥ 90 on production (three.js is deferred, so LCP shouldn't move). If it drops below, lazy-load harder or reduce shadow map.
- Verify at 1280 and 390 (touch drag rotates; no page scroll hijack — `touch-action: pan-y` on the canvas), both schemes, reduced-motion fallback works. Deploy, commit, push, screenshot open + closed states.

## G2. 3D demo fixes (reviewed live at 1280)
1. **Framing**: the laptop is clipped on the left in both states. Compute the laptop's bounding box and fit the camera so the whole model has ≥ 10 % margin on every side at every aspect (16:10 desktop, ~4:3 mobile). Camera: fov 30, position roughly (3.4, 2.2, 4.4) looking at (0, 0.25, −0.2); adjust programmatically from the bounding sphere, don't hand-tune.
2. **Screen**: open state shows a blank grey lid. The CanvasTexture terminal must be visible: draw it *before* first render (`texture.needsUpdate = true`), screen plane faces the camera when open (check normal direction; flip `rotation.y = Math.PI` if it's showing the back), `MeshBasicMaterial` with `toneMapped = false` so text isn't washed out. Lines: `$ npm test`, `✓ 3 passing`, `Done — 3 tests fixed`, 28 px mono on #0d1117, green `✓`.
3. **Load on scroll, not click**: IntersectionObserver with `rootMargin: 300px` boots Three.js; the PNG fallback fades out once the first frame renders. First click must not wait for a download.
4. **Closed state**: bolt badge floats 0.35 above the lid centre with a 2 s sine bob (amplitude 0.05), faces camera (Sprite), 0.5 units wide, slight soft shadow under it via a second transparent radial sprite. Hinge glow: 3.0 × 0.02 × 0.03 box at the hinge, emissive #c8941a intensity 3, plus a 2 s opacity pulse 0.7↔1. Both fade in over 400 ms after the lid finishes closing.
5. **Grounding**: laptop must read as sitting on a surface — `ShadowMaterial` plane opacity 0.28, directional light with `shadow.radius = 6`, `shadow.mapSize` 2048, light from front-upper-left so the shadow falls back-right. Add a faint contact darkening under the base (a 3.4 × 2.4 radial gradient sprite, opacity 0.35).
6. **Materials**: aluminium is too flat — `metalness 0.9, roughness 0.35, clearcoat 0.25, clearcoatRoughness 0.3, envMapIntensity 1.1`; base deck slightly lighter (#7a7c84) than lid (#66686f). Keyboard well 40 % of deck depth, keys as an instanced grid of 14×5 tiny rounded boxes (#1c1c1e) — cheap, sells it.
7. Re-render the two fallback PNGs from the fixed scene. Caption left stays `Lid closed, on battery`.
8. Lighthouse ≥ 90 still. Deploy, commit, push, screenshot open + closed at 1280.
