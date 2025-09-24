package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ollamaGenerator struct {
	endpoint      string
	modelFast     string
	modelBalanced string
}

func NewOllamaGenerator(endpoint, fastModel, balancedModel string) Generator {
	return &ollamaGenerator{endpoint: endpoint, modelFast: fastModel, modelBalanced: balancedModel}
}

func (g *ollamaGenerator) modelForTier(tier string) string {
	switch tier {
	case "fast":
		if g.modelFast != "" {
			return g.modelFast
		}
	case "balanced":
		if g.modelBalanced != "" {
			return g.modelBalanced
		}
	}
	if g.modelBalanced != "" {
		return g.modelBalanced
	}
	if g.modelFast != "" {
		return g.modelFast
	}
	return "llama3.2:latest"
}

type ollamaRequest struct {
	Model   string        `json:"model"`
	Prompt  string        `json:"prompt"`
	System  string        `json:"system,omitempty"`
	Stream  bool          `json:"stream"`
	Options ollamaOptions `json:"options"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaStreamResponse struct {
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	EvalCount       int    `json:"eval_count,omitempty"`
	PromptEvalCount int    `json:"prompt_eval_count,omitempty"`
}

func (g *ollamaGenerator) Generate(ctx context.Context, req Request, consumer func(Chunk) error) error {
	model := g.modelForTier(req.Tier)
	payload := ollamaRequest{
		Model:  model,
		Prompt: req.Prompt,
		System: req.System,
		Stream: true,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, g.endpoint+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ollama returned status %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	esStart := time.Now()
	var accumulated string
	var promptTokens, completionTokens int
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var chunk ollamaStreamResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			return err
		}
		accumulated += chunk.Response
		if chunk.EvalCount > 0 {
			completionTokens = chunk.EvalCount
		}
		if chunk.PromptEvalCount > 0 {
			promptTokens = chunk.PromptEvalCount
		}
		partial := !chunk.Done
		if err := consumer(Chunk{
			SessionID:        req.SessionID,
			Content:          chunk.Response,
			Partial:          partial,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			Latency:          time.Since(esStart),
			TraceID:          req.TraceID,
		}); err != nil {
			return err
		}
	}
	return scanner.Err()
}
