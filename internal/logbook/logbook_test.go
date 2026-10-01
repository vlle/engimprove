package logbook

import (
	"os"
	"strings"
	"testing"

	"engimprove/internal/testutil"
)

const seed = `{"id":"2026-10-01-001","date":"2026-10-01","text_id":"t","kind":"grammar","category":"articles","rule":"missing definite article before a known referent","before":"a","after":"the a"}`

func TestLogAssignsIDsCountsAndArchives(t *testing.T) {
	st, _ := testutil.Store(t, seed)
	logged, textID, err := Log(st, Input{
		Date: "2026-10-01", Source: "prompt", Project: "PROJECT",
		Original: "make hook async", Corrected: "make the hook async",
		Errors: []NewError{
			{Category: "articles", Rule: "missing definite article before a known referent", Before: "make hook async", After: "make the hook async"},
			{Category: "modals", Rule: "would in a purpose clause", Before: "so i would not wait", After: "so that I don't have to wait"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if textID != "2026-10-01-prompts-project" {
		t.Fatalf("text_id = %s", textID)
	}
	if logged[0].ID != "2026-10-01-002" || logged[0].Count != 2 || len(logged[0].Similar) != 0 {
		t.Fatalf("first = %+v", logged[0])
	}
	if logged[1].ID != "2026-10-01-003" || logged[1].Count != 1 {
		t.Fatalf("second = %+v", logged[1])
	}
	entries, err := st.Entries()
	if err != nil || len(entries) != 3 || entries[2].Kind != "grammar" {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	archive, err := os.ReadFile(st.Path("texts", textID+".md"))
	if err != nil || !strings.Contains(string(archive), "- corrected: make the hook async") {
		t.Fatalf("archive = %q, %v", archive, err)
	}
	if _, err := os.Stat(st.Path("errors", "stats.md")); err != nil {
		t.Fatal("stats.md not regenerated")
	}
}

func TestLogKeepsCategoryOfReusedRule(t *testing.T) {
	st, _ := testutil.Store(t, seed)
	logged, _, err := Log(st, Input{TextID: "x", Errors: []NewError{{Kind: "grammar", Category: "modals",
		Rule: "missing definite article before a known referent", Before: "fix bug", After: "fix the bug"}}})
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := st.Entries()
	if logged[0].Category != "articles" || entries[1].Category != "articles" || entries[1].Kind != "grammar" {
		t.Fatalf("logged %+v, stored %+v", logged[0], entries[1])
	}
}

func TestLogRejectsUnknownCategoryAndKindMismatch(t *testing.T) {
	st, _ := testutil.Store(t)
	if _, _, err := Log(st, Input{TextID: "x", Errors: []NewError{{Category: "nope", Rule: "r", After: "a"}}}); err == nil {
		t.Fatal("unknown category accepted")
	}
	if _, _, err := Log(st, Input{TextID: "x", Errors: []NewError{{Kind: "lexical", Category: "articles", Rule: "r", After: "a"}}}); err == nil {
		t.Fatal("kind mismatch accepted")
	}
}
