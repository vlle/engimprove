package llm

import (
	"encoding/json"
	"testing"
)

func TestParseChatResponseWithUsage(t *testing.T) {
	raw := []byte(`{
		"choices":[{"message":{"content":"{\"ok\":true}"}}],
		"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":0.00075}
	}`)
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *Usage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Usage == nil {
		t.Fatal("usage nil")
	}
	if out.Usage.Cost != 0.00075 {
		t.Fatalf("cost = %v", out.Usage.Cost)
	}
	if out.Usage.TotalTokens != 15 {
		t.Fatalf("tokens = %d", out.Usage.TotalTokens)
	}
}

func TestParseSTTResponse(t *testing.T) {
	raw := []byte(`{"text":"hello world","usage":{"seconds":1.2,"cost":0.0001}}`)
	var out struct {
		Text  string `json:"text"`
		Usage *Usage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Text != "hello world" {
		t.Fatalf("text = %q", out.Text)
	}
	if out.Usage == nil || out.Usage.Cost != 0.0001 || out.Usage.Seconds != 1.2 {
		t.Fatalf("usage = %+v", out.Usage)
	}
}
