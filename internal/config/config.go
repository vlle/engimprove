// Package config holds the tunable knobs from config/eng.json.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"engimprove/internal/data"
)

// Topic groups mistakes that are drilled together.
type Topic struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Categories []string `json:"categories"`
	// Tokens narrows a topic to changes that add or remove one of these words.
	Tokens []string `json:"tokens,omitempty"`
	// Threshold is how many fresh mistakes make the drill fire.
	Threshold int `json:"threshold"`
}

// Config is the whole of config/eng.json.
type Config struct {
	Addr            string `json:"addr"`
	ReviewThreshold int    `json:"review_threshold"`
	Notify          bool   `json:"notify"`
	WhisperModel    string `json:"whisper_model"`
	// ClaudeModel is used by `claude -p` when OpenRouter is unavailable or the text is private.
	ClaudeModel     string `json:"claude_model"`
	OpenRouterEnv   string `json:"openrouter_env"`
	OpenRouterModel string `json:"openrouter_model"`
	// PrivateRoots are project trees whose prompts never leave for OpenRouter.
	PrivateRoots []string `json:"private_roots"`
	Topics       []Topic  `json:"topics"`
}

// Load reads config/eng.json and fills defaults for unset fields.
func Load(s data.Store) (Config, error) {
	var cfg Config
	if err := data.ReadJSON(s.Path("config", "eng.json"), &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:7421"
	}
	if cfg.ReviewThreshold <= 0 {
		cfg.ReviewThreshold = 5
	}
	if cfg.WhisperModel == "" {
		cfg.WhisperModel = "models/ggml-small.en.bin"
	}
	if cfg.ClaudeModel == "" {
		cfg.ClaudeModel = "sonnet"
	}
	if cfg.OpenRouterModel == "" {
		cfg.OpenRouterModel = "anthropic/claude-sonnet-5.5"
	}
	if len(cfg.Topics) == 0 {
		return Config{}, fmt.Errorf("%s: no topics", s.Path("config", "eng.json"))
	}
	for i, t := range cfg.Topics {
		if t.ID == "" || len(t.Categories) == 0 {
			return Config{}, fmt.Errorf("topic %d: id and categories are required", i+1)
		}
		if t.Threshold <= 0 {
			cfg.Topics[i].Threshold = 5
		}
	}
	return cfg, nil
}

// Topic returns the topic with id.
func (c Config) Topic(id string) (Topic, bool) {
	for _, t := range c.Topics {
		if t.ID == id {
			return t, true
		}
	}
	return Topic{}, false
}

// Private reports whether a working directory belongs to a private root.
func (c Config) Private(dir string) bool {
	home, _ := os.UserHomeDir()
	dir = filepath.Clean(dir)
	for _, root := range c.PrivateRoots {
		if rest, ok := strings.CutPrefix(root, "~/"); ok {
			root = filepath.Join(home, rest)
		}
		root = filepath.Clean(root)
		if dir == root || strings.HasPrefix(dir, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// URL is the base address of the web app.
func (c Config) URL() string {
	return "http://" + c.Addr
}
