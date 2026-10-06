package coach

import (
	"context"
	"fmt"
	"strings"

	"engimprove/internal/data"
	"engimprove/internal/logbook"
)

// TextCheck is the verdict on one pasted text in the web app.
type TextCheck struct {
	Corrected string `json:"corrected"`
	// Perception is how a native reader takes the message and its author.
	Perception string `json:"perception"`
	// Native is the message rephrased the way a native colleague would write it.
	Native string             `json:"native"`
	Errors []logbook.NewError `json:"errors"`
	Cost   float64            `json:"cost"`
}

var textCheckSchema = `{"type":"object","additionalProperties":false,"properties":{
"corrected":{"type":"string"},
"perception":{"type":"string"},
"native":{"type":"string"},
"errors":{"type":"array","items":` + fmt.Sprintf(errorItem, `"grammar","punctuation","lexical","spelling","style"`) + `}},
"required":["corrected","perception","native","errors"]}`

// CheckText reviews a pasted text and finds mistakes worth learning from.
func (c *Coach) CheckText(ctx context.Context, s data.Store, text string) (TextCheck, string, error) {
	categories, tax, err := taxonomyBlock(s, "grammar", "punctuation", "lexical", "spelling", "style")
	if err != nil {
		return TextCheck{}, "", err
	}
	rules, err := rulesBlock(s)
	if err != nil {
		return TextCheck{}, "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `You check English written by a software engineer (B1-B2), a %[1]s-speaking learner, for his private trainer.
Log only real mistakes worth learning from; he wants to fix his grammar, articles above all.

Log:
- grammar: every category (articles, tenses, prepositions, agreement, word order, conditionals, modals, ...).
- punctuation: commas, apostrophes, sentence boundaries, hyphenation. Never capitalization.
- lexical: calques from %[1]s, wrong collocations, false friends, wrong word.
- spelling: consistent misspellings (everytime, recieve) and homophone mix-ups (its/it's, then/than, lose/loose), not finger typos.
- style: only clearly unnatural or wordy phrasing; at most 2, only when they are worth learning from.

Never log (acceptable everywhere): lowercase sentence starts, lowercase "i", chat slang, abbreviations (u, pls, MR, PR, FF, llm),
finger typos (teh, swapped letters).
When in doubt, skip. If both variants are acceptable, it is not a mistake. Never invent mistakes.
before/after: short fragments (3-10 words) quoted exactly from the text and corrected; one entry per mistake.
The category must describe the change itself: adding or removing a/an/the is always "articles", never "prepositions".
Reuse an existing rule only when it names exactly the same mistake; otherwise write a new short rule in the same style.
note: why, at most 12 words, in English.
corrected: the whole text with only grammar, punctuation, lexical and spelling mistakes fixed; style entries are NOT applied there.
perception: 1-2 short lines on how a native reader takes the message and its author: tone, politeness, directness,
and where the word choice exposes a non-native writer. Judge the wording, not the content and not the grammar spots
you logged. Plain text, no markdown. Never leave this empty.
native: the whole message rewritten the way a native colleague of the same register would naturally phrase it.
Same meaning, same level of formality the writer chose; do not inflate, do not explain. Never leave this empty.
No mistakes: errors [], corrected = text as-is.

`, c.cfg.Language)
	b.WriteString(tax)
	b.WriteString("\n")
	b.WriteString(rules)
	b.WriteString("\nText:\n<<<\n" + text + "\n>>>\n")

	var out TextCheck
	backend, usage, err := c.structured(ctx, false, "text_check", b.String(), textCheckSchema, &out)
	if err != nil {
		return TextCheck{}, "", err
	}
	if usage != nil {
		out.Cost = usage.Cost
	}
	out.Errors = keepValid(categories, out.Errors, "capitalization")
	if out.Errors == nil {
		out.Errors = []logbook.NewError{}
	}
	if out.Corrected == "" {
		out.Corrected = text
	}
	return out, backend, nil
}

// GrowthInput is the data the growth card cheer is built from.
type GrowthInput struct {
	Now           string  `json:"now"`
	Streak        int     `json:"streak"`
	WeekRate      float64 `json:"week_rate"`
	PrevWeekRate  float64 `json:"prev_week_rate"`
	TotalMistakes int     `json:"total_mistakes"`
	TotalTexts    int     `json:"total_texts"`
	DrillAccuracy float64 `json:"drill_accuracy"`
	RipeTopics    int     `json:"ripe_topics"`
	TotalTopics   int     `json:"total_topics"`
	TopRule       string  `json:"top_rule"`
	TopRuleCount  int     `json:"top_rule_count"`
	BestMonthRate float64 `json:"best_month_rate"`
	CurrentRate   float64 `json:"current_rate"`
}

var growthSchema = `{"type":"object","additionalProperties":false,"properties":{
"headline":{"type":"string"},
"cheer":{"type":"string"},
"nudge":{"type":"string"}},
"required":["headline","cheer","nudge"]}`

// GrowthCard is the LLM part of the growth card.
type GrowthCard struct {
	Headline string `json:"headline"`
	Cheer    string `json:"cheer"`
	Nudge    string `json:"nudge"`
	Backend  string `json:"backend,omitempty"`
}

// Cheer writes the headline, praise and one gentle nudge for the growth card.
func (c *Coach) Cheer(ctx context.Context, in GrowthInput) (GrowthCard, string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, `You write the motivational card of an English trainer for a %[1]s-speaking software engineer (B1-B2).
He asked to cheer him up, but wants more than a pat on the back: he wants to see his growth and keep it juicy and personal.

Use only the metrics below; never invent numbers. Speak directly to him ("ты" level, warm, playful).
Rules for each field:
- headline: one punchy status line about the most impressive delta, with the real number in it. At most 8 words.
- cheer: 1-2 sentences naming a concrete improvement from the data (a dropped mistake rate, a streak, accuracy).
- nudge: one sentence about his most frequent rule; light, never scolding.

`, c.cfg.Language)
	fmt.Fprintf(&b, "Today: %s\n", in.Now)
	fmt.Fprintf(&b, "Answer streak: %d days (a day answered at least one drill card).\n", in.Streak)
	fmt.Fprintf(&b, "Mistakes per 100 checked prompt words: last week %.1f, week before %.1f (lower is better; a 0 for the week before means it had no checked words, so there is no week-over-week comparison — pick another angle).\n", in.WeekRate, in.PrevWeekRate)
	fmt.Fprintf(&b, "Rate over the whole last month so far: %.2f; his best full month: %.2f.\n", in.CurrentRate, in.BestMonthRate)
	fmt.Fprintf(&b, "Drill accuracy over the last 20 answers: %d%%.\n", int(in.DrillAccuracy*100))
	fmt.Fprintf(&b, "Topics: %d of %d are ripe for a drill right now.\n", in.RipeTopics, in.TotalTopics)
	if in.TopRule != "" {
		fmt.Fprintf(&b, "Most repeated rule of all time: %q — %d times.\n", in.TopRule, in.TopRuleCount)
	}
	fmt.Fprintf(&b, "Mistakes logged in total: %d from %d texts.\n", in.TotalMistakes, in.TotalTexts)

	var out GrowthCard
	backend, _, err := c.structured(ctx, false, "growth_card", b.String(), growthSchema, &out)
	if err != nil {
		return GrowthCard{}, "", err
	}
	return out, backend, nil
}
