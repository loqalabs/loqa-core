package llm

import (
	"context"
	"strings"
	"time"
)

type mockGenerator struct{}

func NewMockGenerator() Generator { return &mockGenerator{} }

func (m *mockGenerator) Generate(ctx context.Context, req Request, consumer func(Chunk) error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(20 * time.Millisecond):
	}
	content := "[mock completion for " + strings.TrimSpace(req.Prompt) + "]"
	return consumer(Chunk{
		SessionID: req.SessionID,
		Content:   content,
		Partial:   false,
		Latency:   20 * time.Millisecond,
	})
}
