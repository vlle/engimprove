---
name: eng
description: Airport wayfinding for your own English mistakes; one yellow sign names the next step on a calm graphite board.
colors:
  ground: "#16171A"
  panel: "#1E2024"
  raised: "#282B31"
  line: "#31353C"
  bar: "#0B0B0C"
  ink: "#F3F4F6"
  ink-2: "#B4B9C2"
  ink-3: "#8A909A"
  sign: "#FFCC00"
  on-sign: "#0B0B0C"
  on-sign-2: "#4A3D00"
  paper: "#FFFFFF"
  paper-2: "#F0F1F3"
  on-paper: "#121316"
  on-paper-2: "#5A5F69"
  paper-ink-3: "#6B7079"
  paper-line: "#E2E4E8"
  slip: "#FF7B70"
  fix: "#86B6FF"
  ok: "#4CC38A"
  warn: "#F5A524"
  paper-slip: "#D3271B"
  paper-fix: "#1C5FD4"
  paper-ok: "#14804A"
typography:
  display:
    fontFamily: "Fira Sans, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "clamp(2rem, 1.1rem + 3vw, 4.25rem)"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "-0.03em"
  figure:
    fontFamily: "Fira Sans, system-ui, sans-serif"
    fontSize: "clamp(3rem, 2rem + 4.4vw, 6rem)"
    fontWeight: 700
    lineHeight: 0.85
    letterSpacing: "-0.04em"
    fontFeature: "tnum"
  sign-sub:
    fontFamily: "Fira Sans, system-ui, sans-serif"
    fontSize: "clamp(1rem, 0.9rem + 0.45vw, 1.35rem)"
    fontWeight: 500
    lineHeight: 1.35
  headline:
    fontFamily: "Fira Sans, system-ui, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 500
    lineHeight: 1.25
    letterSpacing: "-0.01em"
  title:
    fontFamily: "Fira Sans, system-ui, sans-serif"
    fontSize: "1.15rem"
    fontWeight: 500
    lineHeight: 1.25
  body:
    fontFamily: "Fira Sans, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.5
    fontFeature: "tnum"
  label:
    fontFamily: "Fira Sans, system-ui, sans-serif"
    fontSize: "0.85rem"
    fontWeight: 400
    lineHeight: 1.4
  wordmark:
    fontFamily: "Fira Sans, system-ui, sans-serif"
    fontSize: "1.65rem"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "-0.03em"
  sentence:
    fontFamily: "Literata, Georgia, Times New Roman, serif"
    fontSize: "clamp(1.5rem, 1.05rem + 1.6vw, 2.4rem)"
    fontWeight: 400
    lineHeight: 1.32
    letterSpacing: "-0.01em"
  sentence-small:
    fontFamily: "Literata, Georgia, Times New Roman, serif"
    fontSize: "1.15rem"
    fontWeight: 400
    lineHeight: 1.5
rounded:
  xs: "4px"
  sm: "6px"
  md: "8px"
  lg: "10px"
  pill: "999px"
spacing:
  gap: "1.25rem"
  gutter: "clamp(1rem, 3vw, 2.25rem)"
  page: "92rem"
  bar: "4rem"
  edge: "3.25rem"
components:
  sign:
    backgroundColor: "{colors.sign}"
    textColor: "{colors.on-sign}"
    typography: "{typography.display}"
    rounded: "{rounded.lg}"
    padding: "clamp(1.25rem, 2.2vw, 2rem)"
  sign-button-primary:
    backgroundColor: "{colors.on-sign}"
    textColor: "{colors.paper}"
    rounded: "{rounded.md}"
    padding: "1rem 1.4rem"
  sign-button:
    backgroundColor: "transparent"
    textColor: "{colors.on-sign}"
    rounded: "{rounded.md}"
    padding: "0.85rem 1.2rem"
  button:
    backgroundColor: "{colors.raised}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "0.8rem 1.15rem"
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.bar}"
    rounded: "{rounded.md}"
    padding: "0.8rem 1.15rem"
  button-primary-hover:
    backgroundColor: "{colors.paper}"
  post:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    typography: "{typography.title}"
    rounded: "{rounded.lg}"
    height: "5.25rem"
  post-hover:
    backgroundColor: "{colors.raised}"
  post-edge:
    backgroundColor: "{colors.sign}"
    textColor: "{colors.on-sign}"
    width: "{spacing.edge}"
  post-edge-quiet:
    backgroundColor: "{colors.raised}"
    textColor: "{colors.ink-3}"
  tile:
    backgroundColor: "{colors.bar}"
    textColor: "{colors.paper}"
    rounded: "{rounded.sm}"
    size: "3.1rem"
  tile-on-graphite:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.bar}"
  panel:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.lg}"
    padding: "clamp(1.1rem, 2vw, 1.6rem)"
  paper:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.on-paper}"
    typography: "{typography.sentence}"
    rounded: "{rounded.md}"
    padding: "clamp(1.1rem, 2vw, 1.75rem) clamp(1.2rem, 2.2vw, 2rem)"
  top-bar:
    backgroundColor: "{colors.bar}"
    textColor: "{colors.ink-2}"
    height: "{spacing.bar}"
  tab-active:
    textColor: "{colors.sign}"
  textarea:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.on-paper}"
    typography: "{typography.sentence-small}"
    rounded: "{rounded.md}"
    padding: "1rem 1.1rem"
  input-search:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "0.85rem 1rem 0.85rem 2.75rem"
  choice-key:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.on-paper}"
    rounded: "{rounded.md}"
    height: "5.5rem"
  filter-chip:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    padding: "0.5rem 0.9rem"
  filter-chip-active:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.bar}"
