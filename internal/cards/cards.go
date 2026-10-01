// Package cards turns logged mistakes and generated packs into drill cards with spaced repetition.
package cards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/diff"
	"engimprove/internal/topics"
)

// Card types.
const (
	Cloze = "cloze"
	Fix   = "fix"
)

// None marks a gap where the right answer is no word at all.
const None = "—"

// Card is one exercise.
type Card struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Source   string   `json:"source"`
	EntryID  string   `json:"entry_id,omitempty"`
	Topics   []string `json:"topics"`
	Category string   `json:"category"`
	Rule     string   `json:"rule"`
	Text     string   `json:"text"`
	Answers  []string `json:"answers"`
	Choices  []string `json:"choices,omitempty"`
	Before   string   `json:"before,omitempty"`
	After    string   `json:"after"`
	Note     string   `json:"note,omitempty"`
	Date     string   `json:"date,omitempty"`
	// Context is the whole original sentence the mistake came from, when the archive has it.
	Context string `json:"context,omitempty"`
	Box     int    `json:"box"`
	Seen    int    `json:"seen"`
	Due     string `json:"due,omitempty"`
}

// PackItem is one generated exercise in drills/packs/<topic>.jsonl.
type PackItem struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Text    string   `json:"text"`
	Answers []string `json:"answers"`
	Choices []string `json:"choices,omitempty"`
	After   string   `json:"after"`
	Rule    string   `json:"rule"`
	Note    string   `json:"note,omitempty"`
}

var (
	articles     = []string{"a", "an", "the"}
	prepositions = []string{"in", "on", "at", "for", "to", "of", "with", "from", "by", "about", "into", "as", "over", "since", "during"}
	// function words can be guessed from the sentence; content words cannot.
	functionWords = append(append([]string{
		"and", "or", "but", "so", "than", "then", "that", "which", "who", "if", "not", "no",
		"is", "are", "was", "were", "be", "been", "being", "am", "have", "has", "had",
		"do", "does", "did", "don't", "doesn't", "didn't", "will", "would", "can", "could",
		"should", "must", "may", "might", "it", "its", "it's", "there", "their", "they",
		"them", "this", "these", "those", "i", "we", "you", "he", "she", "my", "your", "our",
	}, articles...), prepositions...)
)

func guessable(hunks []diff.Hunk) bool {
	for _, h := range hunks {
		for _, t := range h.Added {
			if !slices.Contains(functionWords, t.Lower) {
				// a single swap still works: both versions become the choices.
				if len(hunks) == 1 && len(h.Removed) > 0 {
					continue
				}
				return false
			}
		}
	}
	return true
}

// FromEntry builds the card for one mistake: a cloze when the change is small, a fix-it otherwise.
func FromEntry(cfg config.Config, e data.Entry) Card {
	c := Card{
		ID:       e.ID,
		Source:   "mistake",
		EntryID:  e.ID,
		Topics:   topics.Of(cfg, e),
		Category: e.Category,
		Rule:     e.Rule,
		Before:   e.Before,
		After:    e.After,
		Note:     e.Note,
		Date:     e.Date,
	}
	if text, answers, removed, ok := cloze(e.Before, e.After); ok {
		c.Type = Cloze
		c.Text = text
		c.Answers = answers
		if len(answers) == 1 {
			c.Choices = choices(answers[0], removed[0])
		}
		return c
	}
	c.Type = Fix
	c.Text = e.Before
	c.Answers = []string{e.After}
	return c
}

func cloze(before, after string) (string, []string, [][]string, bool) {
	tokens, hunks := diff.Hunks(before, after)
	if len(hunks) == 0 || len(hunks) > 2 || len(tokens) < 2 {
		return "", nil, nil, false
	}
	for _, h := range hunks {
		if len(h.Added) > 2 || len(h.Removed) > 2 {
			return "", nil, nil, false
		}
	}
	if !guessable(hunks) {
		return "", nil, nil, false
	}
	var b strings.Builder
	var answers []string
	var removed [][]string
	pos := 0
	for _, h := range hunks {
		removed = append(removed, h.Removed)
		if len(h.Added) > 0 {
			start, end := h.Added[0].Start, h.Added[len(h.Added)-1].End
			b.WriteString(after[pos:start])
			b.WriteString("___")
			pos = end
			words := make([]string, len(h.Added))
			for i, t := range h.Added {
				words[i] = t.Text
			}
			answers = append(answers, strings.Join(words, " "))
			continue
		}
		if h.At < len(tokens) {
			at := tokens[h.At].Start
			b.WriteString(after[pos:at])
			b.WriteString("___ ")
			pos = at
		} else {
			b.WriteString(after[pos:])
			b.WriteString(" ___")
			pos = len(after)
		}
		answers = append(answers, None)
	}
	b.WriteString(after[pos:])
	return b.String(), answers, removed, true
}

