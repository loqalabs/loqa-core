package llm

import (
	"context"
	"time"

	"github.com/ambiware-labs/loqa-core/internal/config"
)

// Request describes a language model prompt.
type Request struct {
	SessionID   string
	Prompt      string
	System      string
	Tier        string
	MaxTokens   int
	Temperature float64
	TraceID     string
}

// Chunk represents streamed model output.
type Chunk struct {
	SessionID        string
	Content          string
	Partial          bool
	PromptTokens     int
	CompletionTokens int
	Latency          time.Duration
	TraceID          string
}

// Generator defines a pluggable LLM backend.
type Generator interface {
	Generate(ctx context.Context, req Request, consumer func(Chunk) error) error
}

// OptionsFromConfig builds defaults from config.
func OptionsFromConfig(cfg config.LLMConfig, reqTier string) (Request, error) {
	req := Request{Tier: cfg.DefaultTier, MaxTokens: cfg.MaxTokens, Temperature: cfg.Temperature}
	if reqTier != "" {
		req.Tier = reqTier
	}
	return req, nil
}
