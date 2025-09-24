package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

const validYAML = `metadata:
  name: timer
  version: 0.1.0
  description: Timer skill
  author: Ambiware Labs
runtime:
  mode: wasm
  module: build/timer.wasm
  entrypoint: handle
  host_version: v1
capabilities:
  bus:
    publish:
      - tts.request
    subscribe:
      - skill.timer.input
  storage:
    kv: true
  timers: true
permissions:
  - event_store:read
surfaces:
  voice: true
`

func TestValidateValidManifest(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "skill.yaml")
	if err := os.WriteFile(path, []byte(validYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := Validate(m); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateMissingFields(t *testing.T) {
	m := Manifest{}
	if err := Validate(m); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateUnsupportedMode(t *testing.T) {
	m := Manifest{
		Metadata:     Metadata{Name: "x", Version: "1"},
		Runtime:      RuntimeSpec{Mode: "python"},
		Capabilities: Capabilities{Bus: BusSpec{Publish: []string{"foo"}}},
		Permissions:  []string{"foo"},
	}
	if err := Validate(m); err == nil {
		t.Fatalf("expected error for unsupported runtime")
	}
}
