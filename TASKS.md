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
