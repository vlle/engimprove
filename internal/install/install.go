// Package install scaffolds a fresh engimprove checkout and registers the agent hooks
// it can find: Claude Code and opencode. Other agents integrate by the eng hook stdin contract.
package install

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"engimprove/internal/data"
)

// starterConfig is written when config/eng.json is missing on a fresh clone.
const starterConfig = `{
  "addr": "127.0.0.1:7421",
  "review_threshold": 5,
  "notify": true,
  "whisper_model": "models/ggml-small.en.bin",
  "claude_model": "sonnet",
  "openrouter_model": "anthropic/claude-sonnet-5.5",
  "language": "English",
  "private_roots": [],
  "topics": [
    {"id": "the", "title": "the", "categories": ["articles"], "tokens": ["the"], "threshold": 5},
    {"id": "a-an", "title": "a and an", "categories": ["articles", "plurals-countability"], "tokens": ["a", "an"], "threshold": 5},
    {"id": "prepositions", "title": "prepositions", "categories": ["prepositions", "preposition-collocation"], "threshold": 4},
    {"id": "verbs", "title": "verb forms", "categories": ["tense-aspect", "modals", "conditionals", "infinitive-gerund", "passive-voice-form", "subject-verb-agreement", "questions-negation"], "threshold": 4},
    {"id": "structure", "title": "sentence structure", "categories": ["coordination", "word-order", "dangling-modifier", "missing-head-noun", "relative-clauses", "pronouns-reference", "comparatives", "plurals-countability"], "threshold": 4},
    {"id": "punctuation", "title": "punctuation", "categories": ["commas", "apostrophes", "sentence-boundaries", "hyphenation", "capitalization"], "threshold": 4},
    {"id": "lexis", "title": "word choice", "categories": ["word-choice", "collocation", "calque", "false-friend", "phrasal-verbs"], "threshold": 5},
    {"id": "style", "title": "style", "categories": ["wordiness", "register", "hedging", "redundancy", "clarity", "paragraphing", "number-format"], "threshold": 6},
    {"id": "spelling", "title": "spelling", "categories": ["spelling"], "threshold": 4}
  ]
}
`

// Root resolves the checkout without demanding a filled error database: ENG_ROOT,
// the directory above the binary's, or a walk up from the cwd looking for go.mod.
func Root() (string, error) {
	if root := os.Getenv("ENG_ROOT"); root != "" {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
			return "", fmt.Errorf("ENG_ROOT=%s has no go.mod", root)
		}
		return filepath.Clean(root), nil
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(filepath.Dir(exe))
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no engimprove checkout: run bin/eng from inside it or set ENG_ROOT")
		}
		dir = parent
	}
}

// Scaffold creates the gitignored files a fresh clone lacks; it reports what it created.
func Scaffold(root string) ([]string, error) {
	var created []string
	files := map[string][]byte{
		"errors/errors.jsonl": nil,
		"config/eng.json":     []byte(starterConfig),
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := data.WriteFile(path, content); err != nil {
			return created, err
		}
		created = append(created, rel)
	}
	stateDir := filepath.Join(root, "state")
	if _, err := os.Stat(stateDir); err != nil {
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return created, err
		}
		created = append(created, "state/")
	}
	return created, nil
}

type hookGroup struct {
	Matcher string           `json:"matcher,omitempty"`
	Hooks   []map[string]any `json:"hooks"`
}

func hookCommand(engBin, stateDir, sub string) string {
	return fmt.Sprintf("%s %s 2>>%s || true", engBin, sub, filepath.Join(stateDir, "hook.err"))
}

func hookEntry(engBin, stateDir, sub string, timeout int, async bool) map[string]any {
	entry := map[string]any{"type": "command", "command": hookCommand(engBin, stateDir, sub), "timeout": timeout}
	if async {
		entry["async"] = true
	}
	return entry
}

// RegisterHooks adds the engimprove entries to ~/.claude/settings.json and reports
// the events it touched; entries from another checkout path are rewritten in place.
func RegisterHooks(home, engBin, stateDir string) ([]string, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(raw) > 0 {
		if backupName, err := backup(path, raw); err != nil {
			return nil, err
		} else {
			fmt.Printf("backed up %s → %s\n", path, backupName)
		}
	}

	var doc map[string]json.RawMessage
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	if doc == nil {
		doc = map[string]json.RawMessage{}
	}
	var hooks map[string]json.RawMessage
	if raw := doc["hooks"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return nil, fmt.Errorf("%s: hooks: %w", path, err)
		}
	}
	if hooks == nil {
		hooks = map[string]json.RawMessage{}
	}

	desired := []struct {
		event, sub string
	}{
		{"UserPromptSubmit", "hook"},
		{"Stop", "hook-stop"},
	}
	var updated []string
	changed := false
	for _, d := range desired {
		var groups []hookGroup
		if raw := hooks[d.event]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &groups); err != nil {
				return nil, fmt.Errorf("%s: hooks.%s: %w", path, d.event, err)
			}
		}
		marker := "bin/eng " + d.sub
		found := false
		rewritten := false
		for gi := range groups {
			for hi := range groups[gi].Hooks {
				cmd, _ := groups[gi].Hooks[hi]["command"].(string)
				if !strings.Contains(cmd, marker) {
					continue
				}
				found = true
				if !strings.HasPrefix(cmd, engBin+" ") {
					// stale path from another checkout; keep the entry, refresh the command
					groups[gi].Hooks[hi]["command"] = hookCommand(engBin, stateDir, d.sub)
					rewritten = true
				}
			}
		}
		if found {
			if rewritten {
				if raw, err := json.Marshal(groups); err != nil {
					return nil, err
				} else {
					hooks[d.event] = raw
				}
			}
			continue
		}
		timeout, async := 10, d.sub == "hook"
		if d.sub == "hook-stop" {
			timeout = 5
		}
		groups = append(groups, hookGroup{Matcher: "",
			Hooks: []map[string]any{hookEntry(engBin, stateDir, d.sub, timeout, async)}})
		raw, err := json.Marshal(groups)
		if err != nil {
			return nil, err
		}
		hooks[d.event] = raw
		changed = true
		updated = append(updated, d.event)
	}
	if !changed {
		return nil, nil
	}
	rawHooks, err := json.Marshal(hooks)
	if err != nil {
		return nil, err
	}
	doc["hooks"] = rawHooks
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return updated, data.WriteFile(path, append(out, '\n'))
}

