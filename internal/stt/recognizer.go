package stt

import (
	"context"
)

// TranscriptResult captures recognizer output.
type TranscriptResult struct {
	Text       string
	Confidence float64
}

// Recognizer abstracts STT backends.
type Recognizer interface {
	Transcribe(ctx context.Context, pcm []byte, sampleRate int, channels int, final bool) (TranscriptResult, error)
}
