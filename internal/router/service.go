package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/ambiware-labs/loqa-core/internal/bus"
	"github.com/ambiware-labs/loqa-core/internal/config"
	"github.com/ambiware-labs/loqa-core/internal/protocol"
	"github.com/nats-io/nats.go"
)

type Service struct {
	cfg            config.RouterConfig
	bus            *bus.Client
	logger         *slog.Logger
	subTranscripts *nats.Subscription
	subLLM         *nats.Subscription
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	sessions       map[string]*sessionState
	mu             sync.Mutex
}

type sessionState struct {
	LastPrompt string
	Voice      string
	Tier       string
}

func NewService(parent context.Context, cfg config.RouterConfig, busClient *bus.Client, logger *slog.Logger) *Service {
	ctx, cancel := context.WithCancel(parent)
	return &Service{
		cfg:      cfg,
		bus:      busClient,
		logger:   logger.With(slog.String("component", "router")),
		ctx:      ctx,
		cancel:   cancel,
		sessions: make(map[string]*sessionState),
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
	s.wg.Wait()
}

func (s *Service) Healthy() bool {
	return !s.cfg.Enabled || (s.subTranscripts != nil && s.subLLM != nil)
}

func (s *Service) handleTranscript(msg *nats.Msg) {
	var transcript protocol.Transcript
	if err := json.Unmarshal(msg.Data, &transcript); err != nil {
		s.logger.Warn("router failed to decode transcript", slogError(err))
		return
	}
	prompt := transcript.Text
	if prompt == "" {
		return
	}

	s.mu.Lock()
	s.sessions[transcript.SessionID] = &sessionState{
		LastPrompt: prompt,
		Voice:      s.cfg.DefaultVoice,
		Tier:       s.cfg.DefaultTier,
	}
	s.mu.Unlock()

	req := protocol.LLMRequest{
		SessionID: transcript.SessionID,
		Prompt:    prompt,
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
	if state != nil {
		delete(s.sessions, resp.SessionID)
	}
	s.mu.Unlock()

	voice := s.cfg.DefaultVoice
	if state != nil && state.Voice != "" {
		voice = state.Voice
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

func slogError(err error) slog.Attr {
	return slog.String("error", err.Error())
}
