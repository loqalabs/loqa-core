package runtime

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ambiware-labs/loqa-core/internal/skills/manifest"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime wraps a wazero runtime for executing skill modules.
type Runtime struct {
	rt wazero.Runtime
}

// New creates a new skill runtime using wazero.
func New(ctx context.Context) (*Runtime, error) {
	rt := wazero.NewRuntime(ctx)
	if err := instantiateHostModule(ctx, rt); err != nil {
		return nil, fmt.Errorf("instantiate host module: %w", err)
	}
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}
	return &Runtime{rt: rt}, nil
}

// Close releases resources held by the runtime.
func (r *Runtime) Close(ctx context.Context) error {
	if r == nil || r.rt == nil {
		return nil
	}
	return r.rt.Close(ctx)
}

// Skill represents a loaded skill module.
type Skill struct {
	Manifest manifest.Manifest
	module   api.Module
	entry    api.Function
	compiled wazero.CompiledModule
}

// Close releases resources for the skill.
func (s *Skill) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if s.module != nil {
		if err := s.module.Close(ctx); err != nil {
			return err
		}
	}
	if s.compiled != nil {
		if err := s.compiled.Close(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Load compiles and instantiates a skill from a manifest.
func (r *Runtime) Load(ctx context.Context, m manifest.Manifest) (*Skill, error) {
	if r == nil || r.rt == nil {
		return nil, fmt.Errorf("runtime not initialized")
	}
	if m.Runtime.Mode != "wasm" {
		return nil, fmt.Errorf("unsupported runtime mode %q", m.Runtime.Mode)
	}
	wasmBytes, err := os.ReadFile(m.Runtime.Module)
	if err != nil {
		return nil, fmt.Errorf("read wasm module: %w", err)
	}
	compiled, err := r.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile module: %w", err)
	}
	module, err := r.rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		compiled.Close(ctx)
		return nil, fmt.Errorf("instantiate module: %w", err)
	}
	entry := module.ExportedFunction(m.Runtime.Entrypoint)
	if entry == nil {
		module.Close(ctx)
		compiled.Close(ctx)
		return nil, fmt.Errorf("entrypoint %q not found", m.Runtime.Entrypoint)
	}
	return &Skill{
		Manifest: m,
		module:   module,
		entry:    entry,
		compiled: compiled,
	}, nil
}

// Invoke executes the skill entrypoint. Currently no parameters are passed.
func (s *Skill) Invoke(ctx context.Context) error {
	if s == nil || s.entry == nil {
		return fmt.Errorf("skill entrypoint not available")
	}
	_, err := s.entry.Call(ctx)
	return err
}

func instantiateHostModule(ctx context.Context, rt wazero.Runtime) error {
	logger := log.New(os.Stdout, "[skill] ", 0)
	builder := rt.NewHostModuleBuilder("env")
	hostLogFn := api.GoModuleFunc(func(_ context.Context, mod api.Module, stack []uint64) {
		if len(stack) < 2 {
			return
		}
		ptr := api.DecodeU32(stack[0])
		length := api.DecodeU32(stack[1])
		if length == 0 {
			return
		}
		mem := mod.Memory()
		if mem == nil {
			logger.Printf("host_log: module has no memory (ptr=%d len=%d)", ptr, length)
			return
		}
		data, ok := mem.Read(ptr, length)
		if !ok {
			logger.Printf("host_log: unable to read memory (ptr=%d len=%d)", ptr, length)
			return
		}
		logger.Print(string(data))
	})
	builder.NewFunctionBuilder().
		WithGoModuleFunction(hostLogFn, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil).
		Export("host_log")
	_, err := builder.Instantiate(ctx)
	return err
}
