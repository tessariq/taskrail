# Taskrail Brand Guide

## Name

"Taskrail" joins "task" with "rail" — the structured unit of work and the
authoritative line it travels on. Taskrail is a deterministic execution harness
that turns goals into structured tasks and advances them along a single
authoritative state file (`planning/STATE.md`). The visual identity encodes that
idea directly: rails, evenly spaced ties, and one node moving forward.

The name is one atomic lowercase unit — `taskrail` — never split, hyphenated, or
camelCased.

## Logo

### Primary Mark — Rail

Two horizontal rails, three evenly spaced vertical ties bridging them, and a
filled circular "active task" node advancing ahead along the track. The mark
reads left-to-right as deterministic forward progression along one line.

The meaning maps directly to the product workflow:

| Element       | Represents                                                        |
|---------------|-------------------------------------------------------------------|
| Two rails     | The single authoritative line both humans and agents follow       |
| Three ties    | Structured, evenly spaced tasks                                    |
| Leading node  | The active task advancing — `validate → next → start → complete → verify` |

The mark uses a **single solid color** with no opacity differentiation: the
strokes and the node fill share one color. Determinism is the brand — there is
one line, one color, one direction.

#### Geometry (canonical 64×64 viewBox)

Root element carries `fill="none"`. All visible geometry shares one color
(strokes plus the node fill).

- **Rails (2 horizontal lines):**
  - Top: `(6,22)→(58,22)`
  - Bottom: `(6,42)→(58,42)`
  - `stroke-width 2.5`, `stroke-linecap="round"`
- **Ties (3 vertical lines):** at `x=14`, `x=26`, `x=38`, each `y=18 → y=46`
  - `stroke-width 3`, `stroke-linecap="round"`
- **Node (active task):** `circle cx=50 cy=32 r=5.5`, filled (no stroke)

The rails are lighter than the ties (stroke 2.5 vs 3): the rails are the guide
line, the ties are the structured tasks that carry the weight. The ties overrun
the rails at top and bottom (y=18 and y=46 against rails at y=22 and y=42), so
the round caps read as deliberate crossties rather than touching the rails. The
node sits centered between the rails (`cy=32`, the midpoint of 22 and 42) and
leads the ties to the right, signalling forward motion.

#### Scaling

| Size      | Context        | Adjustments                                                            |
|-----------|----------------|------------------------------------------------------------------------|
| 64px+     | Full icon      | All elements rendered at canonical geometry                            |
| 32–48px   | Small icon     | Thicken strokes slightly for clarity; keep all three ties              |
| 16–24px   | Favicon        | Thicken strokes; if needed reduce to two ties with wider spacing and a slightly larger node |

At favicon sizes (16–24px), prioritize pixel clarity over literal fidelity:
thicken the rail and tie strokes, and if three ties crowd, drop to two ties with
wider spacing and bump the node radius so the "leading task" stays legible. The
favicon is **petrol dark only** — no theme switching — matching browser-tab
convention for consistent recognition.

### Lockup Variants

- **Horizontal:** Rail icon (left) + `taskrail` wordmark, side by side on a
  canonical `702×160` viewBox. For: README header, docs.

In the lockup the icon is scaled so the **two rails land on the wordmark's type
lines**: the bottom rail sits on the **baseline** (the lowercase letters rest on
it) and the top rail meets the **t-crossbar / x-height**. The track therefore
reads as continuing straight into `taskrail`. This is the defining alignment of
the lockup — preserve it when regenerating: pick the icon scale and vertical
offset so rail centers fall on those two type lines rather than choosing an
arbitrary icon size.

The horizontal lockup is the only lockup form. There is no stacked or secondary
mark — the brand stays as singular and direct as the product's one authoritative
line.

## Typography

### Wordmark — Sora

| Property       | Value                                          |
|----------------|------------------------------------------------|
| Font           | Sora                                           |
| Source         | BunnyFonts (OFL-1.1)                            |
| Weight         | 300 (Light)                                     |
| Letter-spacing | 0.18em                                          |
| Text-transform | lowercase                                       |
| URL            | `https://fonts.bunny.net/css?family=sora:300`   |

The wordmark reads as one atomic lowercase unit — `taskrail` — with no split,
separator, or camelCase. Sora's squarish proportions and engineered geometry
echo the precision of the rail mark without competing with it. The 0.18em
letter-spacing gives the lowercase word an even, track-like rhythm that mirrors
the evenly spaced ties.

In shipped SVGs the wordmark is **outlined to vector paths** — there is no
external font dependency at render time. Sora is required only when regenerating
or editing the source artwork.

### UI — Albert Sans (optional / shared)

| Property | Value                                                              |
|----------|--------------------------------------------------------------------|
| Font     | Albert Sans                                                        |
| Source   | BunnyFonts (OFL-1.1)                                               |
| URL      | `https://fonts.bunny.net/css?family=albert-sans:300,400,500,600`  |

Albert Sans is an **optional, shared** UI face consistent with the sibling
project. Taskrail is a repo-local CLI and has no rendered UI of its own, so this
is a recommendation for any future docs site or marketing surface, not a
shipping requirement.

| Context      | Weight           |
|--------------|------------------|
| Headlines    | 600              |
| UI / buttons | 500              |
| Body text    | 400              |
| Light body   | 300              |
| Code         | System monospace |

## Color

### Monochrome (default)

| Background | Fill color |
|-----------|------------|
| Dark      | `#ffffff`  |
| Light     | `#111111`  |

The mark is a single solid color end to end — strokes and node fill share the
one value above. There is no opacity differentiation anywhere in the mark.

### Accent — Petrol

| Theme      | Hex       |
|-----------|-----------|
| Dark mode  | `#0891b2` |
| Light mode | `#155e75` |

Petrol reads as industrial and infrastructure-grade, matching Taskrail's
positioning as a deterministic execution harness rather than a generic SaaS tool.

### Rules

- Monochrome is the default for all contexts.
- Petrol accent is optional — use when color adds value (marketing, website, social).
- Never mix monochrome and petrol in the same mark.
- On light backgrounds, always use `#155e75`, never the dark-mode `#0891b2`.
- The favicon is the one fixed exception: petrol dark only, regardless of theme.

## Usage Quick Reference

| Context              | Mark    | Color       | Lockup     |
|----------------------|---------|-------------|------------|
| GitHub repo avatar   | Primary | Mono        | Icon only  |
| README header        | Primary | Mono        | Horizontal |
| Favicon              | Primary | Petrol      | Icon only  |
| Docs site            | Primary | Mono/Petrol | Horizontal |
| Website nav          | Primary | Petrol      | Horizontal |
| Social preview       | Primary | Petrol      | Horizontal |
| CLI banner / output  | Primary | Mono        | Icon only  |

## Assets

All production SVGs live in `assets/logo/`. Lockups use outlined wordmark text
(no external font dependency at render time).

| File pattern                            | Description                                |
|-----------------------------------------|--------------------------------------------|
| `icon-mono-{dark,light}.svg`            | Primary mark, monochrome (2 variants)      |
| `icon-petrol-{dark,light}.svg`          | Primary mark, petrol (2 variants)          |
| `lockup-horizontal-mono-{dark,light}.svg`   | Icon + wordmark, monochrome (2 variants) |
| `lockup-horizontal-petrol-{dark,light}.svg` | Icon + wordmark, petrol (2 variants)     |
| `favicon.svg`                           | Simplified primary mark (petrol dark only) |
