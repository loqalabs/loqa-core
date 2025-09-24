package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"

	"github.com/mattn/go-shellwords"
)

type execGenerator struct {
	cmd []string
	mu  sync.Mutex
}

type execResponse struct {
	Content          string `json:"content"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
}

func NewExecGenerator(command string) (Generator, error) {
	parser := shellwords.NewParser()
	args, err := parser.Parse(command)
	if err != nil {
		return nil, fmt.Errorf("parse llm command: %w", err)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("llm command empty")
	}
	return &execGenerator{cmd: args}, nil
}

func (g *execGenerator) Generate(ctx context.Context, req Request, consumer func(Chunk) error) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	payload := map[string]any{
		"prompt":      req.Prompt,
		"system":      req.System,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}
	input, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	base := g.cmd[0]
	args := append([]string{}, g.cmd[1:]...)
	cmd := exec.CommandContext(ctx, base, args...)
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("llm exec command failed: %w", err)
	}

	var resp execResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return fmt.Errorf("decode llm exec response: %w", err)
	}

	return consumer(Chunk{
		SessionID:        req.SessionID,
		Content:          resp.Content,
		Partial:          false,
		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		Latency:          0,
		TraceID:          req.TraceID,
	})
}