func backup(path string, raw []byte) (string, error) {
	backup, err := os.CreateTemp(os.TempDir(), "claude-bkp-*-"+filepath.Base(path))
	if err != nil {
		return "", err
	}
	name := backup.Name()
	if _, err := backup.Write(raw); err != nil {
		backup.Close()
		return "", err
	}
	return name, backup.Close()
}

// OpenCodePlugin renders the bundled plugin with the eng binary path and installs it
// into the global plugin directory. It reports the action it took: "skip" (no opencode),
// "created", "updated" or "unchanged".
func OpenCodePlugin(home, engBin, root string) (string, string, error) {
	dir := filepath.Join(home, ".config", "opencode")
	if _, err := os.Stat(dir); err != nil {
		return "skip", "", nil
	}
	source, err := os.ReadFile(filepath.Join(root, "agents", "opencode", "engimprove.js"))
	if err != nil {
		return "", "", fmt.Errorf("opencode plugin template: %w", err)
	}
	rendered := strings.ReplaceAll(string(source), "__ENGBIN__", engBin)
	path := filepath.Join(dir, "plugins", "engimprove.js")
	if raw, err := os.ReadFile(path); err == nil {
		if string(raw) == rendered {
			return "unchanged", "", nil
		}
		if err := data.WriteFile(path, []byte(rendered)); err != nil {
			return "", "", err
		}
		return "updated", path, nil
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	if err := data.WriteFile(path, []byte(rendered)); err != nil {
		return "", "", err
	}
	return "created", path, nil
}

// FishWrapper writes the fish wrapper when fish is in use and reports the file it wrote.
func FishWrapper(home, engBin string) (string, error) {
	fishDir := filepath.Join(home, ".config", "fish")
	if _, err := os.Stat(fishDir); err != nil {
		return "", nil
	}
	path := filepath.Join(fishDir, "conf.d", "engimprove.fish")
	if _, err := os.Stat(path); err == nil {
		// a hand-written wrapper refers to eng directly or via $HOME; never fight it
		return "", nil
	}
	content := "function eng --description 'engimprove CLI'\n    " + engBin + " $argv\nend\n"
	if err := data.WriteFile(path, []byte(content)); err != nil {
		return "", err
	}
	return path, nil
}

// Run scaffolds, registers every agent it finds and prints what the learner has to do by hand.
func Run(out io.Writer) error {
	root, err := Root()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	engBin := exe
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "root %s\n", root)

	created, err := Scaffold(root)
	if err != nil {
		return err
	}
	for _, f := range created {
		fmt.Fprintf(out, "created %s\n", f)
	}
	if events, err := RegisterHooks(home, engBin, filepath.Join(root, "state")); err != nil {
		return err
	} else if len(events) > 0 {
		fmt.Fprintf(out, "hooks   ~/.claude/settings.json: %s\n", strings.Join(events, ", "))
	} else {
		fmt.Fprintln(out, "hooks   already registered in ~/.claude/settings.json")
	}
	if action, path, err := OpenCodePlugin(home, engBin, root); err != nil {
		return err
	} else if action == "created" || action == "updated" {
		fmt.Fprintf(out, "opencode %s %s (restart opencode to load it)\n", action, path)
	} else if action == "unchanged" {
		fmt.Fprintln(out, "opencode plugin already installed")
	} else {
		fmt.Fprintln(out, "opencode skipped: ~/.config/opencode not found")
	}
	if wrapper, err := FishWrapper(home, engBin); err != nil {
		return err
	} else if wrapper != "" {
		fmt.Fprintf(out, "fish    %s\n", wrapper)
	}

	fmt.Fprint(out, `
left to do by hand:
  1. export OPENROUTER_API=sk-or-...   (or set "openrouter_env" in config/eng.json)
  2. set "language" in config/eng.json to your native language
  3. run `+engBin+` doctor
  codex and other agents have no hook surface: follow README.md, section "Agent integration".
`)
	return nil
}
