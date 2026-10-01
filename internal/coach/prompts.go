package coach

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"engimprove/internal/cards"
	"engimprove/internal/data"
	"engimprove/internal/diff"
	"engimprove/internal/logbook"
)

const errorItem = `{"type":"object","additionalProperties":false,"properties":{
"kind":{"type":"string","enum":[%s]},
"category":{"type":"string"},"rule":{"type":"string"},
"before":{"type":"string"},"after":{"type":"string"},"note":{"type":"string"}},
"required":["kind","category","rule","before","after","note"]}`

func taxonomyBlock(s data.Store, kinds ...string) (map[string]string, string, error) {
	categories, err := s.Categories()
	if err != nil {
		return nil, "", err
	}
	byKind := map[string][]string{}
	for c, k := range categories {
		byKind[k] = append(byKind[k], c)
	}
	var b strings.Builder
	b.WriteString("Allowed categories by kind:\n")
	for _, k := range kinds {
		sort.Strings(byKind[k])
		fmt.Fprintf(&b, "- %s: %s\n", k, strings.Join(byKind[k], ", "))
	}
	return categories, b.String(), nil
}

func rulesBlock(s data.Store, skipKinds ...string) (string, error) {
	entries, err := s.Entries()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Existing rules (category: rule) — reuse the wording verbatim for the same mistake:\n")
	seen := map[string]bool{}
	for _, e := range entries {
		key := e.Category + ": " + e.Rule
		skip := false
		for _, k := range skipKinds {
			skip = skip || e.Kind == k
		}
		if !seen[key] && !skip {
			seen[key] = true
			b.WriteString("- " + key + "\n")
		}
	}
	return b.String(), nil
}

// articleOnly reports the article change when a fix only adds or removes a, an or the.
func articleOnly(before, after string) (added, removed string, ok bool) {
	_, hunks := diff.Hunks(before, after)
	if len(hunks) != 1 || len(hunks[0].Added)+len(hunks[0].Removed) != 1 {
		return "", "", false
	}
	h := hunks[0]
	word := ""
	if len(h.Added) == 1 {
		word, added = h.Added[0].Lower, h.Added[0].Lower
	} else {
		word, removed = h.Removed[0], h.Removed[0]
	}
	return added, removed, word == "a" || word == "an" || word == "the"
}

func keepValid(categories map[string]string, in []logbook.NewError, banned ...string) []logbook.NewError {
	var out []logbook.NewError
	for _, e := range in {
		// models file pure article fixes under unrelated reused rules.
		if added, _, ok := articleOnly(e.Before, e.After); ok && e.Category != "articles" {
			e.Kind, e.Category = "grammar", "articles"
			switch added {
			case "the":
				e.Rule = "missing definite article before a known referent"
			case "a", "an":
				e.Rule = "missing indefinite article before a singular countable noun"
			default:
				e.Rule = "unnecessary article"
			}
		}
		k, ok := categories[e.Category]
		if !ok || k != e.Kind || strings.TrimSpace(e.After) == "" || strings.EqualFold(e.Before, e.After) {
			continue
		}
		drop := false
		for _, b := range banned {
			drop = drop || e.Category == b
		}
		if !drop {
			out = append(out, e)
		}
	}
	return out
}

// PromptCheck is the verdict on one chat prompt.
type PromptCheck struct {
	Errors    []logbook.NewError `json:"errors"`
	Original  string             `json:"original"`
	Corrected string             `json:"corrected"`
}

var promptSchema = `{"type":"object","additionalProperties":false,"properties":{
"errors":{"type":"array","items":` + fmt.Sprintf(errorItem, `"grammar","punctuation","lexical","spelling"`) + `},
"original":{"type":"string"},"corrected":{"type":"string"}},
"required":["errors","original","corrected"]}`

