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
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"engimprove/internal/cards"
	"engimprove/internal/coach"
	"engimprove/internal/buildinfo"
	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/hook"
	"engimprove/internal/logbook"
	"engimprove/internal/resume"
	"engimprove/internal/speak"
	"engimprove/internal/topics"
)

// Server holds what every handler needs.
type Server struct {
	Store  data.Store
	Config config.Config
	Coach  *coach.Coach
	Resume *resume.Loader
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
	mux.HandleFunc("GET /api/work-english", s.workEnglish)
	mux.HandleFunc("POST /api/packs/{topic}", s.generatePack)
	mux.HandleFunc("GET /api/speak/question", s.speakQuestion)
	mux.HandleFunc("GET /api/speak", s.speakList)
	mux.HandleFunc("POST /api/speak", s.speakUpload)
	mux.HandleFunc("POST /api/speak/{id}/review", s.speakReview)
	mux.HandleFunc("POST /api/check", s.checkText)
	mux.HandleFunc("POST /api/check/{id}/log", s.logCheckedText)
	mux.HandleFunc("POST /api/check/explain", s.explainCheckedMistake)
	mux.HandleFunc("GET /api/growth", s.growth)
	mux.HandleFunc("POST /api/growth/cheer", s.cheer)
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
	totalSpend, _ := speak.TotalSpend(s.Store)
	llmName, llmErr := s.Coach.Backend(false)
	llmStatus := map[string]any{"ok": llmErr == nil, "backend": llmName}
	if llmErr != nil {
		llmStatus["error"] = llmErr.Error()
	}
	whisperStatus := map[string]any{
		"backend":       s.Config.WhisperBackend,
		"model":         s.Config.OpenRouterWhisperModel,
		"local_ready":   speak.Detect(s.Store, s.Config.WhisperModel).Ready,
		"openrouter_ok": s.Coach.OpenRouter() != nil,
	}
	resumeStatus := map[string]any{"loaded": s.Resume != nil && s.Resume.Enabled(), "files": []string{}}
	if s.Resume != nil {
		resumeStatus["files"] = s.Resume.Files()
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
		"speaking":  map[string]any{"pending": len(pending), "total": len(recs), "tools": speak.Detect(s.Store, s.Config.WhisperModel), "spend": totalSpend},
		"llm":       llmStatus,
		"whisper":   whisperStatus,
		"resume":    resumeStatus,
		"version":   buildinfo.Read(),
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
		Card     string             `json:"card"`
		Given    string             `json:"given"`
		Previous *coach.Explanation `json:"previous"`
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
	explanation, err := s.Coach.ExplainCard(r.Context(), card, req.Given, lesson.String(), req.Previous)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, map[string]any{"explanation": explanation, "history": ruleHistory(snap.entries, card)})
}

