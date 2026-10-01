// Package lang decides whether a prompt was written in English.
package lang

import (
	"regexp"
	"strings"
)

var (
	pasted     = regexp.MustCompile(`(?s)<pasted_content[^>]*>.*?</pasted_content[^>]*>`)
	fenced     = regexp.MustCompile("(?s)```.*?```")
	inlineCode = regexp.MustCompile("`[^`]*`")
	url        = regexp.MustCompile(`https?://\S+`)
	pathLike   = regexp.MustCompile(`\S*[/\\]\S*`)
	latinWord  = regexp.MustCompile(`[A-Za-z]{2,}`)
	latin      = regexp.MustCompile(`[A-Za-z]`)
	cyrillic   = regexp.MustCompile(`\p{Cyrillic}`)
)

// Result describes the user-written part of a prompt.
type Result struct {
	English bool
	Words   int
	Text    string
}

// Analyze strips inserted material and measures the language of what is left.
func Analyze(prompt string) Result {
	text := strings.TrimSpace(prompt)
	if strings.HasPrefix(text, "/") {
		// slash command name is not prose, its arguments are.
		_, text, _ = strings.Cut(text, " ")
	}
	for _, re := range []*regexp.Regexp{pasted, fenced, inlineCode, url, pathLike} {
		text = re.ReplaceAllString(text, " ")
	}
	words := len(latinWord.FindAllString(text, -1))
	lat := len(latin.FindAllString(text, -1))
	cyr := len(cyrillic.FindAllString(text, -1))
	return Result{
		English: words >= 4 && lat >= 4*cyr,
		Words:   words,
		Text:    strings.Join(strings.Fields(text), " "),
	}
}
