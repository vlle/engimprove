package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engimprove/internal/config"
	"engimprove/internal/data"
)

func TestScaffoldFreshClone(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module engimprove\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err := Scaffold(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 {
		t.Fatalf("created = %v", created)
	}
	st := data.Store{Root: root}
	cfg, err := config.Load(st)
	if err != nil {
		t.Fatalf("starter config rejected: %v", err)
	}
	if len(cfg.Topics) != 9 || cfg.Language != "English" {
		t.Fatalf("starter config = %+v", cfg)
	}
	created, err = Scaffold(root)
	if err != nil || len(created) != 0 {
		t.Fatalf("second scaffold: %v, %v", created, err)
	}
}

func TestRegisterHooksFreshHome(t *testing.T) {
	home := t.TempDir()
	engBin, _ := filepath.Abs(filepath.Join("root", "bin", "eng"))
	state := filepath.Join("root", "state")
	events, err := RegisterHooks(home, engBin, state)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %v", events)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hooks map[string][]hookGroup `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	hooks := doc.Hooks["UserPromptSubmit"]
	if len(hooks) != 1 || len(hooks[0].Hooks) != 1 || hooks[0].Hooks[0]["async"] != true {
		t.Fatalf("UserPromptSubmit = %+v", hooks)
	}
	if got := hooks[0].Hooks[0]["command"].(string); !strings.Contains(got, engBin+" hook") {
		t.Fatalf("command = %q", got)
	}
	events, err = RegisterHooks(home, engBin, state)
	if err != nil || events != nil {
		t.Fatalf("second run should stay silent: %v, %v", events, err)
	}
}

func TestRegisterHooksPreservesOthersAndRewritesStalePath(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(home, ".claude", "settings.json")
	raw := `{"model":"test","hooks":{"Stop":[{"matcher":"","hooks":[
{"type":"command","command":"/old/engimprove/bin/eng hook-stop 2>>/old/state/hook.err || true","timeout":5},
{"type":"command","command":"peon.sh","timeout":10,"async":true}]}]}}`
	if err := os.WriteFile(settings, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	engBin := "/new/engimprove/bin/eng"
	events, err := RegisterHooks(home, engBin, "/new/engimprove/state")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0] != "UserPromptSubmit" {
		t.Fatalf("expected only UserPromptSubmit added, got %v", events)
	}
	var doc struct {
		Model string                 `json:"model"`
		Hooks map[string][]hookGroup `json:"hooks"`
	}
	if raw, err := os.ReadFile(settings); err == nil {
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Fatal(err)
	}
	stop := doc.Hooks["Stop"][0].Hooks
	if doc.Model != "test" || len(stop) != 2 {
		t.Fatalf("other fields lost: %s", doc.Model)
	}
	if got, _ := stop[0]["command"].(string); !strings.HasPrefix(got, engBin+" hook-stop") {
		t.Fatalf("stale command not rewritten: %q", got)
	}
	if got, _ := stop[1]["command"].(string); got != "peon.sh" {
		t.Fatalf("unrelated hook changed: %q", got)
	}
}

func TestOpenCodePlugin(t *testing.T) {
	home := t.TempDir()
	engBin := "/root/bin/eng"
	action, _, err := OpenCodePlugin(home, engBin, "root")
	if err != nil || action != "skip" {
		t.Fatalf("no opencode dir should be a silent skip: %q %v", action, err)
	}
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "agents", "opencode"), 0o755)
	tpl := `export const P = async () => { const eng = "__ENGBIN__"; return {} }`
	if err := os.WriteFile(filepath.Join(root, "agents", "opencode", "engimprove.js"), []byte(tpl), 0o644); err != nil {
		t.Fatal(err)
	}
	action, path, err := OpenCodePlugin(home, engBin, root)
	if err != nil || action != "created" || path == "" {
		t.Fatalf("plugin = %q %q, %v", action, path, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(raw), "__ENGBIN__") || !strings.Contains(string(raw), engBin) {
		t.Fatalf("plugin not rendered: %q, %v", raw, err)
	}
	if action, path, err = OpenCodePlugin(home, engBin, root); err != nil || action != "unchanged" || path != "" {
		t.Fatalf("second run should be unchanged: %q %q %v", action, path, err)
	}
}

func TestFishWrapper(t *testing.T) {
	home := t.TempDir()
	engBin := "/root/bin/eng"
	if path, err := FishWrapper(home, engBin); err != nil || path != "" {
		t.Fatalf("no fish dir should be a silent skip: %q %v", path, err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config", "fish"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := FishWrapper(home, engBin)
	if err != nil || path == "" {
		t.Fatalf("wrapper = %q, %v", path, err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), engBin) {
		t.Fatalf("wrapper = %q", raw)
	}
	if path, err = FishWrapper(home, engBin); err != nil || path != "" {
		t.Fatalf("second run should stay silent: %q %v", path, err)
	}
}
