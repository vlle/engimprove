# examples

Anonymized sample of the mistake database and the text archive, so a fresh clone
shows what the data looks like and how the pieces link together.

- `errors.jsonl` — 84 real mistake records; `text_id`s and names replaced with
  placeholders. One JSON object per line, append-only.
- `texts/` — the archive files those records point to. The `text_id` in each
  archive file matches the `text_id` field in `errors.jsonl`; the stats script
  joins mistakes to texts by it.

Live data lives in `errors/` and `texts/` at the repo root (gitignored) and is
only written by `eng log` / `eng check`. To bootstrap from this sample instead
of an empty database, copy the files up:

```sh
mkdir -p errors texts
cp examples/errors.jsonl errors/
cp -R examples/texts/* texts/
go run ./scripts/engstats   # regenerate errors/stats.md
```
