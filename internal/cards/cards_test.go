package cards

import (
	"reflect"
	"testing"
	"time"

	"engimprove/internal/config"
	"engimprove/internal/data"
)

var testCfg = config.Config{Topics: []config.Topic{
	{ID: "the", Categories: []string{"articles"}, Tokens: []string{"the"}, Threshold: 5},
	{ID: "a-an", Categories: []string{"articles"}, Tokens: []string{"a", "an"}, Threshold: 5},
	{ID: "prepositions", Categories: []string{"prepositions"}, Threshold: 4},
}}

func TestFromEntryCloze(t *testing.T) {
	cases := []struct {
		name    string
		entry   data.Entry
		text    string
		answers []string
		choices []string
		topics  []string
	}{
		{
			name:    "inserted the",
			entry:   data.Entry{ID: "1", Category: "articles", Before: "I opened PR yesterday", After: "I opened the PR yesterday"},
			text:    "I opened ___ PR yesterday",
			answers: []string{"the"},
			choices: []string{"a", "an", "the", None},
			topics:  []string{"the"},
		},
		{
			name:    "deleted a",
			entry:   data.Entry{ID: "2", Category: "articles", Before: "designing a systems", After: "designing systems"},
			text:    "designing ___ systems",
			answers: []string{None},
			choices: []string{"a", "an", "the", None},
			topics:  []string{"a-an"},
		},
		{
			name:    "a to an",
			entry:   data.Entry{ID: "3", Category: "articles", Before: "create a english machine", After: "create an English machine"},
			text:    "create ___ English machine",
			answers: []string{"an"},
			choices: []string{"a", "an", "the", None},
			topics:  []string{"a-an"},
		},
		{
			name:    "preposition swap",
			entry:   data.Entry{ID: "4", Category: "prepositions", Before: "working at corporate teams", After: "working in corporate teams"},
			text:    "working ___ corporate teams",
			answers: []string{"in"},
			choices: []string{"in", "at", "on", "for"},
			topics:  []string{"prepositions"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := FromEntry(testCfg, tc.entry)
			if c.Type != Cloze || c.Text != tc.text || !reflect.DeepEqual(c.Answers, tc.answers) ||
				!reflect.DeepEqual(c.Choices, tc.choices) || !reflect.DeepEqual(c.Topics, tc.topics) {
				t.Fatalf("got type=%s text=%q answers=%q choices=%q topics=%q", c.Type, c.Text, c.Answers, c.Choices, c.Topics)
			}
		})
	}
}

func TestFromEntryFallsBackToFix(t *testing.T) {
	e := data.Entry{ID: "5", Category: "dangling-modifier",
		Before: "My profile is Golang, working at corporate startup teams",
		After:  "My background is Go, and I work in corporate startup teams"}
	c := FromEntry(testCfg, e)
	if c.Type != Fix || c.Text != e.Before || c.Answers[0] != e.After {
		t.Fatalf("got %+v", c)
	}
}

func TestUnguessableGapBecomesFix(t *testing.T) {
	e := data.Entry{ID: "6", Category: "articles", Before: "Russia's Siberian city", After: "a Siberian city in Russia"}
	if c := FromEntry(testCfg, e); c.Type != Fix {
		t.Fatalf("content-word insertion kept as cloze: %q %q", c.Text, c.Answers)
	}
	swap := data.Entry{ID: "7", Category: "word-choice", Before: "my job laid the foundation", After: "my work laid the foundation"}
	if c := FromEntry(testCfg, swap); c.Type != Cloze || len(c.Choices) != 2 {
		t.Fatalf("single swap should stay a two-choice cloze: %+v", c)
	}
}

func TestContext(t *testing.T) {
	text := "# x\n\n## Original\nI helped introduce the school to the city (Russia's Siberian city) as volunteer from Moscow. " +
		"Managed exams.\n\n## Corrected\nI helped introduce the school to the city (a Siberian city in Russia).\n"
	got := Context(text, "Russia's   Siberian city")
	want := "I helped introduce the school to the city (Russia's Siberian city) as volunteer from Moscow."
	if got != want {
		t.Fatalf("Context = %q", got)
	}
	if got := Context(text, "a Siberian city in Russia"); got != "" {
		t.Fatalf("corrected section must not be searched, got %q", got)
	}
	prompts := "### 10:31\n- original: make an extensive change to global claude.md so whenever i prompt. overall goal is fine\n" +
		"- corrected: Make an extensive change to the global CLAUDE.md.\n"
	if got := Context(prompts, "change to global claude.md"); got != "make an extensive change to global claude.md so whenever i prompt." {
		t.Fatalf("prompt context = %q", got)
	}
}

func TestBoxesAndNextDue(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	all := []Card{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "unseen"}}
	tests := []struct {
		name    string
		grades  map[string][]bool
		boxes   []int
		nextDue string
	}{
		{"nothing seen", nil, []int{0, 0, 0, 0, 0, 0}, ""},
		{
			"missed and climbed",
			map[string][]bool{"a": {false}, "b": {true}, "c": {true, true}},
			[]int{1, 1, 1, 0, 0, 0},
			now.Add(10 * time.Minute).Format(time.RFC3339),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srs := SRS{}
			for id, oks := range tt.grades {
				for _, ok := range oks {
					srs.Grade(id, ok, now)
				}
			}
			if got := Boxes(all, srs); !reflect.DeepEqual(got, tt.boxes) {
				t.Fatalf("boxes = %v, want %v", got, tt.boxes)
			}
			if got := NextDue(all, srs, now); got != tt.nextDue {
				t.Fatalf("next due = %q, want %q", got, tt.nextDue)
			}
		})
	}
}

func TestGradeAndSelect(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	all := []Card{
		{ID: "old", Topics: []string{"the"}, Source: "mistake"},
		{ID: "new", Topics: []string{"the"}, Source: "mistake"},
		{ID: "pack:the:1", Topics: []string{"the"}, Source: "pack"},
		{ID: "other", Topics: []string{"a-an"}, Source: "mistake"},
	}
	srs := SRS{}
	srs.Grade("old", false, now.Add(-time.Hour))
	if !srs["old"].IsDue(now) {
		t.Fatal("missed card should be due after 10 minutes")
	}
	got := Select(all, srs, "the", 3, now)
	ids := []string{got[0].ID, got[1].ID, got[2].ID}
	if !reflect.DeepEqual(ids, []string{"old", "new", "pack:the:1"}) {
		t.Fatalf("order = %v", ids)
	}
	if review := Select(all, srs, Review, 10, now); len(review) != 1 || review[0].ID != "old" {
		t.Fatalf("review = %+v", review)
	}
	srs.Grade("old", true, now)
	if srs["old"].Box != 1 || srs["old"].IsDue(now.Add(time.Hour)) {
		t.Fatalf("after success state = %+v", srs["old"])
	}
}