---

# Design System: eng

## Overview

**Creative North Star: "The Junction Sign"**

Every screen of eng is a junction in an airport concourse. One signal-yellow sign hangs over it and states the next decision with a monumental figure: review N due cards, practise a ripe topic, check a text, the next card. Everything else is calm graphite information, read at a glance and left alone. The sign re-signs itself as the learner moves, the way a gate board repaints, and the ways onward are signposts with their arrow locked to the edge.

Two voices share the board. The machine speaks in Fira Sans on graphite. The learner's own words are printed on white in Literata, struck and pencilled like a proof: slips struck in red, fixes set in blue. That split is the whole material logic of the world; it is what lets a stranger see at once which text is theirs.

Density is calm and operational, built for a dedicated practice session or a pre-send check on a desktop browser. The system refuses the category's dashboard of equal stat cards and gamified streak tiles: figures are read off the sign or a single rate chart, never tiled into a grid of badges.

**Key Characteristics:**
- One yellow sign per screen; it carries the next decision and the biggest figure.
- Graphite tiers (ground, panel, raised) instead of shadows.
- The learner's words on white in Literata; machine text on graphite in Fira Sans.
- Signposts with a full-height yellow edge arrow; unripe ways stay open with a graphite edge.
- Stroked pictograms in square tiles, white on black inside the sign, black on white on graphite.
- Two movements only: the sign's 140ms flip and the edge arrow's 3px nudge.

## Colors

A near-neutral cool graphite board with one saturated signal yellow and a white sheet for the learner's words; red and blue are reserved for the proof marks.

### Primary
- **Signal Yellow** (sign): the sign band, signpost edge arrows, the active tab's text and underline. As a line or a point it also marks the focus ring, text selection, the input caret, the wordmark caret and link-hover underlines. Never a fill for anything off the band.
- **Sign Black** (on-sign): text, buttons and pictogram tiles on the sign; the round-number chips in a sign's route.
- **Sign Umber** (on-sign-2): secondary text on the sign (the "N due cards · about M min" line, figure labels).

### Neutral
- **Concourse Graphite** (ground): the page.
- **Board Graphite** (panel): panels, signposts, notes, timetable, alerts, table wraps.
- **Raised Graphite** (raised): hovered signposts, secondary buttons, inline code, tags, skeleton bars, quiet tiles and edges.
- **Seam** (line): hairline dividers and secondary-button borders.
- **Gantry Black** (bar): the top bar, tiles inside the sign, text on light fills.
- **Board White** (ink), **Board Grey** (ink-2), **Board Dim** (ink-3): primary, secondary and tertiary machine text on graphite.
- **Sheet White** (paper), **Sheet Tint** (paper-2), **Sheet Ink** (on-paper), **Sheet Grey** (on-paper-2), **Sheet Dim** (paper-ink-3), **Sheet Rule** (paper-line): the white sheet and its text and rules; the Mistakes archive re-maps the graphite ink and line tokens onto these.

### Semantic
- **Slip Red** (slip) / **Slip Red on Sheet** (paper-slip): struck words in a diff, trap marks, worsening deltas, errors.
- **Fix Blue** (fix) / **Fix Blue on Sheet** (paper-fix): corrected words, cloze input text and caret, focused rule highlights, quiz answers.
- **Clear Green** (ok) / **Clear Green on Sheet** (paper-ok): backend-up dots, right answers, improving deltas, done checklist tiles.
- **Amber Warning** (warn): backend-down dots, failed-check alert dots, warning callouts (as a 45% border over an 8% wash).

### Named Rules
**The One Sign Rule.** Signal yellow fills exactly three things: the sign band, signpost edge arrows, and the active tab. Elsewhere it may appear only as a line or a point (focus ring, selection, caret, wordmark caret, link-hover underline). Primary buttons off the band are white on graphite.

**The Sheet Pair Rule.** Slip, fix and ok each have a graphite value and a darker sheet value. Any white surface (paper, choice keys, textarea, the archive sheet, the correct side of a contrast) switches to the sheet values; never put the graphite pastels on white.