type pastSlip struct {
	Date   string `json:"date"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type history struct {
	Count int        `json:"count"`
	Slips []pastSlip `json:"slips"`
}

// ruleHistory lists the newest other slips on the card's rule; count includes the card's own.
func ruleHistory(entries []data.Entry, card cards.Card) history {
	h := history{Slips: []pastSlip{}}
	for _, e := range slices.Backward(entries) {
		if !strings.EqualFold(e.Rule, card.Rule) {
			continue
		}
		h.Count++
		if e.ID != card.EntryID && len(h.Slips) < 4 && e.Before != "" && e.After != "" {
			h.Slips = append(h.Slips, pastSlip{Date: e.Date, Before: e.Before, After: e.After})
		}
	}
	return h
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

func (s *Server) workEnglish(w http.ResponseWriter, r *http.Request) {
	var out struct {
		Categories []workCategory `json:"categories"`
	}
	if err := data.ReadJSON(s.Store.Path("content", "work-english.json"), &out); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, out)
}

type workCategory struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Phrases  []phrase `json:"phrases,omitempty"`
	Prompts  []string `json:"prompts,omitempty"`
}

type phrase struct {
	Text  string `json:"text"`
	Notes string `json:"notes,omitempty"`
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
	tr, err := s.transcriber()
	if err != nil {
		fail(w, http.StatusServiceUnavailable, err)
		return
	}
	rec, err := speak.Save(r.Context(), s.Store, tr, file, ext, r.FormValue("prompt"), r.FormValue("focus"))
	if err != nil {
		// Fallback to local whisper if OpenRouter failed and local tools are ready.
		if s.Config.WhisperBackend == "openrouter" {
			tools := speak.Detect(s.Store, s.Config.WhisperModel)
			if tools.Ready {
				if file, header, err = r.FormFile("audio"); err == nil {
					defer file.Close()
					tr = speak.NewLocalTranscriber(tools)
					rec, err = speak.Save(r.Context(), s.Store, tr, file, ext, r.FormValue("prompt"), r.FormValue("focus"))
				}
			}
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, rec)
}

// transcriber picks the configured transcription backend.
func (s *Server) transcriber() (speak.Transcriber, error) {
	if s.Config.WhisperBackend == "openrouter" {
		if or := s.Coach.OpenRouter(); or != nil {
			return speak.NewOpenRouterTranscriber(or, s.Config.OpenRouterWhisperModel), nil
		}
	}
	tools := speak.Detect(s.Store, s.Config.WhisperModel)
	if !tools.Ready {
		return nil, errors.New("no transcription backend: " + tools.Missing)
	}
	return speak.NewLocalTranscriber(tools), nil
}

func (s *Server) speakReview(w http.ResponseWriter, r *http.Request) {
	rec, err := speak.Load(s.Store, r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	mode := strings.ToLower(r.URL.Query().Get("mode"))
	if mode == "" {
		mode = "grammar"
	}
	if rec.Review != nil && mode == "grammar" {
		writeJSON(w, rec)
		return
	}
	if rec.InterviewReview != nil && mode == "star" {
		writeJSON(w, rec)
		return
	}
	resumeContext := ""
	if s.Resume != nil {
		resumeContext = s.Resume.Context()
	}
	switch mode {
	case "star":
		review, backend, err := s.Coach.ReviewInterview(r.Context(), s.Store, rec.Prompt, rec.Transcript, resumeContext)
		if err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		rec, err = speak.ApplyInterview(s.Store, rec, review, backend)
	case "grammar":
		review, backend, err := s.Coach.ReviewSpeech(r.Context(), s.Store, rec.Prompt, rec.Transcript, resumeContext)
		if err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		rec, err = speak.Apply(s.Store, rec, review, backend)
	default:
		fail(w, http.StatusBadRequest, errors.New("unknown review mode: "+mode))
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, rec)
}

// checkedText is one pasted text between its LLM check and the confirm-to-log step.
type checkedText struct {
	ID        string             `json:"id"`
	TS        string             `json:"ts"`
	TextID    string             `json:"text_id"`
	Original  string             `json:"original"`
	Corrected string             `json:"corrected"`
	Errors    []logbook.NewError `json:"errors"`
	// Counted is how many times each rule was already in the database at check time.
	Counted map[string]int `json:"counted,omitempty"`
	Backend string         `json:"backend"`
	Cost    float64        `json:"cost"`
	Logged  int            `json:"logged,omitempty"`
}

func (s *Server) checkText(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		fail(w, http.StatusBadRequest, errors.New("text is required"))
		return
	}
	res, backend, err := s.Coach.CheckText(r.Context(), s.Store, req.Text)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	if res.Errors == nil {
		res.Errors = []logbook.NewError{}
	}
	if res.Corrected == "" {
		res.Corrected = req.Text
	}
	out := checkedText{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		TS:        time.Now().Format(time.RFC3339),
		TextID:    data.Today() + "-web-" + textSlug(req.Text),
		Original:  req.Text,
		Corrected: res.Corrected,
		Errors:    res.Errors,
		Backend:   backend,
		Cost:      res.Cost,
	}
	if entries, err := s.Store.Entries(); err == nil {
		out.Counted = ruleCount(entries)
	}
	if err := data.WriteJSON(s.Store.Path("state", "check-web", out.ID+".json"), out); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, out)
}

// textSlug names the text after its first words, for the texts/ archive filename.
func textSlug(text string) string {
	var b strings.Builder
	n := 0
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if n == 4 {
			break
		}
		b.WriteString(strings.ToLower(w))
		b.WriteString("-")
		n++
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "pasted-text"
	}
	return slug
}

// ruleCount counts how often each rule already sits in the database.
func ruleCount(entries []data.Entry) map[string]int {
	counts := map[string]int{}
	for _, e := range entries {
		counts[e.Rule]++
	}
	return counts
}

func (s *Server) logCheckedText(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.ContainsAny(id, "/\\.") {
		fail(w, http.StatusBadRequest, errors.New("bad check id"))
		return
	}
	var draft checkedText
	path := s.Store.Path("state", "check-web", id+".json")
	if _, err := os.Stat(path); err != nil {
		fail(w, http.StatusNotFound, fmt.Errorf("check %s not found", id))
		return
	}
	if err := data.ReadJSON(path, &draft); err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	if draft.Original == "" {
		fail(w, http.StatusNotFound, fmt.Errorf("check %s is empty", id))
		return
	}
	if draft.Logged > 0 {
		fail(w, http.StatusConflict, errors.New("this text is already logged"))
		return
	}
	var loggable []logbook.NewError
	var style []logbook.NewError
	for _, e := range draft.Errors {
		if e.Category == "style" {
			style = append(style, e)
			continue
		}
		loggable = append(loggable, e)
	}
	if len(loggable) > 0 {
		logged, _, err := logbook.Log(s.Store, logbook.Input{
			TextID:    draft.TextID,
			Source:    "web",
			Project:   "pasted text",
			Original:  draft.Original,
			Corrected: draft.Corrected,
			Errors:    loggable,
		})
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		draft.Logged = len(logged)
	}
	draft.Errors = append(loggable, style...)
	entries, err := s.Store.Entries()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	counts := ruleCount(entries)
	if err := data.WriteJSON(s.Store.Path("state", "check-web", id+".json"), draft); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"logged": draft.Logged, "counted": counts})
}

// explainCheckedMistake gives a drill-style modular explanation for one mistake found in a pasted text.
func (s *Server) explainCheckedMistake(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Category string             `json:"category"`
		Rule     string             `json:"rule"`
		Before   string             `json:"before"`
		After    string             `json:"after"`
		Note     string             `json:"note"`
		Previous *coach.Explanation `json:"previous"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Rule == "" || req.Before == "" || req.After == "" || len(req.Before) > 500 || len(req.After) > 500 {
		fail(w, http.StatusBadRequest, errors.New("rule, before and after under 500 characters are required"))
		return
	}
	snap, err := s.load()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	card := cards.Card{
		Type:     cards.Fix,
		Source:   "mistake",
		Category: req.Category,
		Rule:     req.Rule,
		Before:   req.Before,
		After:    req.After,
		Note:     req.Note,
		Answers:  []string{req.After},
	}
	var lessons map[string]json.RawMessage
	if err := data.ReadJSON(s.Store.Path("content", "lessons.json"), &lessons); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	var lesson strings.Builder
	for _, topic := range topics.Of(s.Config, data.Entry{Category: req.Category, Rule: req.Rule}) {
		if raw := lessons[topic]; len(raw) > 0 {
			fmt.Fprintf(&lesson, "\n%s lesson:\n%s", topic, raw)
		}
	}
	explanation, err := s.Coach.ExplainCard(r.Context(), card, req.Before, lesson.String(), req.Previous)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, map[string]any{"explanation": explanation, "history": ruleHistory(snap.entries, card)})
}

