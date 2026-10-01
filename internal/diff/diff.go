// Package diff aligns a wrong fragment with its correction word by word.
package diff

import (
	"regexp"
	"strings"
)

var wordRe = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}'’\-]*`)

// Token is one word with its byte span in the source string.
type Token struct {
	Text  string
	Lower string
	Start int
	End   int
}

// Hunk is one contiguous change: what was removed from before and added in after.
type Hunk struct {
	Removed []string
	Added   []Token
	// At is the index in after tokens where the change sits (insertion point when Added is empty).
	At int
}

// Tokenize splits s into words; punctuation is dropped.
func Tokenize(s string) []Token {
	spans := wordRe.FindAllStringIndex(s, -1)
	out := make([]Token, len(spans))
	for i, sp := range spans {
		text := s[sp[0]:sp[1]]
		out[i] = Token{Text: text, Lower: strings.ToLower(text), Start: sp[0], End: sp[1]}
	}
	return out
}

// Hunks returns the after tokens and the case-insensitive word changes from before to after.
func Hunks(before, after string) ([]Token, []Hunk) {
	b := Tokenize(before)
	a := Tokenize(after)
	n, m := len(b), len(a)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if b[i].Lower == a[j].Lower {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}

	var hunks []Hunk
	var cur *Hunk
	flush := func() {
		if cur != nil {
			hunks = append(hunks, *cur)
			cur = nil
		}
	}
	open := func(at int) {
		if cur == nil {
			cur = &Hunk{At: at}
		}
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && b[i].Lower == a[j].Lower:
			flush()
			i++
			j++
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			open(j)
			cur.Added = append(cur.Added, a[j])
			j++
		default:
			open(j)
			cur.Removed = append(cur.Removed, b[i].Lower)
			i++
		}
	}
	flush()
	return a, hunks
}

// Touches reports whether any changed word is in words.
func Touches(before, after string, words []string) bool {
	if len(words) == 0 {
		return true
	}
	set := map[string]bool{}
	for _, w := range words {
		set[strings.ToLower(w)] = true
	}
	_, hunks := Hunks(before, after)
	for _, h := range hunks {
		for _, r := range h.Removed {
			if set[r] {
				return true
			}
		}
		for _, t := range h.Added {
			if set[t.Lower] {
				return true
			}
		}
	}
	return false
}
