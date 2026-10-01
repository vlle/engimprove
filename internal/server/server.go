// Package server serves the web app and its JSON API on localhost.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"engimprove/internal/cards"
	"engimprove/internal/coach"
	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/hook"
	"engimprove/internal/speak"
	"engimprove/internal/topics"
)

// Server holds what every handler needs.
type Server struct {
	Store  data.Store
	Config config.Config
	Coach  *coach.Coach
	Static fs.FS
	Log    *log.Logger
}

// Handler wires the routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]bool{"ok": true}) })
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/drill", s.drill)
	mux.HandleFunc("POST /api/answer", s.answer)
	mux.HandleFunc("POST /api/explain", s.explainCard)
	mux.HandleFunc("POST /api/session", s.session)
	mux.HandleFunc("GET /api/mistakes", s.mistakes)
	mux.HandleFunc("GET /api/lessons", s.lessons)
	mux.HandleFunc("POST /api/packs/{topic}", s.generatePack)
	mux.HandleFunc("GET /api/speak/question", s.speakQuestion)
	mux.HandleFunc("GET /api/speak", s.speakList)
	mux.HandleFunc("POST /api/speak", s.speakUpload)
	mux.HandleFunc("POST /api/speak/{id}/review", s.speakReview)
	mux.Handle("GET /", http.FileServerFS(s.Static))
	return s.guard(mux)
}

