// Package hook runs the machine around Claude Code turns: queue prompts, check them in the background,
// and show corrections and ripe drills when a turn stops.
package hook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"engimprove/internal/cards"
	"engimprove/internal/coach"
	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/lang"
	"engimprove/internal/logbook"
	"engimprove/internal/speak"
	"engimprove/internal/topics"
)

// Input is the part of the hook payload we read.
type Input struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt"`
	Cwd       string `json:"cwd"`
}

// PromptLog is one English prompt's size, kept to turn mistake counts into a rate; no text is stored.
type PromptLog struct {
	TS      string `json:"ts"`
	Words   int    `json:"words"`
	Project string `json:"project"`
	Session string `json:"session,omitempty"`
}

// Queued is an English prompt waiting for the background checker; the file is deleted after the check.
type Queued struct {
	TS      string `json:"ts"`
	Session string `json:"session"`
	Project string `json:"project"`
	Cwd     string `json:"cwd"`
	Words   int    `json:"words"`
	Text    string `json:"text"`
}

// CheckLog is one finished prompt check; the weekly mistake rate is computed from these alone.
type CheckLog struct {
	TS       string `json:"ts"`
	Project  string `json:"project"`
	Words    int    `json:"words"`
	Mistakes int    `json:"mistakes"`
	Backend  string `json:"backend"`
}

// Prompt records the prompt size and queues English prompts; it returns the queue file, or "" to skip.
func Prompt(s data.Store, in Input, now time.Time) (string, error) {
	res := lang.Analyze(in.Prompt)
	if !res.English {
		return "", nil
	}
	project := filepath.Base(in.Cwd)
	if err := data.AppendJSONL(s.Path("state", "prompts.jsonl"), PromptLog{
		TS: now.Format(time.RFC3339), Words: res.Words, Project: project, Session: in.SessionID,
	}); err != nil {
		return "", err
	}
	path := s.Path("state", "queue", fmt.Sprintf("%d.json", now.UnixNano()))
	return path, data.WriteJSON(path, Queued{
		TS: now.Format(time.RFC3339), Session: in.SessionID, Project: project, Cwd: in.Cwd, Words: res.Words, Text: res.Text,
	})
}

