package hook

import (
	"os"
	"strings"
	"testing"
	"time"

	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/testutil"
	"engimprove/internal/topics"
)

func TestPerceptionNote(t *testing.T) {
	cfg := config.Config{Language: "Russian"}
	if _, ok := perceptionNote(cfg, "", ""); ok {
		t.Fatal("empty result produced a note")
	}
	note, ok := perceptionNote(cfg, "сделай хук асинхронным", "звучит резко")
	if !ok || note != "🌐 eng: как это читается: звучит резко · перевод: сделай хук асинхронным" {
		t.Fatalf("note = %q, ok = %v", note, ok)
	}
	note, ok = perceptionNote(config.Config{}, "", "sounds curt")
	if !ok || note != "🌐 eng: how it reads: sounds curt · translation: —" {
		t.Fatalf("note = %q, ok = %v", note, ok)
	}
}

func entry(id, before, after string) string {
	return `{"id":"` + id + `","date":"2026-10-01","text_id":"t","kind":"grammar","category":"articles","rule":"r","before":"` +
		before + `","after":"` + after + `"}`
}

func TestPromptQueuesOnlyEnglish(t *testing.T) {
	st, _ := testutil.Store(t)
	now := time.Now()
	path, err := Prompt(st, Input{SessionID: "s1", Cwd: "/x/project", Prompt: "сделай ревью и проверь тесты"}, now)
	if err != nil || path != "" {
		t.Fatalf("russian queued: %q %v", path, err)
	}
	path, err = Prompt(st, Input{SessionID: "s1", Cwd: "/x/project",
		Prompt: "make hook async so i would not wait `go test ./...`"}, now)
	if err != nil || path == "" {
		t.Fatalf("english not queued: %v", err)
	}
	var q Queued
	if err := data.ReadJSON(path, &q); err != nil || strings.Contains(q.Text, "go test") || q.Project != "project" {
		t.Fatalf("queued = %+v, %v", q, err)
	}
	logs, _ := data.ReadJSONL[PromptLog](st.Path("state", "prompts.jsonl"))
	if len(logs) != 1 || logs[0].Words != 7 {
		t.Fatalf("prompt log = %+v", logs)
	}
}

func TestStopShowsFeedbackOnceAndNudgesOncePerSession(t *testing.T) {
	st, cfg := testutil.Store(t,
		entry("2026-10-01-001", "opened PR", "opened the PR"),
		entry("2026-10-01-002", "fix bug", "fix the bug"),
	)
	if err := updateFeedback(st, "s1", func(fb *feedback) {
		fb.Items = append(fb.Items,
			FeedbackItem{Before: "everytime", After: "every time", Kind: "spelling", Category: "spelling", Count: 1},
			FeedbackItem{Before: "make hook", After: "make the hook", Kind: "grammar", Category: "articles", Count: 5})
	}); err != nil {
		t.Fatal(err)
	}
	cfg.Notify = true
	now := time.Now()
	msg, notice, err := Stop(st, cfg, "s1", now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "✏️ eng (2): `make hook` → `make the hook` (articles, 5-th time) · `everytime`") {
		t.Fatalf("feedback line wrong:\n%s", msg)
	}
	if !strings.Contains(msg, "🎯 eng: drills ripe — the 2/2") {
		t.Fatalf("nudge missing:\n%s", msg)
	}
	if notice == nil || notice.Route != "drill/the" {
		t.Fatalf("notice = %+v", notice)
	}
	msg, notice, err = Stop(st, cfg, "s1", now)
	if err != nil || msg != "" || notice != nil {
		t.Fatalf("second stop should be silent: %q %+v %v", msg, notice, err)
	}
	msg, notice, _ = Stop(st, cfg, "s2", now)
	if !strings.Contains(msg, "🎯") || notice != nil {
		t.Fatalf("new session should nudge but not re-notify: %q %+v", msg, notice)
	}
	if err := data.AppendJSONL(st.Path("state", "sessions.jsonl"), topics.Session{TS: now.Format(time.RFC3339),
		Topic: "the", Cursor: "2026-10-01-002", Asked: 2, Correct: 2}); err != nil {
		t.Fatal(err)
	}
	msg, _, _ = Stop(st, cfg, "s3", now)
	if strings.Contains(msg, "the 2/2") {
		t.Fatalf("drilled topic still ripe: %q", msg)
	}
	if _, err := os.Stat(st.Path("state", "hook.json")); err != nil {
		t.Fatal(err)
	}
}
