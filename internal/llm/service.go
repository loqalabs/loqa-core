package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/loqalabs/loqa-core/internal/bus"
	"github.com/loqalabs/loqa-core/internal/config"
	"github.com/loqalabs/loqa-core/internal/protocol"
	"github.com/nats-io/nats.go"
)

type Service struct {
	cfg       config.LLMConfig
	bus       *bus.Client
	generator Generator
	sub       *nats.Subscription
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	ready     bool
	logger    *slog.Logger
}

func NewService(parent context.Context, cfg config.LLMConfig, busClient *bus.Client, generator Generator, logger *slog.Logger) *Service {
	ctx, cancel := context.WithCancel(parent)
	return &Service{
		cfg:       cfg,
		bus:       busClient,
		generator: generator,
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger.With(slog.String("component", "llm-service")),
	}
}

func (s *Service) Start() error {
	if !s.cfg.Enabled {
		return nil
	}
	sub, err := s.bus.Conn().Subscribe(protocol.SubjectLLMRequest, s.handleRequest)
	if err != nil {
		return fmt.Errorf("subscribe LLM requests: %w", err)
	}
	s.sub = sub
	s.ready = true
	return nil
}

func (s *Service) Close() {
	s.cancel()
	if s.sub != nil {
		_ = s.sub.Drain()
	}
	s.wg.Wait()
}

func (s *Service) Healthy() bool {
	return !s.cfg.Enabled || s.ready
}

func (s *Service) handleRequest(msg *nats.Msg) {
	var req protocol.LLMRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.logger.Warn("failed to decode llm request", slogError(err))
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(s.ctx, 60*time.Second)
		defer cancel()

		options, err := OptionsFromConfig(s.cfg, req.Tier)
		if err != nil {
			s.logger.Warn("invalid LLM options", slogError(err))
			return
		}
		options.SessionID = req.SessionID
		options.Prompt = req.Prompt
		options.System = req.System
		options.MaxTokens = coalesceInt(req.MaxTokens, s.cfg.MaxTokens)
		if req.Temperature != 0 {
			options.Temperature = req.Temperature
		}
		options.TraceID = req.TraceID

		start := time.Now()
		err = s.generator.Generate(ctx, options, func(chunk Chunk) error {
			return s.publishChunk(chunk)
		})
		if err != nil {
			s.logger.Warn("llm generation failed", slogError(err))
			return
		}
		s.logger.Info("llm generation complete", slog.Duration("latency", time.Since(start)))
	}()
}

func (s *Service) publishChunk(chunk Chunk) error {
	if chunk.Content == "" {
		return nil
	}
	msg := protocol.LLMResponse{
		SessionID:        chunk.SessionID,
		Content:          chunk.Content,
		Partial:          chunk.Partial,
		TraceID:          chunk.TraceID,
		PromptTokens:     chunk.PromptTokens,
		CompletionTokens: chunk.CompletionTokens,
		LatencyMS:        chunk.Latency.Milliseconds(),
		Timestamp:        time.Now().UTC(),
	}
	subject := protocol.SubjectLLMResponsePartial
	if !chunk.Partial {
		subject = protocol.SubjectLLMResponseFinal
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := s.bus.Conn().Publish(subject, data); err != nil {
		s.logger.Warn("failed to publish llm chunk", slogError(err))
		return err
	}
	return nil
}

func coalesceInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func slogError(err error) slog.Attr {
	return slog.String("error", err.Error())
}
