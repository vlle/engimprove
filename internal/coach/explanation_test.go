package coach

import (
	"strings"
	"testing"

	"engimprove/internal/cards"
)

func TestExplanationPromptIncludesFullCardContext(t *testing.T) {
	card := cards.Card{
		Type: cards.Cloze, Source: "mistake", Category: "articles",
		Rule: "missing definite article before a known referent",
		Text: "I opened ___ PR yesterday.", Answers: []string{"the"},
		Choices: []string{"a", "the", "—"}, Before: "opened PR",
		After: "opened the PR", Note: "the PR is already known",
		Context: "I opened a PR yesterday. Can you review the PR?",
	}
	prompt, err := explanationPrompt("Russian", card, "a", "A known referent uses the.")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\"learner_answer\":\"a\"",
		"\"exercise\":\"I opened ___ PR yesterday.\"",
		"\"correct_answers\":[\"the\"]",
		"\"original_sentence\":\"I opened a PR yesterday. Can you review the PR?\"",
		"A known referent uses the.",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt does not include %q", want)
		}
	}
}
