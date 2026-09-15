# KeepGoing landing page — Phase 7 design plan

**Design read:** Developer-tool landing for agents-on-the-go, picture-led and demo-centred, leaning toward asymmetric editorial layout with instrument-panel typography (IBM Plex Sans + JetBrains Mono). Dials: variance 7 / motion 5 / density 3.

## Tokens

| Name | Light | Dark | Role |
|------|-------|------|------|
| `--ink` | `#1a2332` | `#e7ecf3` | Primary text |
| `--paper` | `#f4f6fa` | `#0f1419` | Page background |
| `--surface` | `#ffffff` | `#171d26` | Raised panels |
| `--line` | `#d8dfe8` | `#2a3340` | Hairlines |
| `--muted` | `#5a6778` | `#8b9cb3` | Secondary text |
| `--bolt` | `#c8941a` | `#e6b422` | Single accent (menu bolt, hinge, focus) |

Supporting (not counted in the six): `--code-bg`, Mac menu recreation vars derived from `--surface` / `--line`.

## Typography

| Role | Family | Notes |
|------|--------|-------|
| Display + body | [IBM Plex Sans](https://fonts.google.com/specimen/IBM+Plex+Sans) 400/600 | One family; headlines use weight + size, not a second display face |
| Code + terminal | [JetBrains Mono](https://fonts.google.com/specimen/JetBrains+Mono) 400 | Proof block, install command, copy buttons only |

Scale: H1 `clamp(2.25rem, 5.5vw, 3.25rem)` / 1.05 / -0.02em. Body 17px / 1.65. Max measure ~38rem for prose. `font-display: swap`, preconnect to `fonts.googleapis.com` + `fonts.gstatic.com`.

## Principles

1. **Demo is the bold element** — full-width stage floor, subtle radial spotlight, left/right scenes unequal weight (failure muted, success lit). Everything else stays quiet.
2. **Asymmetric proof, not card kit** — three “what it does” visuals become one wide strip + one split row (alert | menu), not three identical bordered cards.
3. **Mac-adjacent, not Apple-clone** — CSS menu/phone/laptop recreations keep real strings; no frosted HIG cosplay, no cream paper.
4. **One accent, one motion story** — bolt gold only on interactive focus, hinge glow, and active states; motion lives in the lid demo + user-triggered button, not section fade-ins.
5. **Developer vernacular** — terminal proof block reads like a pasted shell session; install stays numbered, not “Step 01”.

## Wireframes

### Desktop (~820px+)

```
┌──────────────────────────────────────────────────────────────┐
│ [skip]                                                       │
│  H1 + lead (left, ~50%)          │  menu mock (right)        │
│  [Download] [GitHub]             │                           │
│  macOS 13+ · MIT · 3 MB          │                           │
├──────────────────────────────────────────────────────────────┤
│ ░░░░░░░░░ DEMO STAGE (surface + spotlight) ░░░░░░░░░░░░░░░░░ │
│   Without          │          With                           │
│   phone + laptop   │   phone + laptop + hinge glow           │
│   captions         │   captions                              │
│              [ Close the lid ]                               │
├──────────────────────────────────────────────────────────────┤
│ What it does                                                 │
│ ┌──────────────────────────── icon states strip ───────────┐ │
│ └──────────────────────────────────────────────────────────┘ │
│ ┌ alert mock ──────────┐  ┌ compact menu mock ────────────┐ │
│ └──────────────────────┘  └─────────────────────────────────┘ │
├──────────────────────────────────────────────────────────────┤
│ numbered install + code                                      │
├──────────────────────────────────────────────────────────────┤
│ terminal proof (full width, mono, left accent bar)           │
├──────────────────────────────────────────────────────────────┤
│ FAQ (border-bottom rows, no card boxes)                      │
├──────────────────────────────────────────────────────────────┤
│ footer                                                       │
└──────────────────────────────────────────────────────────────┘
```

### Mobile (390px)

```
┌─────────────────────┐
│ H1                  │
│ lead                │
│ [Download full]     │
│ [GitHub full]       │
│ meta                │
│ menu mock centered  │
├─────────────────────┤
│ demo: stacked       │
│ scenes (1 col)      │
│ [Close the lid]     │
├─────────────────────┤
│ icon strip          │
│ alert               │
│ menu mock           │
├─────────────────────┤
│ install             │
│ proof               │
│ FAQ                 │
│ footer              │
└─────────────────────┘
```

Alignment: hero copy left on desktop, centered on mobile; demo and proof centered; features left-aligned captions under visuals.

## Self-critique (generic instincts revised)

| First instinct (AI default) | Revised for this brief |
|-----------------------------|-------------------------|
| Near-black `#121212` + gold bolt everywhere | Slate-navy `--paper` / `--ink`; bolt reserved for demo + focus |
| Uppercase tracked “Try it” eyebrow above demo | Drop eyebrow styling; “Try it” stays as plain sentence-case lead-in to the stage (copy verbatim) |
| Three equal `border-radius: 8px` feature cards | Wide icon strip + 2-column split (alert vs menu), different heights |
| System `-apple-system` stack only | IBM Plex Sans (dev-tool, travel-terminal feel) + JetBrains Mono for proof |
| Fade-in every section on scroll | No scroll animations; demo lid sequence only |
| Centered hero with gradient blob | Keep split hero from Phase 5; strengthen demo section below as the visual peak |

## Out of scope

- Copy changes (verbatim per brief)
- `demo.js` behaviour changes
- New sections or FAQ entries
