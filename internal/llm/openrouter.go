// Package llm calls OpenRouter chat completions and transcriptions.
package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const chatEndpoint = "https://openrouter.ai/api/v1/chat/completions"
const sttEndpoint = "https://openrouter.ai/api/v1/audio/transcriptions"

// Usage reports token counts and cost for one OpenRouter request.
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	Seconds          float64 `json:"seconds"`
}

// OpenRouter is an authenticated client.
type OpenRouter struct {
	key  string
	http *http.Client
}

// FromEnv takes the key from $OPENROUTER_API / $OPENROUTER_API_KEY, else from the fetcher's export line.
func FromEnv(fetcher string) (*OpenRouter, error) {
	for _, name := range []string{"OPENROUTER_API", "OPENROUTER_API_KEY"} {
		if v := os.Getenv(name); v != "" {
			return newClient(v), nil
		}
	}
	if fetcher == "" {
		return nil, errors.New("no OpenRouter key: set OPENROUTER_API or openrouter_env in config/eng.json")
	}
	if rest, ok := strings.CutPrefix(fetcher, "~/"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		fetcher = filepath.Join(home, rest)
	}
	out, err := exec.Command(fetcher).Output()
	if err != nil {
		return nil, fmt.Errorf("openrouter key fetcher %s: %w", fetcher, err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimPrefix(strings.TrimSpace(line), "export ")
		name, value, ok := strings.Cut(line, "=")
		if ok && (name == "OPENROUTER_API" || name == "OPENROUTER_API_KEY") {
			if value = strings.Trim(value, `"'`); value != "" {
				return newClient(value), nil
			}
		}
	}
	return nil, errors.New("openrouter key fetcher printed no OPENROUTER_API export")
}

func newClient(key string) *OpenRouter {
	return &OpenRouter{key: key, http: &http.Client{Timeout: 3 * time.Minute}}
}

// JSON sends one user message and returns the schema-shaped JSON answer plus usage.
func (o *OpenRouter) JSON(ctx context.Context, model, prompt, name string, schema json.RawMessage) (json.RawMessage, *Usage, error) {
	body, err := json.Marshal(map[string]any{
		"model":       model,
		"temperature": 0.2,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": name, "strict": true, "schema": schema},
		},
	})
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "engimprove")
	resp, err := o.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("openrouter %s: %s", resp.Status, clip(string(raw)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *Usage `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, nil, fmt.Errorf("openrouter response: %w", err)
	}
	if out.Error != nil {
		return nil, nil, fmt.Errorf("openrouter: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return nil, nil, errors.New("openrouter returned no choices")
	}
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	content = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(content, "```json"), "```"), "```")
	content = strings.TrimSpace(content)
	if !json.Valid([]byte(content)) {
		return nil, nil, fmt.Errorf("openrouter answer is not json: %s", clip(content))
	}
	return json.RawMessage(content), out.Usage, nil
}

// Transcribe sends audio bytes to OpenRouter's STT endpoint and returns the transcript plus usage.
func (o *OpenRouter) Transcribe(ctx context.Context, model, format string, audio []byte) (string, *Usage, error) {
	body, err := json.Marshal(map[string]any{
		"model":    model,
		"language": "en",
		"input_audio": map[string]string{
			"data":   base64.StdEncoding.EncodeToString(audio),
			"format": format,
		},
	})
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sttEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "engimprove")
	resp, err := o.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("openrouter stt %s: %s", resp.Status, clip(string(raw)))
	}
	var out struct {
		Text  string `json:"text"`
		Usage *Usage `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", nil, fmt.Errorf("openrouter stt response: %w", err)
	}
	if out.Error != nil {
		return "", nil, fmt.Errorf("openrouter stt: %s", out.Error.Message)
	}
	return strings.TrimSpace(out.Text), out.Usage, nil
}

func clip(s string) string {
	if len(s) > 500 {
		return s[:500] + "…"
	}
	return s
}
