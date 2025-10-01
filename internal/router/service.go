package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/loqalabs/loqa-core/internal/bus"
	"github.com/loqalabs/loqa-core/internal/config"
	"github.com/loqalabs/loqa-core/internal/protocol"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	cfg            config.RouterConfig
	bus            *bus.Client
	logger         *slog.Logger
	subTranscripts *nats.Subscription
	subLLM         *nats.Subscription
	subTTSDone     *nats.Subscription
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup

	tracer         trace.Tracer
	latency        metric.Float64Histogram
	latencyEnabled bool

	mu       sync.Mutex
	sessions map[string]*sessionState
}

type sessionState struct {
	LastPrompt string
	Voice      string
	Tier       string
	Started    time.Time
	Span       trace.Span
}

func NewService(parent context.Context, cfg config.RouterConfig, busClient *bus.Client, logger *slog.Logger) *Service {
	ctx, cancel := context.WithCancel(parent)
	tracer := otel.Tracer("github.com/loqalabs/loqa-core/router")
	meter := otel.Meter("github.com/loqalabs/loqa-core/router")

	hist, err := meter.Float64Histogram(
		"loqa.voice_latency_ms",
		metric.WithDescription("Voice session latency from transcript to playback"),
		metric.WithUnit("ms"),
	)
	enabled := err == nil
	if err != nil {
		logger.Warn("failed to initialize latency histogram", slog.String("error", err.Error()))
	}

	return &Service{
		cfg:            cfg,
		bus:            busClient,
		logger:         logger.With(slog.String("component", "router")),
		ctx:            ctx,
		cancel:         cancel,
		tracer:         tracer,
		latency:        hist,
		latencyEnabled: enabled,
		sessions:       make(map[string]*sessionState),
	}
}

func (s *Service) Start() error {
	if !s.cfg.Enabled {
		return nil
	}

	sub, err := s.bus.Conn().Subscribe(protocol.SubjectTranscriptFinal, s.handleTranscript)
	if err != nil {
		return err
	}
	s.subTranscripts = sub

	subLLM, err := s.bus.Conn().Subscribe(protocol.SubjectLLMResponseFinal, s.handleLLMResponse)
	if err != nil {
		s.subTranscripts.Drain()
		return err
	}
	s.subLLM = subLLM

	subDone, err := s.bus.Conn().Subscribe(protocol.SubjectTTSDone, s.handleTTSDone)
	if err != nil {
		s.subTranscripts.Drain()
		s.subLLM.Drain()
		return err
	}
	s.subTTSDone = subDone
	return nil
}

func (s *Service) Close() {
	s.cancel()
	if s.subTranscripts != nil {
		_ = s.subTranscripts.Drain()
	}
	if s.subLLM != nil {
		_ = s.subLLM.Drain()
	}
	if s.subTTSDone != nil {
		_ = s.subTTSDone.Drain()
	}
	s.wg.Wait()
}

func (s *Service) Healthy() bool {
	if !s.cfg.Enabled {
		return true
	}
	return s.subTranscripts != nil && s.subLLM != nil && s.subTTSDone != nil
}

func (s *Service) handleTranscript(msg *nats.Msg) {
	var transcript protocol.Transcript
	if err := json.Unmarshal(msg.Data, &transcript); err != nil {
		s.logger.Warn("router failed to decode transcript", slogError(err))
		return
	}
	if transcript.Text == "" {
		return
	}

	started := time.Now()
	_, span := s.tracer.Start(context.Background(), "voice.session",
		trace.WithAttributes(
			attribute.String("session_id", transcript.SessionID),
			attribute.String("router.voice", s.cfg.DefaultVoice),
			attribute.String("router.tier", s.cfg.DefaultTier),
		),
	)

	s.mu.Lock()
	s.sessions[transcript.SessionID] = &sessionState{
		LastPrompt: transcript.Text,
		Voice:      s.cfg.DefaultVoice,
		Tier:       s.cfg.DefaultTier,
		Started:    started,
		Span:       span,
	}
	s.mu.Unlock()

	req := protocol.LLMRequest{
		SessionID: transcript.SessionID,
		Prompt:    transcript.Text,
		Tier:      s.cfg.DefaultTier,
		Timestamp: time.Now().UTC(),
	}
	if err := s.publishLLMRequest(req); err != nil {
		s.logger.Warn("router failed to publish llm request", slogError(err))
	}
}

func (s *Service) publishLLMRequest(req protocol.LLMRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return s.bus.Conn().Publish(protocol.SubjectLLMRequest, data)
}

func (s *Service) handleLLMResponse(msg *nats.Msg) {
	var resp protocol.LLMResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		s.logger.Warn("router failed to decode llm response", slogError(err))
		return
	}
	if resp.Content == "" {
		return
	}

	s.mu.Lock()
	state := s.sessions[resp.SessionID]
	s.mu.Unlock()

	voice := s.cfg.DefaultVoice
	if state != nil && state.Voice != "" {
		voice = state.Voice
	}
	if state != nil && state.Span != nil {
		state.Span.AddEvent("llm.response.final",
			trace.WithAttributes(
				attribute.Int("prompt_tokens", resp.PromptTokens),
				attribute.Int("completion_tokens", resp.CompletionTokens),
			),
		)
	}

	req := protocol.TTSRequest{
		SessionID: resp.SessionID,
		Text:      resp.Content,
		Voice:     voice,
		Target:    s.cfg.Target,
		TraceID:   resp.TraceID,
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.publishTTSRequest(req); err != nil {
			s.logger.Warn("router failed to publish tts request", slogError(err))
		}
	}()
}

func (s *Service) publishTTSRequest(req protocol.TTSRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return s.bus.Conn().Publish(protocol.SubjectTTSRequest, data)
}

func (s *Service) handleTTSDone(msg *nats.Msg) {
	var status protocol.TTSStatus
	if err := json.Unmarshal(msg.Data, &status); err != nil {
		s.logger.Warn("router failed to decode tts status", slogError(err))
		return
	}
	if !status.Completed {
		return
	}

	s.mu.Lock()
	state := s.sessions[status.SessionID]
	if state != nil {
		delete(s.sessions, status.SessionID)
	}
	s.mu.Unlock()

	if state == nil {
		return
	}

	if state.Span != nil {
		state.Span.AddEvent("tts.done")
		state.Span.End()
	}

	if s.latencyEnabled {
		duration := time.Since(state.Started)
		s.latency.Record(context.Background(), float64(duration)/float64(time.Millisecond),
			metric.WithAttributes(
				attribute.String("router.voice", state.Voice),
				attribute.String("router.tier", state.Tier),
			),
		)
	}
}

func slogError(err error) slog.Attr {
	return slog.String("error", err.Error())
}
