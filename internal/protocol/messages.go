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
	SubjectAudioFramePrefix   = "audio.frame"
	SubjectTranscriptPartial  = "stt.text.partial"
	SubjectTranscriptFinal    = "stt.text.final"
	SubjectLLMRequest         = "nlu.request"
	SubjectLLMResponsePartial = "nlu.response.partial"
	SubjectLLMResponseFinal   = "nlu.response.final"
	SubjectTTSRequest         = "tts.request"
	SubjectTTSAudio           = "tts.audio"
	SubjectTTSDone            = "tts.done"
)

// LLMRequest represents a prompt sent to the language model harness.
type LLMRequest struct {
	SessionID   string    `json:"session_id"`
	Prompt      string    `json:"prompt"`
	System      string    `json:"system,omitempty"`
	Tier        string    `json:"tier,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	TraceID     string    `json:"trace_id,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// LLMResponse represents streamed or final completions from the harness.
type LLMResponse struct {
	SessionID        string    `json:"session_id"`
	Content          string    `json:"content"`
	Partial          bool      `json:"partial"`
	TraceID          string    `json:"trace_id,omitempty"`
	PromptTokens     int       `json:"prompt_tokens,omitempty"`
	CompletionTokens int       `json:"completion_tokens,omitempty"`
	LatencyMS        int64     `json:"latency_ms,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
}

// TTSRequest asks the TTS service to synthesize a phrase.
type TTSRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	Voice     string `json:"voice,omitempty"`
	Target    string `json:"target,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
}

// AudioChunk carries synthesized PCM audio destined for output devices.
type AudioChunk struct {
	SessionID  string `json:"session_id"`
	Target     string `json:"target,omitempty"`
	Sequence   int    `json:"sequence"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
	PCM        []byte `json:"pcm"`
	Final      bool   `json:"final"`
}

type TTSStatus struct {
	SessionID string    `json:"session_id"`
	Target    string    `json:"target,omitempty"`
	Completed bool      `json:"completed"`
	Timestamp time.Time `json:"timestamp"`
}
