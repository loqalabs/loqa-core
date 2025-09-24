package tts

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"

	"github.com/mattn/go-shellwords"
)

type execSynth struct {
	cmd        []string
	sampleRate int
	channels   int
	mu         sync.Mutex
}

type execRequest struct {
	Text       string `json:"text"`
	Voice      string `json:"voice"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
}

type execResponse struct {
	PCMBase64 string `json:"pcm_base64"`
	Final     bool   `json:"final"`
}

func NewExecSynth(command string, sampleRate, channels int) (Synthesizer, error) {
	parser := shellwords.NewParser()
	args, err := parser.Parse(command)
	if err != nil {
		return nil, fmt.Errorf("parse tts command: %w", err)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("tts command empty")
	}
	return &execSynth{cmd: args, sampleRate: sampleRate, channels: channels}, nil
}

func (e *execSynth) Synthesize(ctx context.Context, req SynthRequest) (<-chan SynthChunk, <-chan error) {
	e.mu.Lock()
	schunks := make(chan SynthChunk)
	errs := make(chan error, 1)
	go func() {
		defer close(schunks)
		defer close(errs)
		defer e.mu.Unlock()

		reqPayload := execRequest{
			Text:       req.Text,
			Voice:      req.Voice,
			SampleRate: e.sampleRate,
			Channels:   e.channels,
		}
		data, err := json.Marshal(reqPayload)
		if err != nil {
			errs <- err
			return
		}

		base := e.cmd[0]
		args := append([]string{}, e.cmd[1:]...)
		cmd := exec.CommandContext(ctx, base, args...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			errs <- err
			return
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errs <- err
			return
		}
		if err := cmd.Start(); err != nil {
			errs <- err
			return
		}

		if _, err := stdin.Write(data); err != nil {
			errs <- err
			cmd.Wait()
			return
		}
		stdin.Close()

		scanner := bufio.NewScanner(stdout)
		sequence := 0
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var resp execResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				errs <- err
				cmd.Wait()
				return
			}
			pcm, err := base64.StdEncoding.DecodeString(resp.PCMBase64)
			if err != nil {
				errs <- err
				cmd.Wait()
				return
			}
			schunks <- SynthChunk{
				SessionID:  req.SessionID,
				Sequence:   sequence,
				SampleRate: e.sampleRate,
				Channels:   e.channels,
				PCM:        pcm,
				Final:      resp.Final,
			}
			sequence++
		}
		err = cmd.Wait()
		if err != nil {
			errs <- err
			return
		}
		if scanErr := scanner.Err(); scanErr != nil {
			errs <- scanErr
		}
	}()
	return schunks, errs
}