## Typography

**Display Font:** Fira Sans (with system-ui, -apple-system, Segoe UI, sans-serif), self-hosted 400/500/600/700
**Body Font:** Fira Sans
**Learner's-words Font:** Literata (with Georgia, Times New Roman, serif), self-hosted variable 400–600 plus italic

**Character:** Fira Sans is the signage voice: compact, legible at a distance, with tabular figures on by default so counts and times line up. Literata is the printed voice of the learner's own sentences, set large enough to read like a proof sheet.

### Hierarchy
- **Display** (600, clamp(2rem → 4.25rem), 1, -0.03em): the sign's title only ("Review", "Not quite"). A long sign drops to clamp(1.4rem → 2.2rem) at 1.2 and caps at 44ch.
- **Figure** (700, clamp(3rem → 6rem), 0.85, -0.04em): the sign's monumental number. Secondary figures in panels use 600 at clamp(1.6rem → 2.2rem).
- **Sign sub** (500, clamp(1rem → 1.35rem), 1.35): the sign's explanatory line, in Sign Umber.
- **Headline** (500, 1.25rem, 1.25, -0.01em): panel titles.
- **Title** (500, 1.15rem, 1.25): signpost names, timetable counts.
- **Body** (400, 1rem, 1.5): machine prose; long prose caps at 68ch with line-height 1.6.
- **Label** (400, 0.85rem): header health, meta lines, axis and history times, in Board Grey or Board Dim.
- **Sentence** (Literata 400, clamp(1.5rem → 2.4rem), 1.32, -0.01em): the learner's sentence on a paper card; hero step to clamp(1.75rem → 3rem), medium to clamp(1.25rem → 1.7rem).
- **Sentence small** (Literata 400, 1.05–1.2rem, 1.45–1.55): diffs in tables and fix lists, textarea input, examples, choice keys at 1.8rem.

### Named Rules
**The Two Voices Rule.** Literata is reserved for words the learner wrote or said, and for example sentences in English; every label, count, explanation and control is Fira Sans. If a string came from the machine, it is not set in Literata.

**The Proof Mark Rule.** A slip is struck through in Slip Red (0.08em line); a fix is Fix Blue at weight 600 with no underline; struck words come before the fix. A wavy 1px underline marks the slip inside the quoted original.

## Layout

A single centred column capped at 92rem with a fluid gutter (clamp(1rem, 3vw, 2.25rem)); blocks stack on one rhythm of 1.25rem gaps. The sign spans the full width at the top of every route. Below it, signposts auto-fit in a row (min 15rem each); content pairs use a 3:2 split (the white last-caught card at 60% beside the rate chart), a 1:1 split, a fixed side rail (14–19rem list beside the main column) or a leaf layout (main column beside a 20rem rule card). A bottom strip pairs the Leitner timetable with alerts.

The top bar is 4rem tall and sticky above 64rem: wordmark, seven tabs, then health and weekly spend pushed to the right. At 64rem and below it stops sticking, tabs wrap to a full-width row at 2.75rem touch height, and every two-column layout collapses to one. At 52rem the sign stacks figure and action under the title; at 44rem fix lists, contrast pairs and the archive table go single-column (the archive turns rows into cards).

## Elevation & Depth

Flat. Depth is tonal: ground, then panel, then raised, with the white sheet as the brightest plane and the black bar as the darkest. There are no drop shadows. The only box-shadows are inset rings that draw an outline without a border: the current tick in the sign's progress row (2px Sign Black), the record button's ring (3px Board White) and the textarea's focus ring (2px ground gap, then 2px Board Grey).

### Named Rules
**The Graphite Tiers Rule.** To lift something, move it one tone up the ground → panel → raised ladder; never add a shadow. Hover on a signpost is exactly that step.

## Shapes

Gently rounded rectangles throughout. Containers (sign, panels, signposts, notes, alerts) use 10px; buttons, paper cards, inputs and toasts use 8px; pictogram tiles 6px (8px for the large sign tile); code, kbd, tags and step numbers 4px. Pills (999px) are reserved for filter chips and the record button; circles for health dots, hear buttons and the record dot. Signposts clip their edge arrow into the right side of the rounded container, so the yellow column inherits the corner.

Pictograms are drawn on a 24-unit grid as 2px round-capped, round-joined strokes with no fill (2.5px inside edge arrows and routes), sitting at 56% of their tile.

## Components