func (s *Server) growth(w http.ResponseWriter, r *http.Request) {
	snap, err := s.load()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	stats := newGrowthStats(s.Store, s.Config, snap.entries, time.Now())
	name, llmErr := s.Coach.Backend(false)
	stats.LLMOk = llmErr == nil
	stats.LLMBackend = name
	writeJSON(w, stats)
}

func (s *Server) cheer(w http.ResponseWriter, r *http.Request) {
	snap, err := s.load()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	stats := newGrowthStats(s.Store, s.Config, snap.entries, time.Now())
	card, backend, err := s.Coach.Cheer(r.Context(), coach.GrowthInput{
		Now:           stats.Now,
		Streak:        stats.Streak,
		WeekRate:      stats.WeekRate,
		PrevWeekRate:  stats.PrevWeekRate,
		TotalMistakes: stats.TotalMistakes,
		TotalTexts:    stats.TotalTexts,
		DrillAccuracy: stats.DrillAccuracy,
		RipeTopics:    stats.RipeTopics,
		TotalTopics:   stats.TotalTopics,
		TopRule:       stats.TopRule,
		TopRuleCount:  stats.TopRuleCount,
		BestMonthRate: stats.BestMonthRate,
		CurrentRate:   stats.CurrentRate,
	})
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	card.Backend = backend
	writeJSON(w, card)
}

