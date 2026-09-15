# keepgoing — task queue for Cursor agent

**Phase 7: design pass on the landing page with the new skills.** `site/` only.
Live: https://keepgoing-pi.vercel.app. Elijah: keep it few-words, picture-led, demo-centred — but make it look *designed*, not generated.

## Skills — read all three first, in this order
1. `~/.agents/skills/frontend-design/SKILL.md` — follow its two-pass process (plan → critique against the brief → build). It lists AI-design tells; our current page has several: "TRY IT" tracked all-caps eyebrow, `macOS 13+ · MIT · 3 MB` middle-dot strings, three identical rounded cards, tinted near-black. Remove them.
2. `~/.agents/skills/design-taste-frontend/SKILL.md` — audit-first for redesigns; run its pre-flight check before shipping.
3. `~/.agents/skills/web-design-guidelines/SKILL.md` — final audit (a11y, contrast, focus, motion).
Also still binding: `~/.cursor/skills/landing-page-polish/SKILL.md`, `~/.cursor/skills/native-mac-polish/SKILL.md` (Mac UI recreations use the real strings).

## Brief (the skill asks for one — this is it)
- Subject: a Mac utility for people who run coding agents and travel. Job: make a visitor understand in 5 s that they can shut the laptop and the agent keeps working, then download.
- Audience: developers using Claude Code / Codex, on phone or laptop, likely arriving from GitHub/HN/X.
- Keep: all current copy verbatim (≤ 180 words), the interactive lid demo (it is the memorable thing — spend the boldness there), the three what-it-does pictures, install, proof block, 4 FAQs, footer. Sections may be re-shaped, merged, or re-ordered; nothing added.
- Free axes for you: palette, typography (one or two families; Google Fonts allowed now, ≤ 2 families, `font-display: swap`, preconnect), layout, how the demo is staged, how the three pictures are presented (not three identical cards), light/dark treatment.
- Mac feel is fine; Apple-clone is not. Do not use the cream+terracotta or black+acid-green defaults the skill warns about.

## Hard rules
- Static HTML/CSS + the existing vanilla JS. No frameworks. Keep `demo.js` behaviour; restyle freely.
- Word count ≤ 180 (excluding code blocks, FAQ). State it in the commit.
- No sudo/pmset/wifi. No git history rewriting. Commit per task, push each.
- Deploy: `cd site && vercel --prod --yes`.

## Tasks
1. **Plan** (no code yet): write `site/DESIGN.md` — tokens (4–6 named hex, both schemes), typefaces + roles, ASCII wireframe for desktop and mobile, 3–5 principles, and the skill's critique: which of your first instincts were generic and what you changed. Commit.
2. **Build** per the plan. Keep the demo working. Commit.
3. **Pre-flight** from `design-taste-frontend`, then `web-design-guidelines` audit; fix findings. Commit.
4. **Verify** at 390 / 820 / 1440, light + dark: no horizontal scroll, demo runs, contrast ≥ 4.5:1, focus visible, reduced-motion respected. Deploy. Report: URL, word count, the DESIGN.md summary, audit findings fixed.
