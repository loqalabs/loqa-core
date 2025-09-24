package stt

import (
	"context"
	"fmt"
)

type mockRecognizer struct{}

func NewMockRecognizer() Recognizer {
	return &mockRecognizer{}
}

func (m *mockRecognizer) Transcribe(_ context.Context, pcm []byte, _ int, _ int, final bool) (TranscriptResult, error) {
	mode := "partial"
	if final {
		mode = "final"
	}
	return TranscriptResult{
		Text:       fmt.Sprintf("[%s transcript length=%d]", mode, len(pcm)),
		Confidence: 0,
	}, nil
}
