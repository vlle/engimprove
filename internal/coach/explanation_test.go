package coach

import (
	"encoding/json"
	"slices"
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
	tests := []struct {
		name     string
		previous *Explanation
		want     []string
		absent   []string
	}{
		{
			name: "first explanation",
			want: []string{
				"\"learner_answer\":\"a\"",
				"\"exercise\":\"I opened ___ PR yesterday.\"",
				"\"correct_answers\":[\"the\"]",
				"\"original_sentence\":\"I opened a PR yesterday. Can you review the PR?\"",
				"A known referent uses the.",
			},
			absent: []string{"\"previous_explanation\"", anotherWayInstructions},
		},
		{
			name:     "another way",
			previous: &Explanation{Verdict: "PR уже известен", Mnemonic: "known one, the one"},
			want:     []string{"\"previous_explanation\":{\"verdict\":\"PR уже известен\"", "known one, the one", anotherWayInstructions},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := explanationPrompt("Russian", card, "a", "A known referent uses the.", tt.previous)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(prompt, want) {
					t.Errorf("prompt does not include %q", want)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(prompt, absent) {
					t.Errorf("prompt includes %q", absent)
				}
			}
		})
	}
}

// strict mode rejects a schema with an optional or undeclared property.
func TestExplanationSchemaIsStrict(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(explanationSchema), &schema); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, node map[string]any)
	walk = func(path string, node map[string]any) {
		switch node["type"] {
		case "object":
			if node["additionalProperties"] != false {
				t.Errorf("%s allows additional properties", path)
			}
			props, _ := node["properties"].(map[string]any)
			required, _ := node["required"].([]any)
			for name, sub := range props {
				if !slices.Contains(required, any(name)) {
					t.Errorf("%s.%s is not required", path, name)
				}
				walk(path+"."+name, sub.(map[string]any))
			}
			if len(required) != len(props) {
				t.Errorf("%s requires %d fields but declares %d", path, len(required), len(props))
			}
		case "array":
			walk(path+"[]", node["items"].(map[string]any))
		}
	}
	walk("$", schema)
}

func TestExplanationClean(t *testing.T) {
	tests := []struct {
		name  string
		in    Explanation
		check func(t *testing.T, got Explanation)
	}{
		{
			name: "trims and keeps filled modules",
			in: Explanation{
				Verdict:  "  PR уже упомянут  ",
				Contrast: Contrast{Yours: " новый PR ", Correct: "тот самый PR"},
				Examples: []Example{{Sentence: "Close the ticket.", Focus: "the ticket", Note: ""}},
				Quiz:     []Quiz{{Text: "Merge ___ branch.", Choices: []string{"a", " the ", "the", "—"}, Answer: "the", Why: "known"}},
			},
			check: func(t *testing.T, got Explanation) {
				if got.Verdict != "PR уже упомянут" || got.Contrast.Yours != "новый PR" {
					t.Errorf("not trimmed: %+v", got)
				}
				if len(got.Examples) != 1 || got.Examples[0].Focus != "the ticket" {
					t.Errorf("examples = %+v", got.Examples)
				}
				if len(got.Quiz) != 1 || !slices.Equal(got.Quiz[0].Choices, []string{"a", "the", "—"}) {
					t.Errorf("quiz = %+v", got.Quiz)
				}
			},
		},
		{
			name: "drops half-filled items",
			in: Explanation{
				Verdict:  "x",
				Contrast: Contrast{Yours: "only one side"},
				Steps:    []Step{{Question: "known?", Answer: ""}, {Question: "known?", Answer: "yes"}},
				Examples: []Example{{Sentence: ""}, {Sentence: "Fix the bug.", Focus: "a bug"}},
				Pairs:    []Pair{{A: "a dog", B: "a dog", Difference: "same"}, {A: "a dog", B: "the dog"}},
				Traps:    []Trap{{Trap: "at home", Why: ""}},
				Quiz: []Quiz{
					{Text: "no gap", Choices: []string{"a", "the"}, Answer: "the", Why: "-"},
					{Text: "___ and ___", Choices: []string{"a", "the"}, Answer: "the", Why: "-"},
					{Text: "Fix ___ bug.", Choices: []string{"a", "the"}, Answer: "an", Why: "-"},
					{Text: "Fix ___ bug.", Choices: []string{"the"}, Answer: "the", Why: "-"},
				},
			},
			check: func(t *testing.T, got Explanation) {
				if got.Contrast != (Contrast{}) {
					t.Errorf("contrast = %+v, want empty", got.Contrast)
				}
				if len(got.Steps) != 1 {
					t.Errorf("steps = %+v", got.Steps)
				}
				if len(got.Examples) != 1 || got.Examples[0].Focus != "" {
					t.Errorf("examples = %+v, want focus dropped when absent from the sentence", got.Examples)
				}
				if len(got.Pairs)+len(got.Traps)+len(got.Quiz) != 0 {
					t.Errorf("pairs %v traps %v quiz %v, want none", got.Pairs, got.Traps, got.Quiz)
				}
			},
		},
		{
			name: "caps long modules",
			in: Explanation{
				Verdict:  "x",
				Steps:    make([]Step, 9),
				Examples: slices.Repeat([]Example{{Sentence: "Ship the build."}}, 8),
			},
			check: func(t *testing.T, got Explanation) {
				if len(got.Steps) != 0 || len(got.Examples) != 5 {
					t.Errorf("steps %d examples %d, want 0 and 5", len(got.Steps), len(got.Examples))
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.in.clean())
		})
	}
}