### Buttons
Solid and quiet; the weight is carried by contrast, not colour.
- **Shape:** gently rounded (8px), 1rem Fira Sans 500, 0.8rem × 1.15rem padding, icon gap 0.6rem.
- **Primary (off the sign):** Board White fill, Gantry Black text, weight 600; hover goes to pure white.
- **Secondary:** Raised Graphite fill with a Seam border; hover one tone lighter.
- **On the sign:** primary is Sign Black with white text (1.15rem, 1rem × 1.4rem); secondary is a 2px Sign Black outline on the yellow. Keyboard hints sit in a bordered kbd at reduced opacity.
- **Link button:** text with an underline in Board Dim that turns yellow on hover.
- **Focus:** 2px Signal Yellow outline, 3px offset; on the sign the outline turns Sign Black.

### Chips
- **Style:** pill filter buttons on Board Graphite with a count in Board Dim.
- **State:** pressed chips invert to Board White with Gantry Black text. Kind tags and cost tags are small 4px Raised Graphite labels.

### Cards / Containers
- **Corner Style:** 10px for graphite panels, 8px for white paper.
- **Background:** Board Graphite panels; Sheet White paper for the learner's words.
- **Shadow Strategy:** none (see Elevation & Depth).
- **Border:** none; dividers inside are 1px Seam hairlines.
- **Internal Padding:** clamp(1.1rem, 2vw, 1.6rem) for panels; paper adds a little more horizontally.

### Inputs / Fields
- **Textarea:** a white sheet (Sheet White, Literata 1.15rem/1.55) with a Fix Blue caret; focus draws a ring offset from the ground.
- **Search:** Board Graphite with a Seam border and a stroked magnifier at left; focus turns the border Board Grey.
- **Cloze gap:** an underlined slot inside the sentence; typed text and caret are Fix Blue, and the underline turns Fix Blue on focus.
- **Choice keys:** white keys in Literata 1.8rem, 5.5rem tall, with a black numbered kbd in the corner; hover shows a Sheet Grey border.
- **Segmented control:** a Board Graphite track; the checked option inverts to Board White.

### Navigation
- **Top bar:** Gantry Black, 4rem, wordmark "eng" with a yellow proofreader's caret. Tabs are Fira Sans 500 in Board Grey, white on hover; the current tab is Signal Yellow with a 3px yellow underline flush to the bar's bottom. Health dots (green up, amber down) and "$N this week" sit at the right; the build stamp lives in the footer in Board Dim.
- **Side list:** stacked signposts where the edge is transparent until current; the current one is raised and gets the yellow edge.

### The Sign
The signature component and the first thing on every route. A yellow band: black pictogram tile, display title with its sub-line, monumental figure with a label, and the action at the right. The tile is always the arrow; only an answered card swaps it for its verdict (check, alert, help). During a round a full-width tick row runs under it: done ticks Sign Black, the current tick an inset ring, empty ticks a 16% black wash. On each step the band re-signs with a 140ms vertical flip (from -0.6rem, 25% opacity) on the brand ease.

### Signposts
A way to go: a tile, a title with a one-line sub, and a full-height 3.25rem edge column holding the arrow. Ripe ways get the yellow edge; unripe ways stay open with a Raised Graphite edge and tile, muted text and the line "open to practise now", never a lock. On hover the container steps to Raised Graphite and the arrow nudges 3px right in 140ms.

### Paper card
The learner's sentence in Literata on Sheet White, with the diff marked by the Proof Mark Rule and, below, the original quote ("You wrote: …") in smaller Literata with the slip wavy-underlined. Sheet values apply to every colour inside.

## Do's and Don'ts

### Do:
- **Do** open every route with the sign, and put the next decision and its figure on it (display clamp(2rem, 1.1rem + 3vw, 4.25rem), figure up to 6rem).
- **Do** set every learner-written word on a white surface in Literata, and switch slip, fix and ok to their sheet values there (#D3271B, #1C5FD4, #14804A).
- **Do** lift by tone: ground #16171A, panel #1E2024, raised #282B31.
- **Do** draw pictograms as 24-grid 2px round strokes in square tiles: white on black inside the sign, black on white on graphite.
- **Do** keep focus-visible outlines Signal Yellow (2px, 3px offset), and Sign Black on the sign itself.
- **Do** keep tabular figures on for every count, rate and time.
- **Do** limit movement to the sign's 140ms flip and the edge arrow's 3px nudge, both on cubic-bezier(.16, 1, .3, 1), and drop all animation under prefers-reduced-motion.

### Don't:
- **Don't** fill a button, card, chip or badge with Signal Yellow off the band; off the sign, primary buttons are white on graphite.
- **Don't** put a second sign on a screen or tile equal stat cards into a dashboard grid; extra figures live in one panel under a hairline.
- **Don't** add drop shadows; the only box-shadows are inset rings.
- **Don't** set machine text (labels, explanations, counts, controls) in Literata.
- **Don't** put the graphite pastels (#FF7B70, #86B6FF) on white.
- **Don't** show a locked signpost for an unripe topic; it stays an open way with a graphite edge.
- **Don't** use emoji or text glyphs as icons; every icon is a stroked pictogram.
