package stt

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
	cfg        config.STTConfig
	bus        *bus.Client
	recognizer Recognizer
	sessions   map[string]*sessionState
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	sub        *nats.Subscription
	wg         sync.WaitGroup
	ready      bool
}

type sessionState struct {
	Buffer       []byte
	LastPartial  time.Time
	Inflight     bool
	PendingFinal bool
}

func NewService(parent context.Context, cfg config.STTConfig, busClient *bus.Client, recognizer Recognizer) *Service {
	ctx, cancel := context.WithCancel(parent)
	return &Service{
		cfg:        cfg,
		bus:        busClient,
		recognizer: recognizer,
		sessions:   make(map[string]*sessionState),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (s *Service) Start() error {
	if !s.cfg.Enabled {
		return nil
	}
	subject := protocol.SubjectAudioFramePrefix + ".>"
	sub, err := s.bus.Conn().Subscribe(subject, s.handleFrame)
	if err != nil {
		return fmt.Errorf("subscribe audio frames: %w", err)
	}
	s.sub = sub
	s.ready = true
	s.bus.Logger().Info("STT service started", slog.String("mode", s.cfg.Mode), slog.String("subject", subject))
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

func (s *Service) handleFrame(msg *nats.Msg) {
	var frame protocol.AudioFrame
	if err := json.Unmarshal(msg.Data, &frame); err != nil {
		s.bus.Logger().Warn("failed to decode audio frame", slogError(err))
		return
	}

	s.mu.Lock()
	state := s.sessions[frame.SessionID]
	if state == nil {
		state = &sessionState{}
		s.sessions[frame.SessionID] = state
		s.bus.Logger().Info("new STT session started", slog.String("session_id", frame.SessionID))
	}
	state.Buffer = append(state.Buffer, frame.PCM...)
	bufferSize := len(state.Buffer)
	s.mu.Unlock()

	s.bus.Logger().Debug("received audio frame",
		slog.String("session_id", frame.SessionID),
		slog.Int("sequence", frame.Sequence),
		slog.Int("pcm_bytes", len(frame.PCM)),
		slog.Int("buffer_size", bufferSize),
		slog.Bool("final", frame.Final))

	if s.cfg.PublishInterim && !frame.Final {
		schedulePartial := s.shouldSchedulePartial(frame.SessionID)
		if schedulePartial {
			s.bus.Logger().Info("scheduling partial transcription", slog.String("session_id", frame.SessionID))
			s.scheduleTranscription(frame.SessionID, false)
		}
	}
	if frame.Final {
		s.bus.Logger().Info("scheduling final transcription",
			slog.String("session_id", frame.SessionID),
			slog.Int("total_buffer_size", bufferSize))
		s.scheduleTranscription(frame.SessionID, true)
	}
}

func (s *Service) shouldSchedulePartial(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sessions[sessionID]
	if state == nil {
		return false
	}
	if state.Inflight {
		return false
	}
	if state.LastPartial.IsZero() {
		state.LastPartial = time.Now()
		return true
	}
	interval := time.Duration(s.cfg.PartialEveryMS) * time.Millisecond
	if interval <= 0 {
		return false
	}
	if time.Since(state.LastPartial) >= interval {
		state.LastPartial = time.Now()
		return true
	}
	return false
}

func (s *Service) scheduleTranscription(sessionID string, final bool) {
	s.mu.Lock()
	state := s.sessions[sessionID]
	if state == nil {
		s.mu.Unlock()
		return
	}
	if state.Inflight {
		if final {
			state.PendingFinal = true
		}
		s.mu.Unlock()
		return
	}
	pcm := append([]byte(nil), state.Buffer...)
	state.Inflight = true
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(s.ctx, 45*time.Second)
		defer cancel()

		s.bus.Logger().Info("starting transcription",
			slog.String("session_id", sessionID),
			slog.Int("pcm_bytes", len(pcm)),
			slog.Bool("final", final))

		result, err := s.recognizer.Transcribe(ctx, pcm, s.cfg.SampleRate, s.cfg.Channels, final)
		if err != nil {
			s.bus.Logger().Warn("stt transcription failed",
				slog.String("session_id", sessionID),
				slogError(err))
		} else {
			s.bus.Logger().Info("transcription completed",
				slog.String("session_id", sessionID),
				slog.String("text", result.Text),
				slog.Float64("confidence", result.Confidence),
				slog.Bool("final", final))
			s.publishTranscript(sessionID, result.Text, result.Confidence, final)
		}

		s.mu.Lock()
		state := s.sessions[sessionID]
		var pendingFinal bool
		if state != nil {
			state.Inflight = false
			pendingFinal = state.PendingFinal
			if !final {
				state.LastPartial = time.Now()
			}
			if final {
				delete(s.sessions, sessionID)
			}
		}
		s.mu.Unlock()

		if pendingFinal && !final {
			s.scheduleTranscription(sessionID, true)
		}
	}()
}

func (s *Service) publishTranscript(sessionID, text string, confidence float64, final bool) {
	if text == "" {
		s.bus.Logger().Warn("skipping empty transcript", slog.String("session_id", sessionID))
		return
	}
	subject := protocol.SubjectTranscriptPartial
	if final {
		subject = protocol.SubjectTranscriptFinal
	}
	msg := protocol.Transcript{
		SessionID:  sessionID,
		Text:       text,
		Partial:    !final,
		Timestamp:  time.Now().UTC(),
		Confidence: confidence,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		s.bus.Logger().Warn("failed to marshal transcript", slogError(err))
		return
	}
	if err := s.bus.Conn().Publish(subject, data); err != nil {
		s.bus.Logger().Warn("failed to publish transcript", slogError(err))
	} else {
		s.bus.Logger().Info("published transcript",
			slog.String("session_id", sessionID),
			slog.String("subject", subject),
			slog.Int("text_length", len(text)))
	}
}

func slogError(err error) slog.Attr {
	return slog.String("error", err.Error())
}
