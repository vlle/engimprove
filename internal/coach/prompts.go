package coach

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"engimprove/internal/cards"
	"engimprove/internal/config"
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
	// Translation is the message rendered in the learner's language, when enabled.
	Translation string `json:"translation,omitempty"`
	// Perception is one line on how a native reader takes the message, when enabled.
	Perception string  `json:"perception,omitempty"`
	Cost       float64 `json:"cost"`
}

var promptKinds = []string{"grammar", "punctuation", "lexical", "spelling"}

// CheckPrompt finds mistakes worth learning from in a chat message to a coding assistant.
func (c *Coach) CheckPrompt(ctx context.Context, s data.Store, text string, private bool) (PromptCheck, string, error) {
	kinds := kindsFor(c.cfg)
	skip := []string{"style"}
	if c.cfg.PromptStyle {
		skip = nil
	}
	categories, tax, err := taxonomyBlock(s, kinds...)
	if err != nil {
		return PromptCheck{}, "", err
	}
	rules, err := rulesBlock(s, skip...)
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
%[2]s
Chat register is NOT a mistake, never log it: lowercase sentence starts, lowercase "i", lowercase names
(english, claude.md, kafka), a missing final period, abbreviations (u, pls, MR, PR, FF, llm), slang and swearing,
finger typos (teh, adjacent keys, swapped letters). Missing articles inside short commands are still mistakes
("make hook async" -> "make the hook async"). This applies to style too: terse chat commands are fine.

When in doubt, skip. If both variants are acceptable, it is not a mistake. Never invent mistakes.
before/after: short fragments (3-10 words) quoted exactly from the text and corrected; one entry per mistake.
The category must describe the change itself: adding or removing a/an/the is always "articles", never "prepositions".
Reuse an existing rule only when it names exactly the same mistake; otherwise write a new short rule in the same style.
note: why, at most 12 words, in English.
original: only the sentences that contain logged mistakes, verbatim, except that every person's name
(e.g. "Petr" -> "<name>"), hostname, URL, token and ticket id is replaced. corrected: those same sentences,
with the same replacements, and with the logged mistakes fixed.
%[3]sNo mistakes: errors [], original "", corrected "".

`, c.cfg.Language, styleBlock(c.cfg), perceptionBlock(c.cfg))
	b.WriteString(tax)
	b.WriteString("\n")
	b.WriteString(rules)
	b.WriteString("\nMessage:\n<<<\n" + text + "\n>>>\n")

	var out PromptCheck
	backend, usage, err := c.structured(ctx, private, "prompt_check", b.String(), promptSchema(c.cfg), &out)
	if err != nil {
		return PromptCheck{}, "", err
	}
	if usage != nil {
		out.Cost = usage.Cost
	}
	out.Errors = keepValid(categories, out.Errors, "capitalization")
	if len(out.Errors) == 0 {
		out.Original, out.Corrected = "", ""
	}
	return out, backend, nil
}

// styleBlock is the style section of the prompt check, empty when the flag is off.
func styleBlock(cfg config.Config) string {
	if !cfg.PromptStyle {
		return ""
	}
	return `- style: wordiness, hedging, redundancy, register that a native reader would find off
  (rude, abrupt, grovelling). Log one entry per clearly improvable phrase, not a rewrite.
`
}

// perceptionBlock is the translation section of the prompt check, empty when the flag is off.
func perceptionBlock(cfg config.Config) string {
	if !cfg.PromptTranslation {
		return ""
	}
	return fmt.Sprintf(`translation: the whole message rendered in %[1]s, keeping code, identifiers and names as they are;
it shows him what he actually conveyed. Empty for trivial prompts ("continue", "ok").
perception: one short line in %[1]s on how a native reader would take the message and its author:
tone, politeness, directness. Base it on wording, not content. Empty for trivial prompts.
`, cfg.Language)
}

func promptSchema(cfg config.Config) string {
	props, required := `"original":{"type":"string"},"corrected":{"type":"string"}`,
		`["errors","original","corrected"]`
	if cfg.PromptTranslation {
		props += `,"translation":{"type":"string"},"perception":{"type":"string"}`
		required = `["errors","original","corrected","translation","perception"]`
	}
	return `{"type":"object","additionalProperties":false,"properties":{
"errors":{"type":"array","items":` + fmt.Sprintf(errorItem, `"`+strings.Join(kindsFor(cfg), `","`)+`"`) + `},
` + props + `},
"required":` + required + `}`
}

func kindsFor(cfg config.Config) []string {
	if cfg.PromptStyle {
		return append(slices.Clone(promptKinds), "style")
	}
	return promptKinds
}

// SpeechReview is the verdict on one spoken answer.
type SpeechReview struct {
	Corrected string             `json:"corrected"`
	Errors    []logbook.NewError `json:"errors"`
	Tips      []string           `json:"tips"`
	Fluency   string             `json:"fluency"`
	Cost      float64            `json:"cost"`
}

var speechSchema = `{"type":"object","additionalProperties":false,"properties":{
"corrected":{"type":"string"},
"errors":{"type":"array","items":` + fmt.Sprintf(errorItem, `"grammar","lexical","style"`) + `},
"tips":{"type":"array","items":{"type":"string"}},
"fluency":{"type":"string"}},
"required":["corrected","errors","tips","fluency"]}`

// ReviewSpeech finds grammar and word-choice mistakes in a Whisper transcript.
func (c *Coach) ReviewSpeech(ctx context.Context, s data.Store, question, transcript, resumeContext string) (SpeechReview, string, error) {
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
	if resumeContext != "" && isAboutSpeaker(question) {
		b.WriteString("\nThe speaker's verified background (use it to keep claims accurate and suggest concrete facts when the answer is vague):\n")
		b.WriteString(resumeContext)
	}
	fmt.Fprintf(&b, "\nSpeaking prompt: %s\n\nTranscript:\n%s\n", question, transcript)

	var out SpeechReview
	backend, usage, err := c.structured(ctx, false, "speech_review", b.String(), speechSchema, &out)
	if err != nil {
		return SpeechReview{}, "", err
	}
	if usage != nil {
		out.Cost = usage.Cost
	}
	out.Errors = keepValid(categories, out.Errors)
	return out, backend, nil
}

// isAboutSpeaker guesses whether a prompt asks the user to talk about themselves.
func isAboutSpeaker(question string) bool {
	q := strings.ToLower(question)
	for _, phrase := range []string{
		"tell me about yourself", "yourself", "your experience", "your background",
		"your work", "you do", "you have solved", "you disagreed", "you made",
		"why are you", "looking for", "yourself in two minutes", "job interview",
		"about a time", "tell me about a time", "describe a time", "give an example",
	} {
		if strings.Contains(q, phrase) {
			return true
		}
	}
	return false
}

// StarScore is one STAR dimension with a numeric score and feedback.
type StarScore struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

// InterviewReview is structured feedback on a behavioral interview answer.
type InterviewReview struct {
	STAR struct {
		Situation StarScore `json:"situation"`
		Task      StarScore `json:"task"`
		Action    StarScore `json:"action"`
		Result    StarScore `json:"result"`
	} `json:"star"`
	OverallScore int      `json:"overall_score"`
	Rewrite      string   `json:"rewrite"`
	FollowUps    []string `json:"follow_ups"`
	Strengths    []string `json:"strengths"`
	Gaps         []string `json:"gaps"`
	StoryMatch   string   `json:"story_match"`
	Cost         float64  `json:"cost"`
}

var interviewSchema = `{"type":"object","additionalProperties":false,"properties":{
"star":{"type":"object","additionalProperties":false,"properties":{
  "situation":{"type":"object","additionalProperties":false,"properties":{"score":{"type":"integer","minimum":1,"maximum":5},"feedback":{"type":"string"}},"required":["score","feedback"]},
  "task":{"type":"object","additionalProperties":false,"properties":{"score":{"type":"integer","minimum":1,"maximum":5},"feedback":{"type":"string"}},"required":["score","feedback"]},
  "action":{"type":"object","additionalProperties":false,"properties":{"score":{"type":"integer","minimum":1,"maximum":5},"feedback":{"type":"string"}},"required":["score","feedback"]},
  "result":{"type":"object","additionalProperties":false,"properties":{"score":{"type":"integer","minimum":1,"maximum":5},"feedback":{"type":"string"}},"required":["score","feedback"]}
},"required":["situation","task","action","result"]},
"overall_score":{"type":"integer","minimum":1,"maximum":5},
"rewrite":{"type":"string"},
"follow_ups":{"type":"array","items":{"type":"string"}},
"strengths":{"type":"array","items":{"type":"string"}},
"gaps":{"type":"array","items":{"type":"string"}},
"story_match":{"type":"string"}},
"required":["star","overall_score","rewrite","follow_ups","strengths","gaps","story_match"]}`

// ReviewInterview evaluates a spoken behavioral answer against the STAR method.
func (c *Coach) ReviewInterview(ctx context.Context, s data.Store, question, transcript, resumeContext string) (InterviewReview, string, error) {
	_, tax, err := taxonomyBlock(s, "grammar", "lexical", "style")
	if err != nil {
		return InterviewReview{}, "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `You are an interview coach for a %[1]s-speaking software engineer (B1-B2) applying to backend roles.
He answered a behavioral interview question out loud; the text is a Whisper transcript.
Evaluate the answer using the Amazon STAR method (Situation, Task, Action, Result).

Rules:
- Ignore punctuation, capitalization and spelling from the transcriber.
- Ignore fillers (um, uh, like, you know), false starts and self-corrections.
- Do not invent facts. If the answer contradicts the verified background below, flag it.
- Grammar mistakes are secondary here; focus on structure, content and impact.

Output:
- star.situation/task/action/result: score 1-5 and one-sentence feedback each.
- overall_score: 1-5.
- rewrite: a tighter, natural version of the same answer in 60-120 words.
- follow_ups: 2 likely follow-up questions an interviewer would ask.
- strengths: 2-3 things the answer did well.
- gaps: 2-3 missing or weak parts.
- story_match: which of the verified stories below fits this question, or "none".

`, c.cfg.Language)
	b.WriteString(tax)
	b.WriteString("\n")
	if resumeContext != "" {
		b.WriteString("Verified background (resume + STAR stories):\n")
		b.WriteString(resumeContext)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Interview question: %s\n\nTranscript:\n%s\n", question, transcript)

	var out InterviewReview
	backend, usage, err := c.structured(ctx, false, "interview_review", b.String(), interviewSchema, &out)
	if err != nil {
		return InterviewReview{}, "", err
	}
	if usage != nil {
		out.Cost = usage.Cost
	}
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
	if _, _, err := c.structured(ctx, false, "exercise_pack", b.String(), packSchema, &out); err != nil {
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
