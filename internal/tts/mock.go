package tts

import (
	"context"
	"time"
)

type mockSynth struct {
	sampleRate int
	channels   int
}

func NewMockSynth(sampleRate, channels int) Synthesizer {
	return &mockSynth{sampleRate: sampleRate, channels: channels}
}

func (m *mockSynth) Synthesize(ctx context.Context, req SynthRequest) (<-chan SynthChunk, <-chan error) {
	chunks := make(chan SynthChunk, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(chunks)
		defer close(errs)
		select {
		case <-ctx.Done():
			errs <- ctx.Err()
			return
		case <-time.After(50 * time.Millisecond):
		}
		chunks <- SynthChunk{
			SessionID:  req.SessionID,
			Sequence:   0,
			SampleRate: m.sampleRate,
			Channels:   m.channels,
			PCM:        []byte{},
			Final:      true,
		}
	}()
	return chunks, errs
}
