package manifest

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// Manifest describes a Loqa skill package.
type Manifest struct {
	Metadata     Metadata     `yaml:"metadata"`
	Runtime      RuntimeSpec  `yaml:"runtime"`
	Capabilities Capabilities `yaml:"capabilities"`
	Permissions  []string     `yaml:"permissions"`
	Surfaces     Surfaces     `yaml:"surfaces,omitempty"`
}

type Metadata struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	Tags        []string `yaml:"tags,omitempty"`
}

type RuntimeSpec struct {
	Mode        string `yaml:"mode"`
	Module      string `yaml:"module"`
	Entrypoint  string `yaml:"entrypoint"`
	HostVersion string `yaml:"host_version"`
}

type Capabilities struct {
	Bus     BusSpec     `yaml:"bus"`
	Storage StorageSpec `yaml:"storage,omitempty"`
	Timers  bool        `yaml:"timers,omitempty"`
}

type BusSpec struct {
	Publish   []string `yaml:"publish,omitempty"`
	Subscribe []string `yaml:"subscribe,omitempty"`
}

type StorageSpec struct {
	KV bool `yaml:"kv"`
}

type Surfaces struct {
	Voice       bool `yaml:"voice,omitempty"`
	Display     bool `yaml:"display,omitempty"`
	Automations bool `yaml:"automations,omitempty"`
}

// Load reads a manifest from disk.
func Load(path string) (Manifest, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Validate ensures manifest contains required fields.
func Validate(m Manifest) error {
	if m.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if m.Metadata.Version == "" {
		return fmt.Errorf("metadata.version is required")
	}
	if m.Runtime.Mode == "" {
		return fmt.Errorf("runtime.mode is required")
	}
	switch m.Runtime.Mode {
	case "wasm":
		if m.Runtime.Module == "" {
			return fmt.Errorf("runtime.module is required for wasm")
		}
		if m.Runtime.Entrypoint == "" {
			return fmt.Errorf("runtime.entrypoint is required for wasm")
		}
	default:
		return fmt.Errorf("runtime.mode %q not supported", m.Runtime.Mode)
	}
	if len(m.Capabilities.Bus.Publish) == 0 && len(m.Capabilities.Bus.Subscribe) == 0 {
		return fmt.Errorf("capabilities.bus must declare publish or subscribe subjects")
	}
	if len(m.Permissions) == 0 {
		return fmt.Errorf("permissions must include at least one entry")
	}
	return nil
}
