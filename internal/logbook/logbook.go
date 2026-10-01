// Package logbook appends new mistakes with validated categories, fresh ids and repeat counts.
package logbook

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"engimprove/internal/data"
	"engimprove/internal/stats"
)

// NewError is one mistake before it gets an id.
type NewError struct {
	Kind     string `json:"kind,omitempty"`
	Category string `json:"category"`
	Rule     string `json:"rule"`
	Before   string `json:"before"`
	After    string `json:"after"`
	Note     string `json:"note,omitempty"`
}

// Input is a batch of mistakes from one text, prompt or recording.
type Input struct {
	TextID    string `json:"text_id,omitempty"`
	Date      string `json:"date,omitempty"`
	Project   string `json:"project,omitempty"`
	Source    string `json:"source,omitempty"`
	Original  string `json:"original,omitempty"`
	Corrected string `json:"corrected,omitempty"`
	// Language is the learner's language from the config; it heads the regenerated stats.
	Language string     `json:"language,omitempty"`
	Errors   []NewError `json:"errors"`
}

// Logged reports one appended mistake.
type Logged struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Rule     string `json:"rule"`
	// Count is how many times the rule appears in the log, this entry included.
	Count int `json:"count"`
	// Similar lists existing rules of the same category when the rule is new.
	Similar []string `json:"similar,omitempty"`
}

// Log validates and appends a batch, archives the text and regenerates stats.md.
func Log(s data.Store, in Input) ([]Logged, string, error) {
	if len(in.Errors) == 0 {
		return nil, "", fmt.Errorf("no errors to log")
	}
	if in.Date == "" {
		in.Date = data.Today()
	}
	if _, err := time.Parse("2006-01-02", in.Date); err != nil {
		return nil, "", fmt.Errorf("date must be YYYY-MM-DD, got %q", in.Date)
	}
	if in.TextID == "" {
		if in.Source == "" || in.Project == "" {
			return nil, "", fmt.Errorf("text_id is required unless source and project are set")
		}
		kind := in.Source
		if kind == "prompt" {
			kind = "prompts"
		}
		in.TextID = fmt.Sprintf("%s-%s-%s", in.Date, kind, slug(in.Project))
	}
	categories, err := s.Categories()
	if err != nil {
		return nil, "", err
	}
	existing, err := s.Entries()
	if err != nil {
		return nil, "", err
	}
	ruleCategory := map[string]string{}
	for _, e := range existing {
		ruleCategory[e.Rule] = e.Category
	}
	for i, e := range in.Errors {
		// a reused rule keeps its category, or stats split.
		if c, ok := ruleCategory[e.Rule]; ok && c != e.Category {
			in.Errors[i].Category, in.Errors[i].Kind = c, ""
			e = in.Errors[i]
		}
		kind, ok := categories[e.Category]
		if !ok {
			return nil, "", fmt.Errorf("error %d: category %q is not in taxonomy.md", i+1, e.Category)
		}
		if e.Kind == "" {
			in.Errors[i].Kind = kind
		} else if e.Kind != kind {
			return nil, "", fmt.Errorf("error %d: category %q belongs to kind %q, not %q", i+1, e.Category, kind, e.Kind)
		}
		if strings.TrimSpace(e.Rule) == "" || strings.TrimSpace(e.After) == "" {
			return nil, "", fmt.Errorf("error %d: rule and after are required", i+1)
		}
	}

	var logged []Logged
	err = s.Locked("errors", func() error {
		entries, err := s.Entries()
		if err != nil {
			return err
		}
		ids := data.NextIDs(entries, in.Date, len(in.Errors))
		counts := data.RuleCounts(entries)
		fresh := make([]data.Entry, len(in.Errors))
		for i, e := range in.Errors {
			fresh[i] = data.Entry{
				ID: ids[i], Date: in.Date, TextID: in.TextID, Kind: in.Errors[i].Kind, Category: e.Category,
				Rule: e.Rule, Before: e.Before, After: e.After, Note: e.Note,
			}
			l := Logged{ID: ids[i], Category: e.Category, Rule: e.Rule}
			if counts[e.Rule] == 0 {
				l.Similar = similar(entries, e.Category)
			}
			counts[e.Rule]++
			l.Count = counts[e.Rule]
			logged = append(logged, l)
		}
		if err := data.AppendJSONL(s.Path("errors", "errors.jsonl"), fresh...); err != nil {
			return err
		}
		return stats.WriteMarkdown(s, append(entries, fresh...), in.Language)
	})
	if err != nil {
		return nil, "", err
	}
	if in.Original != "" {
		if err := archive(s, in); err != nil {
			return logged, in.TextID, fmt.Errorf("mistakes logged, archive failed: %w", err)
		}
	}
	return logged, in.TextID, nil
}

func similar(entries []data.Entry, category string) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		if e.Category == category && !seen[e.Rule] {
			seen[e.Rule] = true
			out = append(out, e.Rule)
		}
	}
	sort.Strings(out)
	return out
}

func archive(s data.Store, in Input) error {
	path := s.Path("texts", in.TextID+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(&b, "# %s\n\n- date: %s\n- text_id: %s\n- source: %s\n",
			strings.TrimPrefix(in.TextID, in.Date+"-"), in.Date, in.TextID,
			strings.Trim(in.Source+" / "+in.Project, " /"))
	}
	fmt.Fprintf(&b, "\n### %s\n- original: %s\n", time.Now().Format("15:04"), oneLine(in.Original))
	if in.Corrected != "" {
		fmt.Fprintf(&b, "- corrected: %s\n", oneLine(in.Corrected))
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case slices.Contains([]rune{'-', '_', ' ', '.'}, r):
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
