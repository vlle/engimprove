// Package testutil builds a throwaway engimprove root for tests.
package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engimprove/internal/config"
	"engimprove/internal/data"
)

const taxonomy = "# Taxonomy\n\n## kind: grammar\n\n| category | что это |\n|---|---|\n" +
	"| `articles` | a / an / the |\n| `prepositions` | x |\n| `modals` | x |\n\n" +
	"## kind: punctuation\n\n| category | что это |\n|---|---|\n| `apostrophes` | x |\n| `capitalization` | x |\n\n" +
	"## kind: lexical (в тексте не применяется)\n\n| category | что это |\n|---|---|\n| `calque` | x |\n"

// Store creates a root with the given error log lines and a small config.
func Store(t *testing.T, lines ...string) (data.Store, config.Config) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var log strings.Builder
	for _, l := range lines {
		log.WriteString(l + "\n")
	}
	write("go.mod", "module engimprove\n")
	write("errors/errors.jsonl", log.String())
	write("errors/taxonomy.md", taxonomy)
	write("config/eng.json", `{"topics":[
{"id":"the","title":"the","categories":["articles"],"tokens":["the"],"threshold":2},
{"id":"a-an","title":"a/an","categories":["articles"],"tokens":["a","an"],"threshold":2},
{"id":"prepositions","title":"prepositions","categories":["prepositions"],"threshold":3}]}`)
	st := data.Store{Root: root}
	cfg, err := config.Load(st)
	if err != nil {
		t.Fatal(err)
	}
	return st, cfg
}
