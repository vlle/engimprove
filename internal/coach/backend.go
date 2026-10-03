package coach

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"engimprove/internal/config"
	"engimprove/internal/llm"
)

// Coach routes structured requests to OpenRouter or to `claude -p`.
type Coach struct {
	cfg    config.Config
	or     *llm.OpenRouter
	orErr  error
	cli    *Runner
	cliErr error
}

// New resolves both backends once; either may be missing.
func New(cfg config.Config) *Coach {
	c := &Coach{cfg: cfg}
	c.or, c.orErr = llm.FromEnv(cfg.OpenRouterEnv)
	if r, err := Find(cfg.ClaudeModel); err == nil {
		c.cli = &r
	} else {
		c.cliErr = err
	}
	return c
}

// OpenRouter returns the underlying OpenRouter client, if any.
func (c *Coach) OpenRouter() *llm.OpenRouter { return c.or }

// Backend names the service that would answer; private text never goes to OpenRouter.
func (c *Coach) Backend(private bool) (string, error) {
	switch {
	case !private && c.or != nil:
		return "openrouter " + c.cfg.OpenRouterModel, nil
	case c.cli != nil:
		return "claude -p " + c.cfg.ClaudeModel, nil
	case private:
		return "", fmt.Errorf("private text needs the claude cli: %v", c.cliErr)
	}
	return "", fmt.Errorf("no llm backend: %v; %v", c.orErr, c.cliErr)
}

func (c *Coach) structured(ctx context.Context, private bool, name, prompt, schema string, out any) (string, *llm.Usage, error) {
	fallback := ""
	if !private && c.or != nil {
		raw, usage, err := c.or.JSON(ctx, c.cfg.OpenRouterModel, prompt, name, json.RawMessage(schema))
		if err == nil {
			err = json.Unmarshal(raw, out)
		}
		if err == nil {
			return "openrouter " + c.cfg.OpenRouterModel, usage, nil
		}
		if c.cli == nil {
			return "", nil, err
		}
		fallback = fmt.Sprintf(" (openrouter failed: %v)", err)
	}
	if c.cli == nil {
		if private {
			return "", nil, errors.New("private text needs the claude cli, which was not found")
		}
		return "", nil, fmt.Errorf("no llm backend: %v; %v", c.orErr, c.cliErr)
	}
	if err := c.cli.Structured(ctx, prompt, schema, out); err != nil {
		return "", nil, err
	}
	return "claude -p " + c.cfg.ClaudeModel + fallback, nil, nil
}

// Probe makes one tiny call per backend and reports what works.
func (c *Coach) Probe(ctx context.Context) map[string]string {
	out := map[string]string{}
	const schema = `{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}},"required":["ok"]}`
	var v struct {
		OK bool `json:"ok"`
	}
	if c.or == nil {
		out["openrouter"] = "unavailable: " + c.orErr.Error()
	} else if raw, _, err := c.or.JSON(ctx, c.cfg.OpenRouterModel, `Answer {"ok": true}`, "probe", json.RawMessage(schema)); err != nil {
		out["openrouter"] = "error: " + err.Error()
	} else if err := json.Unmarshal(raw, &v); err != nil || !v.OK {
		out["openrouter"] = fmt.Sprintf("unexpected answer: %s", raw)
	} else {
		out["openrouter"] = "ok " + c.cfg.OpenRouterModel
	}
	if c.cli == nil {
		out["claude"] = "unavailable: " + c.cliErr.Error()
	} else {
		out["claude"] = "found " + c.cli.Bin + " (not called: costs a full session)"
	}
	return out
}
