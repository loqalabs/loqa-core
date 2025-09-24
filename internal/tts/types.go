package tts

import "context"

// SynthRequest contains parameters to synthesize speech.
type SynthRequest struct {
	SessionID string
	Text      string
	Voice     string
}

// SynthChunk contains PCM data.
type SynthChunk struct {
	SessionID  string
	Sequence   int
	SampleRate int
	Channels   int
	PCM        []byte
	Final      bool
}

// Synthesizer is the contract for producing audio.
type Synthesizer interface {
	Synthesize(ctx context.Context, req SynthRequest) (<-chan SynthChunk, <-chan error)
}
