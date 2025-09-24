package protocol

import "time"

// AudioFrame represents PCM audio data streamed from edge devices.
type AudioFrame struct {
	SessionID  string `json:"session_id"`
	Sequence   int    `json:"sequence"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
	PCM        []byte `json:"pcm"`
	Final      bool   `json:"final"`
}

// Transcript represents STT output broadcast on the bus.
type Transcript struct {
	SessionID  string    `json:"session_id"`
	Text       string    `json:"text"`
	Partial    bool      `json:"partial"`
	Timestamp  time.Time `json:"timestamp"`
	Confidence float64   `json:"confidence,omitempty"`
}

const (
	SubjectAudioFramePrefix  = "audio.frame"
	SubjectTranscriptPartial = "stt.text.partial"
	SubjectTranscriptFinal   = "stt.text.final"
)
