package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/loqalabs/loqa-core/skills/examples/internal/host"
)

type timerRequest struct {
	DurationMS int    `json:"duration_ms"`
	Label      string `json:"label"`
}

type timerStatus struct {
	Label   string `json:"label"`
	State   string `json:"state"`
	Seconds int    `json:"seconds_remaining,omitempty"`
}

type ttsRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
}

//export run
func run() {
	host.Log("timer skill invocation")
	subject := os.Getenv("LOQA_EVENT_SUBJECT")
	payload := os.Getenv("LOQA_EVENT_PAYLOAD")

	switch subject {
	case "skill.timer.start":
		handleStart(payload)
	case "skill.timer.cancel":
		host.Log("cancel timers not implemented yet")
	default:
		host.Log("unrecognized subject: " + subject)
	}
}

func handleStart(payload string) {
	if payload == "" {
		host.Log("missing payload for timer start")
		return
	}
	var req timerRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		host.Log("failed to decode timer request: " + err.Error())
		return
	}
	if req.DurationMS <= 0 {
		host.Log("timer duration must be positive")
		return
	}
	label := req.Label
	if label == "" {
		label = "timer"
	}
	delay := time.Duration(req.DurationMS) * time.Millisecond

	reportStatus(timerStatus{Label: label, State: "started", Seconds: req.DurationMS / 1000})
	host.Log(fmt.Sprintf("starting %s for %s", label, delay))
	time.Sleep(delay)
	reportStatus(timerStatus{Label: label, State: "completed"})
	host.Log(fmt.Sprintf("%s complete", label))
	announceCompletion(label)
}

func reportStatus(status timerStatus) {
	if data, err := json.Marshal(status); err == nil {
		host.Publish("skill.timer.status", data)
	}
}

func announceCompletion(label string) {
	msg := ttsRequest{Text: fmt.Sprintf("%s timer complete", label)}
	if data, err := json.Marshal(msg); err == nil {
		host.Publish("tts.request", data)
	}
}

func main() {}