// CheckPrompt finds mistakes worth learning from in a chat message to a coding assistant.
func (c *Coach) CheckPrompt(ctx context.Context, s data.Store, text string, private bool) (PromptCheck, string, error) {
	categories, tax, err := taxonomyBlock(s, "grammar", "punctuation", "lexical", "spelling")
	if err != nil {
		return PromptCheck{}, "", err
	}
	rules, err := rulesBlock(s, "style")
	if err != nil {
		return PromptCheck{}, "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `You check English written by a software engineer (B1-B2), a %[1]s-speaking learner, in a chat message to an AI coding assistant.
Log only real mistakes worth learning from; he wants to fix his grammar, articles above all.

Log:
- grammar: every category (articles, tenses, prepositions, agreement, word order, conditionals, modals, ...).
- punctuation: commas, apostrophes, sentence-boundaries, hyphenation. Never capitalization.
- lexical: calques from %[1]s, wrong collocations, false friends, wrong word.
- spelling: only consistent misspellings (everytime, recieve) and homophone mix-ups (its/it's, then/than, lose/loose).

Chat register is NOT a mistake, never log it: lowercase sentence starts, lowercase "i", lowercase names
(english, claude.md, kafka), a missing final period, abbreviations (u, pls, MR, PR, FF, llm), slang and swearing,
finger typos (teh, adjacent keys, swapped letters). Missing articles inside short commands are still mistakes
("make hook async" -> "make the hook async").

When in doubt, skip. If both variants are acceptable, it is not a mistake. Never invent mistakes.
before/after: short fragments (3-10 words) quoted exactly from the text and corrected; one entry per mistake.
The category must describe the change itself: adding or removing a/an/the is always "articles", never "prepositions".
Reuse an existing rule only when it names exactly the same mistake; otherwise write a new short rule in the same style.
note: why, at most 12 words, in English.
original: only the sentences that contain logged mistakes, verbatim, except that every person's name
(e.g. "Petr" -> "<name>"), hostname, URL, token and ticket id is replaced. corrected: those same sentences,
with the same replacements, and with the logged mistakes fixed.
No mistakes: errors [], original "", corrected "".

`, c.cfg.Language)
	b.WriteString(tax)
	b.WriteString("\n")
	b.WriteString(rules)
	b.WriteString("\nMessage:\n<<<\n" + text + "\n>>>\n")

	var out PromptCheck
	backend, err := c.structured(ctx, private, "prompt_check", b.String(), promptSchema, &out)
	if err != nil {
		return PromptCheck{}, "", err
	}
	out.Errors = keepValid(categories, out.Errors, "capitalization")
	if len(out.Errors) == 0 {
		out.Original, out.Corrected = "", ""
	}
	return out, backend, nil
}

// SpeechReview is the verdict on one spoken answer.
type SpeechReview struct {
	Corrected string             `json:"corrected"`
	Errors    []logbook.NewError `json:"errors"`
	Tips      []string           `json:"tips"`
	Fluency   string             `json:"fluency"`
}

var speechSchema = `{"type":"object","additionalProperties":false,"properties":{
"corrected":{"type":"string"},
"errors":{"type":"array","items":` + fmt.Sprintf(errorItem, `"grammar","lexical","style"`) + `},
"tips":{"type":"array","items":{"type":"string"}},
"fluency":{"type":"string"}},
"required":["corrected","errors","tips","fluency"]}`

// ReviewSpeech finds grammar and word-choice mistakes in a Whisper transcript.
func (c *Coach) ReviewSpeech(ctx context.Context, s data.Store, question, transcript string) (SpeechReview, string, error) {
	categories, tax, err := taxonomyBlock(s, "grammar", "lexical", "style")
	if err != nil {
		return SpeechReview{}, "", err
	}
	rules, err := rulesBlock(s, "spelling", "punctuation")
	if err != nil {
		return SpeechReview{}, "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `You are an English grammar coach for a %[1]s-speaking software engineer (B1-B2).
He answered a speaking prompt out loud; the text below is a Whisper transcript.
Find his real grammar and word-choice mistakes. Rules:
- Ignore punctuation, capitalization and spelling: those come from the transcriber, not from him.
- Ignore fillers (um, uh, like, you know), false starts and self-corrections.
- Whisper sometimes silently fixes small slips; never invent mistakes that are not in the text.
- Do not rewrite for style. A "style" entry only for clearly unnatural phrasing.
- before/after: short fragments (3-10 words) quoted from the transcript and its fix.
- corrected: the whole transcript with only real mistakes fixed, punctuated for reading.
- tips: at most 3 short, concrete tips in English aimed at his most frequent patterns.
- fluency: one sentence in English about how natural the answer sounded.

`, c.cfg.Language)
	b.WriteString(tax)
	b.WriteString("\n")
	b.WriteString(rules)
	fmt.Fprintf(&b, "\nSpeaking prompt: %s\n\nTranscript:\n%s\n", question, transcript)

	var out SpeechReview
	backend, err := c.structured(ctx, false, "speech_review", b.String(), speechSchema, &out)
	if err != nil {
		return SpeechReview{}, "", err
	}
	out.Errors = keepValid(categories, out.Errors)
	return out, backend, nil
}

const packSchema = `{"type":"object","additionalProperties":false,"properties":{"items":{"type":"array","items":{
"type":"object","additionalProperties":false,"properties":{
"type":{"type":"string","enum":["cloze","fix"]},
"text":{"type":"string"},
"answers":{"type":"array","items":{"type":"string"}},
"choices":{"type":"array","items":{"type":"string"}},
"after":{"type":"string"},
"rule":{"type":"string"},
"note":{"type":"string"}},
"required":["type","text","answers","choices","after","rule","note"]}}},"required":["items"]}`

// GeneratePack writes n new exercises that target the rules behind the given mistakes.
func (c *Coach) GeneratePack(ctx context.Context, title string, mistakes []data.Entry, n int) ([]cards.PackItem, error) {
	var b strings.Builder
	fmt.Fprintf(&b, `Create %d new English exercises on the topic %[2]q for a %[3]s-speaking software engineer.
They must target exactly the rules behind his real mistakes listed below, weighted toward the most frequent rules.
Contexts: his work (software engineering: backend services, code review, merge requests), job hunting
(cover letters, interviews), and daily life. Never reuse his sentences verbatim.

Exercise formats:
- "cloze": text with one or two gaps written as ___ ; answers holds one string per gap in order;
  use "—" as the answer when the correct choice is no word at all (zero article, no preposition).
  For a single gap about articles give choices ["a","an","the","—"]; about prepositions give 4 choices incl. the answer;
  otherwise choices [].
- "fix": text is a natural sentence containing exactly one mistake of the kind he makes; answers holds the corrected sentence; choices [].
- after: the full correct sentence. rule: reuse his rule wording verbatim. note: why, in at most 12 words.
Mix: about two thirds cloze, one third fix. Vary difficulty; avoid items where two options are both fine.

His mistakes (before → after · rule):
`, n, title, c.cfg.Language)
	for _, e := range mistakes {
		fmt.Fprintf(&b, "- %s → %s · %s\n", e.Before, e.After, e.Rule)
	}
	var out struct {
		Items []cards.PackItem `json:"items"`
	}
	if _, err := c.structured(ctx, false, "exercise_pack", b.String(), packSchema, &out); err != nil {
		return nil, err
	}
	var keep []cards.PackItem
	for _, it := range out.Items {
		if it.Type == cards.Cloze && strings.Count(it.Text, "___") != len(it.Answers) {
			continue
		}
		keep = append(keep, it)
	}
	return keep, nil
}
