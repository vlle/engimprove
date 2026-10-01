// Package coach asks Claude Code in headless mode for reviews and new exercises.
package coach

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Runner calls `claude -p` with a JSON schema and decodes the structured output.
type Runner struct {
	Bin   string
	Model string
}

// Find locates the claude binary on PATH or in ~/.local/bin.
func Find(model string) (Runner, error) {
	if bin, err := exec.LookPath("claude"); err == nil {
		return Runner{Bin: bin, Model: model}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Runner{}, err
	}
	bin := filepath.Join(home, ".local", "bin", "claude")
	if _, err := os.Stat(bin); err != nil {
		return Runner{}, errors.New("claude CLI not found on PATH or in ~/.local/bin")
	}
	return Runner{Bin: bin, Model: model}, nil
}

// Structured runs one prompt and decodes the schema-shaped answer into out.
func (r Runner) Structured(ctx context.Context, prompt, schema string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	// project-only settings keep user hooks, ours included, out.
	cmd := exec.CommandContext(ctx, r.Bin, "-p",
		"--setting-sources", "project",
		"--tools", "",
		"--model", r.Model,
		"--output-format", "json",
		"--json-schema", schema,
		prompt)
	cmd.Dir = os.TempDir()
	cmd.Env = append(os.Environ(), "ENG_HOOK_OFF=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude -p: %w: %s", err, tail(stderr.String()+stdout.String()))
	}
	var res struct {
		IsError    bool            `json:"is_error"`
		Result     string          `json:"result"`
		Structured json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return fmt.Errorf("claude -p output is not json: %w: %s", err, tail(stdout.String()))
	}
	if res.IsError {
		return fmt.Errorf("claude -p failed: %s", tail(res.Result))
	}
	raw := res.Structured
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage(res.Result)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("claude -p answer does not match schema: %w: %s", err, tail(string(raw)))
	}
	return nil
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 600 {
		return "…" + s[len(s)-600:]
	}
	return s
}