// FeedbackItem is one logged correction waiting to be shown in its session.
type FeedbackItem struct {
	TS       string `json:"ts"`
	Before   string `json:"before"`
	After    string `json:"after"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Rule     string `json:"rule"`
	Count    int    `json:"count"`
	Shown    bool   `json:"shown"`
}

// Note is a one-off status line, such as a failed check.
type Note struct {
	TS    string `json:"ts"`
	Text  string `json:"text"`
	Shown bool   `json:"shown"`
}

type feedback struct {
	Items []FeedbackItem `json:"items"`
	Notes []Note         `json:"notes"`
}

func feedbackPath(s data.Store, session string) string {
	if session == "" || strings.ContainsAny(session, "/\\") {
		session = "nosession"
	}
	return s.Path("state", "feedback", session+".json")
}

func updateFeedback(s data.Store, session string, fn func(*feedback)) error {
	return s.Locked("feedback", func() error {
		var fb feedback
		path := feedbackPath(s, session)
		if err := data.ReadJSON(path, &fb); err != nil {
			return err
		}
		fn(&fb)
		return data.WriteJSON(path, fb)
	})
}

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// Check runs one queued prompt through the coach, logs its mistakes and leaves feedback for the session.
func Check(ctx context.Context, s data.Store, cfg config.Config, c *coach.Coach, path string) (int, string, error) {
	var q Queued
	if err := data.ReadJSON(path, &q); err != nil {
		return 0, "", err
	}
	if q.Text == "" {
		return 0, "", fmt.Errorf("%s: empty or missing queue entry", path)
	}
	res, backend, err := c.CheckPrompt(ctx, s, q.Text, cfg.Private(q.Cwd))
	if err != nil {
		failed := s.Path("state", "queue", "failed", filepath.Base(path))
		_ = os.MkdirAll(filepath.Dir(failed), 0o755)
		_ = os.Rename(path, failed)
		_ = updateFeedback(s, q.Session, func(fb *feedback) {
			fb.Notes = append(fb.Notes, Note{TS: time.Now().Format(time.RFC3339),
				Text: "⚠ eng: проверка промпта не удалась, он лежит в state/queue/failed — " + clip(err.Error())})
		})
		return 0, backend, err
	}

	var seen []string
	var fb feedback
	if err := data.ReadJSON(feedbackPath(s, q.Session), &fb); err != nil {
		return 0, backend, err
	}
	for _, it := range fb.Items {
		seen = append(seen, normalize(it.Before))
	}
	var fresh []logbook.NewError
	for _, e := range res.Errors {
		// a phrase repeated within one session counts once.
		if !slices.Contains(seen, normalize(e.Before)) {
			fresh = append(fresh, e)
			seen = append(seen, normalize(e.Before))
		}
	}
	if len(fresh) > 0 {
		logged, _, err := logbook.Log(s, logbook.Input{
			Date: q.TS[:10], Source: "prompt", Project: q.Project,
			Original: res.Original, Corrected: res.Corrected, Errors: fresh,
		})
		if err != nil {
			return 0, backend, err
		}
		now := time.Now().Format(time.RFC3339)
		if err := updateFeedback(s, q.Session, func(fb *feedback) {
			for i, l := range logged {
				fb.Items = append(fb.Items, FeedbackItem{TS: now, Before: fresh[i].Before, After: fresh[i].After,
					Kind: fresh[i].Kind, Category: l.Category, Rule: l.Rule, Count: l.Count})
			}
		}); err != nil {
			return len(fresh), backend, err
		}
	}
	if err := data.AppendJSONL(s.Path("state", "checks.jsonl"), CheckLog{
		TS: q.TS, Project: q.Project, Words: q.Words, Mistakes: len(fresh), Backend: backend,
	}); err != nil {
		return len(fresh), backend, err
	}
	return len(fresh), backend, os.Remove(path)
}

func clip(s string) string {
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

// Notice is a desktop notification to show.
type Notice struct {
	Message string
	Route   string
}

type sessionMemo struct {
	TS     string   `json:"ts"`
	Nudged []string `json:"nudged"`
}

type memory struct {
	Sessions map[string]sessionMemo `json:"sessions"`
	// Notified maps topic to the drill cursor it was announced for: one notification per cycle.
	Notified map[string]string `json:"notified"`
	// Weekly is the ISO week whose summary was last shown.
	Weekly string `json:"weekly"`
}

func isoWeek(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// weeklyLine compares last week's checked prompts with the week before.
func weeklyLine(s data.Store, entries []data.Entry, now time.Time) string {
	checks, err := data.ReadJSONL[CheckLog](s.Path("state", "checks.jsonl"))
	if err != nil {
		return ""
	}
	last, prev := isoWeek(now.AddDate(0, 0, -7)), isoWeek(now.AddDate(0, 0, -14))
	words, mistakes := map[string]int{}, map[string]int{}
	for _, c := range checks {
		if t, err := time.Parse(time.RFC3339, c.TS); err == nil {
			wk := isoWeek(t.In(now.Location()))
			words[wk] += c.Words
			mistakes[wk] += c.Mistakes
		}
	}
	if words[last] < 50 {
		return ""
	}
	rate := func(wk string) float64 { return float64(mistakes[wk]) * 100 / float64(words[wk]) }
	line := fmt.Sprintf("📈 eng: прошлая неделя — %.1f ошибки на 100 слов в %d словах промптов", rate(last), words[last])
	if words[prev] >= 50 {
		line += fmt.Sprintf(" (неделей раньше %.1f)", rate(prev))
	}
	counts := map[string]int{}
	for _, e := range entries {
		if t, err := time.ParseInLocation("2006-01-02", e.Date, now.Location()); err == nil && isoWeek(t) == last {
			counts[e.Rule]++
		}
	}
	top, n := "", 0
	for r, c := range counts {
		if c > n || (c == n && r < top) {
			top, n = r, c
		}
	}
	if top != "" {
		line += fmt.Sprintf("; чаще всего: %s (%d)", top, n)
	}
	return line
}

// Stop builds the system message shown when a turn ends: corrections first, then ripe drills.
func Stop(s data.Store, cfg config.Config, session string, now time.Time) (string, *Notice, error) {
	var lines []string
	if err := updateFeedback(s, session, func(fb *feedback) {
		var items []FeedbackItem
		for i := range fb.Items {
			if !fb.Items[i].Shown {
				items = append(items, fb.Items[i])
				fb.Items[i].Shown = true
			}
		}
		if len(items) > 0 {
			lines = append(lines, formatItems(items))
		}
		for i := range fb.Notes {
			if !fb.Notes[i].Shown {
				lines = append(lines, fb.Notes[i].Text)
				fb.Notes[i].Shown = true
			}
		}
	}); err != nil {
		return "", nil, err
	}

	entries, err := s.Entries()
	if err != nil {
		return strings.Join(lines, "\n"), nil, err
	}
	sessions, err := topics.Sessions(s)
	if err != nil {
		return strings.Join(lines, "\n"), nil, err
	}
	statuses := topics.Statuses(cfg, entries, sessions)
	all, err := cards.All(s, cfg, entries)
	if err != nil {
		return strings.Join(lines, "\n"), nil, err
	}
	srs, err := cards.LoadSRS(s)
	if err != nil {
		return strings.Join(lines, "\n"), nil, err
	}
	due := cards.CountDue(all, srs, now)
	pending, err := speak.Pending(s)
	if err != nil {
		return strings.Join(lines, "\n"), nil, err
	}

	var notice *Notice
	err = s.Locked("hook", func() error {
		mem := memory{}
		if err := data.ReadJSON(s.Path("state", "hook.json"), &mem); err != nil {
			return err
		}
		if mem.Sessions == nil {
			mem.Sessions = map[string]sessionMemo{}
		}
		if mem.Notified == nil {
			mem.Notified = map[string]string{}
		}
		memo := mem.Sessions[session]
		first := func(key string) bool {
			if slices.Contains(memo.Nudged, key) {
				return false
			}
			memo.Nudged = append(memo.Nudged, key)
			return true
		}
		var ripe, announce []string
		for _, st := range statuses {
			if !st.Ready {
				continue
			}
			if first("topic:" + st.ID) {
				ripe = append(ripe, fmt.Sprintf("%s %d/%d", st.ID, st.Fresh, st.Threshold))
			}
			if prev, ok := mem.Notified[st.ID]; !ok || prev != st.Cursor {
				mem.Notified[st.ID] = st.Cursor
				announce = append(announce, st.ID)
			}
		}
		if len(ripe) > 0 {
			id := strings.Fields(ripe[0])[0]
			lines = append(lines, fmt.Sprintf("🎯 eng: созрели дриллы — %s · /eng-drill %s · eng open drill/%s",
				strings.Join(ripe, ", "), id, id))
		}
		if due >= cfg.ReviewThreshold && first("review") {
			lines = append(lines, fmt.Sprintf("🔁 eng: %d карточек к повторению · /eng-drill review", due))
		}
		if len(pending) > 0 && first("speaking") {
			lines = append(lines, fmt.Sprintf("🎙 eng: %d записей речи без разбора · eng speak review", len(pending)))
		}
		if wk := isoWeek(now); mem.Weekly != wk {
			if line := weeklyLine(s, entries, now); line != "" {
				lines = append(lines, line)
				mem.Weekly = wk
			}
		}
		if cfg.Notify && len(announce) > 0 {
			route := ""
			if len(announce) == 1 {
				route = "drill/" + announce[0]
			}
			notice = &Notice{Message: "Drill ready: " + strings.Join(announce, ", "), Route: route}
		}
		memo.TS = now.Format(time.RFC3339)
		if session != "" {
			mem.Sessions[session] = memo
		}
		cutoff := now.Add(-14 * 24 * time.Hour).Format(time.RFC3339)
		for id, m := range mem.Sessions {
			if m.TS < cutoff {
				delete(mem.Sessions, id)
			}
		}
		return data.WriteJSON(s.Path("state", "hook.json"), mem)
	})
	return strings.Join(lines, "\n"), notice, err
}

func formatItems(items []FeedbackItem) string {
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := items[i].Count >= 2, items[j].Count >= 2
		if ri != rj {
			return ri
		}
		gi, gj := items[i].Kind == "grammar", items[j].Kind == "grammar"
		if gi != gj {
			return gi
		}
		return items[i].Count > items[j].Count
	})
	var parts []string
	for _, it := range items[:min(len(items), 3)] {
		tag := it.Category
		if it.Count >= 2 {
			tag += fmt.Sprintf(", %d-й раз", it.Count)
		}
		parts = append(parts, fmt.Sprintf("`%s` → `%s` (%s)", it.Before, it.After, tag))
	}
	line := fmt.Sprintf("✏️ eng (%d): %s", len(items), strings.Join(parts, " · "))
	if len(items) > 3 {
		line += fmt.Sprintf(" · ещё %d в базе", len(items)-3)
	}
	return line
}
