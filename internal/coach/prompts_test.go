package coach

import (
	"strings"
	"testing"

	"engimprove/internal/config"
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

func TestPromptSchemaAndBlocksFollowFlags(t *testing.T) {
	off := config.Config{Language: "Russian"}
	on := off
	on.PromptStyle, on.PromptTranslation = true, true

	offSchema := promptSchema(off)
	if strings.Contains(offSchema, `"style"`) || strings.Contains(offSchema, "translation") {
		t.Fatalf("flags off but schema leaks them: %s", offSchema)
	}
	onSchema := promptSchema(on)
	for _, want := range []string{`"style"`, `"translation"`, `"perception"`} {
		if !strings.Contains(onSchema, want) {
			t.Fatalf("schema misses %s: %s", want, onSchema)
		}
	}
	if styleBlock(off) != "" || perceptionBlock(off) != "" {
		t.Fatal("flag off but block is non-empty")
	}
	if style := styleBlock(on); !strings.Contains(style, "style:") {
		t.Fatalf("style block = %q", style)
	}
	if block := perceptionBlock(on); !strings.Contains(block, "Russian") || !strings.Contains(block, "perception:") {
		t.Fatalf("perception block = %q", block)
	}
}
