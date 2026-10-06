# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Two audiences with equal weight in product decisions:

- **The author** — a Russian-speaking backend developer who writes English all day in coding agents (Claude Code), at work and on side projects. Uses the machine daily; their mistake database is the live dogfood.
- **Open-source users** — non-native developers who write English to coding agents (Claude Code, opencode, or agents without hooks via the Check tab), with any native language. They arrive through the GitHub README, install via `eng install` or the agent install prompt, and start from an empty database.

Jobs they hire the web app for:

- see which mistakes keep coming back and why;
- drill those mistakes as cards built from their own sentences;
- check a text (email, message, MR description) before sending it;
- practise spoken work English: stand-ups, code review, interviews.

## Product Purpose

engimprove turns the English a developer already writes into the study material. Mistakes are caught where they happen — agent prompts, texts sent for review, spoken answers — stored in one append-only database, and returned as spaced-repetition drills.

Success has two equal halves:

- **Written accuracy** — fewer mistakes per 100 checked words, tracked weekly (`state/checks.jsonl`).
- **Spoken confidence at work** — fluent stand-ups, review comments and interview answers (Speak, Work English, STAR review mode).

## Positioning

The curriculum is the learner's own mistakes, captured passively inside the tools they already write in, not a course they visit. Every card traces back to a real sentence the learner wrote or said, shown with its original context. Capture runs through agent hooks in ~40 ms without interrupting work; the data stays local in plain files, and prompts from private project trees never leave the machine.

## Operating Context

- **Surfaces:** the web app (`eng open`, local server on `127.0.0.1:7421`), end-of-turn lines in the agent terminal (`✏️ eng`, `🎯 drills ripe`, `📈 week`), terminal drills (`eng drill`, `/eng-drill` in Claude Code), desktop notifications. All share one database.
- **Confirmed web-app sessions:**
  - a dedicated practice session — Review, Speak, Work English, Lessons, browsing the Mistakes archive;
  - a pre-send Check — paste a text, get corrections, then send it elsewhere.
- **Not confirmed as web-app contexts:** short in-flow drills right after a 🎯 notification (served today by the terminal drills) and phone use. Desktop browser is the working assumption.
- **Background pipeline:** `eng hook` queues an English prompt → detached `eng check` reviews it via OpenRouter (or `claude -p` for `private_roots`) → `eng log` writes `errors/errors.jsonl`, `texts/`, `errors/stats.md`.

## Capabilities and Constraints

- **Web app sections:** Today (ripe topics, weekly growth, streak), Review (due cards), Speak (record → whisper → grammar or STAR review), Work English (phrase sets and prompts per work situation), Check (paste-and-review), Mistakes (archive), Lessons (per-topic rules and examples).
- **Cards:** cloze from the corrected phrase when the gap is guessable (articles, prepositions, connectors, single-word swaps with choices), otherwise "fix the phrase"; on-demand explanation; text-to-speech playback; LLM-generated packs per topic.
- **Repetition:** Leitner intervals 10 min / 1 / 3 / 7 / 21 / 60 days; a miss returns the card to the start. A topic is ripe when `threshold` fresh mistakes accumulate since its last drill.
- **Stack constraints:** Go stdlib only, no external modules. The SPA is vanilla HTML/CSS/JS with no build step, embedded into the binary — every frontend change needs `go build -o bin/eng ./cmd/eng` and a server restart.
- **Language:** UI chrome, the database, rules, taxonomy and CLI are English. Explanations, coaching and hook feedback follow the learner's native language from `config/eng.json` (`language`; Russian and Spanish packs today, English fallback). UI copy is read by non-native speakers.
- **Data rules:** the mistake database is append-only and written only through `eng log`; categories come from the closed list in `errors/taxonomy.md`; `state/`, `speaking/`, `drills/packs/` are machine data.
- **Cost:** each OpenRouter review costs about a cent; `claude -p` uses the user's subscription. The Speak review already shows its cost.
- **Terminology:** mistake, rule, category, kind (`grammar`, `spelling`, `punctuation`, `lexical`, `style`), topic, ripe, threshold, drill, card (cloze / fix), pack, due, review, check, text.
- **Open:** first-run experience in the web app for a fresh clone with an empty database; whether the web app should serve quick post-notification drills or phone use.

## Brand Commitments

- Names: product **engimprove**, CLI and web app **eng**.
- MIT license, public repository `github.com/vlle/engimprove`.

## Evidence on Hand

- `demo/assets/` — web app screenshots (light and dark), web and terminal GIFs, built only from `examples/` via `scripts/demoroot`.
- `examples/` — anonymized sample of the mistake database and text archive.
- `content/lessons.json`, `content/speaking.json`, `content/work-english.json` — real lesson, speaking and work-phrase content.
- The author's live database (`errors/`, `texts/`, `state/`) is private and gitignored: never use it in screenshots, demos or published copy.
- Absent: user counts, stars, testimonials, press, and any measured effectiveness for other learners. Do not invent them.

## Product Principles

1. **Own mistakes are the curriculum.** Every exercise, lesson example and generated pack is anchored to sentences the learner actually wrote or said.
2. **Capture is passive, practice is deliberate.** Writing never stops for the machine; the web app is where the learner chooses to sit down and practise or check.
3. **Accuracy and fluency weigh the same.** Written precision and spoken confidence at work are equal goals; neither is an add-on.
4. **Works for a stranger on day one.** A fresh clone, an empty database, any native language, any agent — nothing may presume the author's data, language or tools.
5. **The record stays trustworthy.** Append-only, local, private by default; LLM spend and backends are visible, never silent.
