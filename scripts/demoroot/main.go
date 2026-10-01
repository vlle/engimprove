// Command demoroot builds a throwaway engimprove root from examples/ for recording demos.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"engimprove/internal/cards"
	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/hook"
	"engimprove/internal/topics"
)

const marker = ".demo-root"

// scrub keeps the author's native language and a stray expletive out of public recordings.
var scrub = strings.NewReplacer(
	"sucks ass", "needs work",
	"Russian has no articles, so ask one question", "Ask one question",
	" Russian «работаю инженером» hides it.", "",
	"comes to mind in Russian first", "comes to mind in your own language first",
)

type weekPlan struct {
	days    []string
	words   int
	mistake int
}

// mistake rate falls week over week so the progress chart has a story.
var plan = []weekPlan{
	{[]string{"2026-09-14", "2026-09-16", "2026-09-18"}, 320, 29},
	{[]string{"2026-09-22", "2026-09-24", "2026-09-26"}, 410, 26},
	{[]string{"2026-09-28", "2026-09-30", "2026-10-01"}, 480, 17},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "demoroot:", err)
		os.Exit(1)
	}
}

func run() error {
	repo := flag.String("repo", ".", "engimprove checkout to take examples, content and config from")
	out := flag.String("out", "", "directory to build the demo root in (required)")
	flag.Parse()
	if *out == "" {
		return errors.New("-out is required")
	}
	if err := prepareOut(*out); err != nil {
		return err
	}
	progress("copying examples into %s", *out)
	if err := copyBase(*repo, *out); err != nil {
		return err
	}
	st := data.Store{Root: *out}
	cfg, err := config.Load(st)
	if err != nil {
		return err
	}
	entries, err := st.Entries()
	if err != nil {
		return err
	}
	progress("seeding state from %d mistakes", len(entries))
	return seedState(st, cfg, entries)
}

func progress(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
}

func prepareOut(out string) error {
	if _, err := os.Stat(filepath.Join(out, marker)); err == nil {
		if err := os.RemoveAll(out); err != nil {
			return err
		}
	} else if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s is not empty and is not a demo root", out)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, marker), nil, 0o644)
}

func copyBase(repo, out string) error {
	pairs := [][2]string{
		{"go.mod", "go.mod"},
		{"examples/errors.jsonl", "errors/errors.jsonl"},
		{"errors/taxonomy.md", "errors/taxonomy.md"},
		{"content/lessons.json", "content/lessons.json"},
		{"content/speaking.json", "content/speaking.json"},
	}
	for _, p := range pairs {
		if err := copyFile(filepath.Join(repo, p[0]), filepath.Join(out, p[1])); err != nil {
			return err
		}
	}
	if err := copyTree(filepath.Join(repo, "examples", "texts"), filepath.Join(out, "texts")); err != nil {
		return err
	}
	return writeConfig(filepath.Join(repo, "config", "eng.json"), filepath.Join(out, "config", "eng.json"))
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(scrub.Replace(string(raw))), 0o644)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		return copyFile(path, filepath.Join(dst, rel))
	})
}

func writeConfig(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	cfg["language"] = "English"
	cfg["notify"] = false
	cfg["private_roots"] = []string{}
	cfg["addr"] = "127.0.0.1:7422"
	enc, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, enc, 0o644)
}

func seedState(st data.Store, cfg config.Config, entries []data.Entry) error {
	var checks []hook.CheckLog
	var prompts []hook.PromptLog
	for _, wk := range plan {
		words, mistakes := wk.words/len(wk.days), wk.mistake/len(wk.days)
		for i, day := range wk.days {
			ts := day + "T10:" + fmt.Sprintf("%02d", 10+i) + ":00+02:00"
			checks = append(checks, hook.CheckLog{TS: ts, Project: "api-gateway", Words: words, Mistakes: mistakes, Backend: "openrouter"})
			prompts = append(prompts, hook.PromptLog{TS: ts, Words: words, Project: "api-gateway"})
		}
	}
	if err := writeJSONL(st.Path("state", "checks.jsonl"), checks); err != nil {
		return err
	}
	if err := writeJSONL(st.Path("state", "prompts.jsonl"), prompts); err != nil {
		return err
	}

	sessions, answers := drillHistory(cfg, entries)
	if err := writeJSONL(st.Path("state", "sessions.jsonl"), sessions); err != nil {
		return err
	}
	if err := writeJSONL(st.Path("state", "answers.jsonl"), answers); err != nil {
		return err
	}
	return writeFeedback(st, entries)
}

// drillHistory marks two topics as drilled recently, so the others stay ripe.
func drillHistory(cfg config.Config, entries []data.Entry) ([]topics.Session, []cards.Answer) {
	var sessions []topics.Session
	var answers []cards.Answer
	days := []string{"2026-09-29", "2026-09-30", "2026-10-01"}
	for i, topic := range []string{"punctuation", "prepositions", "spelling"} {
		cursor := ""
		for _, e := range entries {
			if e.Date > "2026-09-24" {
				continue
			}
			for _, id := range topics.Of(cfg, e) {
				if id == topic && e.ID > cursor {
					cursor = e.ID
				}
			}
		}
		if cursor == "" {
			continue
		}
		day := days[i]
		correct := 3 + i%2
		sessions = append(sessions, topics.Session{TS: day + "T19:05:00+02:00", Topic: topic, Cursor: cursor, Asked: 4, Correct: correct})
		for n := range 4 {
			answers = append(answers, cards.Answer{
				TS:     fmt.Sprintf("%sT19:0%d:00+02:00", day, n+1),
				Card:   fmt.Sprintf("demo-%s-%d", topic, n),
				OK:     n < correct,
				Topics: []string{topic},
			})
		}
	}
	sort.SliceStable(answers, func(i, j int) bool { return answers[i].TS < answers[j].TS })
	return sessions, answers
}

// writeFeedback leaves the corrections the stop hook prints at the end of a turn.
func writeFeedback(st data.Store, entries []data.Entry) error {
	var items []hook.FeedbackItem
	for _, e := range entries {
		if e.Date != "2026-10-01" || e.Kind != "grammar" || len(items) == 3 {
			continue
		}
		items = append(items, hook.FeedbackItem{
			TS: "2026-10-01T12:00:00+02:00", Before: e.Before, After: e.After,
			Kind: e.Kind, Category: e.Category, Rule: e.Rule,
		})
	}
	if len(items) == 0 {
		return errors.New("no 2026-10-01 grammar mistakes in examples to build feedback from")
	}
	dst := st.Path("state", "feedback", "demo.json")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return data.WriteJSON(dst, map[string]any{"items": items, "notes": []any{}})
}

func writeJSONL[T any](path string, rows []T) error {
	var b strings.Builder
	for _, r := range rows {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
