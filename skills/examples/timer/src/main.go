package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ambiware-labs/loqa-core/skills/examples/internal/host"
)

// requestEnvelope models the expected payload for a timer start event.
type requestEnvelope struct {
	DurationMS int    `json:"duration_ms"`
	Label      string `json:"label"`
}

//export run
func run() {
	host.Log("timer skill initialized")

	data := os.Getenv("LOQA_TIMER_REQUEST")
	if data == "" {
		host.Log("no timer payload provided; set LOQA_TIMER_REQUEST to test locally")
		return
	}

	var req requestEnvelope
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		host.Log("failed to decode timer payload: " + err.Error())
		return
	}
	if req.DurationMS <= 0 {
		host.Log("missing or invalid duration_ms in request")
		return
	}

	delay := time.Duration(req.DurationMS) * time.Millisecond
	label := req.Label
	if label == "" {
		label = "timer"
	}

	host.Log(fmt.Sprintf("starting %s for %s", label, delay))
	time.Sleep(delay)
	host.Log(fmt.Sprintf("%s complete", label))
}

func main() {}
