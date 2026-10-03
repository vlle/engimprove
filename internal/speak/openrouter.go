package speak

import (
	"context"

	"engimprove/internal/llm"
)

// OpenRouterTranscriber sends audio to OpenRouter's STT endpoint.
type OpenRouterTranscriber struct {
	Client    *llm.OpenRouter
	ModelName string
}

// NewOpenRouterTranscriber builds a transcriber backed by OpenRouter.
func NewOpenRouterTranscriber(client *llm.OpenRouter, model string) *OpenRouterTranscriber {
	return &OpenRouterTranscriber{Client: client, ModelName: model}
}

func (o *OpenRouterTranscriber) Backend() string { return "openrouter" }
func (o *OpenRouterTranscriber) Model() string   { return o.ModelName }

func (o *OpenRouterTranscriber) Transcribe(ctx context.Context, audio []byte, ext string) (string, float64, float64, error) {
	format := ext
	if format != "" && format[0] == '.' {
		format = format[1:]
	}
	if format == "" {
		format = "webm"
	}
	text, usage, err := o.Client.Transcribe(ctx, o.ModelName, format, audio)
	if err != nil {
		return "", 0, 0, err
	}
	seconds, cost := 0.0, 0.0
	if usage != nil {
		seconds = usage.Seconds
		cost = usage.Cost
	}
	return text, seconds, cost, nil
}
