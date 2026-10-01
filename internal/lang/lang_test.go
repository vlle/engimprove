package lang

import "testing"

func TestAnalyze(t *testing.T) {
	cases := []struct {
		name    string
		prompt  string
		english bool
	}{
		{"english", "make an extensive change to global claude.md so whenever i prompt in english", true},
		{"russian", "сделай ревью MR по ссылке и проверь тесты", false},
		{"russian with log", "почини падение в handler, вот лог: panic: runtime error: invalid memory address or nil pointer dereference at main.go", false},
		{"too short", "yes go", false},
		{"slash command with prose", "/goal create an english learning machine with a frontend", true},
		{"bare slash command", "/code-review high", false},
		{"pasted english, russian ask", "переведи это <pasted_content id=x>We are looking for a senior engineer with strong Go skills</pasted_content id=x>", false},
		{"urls and paths", "глянь https://gitlab.example.com/c2c/adverts/-/merge_requests/12 и ~/wwrum/gateway/cmd/main.go", false},
		{"casual english", "can u check why the consumer in event-content is stuck, it was working yesterday", true},
		{"code fence only", "```go\nfunc main() { fmt.Println(\"hello world from go\") }\n```", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Analyze(tc.prompt).English; got != tc.english {
				t.Fatalf("Analyze(%q).English = %v, want %v", tc.prompt, got, tc.english)
			}
		})
	}
}

func TestAnalyzeWordsIgnoreInserts(t *testing.T) {
	got := Analyze("fix this please `some code here` https://example.com/a/b")
	if got.Words != 3 {
		t.Fatalf("Words = %d, want 3 (%q)", got.Words, got.Text)
	}
}
