package tts

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
	cfg    config.TTSConfig
	bus    *bus.Client
	synth  Synthesizer
	sub    *nats.Subscription
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *slog.Logger
}

func NewService(parent context.Context, cfg config.TTSConfig, busClient *bus.Client, synth Synthesizer, log *slog.Logger) *Service {
	ctx, cancel := context.WithCancel(parent)
	return &Service{
		cfg:    cfg,
		bus:    busClient,
		synth:  synth,
		ctx:    ctx,
		cancel: cancel,
		logger: log.With(slog.String("component", "tts-service")),
	}
}

func (s *Service) Start() error {
	if !s.cfg.Enabled {
		return nil
	}
	sub, err := s.bus.Conn().Subscribe(protocol.SubjectTTSRequest, s.handleRequest)
	if err != nil {
		return err
	}
	s.sub = sub
	return nil
}

func (s *Service) Close() {
	s.cancel()
	if s.sub != nil {
		_ = s.sub.Drain()
	}
	s.wg.Wait()
}

func (s *Service) Healthy() bool { return !s.cfg.Enabled || s.sub != nil }

func (s *Service) handleRequest(msg *nats.Msg) {
	var req protocol.TTSRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.logger.Warn("failed to decode tts request", slogError(err))
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ctx, cancel := context.WithTimeout(s.ctx, 45*time.Second)
		defer cancel()

		chunks, errs := s.synth.Synthesize(ctx, SynthRequest{SessionID: req.SessionID, Text: req.Text, Voice: req.Voice})
		sequence := 0
		for {
			select {
			case chunk, ok := <-chunks:
				if !ok {
					chunks = nil
					continue
				}
				chunk.Sequence = sequence
				sequence++
				s.publishChunk(req, chunk)
			case err, ok := <-errs:
				if ok && err != nil {
					s.logger.Warn("tts synthesis error", slogError(err))
				}
				errs = nil
			case <-ctx.Done():
				s.logger.Warn("tts synthesis cancelled", slogError(ctx.Err()))
				return
			}
			if chunks == nil && errs == nil {
				return
			}
		}
	}()
}

func (s *Service) publishChunk(req protocol.TTSRequest, chunk SynthChunk) {
	packet := protocol.AudioChunk{
		SessionID:  req.SessionID,
		Target:     req.Target,
		SampleRate: chunk.SampleRate,
		Channels:   chunk.Channels,
		Sequence:   chunk.Sequence,
		PCM:        chunk.PCM,
		Final:      chunk.Final,
	}
	data, err := json.Marshal(packet)
	if err != nil {
		s.logger.Warn("failed to marshal tts chunk", slogError(err))
		return
	}
	subject := protocol.SubjectTTSAudio
	if err := s.bus.Conn().Publish(subject, data); err != nil {
		s.logger.Warn("failed to publish tts chunk", slogError(err))
	}
	if chunk.Final {
		finalMsg := protocol.TTSStatus{SessionID: req.SessionID, Target: req.Target, Completed: true, Timestamp: time.Now().UTC()}
		if data, err := json.Marshal(finalMsg); err == nil {
			_ = s.bus.Conn().Publish(protocol.SubjectTTSDone, data)
		}
	}
}

func slogError(err error) slog.Attr {
	return slog.String("error", err.Error())
}
