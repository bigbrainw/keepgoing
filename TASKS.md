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
