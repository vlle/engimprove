package diff

import (
	"reflect"
	"testing"
)

func TestHunks(t *testing.T) {
	cases := []struct {
		name    string
		before  string
		after   string
		removed [][]string
		added   [][]string
	}{
		{"insert article", "I opened PR yesterday", "I opened the PR yesterday", [][]string{nil}, [][]string{{"the"}}},
		{"delete article", "designing a systems", "designing systems", [][]string{{"a"}}, [][]string{nil}},
		{"replace preposition", "working at transportation domain", "working in the transportation domain",
			[][]string{{"at"}}, [][]string{{"in", "the"}}},
		{"case only is no change", "improve my english", "improve my English", nil, nil},
		{"two hunks", "as volunteer from Moscow", "as a volunteer in Moscow",
			[][]string{nil, {"from"}}, [][]string{{"a"}, {"in"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, hunks := Hunks(tc.before, tc.after)
			var removed, added [][]string
			for _, h := range hunks {
				removed = append(removed, h.Removed)
				var words []string
				for _, tok := range h.Added {
					words = append(words, tok.Text)
				}
				added = append(added, words)
			}
			if !reflect.DeepEqual(removed, tc.removed) || !reflect.DeepEqual(added, tc.added) {
				t.Fatalf("hunks removed=%q added=%q, want removed=%q added=%q", removed, added, tc.removed, tc.added)
			}
		})
	}
}

func TestTouches(t *testing.T) {
	if !Touches("in UK/Europe zone", "in the UK/Europe zone", []string{"the"}) {
		t.Fatal("inserted the not detected")
	}
	if Touches("in UK/Europe zone", "in the UK/Europe zone", []string{"a", "an"}) {
		t.Fatal("a/an reported for a the-only change")
	}
	if !Touches("an experience with", "experience with", []string{"a", "an"}) {
		t.Fatal("removed an not detected")
	}
}