func choices(answer string, removed []string) []string {
	ans := strings.ToLower(answer)
	rem := strings.Join(removed, " ")
	inSet := func(set []string, w string) bool { return w == "" || w == None || slices.Contains(set, w) }
	if inSet(articles, ans) && inSet(articles, rem) {
		return []string{"a", "an", "the", None}
	}
	if slices.Contains(prepositions, ans) || slices.Contains(prepositions, rem) {
		out := []string{answer}
		if rem != "" && rem != ans {
			out = append(out, rem)
		}
		for _, p := range prepositions {
			if len(out) == 4 {
				break
			}
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
		return out
	}
	if rem != "" && rem != ans {
		return []string{answer, rem}
	}
	return nil
}

// FromPack converts generated pack items of one topic.
func FromPack(topic string, items []PackItem) []Card {
	out := make([]Card, 0, len(items))
	for _, it := range items {
		if it.Text == "" || len(it.Answers) == 0 {
			continue
		}
		typ := it.Type
		if typ != Cloze {
			typ = Fix
		}
		out = append(out, Card{
			ID:      "pack:" + topic + ":" + it.ID,
			Type:    typ,
			Source:  "pack",
			Topics:  []string{topic},
			Rule:    it.Rule,
			Text:    it.Text,
			Answers: it.Answers,
			Choices: it.Choices,
			After:   it.After,
			Note:    it.Note,
		})
	}
	return out
}

// a dot glued to the next character (claude.md, 1.5) does not end a sentence.
var sentenceRe = regexp.MustCompile(`(?:[^.!?\n]|[.!?][^\s.!?])+[.!?]*`)

// originalText keeps only what the user wrote: the Original section or "- original:" lines.
func originalText(archive string) string {
	var b strings.Builder
	if _, rest, ok := strings.Cut(archive, "\n## Original"); ok {
		section, _, _ := strings.Cut(rest, "\n## ")
		b.WriteString(section)
		b.WriteString("\n")
	}
	for line := range strings.SplitSeq(archive, "\n") {
		if text, ok := strings.CutPrefix(strings.TrimSpace(line), "- original:"); ok {
			b.WriteString(text)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func squash(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// Context finds the sentence of the archived original that contains the fragment.
func Context(archive, fragment string) string {
	want := squash(fragment)
	if want == "" {
		return ""
	}
	for para := range strings.SplitSeq(originalText(archive), "\n") {
		for _, sentence := range sentenceRe.FindAllString(para, -1) {
			sentence = strings.Join(strings.Fields(sentence), " ")
			if strings.Contains(squash(sentence), want) {
				if len(sentence) > 400 {
					return ""
				}
				return sentence
			}
		}
	}
	return ""
}

// All builds cards for every mistake and every pack.
func All(s data.Store, cfg config.Config, entries []data.Entry) ([]Card, error) {
	archives := map[string]string{}
	out := make([]Card, 0, len(entries))
	for _, e := range entries {
		if e.Before == "" || e.After == "" || strings.HasPrefix(e.After, "(") {
			continue
		}
		c := FromEntry(cfg, e)
		archive, ok := archives[e.TextID]
		if !ok && !strings.ContainsAny(e.TextID, "/\\") {
			raw, _ := os.ReadFile(s.Path("texts", e.TextID+".md"))
			archive = string(raw)
			archives[e.TextID] = archive
		}
		if ctx := Context(archive, e.Before); ctx != "" && squash(ctx) != squash(e.Before) {
			c.Context = ctx
		}
		out = append(out, c)
	}
	files, err := filepath.Glob(s.Path("drills", "packs", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		items, err := data.ReadJSONL[PackItem](f)
		if err != nil {
			return nil, err
		}
		out = append(out, FromPack(strings.TrimSuffix(filepath.Base(f), ".jsonl"), items)...)
	}
	return out, nil
}

// State is the spaced-repetition record of one card.
type State struct {
	Box     int    `json:"box"`
	Due     string `json:"due"`
	Seen    int    `json:"seen"`
	Correct int    `json:"correct"`
	Last    string `json:"last"`
}

// SRS maps card id to its state.
type SRS map[string]State

var intervals = []time.Duration{
	10 * time.Minute,
	24 * time.Hour,
	3 * 24 * time.Hour,
	7 * 24 * time.Hour,
	21 * 24 * time.Hour,
	60 * 24 * time.Hour,
}

// LoadSRS reads state/srs.json.
func LoadSRS(s data.Store) (SRS, error) {
	srs := SRS{}
	err := data.ReadJSON(s.Path("state", "srs.json"), &srs)
	return srs, err
}

// Save writes state/srs.json.
func (srs SRS) Save(s data.Store) error {
	return data.WriteJSON(s.Path("state", "srs.json"), srs)
}

// Grade moves a card up a box on success and back to the first box on a miss.
func (srs SRS) Grade(id string, ok bool, now time.Time) State {
	st := srs[id]
	st.Seen++
	if ok {
		st.Correct++
		st.Box = min(st.Box+1, len(intervals)-1)
	} else {
		st.Box = 0
	}
	st.Due = now.Add(intervals[st.Box]).Format(time.RFC3339)
	st.Last = now.Format(time.RFC3339)
	srs[id] = st
	return st
}

// IsDue reports whether a seen card should be reviewed now.
func (st State) IsDue(now time.Time) bool {
	if st.Seen == 0 {
		return false
	}
	due, err := time.Parse(time.RFC3339, st.Due)
	return err != nil || !due.After(now)
}

// Review is the pseudo-topic that serves due cards across all topics.
const Review = "review"

// Select picks up to n cards for a topic: due reviews, then unseen mistakes, then packs, then weakest seen.
func Select(all []Card, srs SRS, topic string, n int, now time.Time) []Card {
	var due, fresh, packs, weak []Card
	for _, c := range all {
		if topic != Review && topic != "" && !slices.Contains(c.Topics, topic) {
			continue
		}
		st := srs[c.ID]
		c.Box, c.Seen, c.Due = st.Box, st.Seen, st.Due
		switch {
		case st.IsDue(now):
			due = append(due, c)
		case topic == Review:
		case st.Seen == 0 && c.Source == "pack":
			packs = append(packs, c)
		case st.Seen == 0:
			fresh = append(fresh, c)
		default:
			weak = append(weak, c)
		}
	}
	sort.SliceStable(due, func(i, j int) bool { return due[i].Due < due[j].Due })
	sort.SliceStable(fresh, func(i, j int) bool { return fresh[i].ID > fresh[j].ID })
	sort.SliceStable(weak, func(i, j int) bool {
		if weak[i].Box != weak[j].Box {
			return weak[i].Box < weak[j].Box
		}
		return weak[i].Due < weak[j].Due
	})
	out := make([]Card, 0, n)
	for _, group := range [][]Card{due, fresh, packs, weak} {
		for _, c := range group {
			if len(out) == n {
				return out
			}
			out = append(out, c)
		}
	}
	return out
}

// CountDue counts seen cards whose review time has come.
func CountDue(all []Card, srs SRS, now time.Time) int {
	n := 0
	for _, c := range all {
		if srs[c.ID].IsDue(now) {
			n++
		}
	}
	return n
}

// Answer is one graded attempt in state/answers.jsonl.
type Answer struct {
	TS     string   `json:"ts"`
	Card   string   `json:"card"`
	OK     bool     `json:"ok"`
	Topics []string `json:"topics,omitempty"`
	Given  string   `json:"given,omitempty"`
}

// Record grades a card, persists the srs state and logs the attempt.
func Record(s data.Store, card Card, ok bool, given string, now time.Time) (State, error) {
	var st State
	err := s.Locked("srs", func() error {
		srs, err := LoadSRS(s)
		if err != nil {
			return err
		}
		st = srs.Grade(card.ID, ok, now)
		if err := srs.Save(s); err != nil {
			return err
		}
		return data.AppendJSONL(s.Path("state", "answers.jsonl"), Answer{
			TS: now.Format(time.RFC3339), Card: card.ID, OK: ok, Topics: card.Topics, Given: given,
		})
	})
	return st, err
}

// Find returns the card with id.
func Find(all []Card, id string) (Card, bool) {
	for _, c := range all {
		if c.ID == id {
			return c, true
		}
	}
	return Card{}, false
}

// AppendPack adds generated items to a topic pack, numbering ids after the existing ones.
func AppendPack(s data.Store, topic string, items []PackItem) (int, error) {
	path := s.Path("drills", "packs", topic+".jsonl")
	added := 0
	err := s.Locked("packs", func() error {
		existing, err := data.ReadJSONL[PackItem](path)
		if err != nil {
			return err
		}
		next := len(existing) + 1
		var keep []PackItem
		seen := map[string]bool{}
		for _, it := range existing {
			seen[strings.ToLower(it.Text)] = true
		}
		for _, it := range items {
			key := strings.ToLower(strings.TrimSpace(it.Text))
			if key == "" || len(it.Answers) == 0 || seen[key] {
				continue
			}
			seen[key] = true
			it.ID = topic + "-" + pad(next)
			next++
			keep = append(keep, it)
		}
		added = len(keep)
		if added == 0 {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return data.AppendJSONL(path, keep...)
	})
	return added, err
}

func pad(n int) string {
	return fmt.Sprintf("%03d", n)
}