// guard blocks dns rebinding and cross-site posts: the api spends tokens.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-Eng") != "1" {
			http.Error(w, "missing X-Eng header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

type snapshot struct {
	entries  []data.Entry
	sessions []topics.Session
	cards    []cards.Card
	srs      cards.SRS
}

func (s *Server) load() (snapshot, error) {
	var snap snapshot
	var err error
	if snap.entries, err = s.Store.Entries(); err != nil {
		return snap, err
	}
	if snap.sessions, err = topics.Sessions(s.Store); err != nil {
		return snap, err
	}
	if snap.cards, err = cards.All(s.Store, s.Config, snap.entries); err != nil {
		return snap, err
	}
	snap.srs, err = cards.LoadSRS(s.Store)
	return snap, err
}

type week struct {
	Week           string  `json:"week"`
	Start          string  `json:"start"`
	Mistakes       int     `json:"mistakes"`
	PromptMistakes int     `json:"prompt_mistakes"`
	Words          int     `json:"words"`
	Prompts        int     `json:"prompts"`
	Rate           float64 `json:"rate"`
	Answers        int     `json:"answers"`
	Accuracy       float64 `json:"accuracy"`
}

type ruleRow struct {
	Rule     string `json:"rule"`
	Category string `json:"category"`
	Count    int    `json:"count"`
	Last     string `json:"last"`
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	snap, err := s.load()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	now := time.Now()
	prompts, err := data.ReadJSONL[hook.PromptLog](s.Store.Path("state", "prompts.jsonl"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	answers, err := data.ReadJSONL[cards.Answer](s.Store.Path("state", "answers.jsonl"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	checks, err := data.ReadJSONL[hook.CheckLog](s.Store.Path("state", "checks.jsonl"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	pending, _ := speak.Pending(s.Store)
	recs, _ := speak.List(s.Store)
	llmName, llmErr := s.Coach.Backend(false)
	llmStatus := map[string]any{"ok": llmErr == nil, "backend": llmName}
	if llmErr != nil {
		llmStatus["error"] = llmErr.Error()
	}

	unseen := 0
	for _, c := range snap.cards {
		if snap.srs[c.ID].Seen == 0 {
			unseen++
		}
	}
	texts := map[string]bool{}
	rules := map[string]*ruleRow{}
	for _, e := range snap.entries {
		texts[e.TextID] = true
		rr := rules[e.Rule]
		if rr == nil {
			rr = &ruleRow{Rule: e.Rule, Category: e.Category}
			rules[e.Rule] = rr
		}
		rr.Count++
		rr.Last = max(rr.Last, e.Date)
	}
	top := make([]ruleRow, 0, len(rules))
	for _, rr := range rules {
		top = append(top, *rr)
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].Count != top[j].Count {
			return top[i].Count > top[j].Count
		}
		return top[i].Last > top[j].Last
	})
	writeJSON(w, map[string]any{
		"topics":    topics.Statuses(s.Config, snap.entries, snap.sessions),
		"due":       cards.CountDue(snap.cards, snap.srs, now),
		"cards":     len(snap.cards),
		"unseen":    unseen,
		"totals":    map[string]int{"mistakes": len(snap.entries), "texts": len(texts), "rules": len(rules), "prompts": len(prompts)},
		"top_rules": top[:min(len(top), 8)],
		"weeks":     weeks(snap.entries, checks, answers, now),
		"streak":    streak(answers, now),
		"today":     countToday(answers, now),
		"speaking":  map[string]any{"pending": len(pending), "total": len(recs), "tools": speak.Detect(s.Store, s.Config.WhisperModel)},
		"llm":       llmStatus,
		"url":       s.Config.URL(),
	})
}

func weekStart(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}

// weeks buckets activity; the rate counts checked prompts only.
func weeks(entries []data.Entry, checks []hook.CheckLog, answers []cards.Answer, now time.Time) []week {
	const n = 8
	first := weekStart(now).AddDate(0, 0, -7*(n-1))
	out := make([]week, n)
	for i := range out {
		start := first.AddDate(0, 0, 7*i)
		y, wk := start.ISOWeek()
		out[i] = week{Week: strconv.Itoa(y) + "-W" + strconv.Itoa(wk), Start: start.Format("2006-01-02")}
	}
	index := func(t time.Time) int {
		d := weekStart(t).Sub(first).Hours() / (24 * 7)
		i := int(d + 0.5)
		if d < 0 || i >= n {
			return -1
		}
		return i
	}
	correct := make([]int, n)
	for _, e := range entries {
		t, err := time.ParseInLocation("2006-01-02", e.Date, now.Location())
		if err != nil {
			continue
		}
		if i := index(t); i >= 0 {
			out[i].Mistakes++
		}
	}
	for _, c := range checks {
		t, err := time.Parse(time.RFC3339, c.TS)
		if err != nil {
			continue
		}
		if i := index(t.In(now.Location())); i >= 0 {
			out[i].Words += c.Words
			out[i].PromptMistakes += c.Mistakes
			out[i].Prompts++
		}
	}
	for _, a := range answers {
		t, err := time.Parse(time.RFC3339, a.TS)
		if err != nil {
			continue
		}
		if i := index(t.In(now.Location())); i >= 0 {
			out[i].Answers++
			if a.OK {
				correct[i]++
			}
		}
	}
	for i := range out {
		if out[i].Words > 0 {
			out[i].Rate = float64(out[i].PromptMistakes) * 100 / float64(out[i].Words)
		}
		if out[i].Answers > 0 {
			out[i].Accuracy = float64(correct[i]) / float64(out[i].Answers)
		}
	}
	return out
}

func streak(answers []cards.Answer, now time.Time) int {
	days := map[string]bool{}
	for _, a := range answers {
		if t, err := time.Parse(time.RFC3339, a.TS); err == nil {
			days[t.In(now.Location()).Format("2006-01-02")] = true
		}
	}
	day := now
	if !days[day.Format("2006-01-02")] {
		day = day.AddDate(0, 0, -1)
	}
	n := 0
	for days[day.Format("2006-01-02")] {
		n++
		day = day.AddDate(0, 0, -1)
	}
	return n
}

func countToday(answers []cards.Answer, now time.Time) int {
	today := now.Format("2006-01-02")
	n := 0
	for _, a := range answers {
		if t, err := time.Parse(time.RFC3339, a.TS); err == nil && t.In(now.Location()).Format("2006-01-02") == today {
			n++
		}
	}
	return n
}

func (s *Server) drill(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n <= 0 || n > 50 {
		n = 10
	}
	title := "review"
	if topic != cards.Review && topic != "" {
		t, ok := s.Config.Topic(topic)
		if !ok {
			fail(w, http.StatusNotFound, errors.New("unknown topic "+topic))
			return
		}
		title = t.Title
	}
	snap, err := s.load()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	picked := cards.Select(snap.cards, snap.srs, topic, n, time.Now())
	writeJSON(w, map[string]any{"topic": topic, "title": title, "cards": picked})
}

func (s *Server) answer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Card  string `json:"card"`
		OK    bool   `json:"ok"`
		Given string `json:"given"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	snap, err := s.load()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	card, ok := cards.Find(snap.cards, req.Card)
	if !ok {
		fail(w, http.StatusNotFound, errors.New("unknown card "+req.Card))
		return
	}
	st, err := cards.Record(s.Store, card, req.OK, req.Given, time.Now())
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, st)
}

func (s *Server) explainCard(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Card  string `json:"card"`
		Given string `json:"given"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Card == "" || len(req.Given) > 5000 {
		fail(w, http.StatusBadRequest, errors.New("card and an answer under 5000 characters are required"))
		return
	}
	snap, err := s.load()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	card, ok := cards.Find(snap.cards, req.Card)
	if !ok {
		fail(w, http.StatusNotFound, errors.New("unknown card "+req.Card))
		return
	}
	var lessons map[string]json.RawMessage
	if err := data.ReadJSON(s.Store.Path("content", "lessons.json"), &lessons); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	var lesson strings.Builder
	for _, topic := range card.Topics {
		if raw := lessons[topic]; len(raw) > 0 {
			fmt.Fprintf(&lesson, "\n%s lesson:\n%s", topic, raw)
		}
	}
	explanation, err := s.Coach.ExplainCard(r.Context(), card, req.Given, lesson.String())
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, map[string]string{"explanation": explanation})
}

// FinishSession logs a drill and moves the topic cursor past every mistake seen so far.
func FinishSession(st data.Store, cfg config.Config, topic string, asked, correct int) (topics.Session, error) {
	sess := topics.Session{TS: time.Now().Format(time.RFC3339), Topic: topic, Asked: asked, Correct: correct}
	if t, ok := cfg.Topic(topic); ok {
		entries, err := st.Entries()
		if err != nil {
			return sess, err
		}
		sess.Cursor = topics.Cursor(t, entries)
	} else if topic != cards.Review {
		return sess, errors.New("unknown topic " + topic)
	}
	return sess, data.AppendJSONL(st.Path("state", "sessions.jsonl"), sess)
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic   string `json:"topic"`
		Asked   int    `json:"asked"`
		Correct int    `json:"correct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, err := FinishSession(s.Store, s.Config, req.Topic, req.Asked, req.Correct)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, sess)
}

type mistake struct {
	data.Entry
	Topics []string `json:"topics"`
}

func (s *Server) mistakes(w http.ResponseWriter, r *http.Request) {
	entries, err := s.Store.Entries()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	data.SortNewestFirst(entries)
	out := make([]mistake, len(entries))
	for i, e := range entries {
		out[i] = mistake{Entry: e, Topics: topics.Of(s.Config, e)}
	}
	writeJSON(w, out)
}

func (s *Server) lessons(w http.ResponseWriter, r *http.Request) {
	var lessons map[string]any
	if err := data.ReadJSON(s.Store.Path("content", "lessons.json"), &lessons); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, lessons)
}

// TopicMistakes returns the newest mistakes of a topic, for prompting Claude.
func TopicMistakes(cfg config.Config, entries []data.Entry, topic string, limit int) []data.Entry {
	t, ok := cfg.Topic(topic)
	if !ok {
		return nil
	}
	var out []data.Entry
	for _, e := range slices.Backward(entries) {
		if topics.Matches(t, e) {
			out = append(out, e)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

func (s *Server) generatePack(w http.ResponseWriter, r *http.Request) {
	topic := r.PathValue("topic")
	t, ok := s.Config.Topic(topic)
	if !ok {
		fail(w, http.StatusNotFound, errors.New("unknown topic "+topic))
		return
	}
	entries, err := s.Store.Entries()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	mistakes := TopicMistakes(s.Config, entries, topic, 25)
	if len(mistakes) == 0 {
		fail(w, http.StatusBadRequest, errors.New("no mistakes in this topic yet"))
		return
	}
	items, err := s.Coach.GeneratePack(r.Context(), t.Title, mistakes, 12)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	added, err := cards.AppendPack(s.Store, topic, items)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]int{"added": added})
}

// WeakRules lists grammar rules with the most fresh mistakes, for speaking focus.
func WeakRules(cfg config.Config, entries []data.Entry, sessions []topics.Session) []string {
	var out []string
	for _, st := range topics.Statuses(cfg, entries, sessions) {
		for _, rc := range st.Rules {
			if rc.Count >= 2 && !slices.Contains(out, rc.Rule) {
				out = append(out, rc.Rule)
			}
		}
	}
	return out
}

func (s *Server) speakQuestion(w http.ResponseWriter, r *http.Request) {
	snap, err := s.load()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	q, err := speak.Pick(s.Store, WeakRules(s.Config, snap.entries, snap.sessions))
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, q)
}

func (s *Server) speakList(w http.ResponseWriter, r *http.Request) {
	recs, err := speak.List(s.Store)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, recs)
}

func (s *Server) speakUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("audio")
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !slices.Contains([]string{".webm", ".ogg", ".mp4", ".m4a", ".wav"}, ext) {
		ext = ".webm"
	}
	tools := speak.Detect(s.Store, s.Config.WhisperModel)
	rec, err := speak.Save(r.Context(), s.Store, tools, file, ext, r.FormValue("prompt"), r.FormValue("focus"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, rec)
}

func (s *Server) speakReview(w http.ResponseWriter, r *http.Request) {
	rec, err := speak.Load(s.Store, r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	if rec.Review != nil {
		writeJSON(w, rec)
		return
	}
	review, backend, err := s.Coach.ReviewSpeech(r.Context(), s.Store, rec.Prompt, rec.Transcript)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	rec, err = speak.Apply(s.Store, rec, review, backend)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, rec)
}
