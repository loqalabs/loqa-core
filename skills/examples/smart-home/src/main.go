package main

import (
	"encoding/json"
	"os"

	"github.com/ambiware-labs/loqa-core/skills/examples/internal/host"
)

type intent struct {
	Room    string `json:"room"`
	Device  string `json:"device"`
	Action  string `json:"action"`
	Payload string `json:"payload"`
}

//export run
func run() {
	host.Log("smart-home bridge skill initialized")

	endpoint := os.Getenv("HOMEASSISTANT_URL")
	if endpoint == "" {
		host.Log("HOMEASSISTANT_URL not set; using http://localhost:8123")
		endpoint = "http://localhost:8123"
	}

	token := os.Getenv("HOMEASSISTANT_TOKEN")
	if token == "" {
		host.Log("HOMEASSISTANT_TOKEN not provided; requests will fail against a real instance")
	}

	payload := os.Getenv("LOQA_SMART_HOME_INTENT")
	if payload == "" {
		host.Log("no intent supplied; set LOQA_SMART_HOME_INTENT to test locally")
		return
	}

	var cmd intent
	if err := json.Unmarshal([]byte(payload), &cmd); err != nil {
		host.Log("failed to parse intent: " + err.Error())
		return
	}

	if cmd.Action == "" || cmd.Device == "" {
		host.Log("intent missing required fields: action/device")
		return
	}

	body, err := json.Marshal(map[string]string{
		"entity_id": cmd.Device,
		"room":      cmd.Room,
		"payload":   cmd.Payload,
	})
	if err != nil {
		host.Log("failed to encode outbound payload: " + err.Error())
		return
	}

	host.Log("would call Home Assistant at " + endpoint)
	host.Log("authorization token present: " + boolText(token != ""))
	host.Log("request body: " + string(body))
}

func boolText(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func main() {}
