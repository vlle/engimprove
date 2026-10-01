package coach

import (
	"testing"

	"engimprove/internal/logbook"
)

func TestKeepValidRefilesArticleFixes(t *testing.T) {
	categories := map[string]string{"articles": "grammar", "prepositions": "grammar", "capitalization": "punctuation"}
	in := []logbook.NewError{
		{Kind: "grammar", Category: "prepositions", Rule: "wrong preposition with team", Before: "Petr from team", After: "Petr from the team"},
		{Kind: "grammar", Category: "prepositions", Rule: "wrong preposition with team", Before: "at corporate teams", After: "in corporate teams"},
		{Kind: "punctuation", Category: "capitalization", Rule: "lowercase i", Before: "i think", After: "I think"},
		{Kind: "grammar", Category: "articles", Rule: "same text", Before: "the pod", After: "The pod"},
	}
	out := keepValid(categories, in, "capitalization")
	if len(out) != 2 {
		t.Fatalf("kept %d, want 2: %+v", len(out), out)
	}
	if out[0].Category != "articles" || out[0].Rule != "missing definite article before a known referent" {
		t.Fatalf("article fix not refiled: %+v", out[0])
	}
	if out[1].Category != "prepositions" {
		t.Fatalf("real preposition fix changed: %+v", out[1])
	}
}
