package stt

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/loqalabs/loqa-core/internal/config"
	"github.com/mattn/go-shellwords"
)

type execRecognizer struct {
	cmd []string
	cfg config.STTConfig
	mu  sync.Mutex
}

type execResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

func NewExecRecognizer(cfg config.STTConfig) (Recognizer, error) {
	parser := shellwords.NewParser()
	args, err := parser.Parse(cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("parse stt command: %w", err)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("stt command is empty")
	}
	return &execRecognizer{cmd: args, cfg: cfg}, nil
}

func (r *execRecognizer) Transcribe(ctx context.Context, pcm []byte, sampleRate int, channels int, final bool) (TranscriptResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tmpDir := os.TempDir()
	file, err := os.CreateTemp(tmpDir, "loqa_stt_*.wav")
	if err != nil {
		return TranscriptResult{}, fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	if err := writePCMToWav(file, pcm, sampleRate, channels); err != nil {
		return TranscriptResult{}, err
	}

	args := append([]string{}, r.cmd...)
	if len(args) == 0 {
		return TranscriptResult{}, fmt.Errorf("stt command empty")
	}
	base := args[0]
	cmdArgs := args[1:]
	cmdArgs = append(cmdArgs, "--audio", file.Name())
	if r.cfg.ModelPath != "" {
		cmdArgs = append(cmdArgs, "--model", r.cfg.ModelPath)
	}
	if r.cfg.Language != "" {
		cmdArgs = append(cmdArgs, "--language", r.cfg.Language)
	}
	if r.cfg.Mode == "exec" && r.cfg.PublishInterim && !final {
		cmdArgs = append(cmdArgs, "--partial")
	}

	command := exec.CommandContext(ctx, base, cmdArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return TranscriptResult{}, fmt.Errorf("stt command failed: %w: %s", err, stderr.String())
	}

	var resp execResult
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return TranscriptResult{}, fmt.Errorf("decode stt response: %w", err)
	}
	return TranscriptResult{Text: resp.Text, Confidence: resp.Confidence}, nil
}

func writePCMToWav(file *os.File, pcm []byte, sampleRate int, channels int) error {
	if len(pcm)%2 != 0 {
		return fmt.Errorf("pcm payload not aligned")
	}
	buffer := &audio.IntBuffer{Format: &audio.Format{NumChannels: channels, SampleRate: sampleRate}}
	samples := make([]int, len(pcm)/2)
	for i := 0; i < len(samples); i++ {
		sample := int(int16(binary.LittleEndian.Uint16(pcm[i*2:])))
		samples[i] = sample
	}
	buffer.Data = samples

	enc := wav.NewEncoder(file, sampleRate, 16, channels, 1)
	if err := enc.Write(buffer); err != nil {
		return fmt.Errorf("write wav: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("close wav encoder: %w", err)
	}
	return nil
}