type growthStats struct {
	Now           string      `json:"now"`
	Streak        int         `json:"streak"`
	BestStreak    int         `json:"best_streak"`
	WeekRate      float64     `json:"week_rate"`
	PrevWeekRate  float64     `json:"prev_week_rate"`
	Months        []monthRate `json:"months"`
	CurrentRate   float64     `json:"current_rate"`
	BestMonthRate float64     `json:"best_month_rate"`
	DrillAccuracy float64     `json:"drill_accuracy"`
	PeakDayRate   float64     `json:"peak_day_rate"`
	RipeTopics    int         `json:"ripe_topics"`
	TotalTopics   int         `json:"total_topics"`
	TopRule       string      `json:"top_rule"`
	TopRuleCount  int         `json:"top_rule_count"`
	NewRules      []ruleRow   `json:"new_rules"`
	TotalMistakes int         `json:"total_mistakes"`
	TotalTexts    int         `json:"total_texts"`
	LLMOk         bool        `json:"llm_ok"`
	LLMBackend    string      `json:"llm_backend,omitempty"`
}

type monthRate struct {
	Month string  `json:"month"`
	Rate  float64 `json:"rate"`
	Words int     `json:"words"`
}

// newGrowthStats assembles the numbers behind the growth card.
func newGrowthStats(st data.Store, cfg config.Config, entries []data.Entry, now time.Time) growthStats {
	checks, err := data.ReadJSONL[hook.CheckLog](st.Path("state", "checks.jsonl"))
	if err != nil {
		checks = nil
	}
	answers, err := data.ReadJSONL[cards.Answer](st.Path("state", "answers.jsonl"))
	if err != nil {
		answers = nil
	}
	sessions, _ := topics.Sessions(st)
	statuses := topics.Statuses(cfg, entries, sessions)

	out := growthStats{
		Now:           now.Format(time.RFC3339),
		Streak:        streak(answers, now),
		TotalTopics:   len(statuses),
		TotalMistakes: len(entries),
	}
	for _, t := range statuses {
		if t.Ready {
			out.RipeTopics++
		}
	}
	texts := map[string]bool{}
	rules := map[string]*ruleRow{}
	for _, e := range entries {
		texts[e.TextID] = true
		if rr, ok := rules[e.Rule]; ok {
			rr.Count++
		} else {
			rules[e.Rule] = &ruleRow{Rule: e.Rule, Category: e.Category, Count: 1, Last: e.Date}
		}
	}
	out.TotalTexts = len(texts)
	for _, rr := range rules {
		if rr.Count > out.TopRuleCount {
			out.TopRule, out.TopRuleCount = rr.Rule, rr.Count
		}
	}
	// rules logged only in the last 30 days are the fresh quarries.
	cutoff := now.AddDate(0, 0, -30).Format("2006-01-02")
	for _, rr := range rules {
		if rr.Last >= cutoff {
			out.NewRules = append(out.NewRules, *rr)
		}
	}
	sort.Slice(out.NewRules, func(i, j int) bool { return out.NewRules[i].Count > out.NewRules[j].Count })
	if len(out.NewRules) > 3 {
		out.NewRules = out.NewRules[:3]
	}

	// monthly mistake rate over checked prompt words, from the oldest month with checks.
	words, mistakes := map[string]int{}, map[string]int{}
	for _, c := range checks {
		if t, err := time.Parse(time.RFC3339, c.TS); err == nil {
			key := t.In(now.Location()).Format("2006-01")
			words[key] += c.Words
			mistakes[key] += c.Mistakes
		}
	}
	current := now.Format("2006-01")
	for key, w := range words {
		if w < 100 {
			continue
		}
		rate := float64(mistakes[key]) * 100 / float64(w)
		out.Months = append(out.Months, monthRate{Month: key, Rate: rate, Words: w})
		if out.BestMonthRate == 0 || rate < out.BestMonthRate {
			out.BestMonthRate = rate
		}
		if key == current {
			out.CurrentRate = rate
		}
	}
	if w := words[current]; w > 0 && out.CurrentRate == 0 {
		out.CurrentRate = float64(mistakes[current]) * 100 / float64(w)
	}
	sort.Slice(out.Months, func(i, j int) bool { return out.Months[i].Month < out.Months[j].Month })

	// last two full ISO weeks for the headline delta.
	lastWeek, prevWeek := weekStart(now).AddDate(0, 0, -7), weekStart(now).AddDate(0, 0, -14)
	out.WeekRate, out.PrevWeekRate = weekRates(checks, lastWeek, prevWeek, now)

	// the worst single day ever, for "you peaked at X".
	dayWords, dayMistakes := map[string]int{}, map[string]int{}
	for _, c := range checks {
		t, err := time.Parse(time.RFC3339, c.TS)
		if err != nil {
			continue
		}
		key := t.In(now.Location()).Format("2006-01-02")
		dayWords[key] += c.Words
		dayMistakes[key] += c.Mistakes
	}
	for key, w := range dayWords {
		if w >= 30 {
			if r := float64(dayMistakes[key]) * 100 / float64(w); r > out.PeakDayRate {
				out.PeakDayRate = r
			}
		}
	}
	// drill accuracy over the last 20 answers.
	const window = 20
	start := max(len(answers)-window, 0)
	right, total := 0, 0
	for _, a := range answers[start:] {
		total++
		if a.OK {
			right++
		}
	}
	if total > 0 {
		out.DrillAccuracy = float64(right) / float64(total)
	}
	return out
}

// weekRates turns two ISO weeks into mistakes per 100 checked words.
func weekRates(checks []hook.CheckLog, lastWeek, prevWeek, now time.Time) (float64, float64) {
	words, mistakes := map[string]int{}, map[string]int{}
	bucket := func(t time.Time) string {
		switch {
		case !t.Before(lastWeek):
			return "last"
		case !t.Before(prevWeek):
			return "prev"
		}
		return ""
	}
	for _, c := range checks {
		t, err := time.Parse(time.RFC3339, c.TS)
		if err != nil {
			continue
		}
		if b := bucket(t.In(now.Location())); b != "" {
			words[b] += c.Words
			mistakes[b] += c.Mistakes
		}
	}
	rate := func(b string) float64 {
		if words[b] == 0 {
			return 0
		}
		return float64(mistakes[b]) * 100 / float64(words[b])
	}
	return rate("last"), rate("prev")
}
