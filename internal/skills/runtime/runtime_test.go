package runtime_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ambiware-labs/loqa-core/internal/skills/manifest"
	runtime "github.com/ambiware-labs/loqa-core/internal/skills/runtime"
)

const sampleManifest = `metadata:
  name: sample
  version: 0.0.1
  description: example skill
  author: test
runtime:
  mode: wasm
  module: %s
  entrypoint: run
  host_version: v1
capabilities:
  bus:
    publish:
      - sample.output
permissions:
  - bus:use
`

func TestRuntimeLoadMissingFile(t *testing.T) {
	ctx := context.Background()
	rt, err := runtime.New(ctx, runtime.HostBindings{})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	t.Cleanup(func() { rt.Close(ctx) })

	mfYAML := []byte(formatManifest(sampleManifest, filepath.Join(t.TempDir(), "missing.wasm")))
	manifestPath := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(manifestPath, mfYAML, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	mf, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	if _, err := rt.Load(ctx, mf, map[string]string{}); err == nil {
		t.Fatalf("expected error for missing module")
	}
}

func formatManifest(template, modulePath string) string {
	return fmt.Sprintf(template, modulePath)
}
