package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/ambiware-labs/loqa-core/internal/skills/manifest"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime wraps a wazero runtime for executing skill modules.
type Runtime struct {
	rt   wazero.Runtime
	host HostBindings
}

// New creates a new skill runtime using wazero.
func New(ctx context.Context, host HostBindings) (*Runtime, error) {
	rt := wazero.NewRuntime(ctx)
	host = host.ensure()
	if err := instantiateHostModule(ctx, rt, host); err != nil {
		return nil, fmt.Errorf("instantiate host module: %w", err)
	}
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}
	return &Runtime{rt: rt, host: host}, nil
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
func (r *Runtime) Load(ctx context.Context, m manifest.Manifest, env map[string]string) (*Skill, error) {
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
	moduleConfig := wazero.NewModuleConfig()
	for k, v := range env {
		moduleConfig = moduleConfig.WithEnv(k, v)
	}
	module, err := r.rt.InstantiateModule(ctx, compiled, moduleConfig)
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

func instantiateHostModule(ctx context.Context, rt wazero.Runtime, host HostBindings) error {
	logger := host.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	printfLogger := log.New(os.Stdout, "[skill] ", 0)
	binding := host.ensure()

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
			printfLogger.Printf("host_log: module has no memory (ptr=%d len=%d)", ptr, length)
			return
		}
		data, ok := mem.Read(ptr, length)
		if !ok {
			printfLogger.Printf("host_log: unable to read memory (ptr=%d len=%d)", ptr, length)
			return
		}
		msg := string(data)
		logger.Info("skill log", slog.String("message", msg))
		if binding.RecordAudit != nil {
			binding.RecordAudit(AuditEvent{Type: "skill.log", Data: map[string]any{"message": msg}})
		}
	})
	builder.NewFunctionBuilder().
		WithGoModuleFunction(hostLogFn, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil).
		WithName("host_log").
		Export("host_log")

	hostPublishFn := api.GoModuleFunc(func(_ context.Context, mod api.Module, stack []uint64) {
		if len(stack) < 4 {
			return
		}
		subjectPtr := api.DecodeU32(stack[0])
		subjectLen := api.DecodeU32(stack[1])
		payloadPtr := api.DecodeU32(stack[2])
		payloadLen := api.DecodeU32(stack[3])

		mem := mod.Memory()
		if mem == nil {
			stack[0] = api.EncodeI32(int32(PublishErrRuntime))
			return
		}
		subjectBytes, ok := mem.Read(subjectPtr, subjectLen)
		if !ok {
			stack[0] = api.EncodeI32(int32(PublishErrRuntime))
			return
		}
		subject := string(subjectBytes)
		if binding.AllowPublish != nil {
			if err := binding.AllowPublish(subject); err != nil {
				stack[0] = api.EncodeI32(int32(PublishErrNotAllowed))
				logger.Warn("skill publish blocked", slog.String("subject", subject), slog.String("error", err.Error()))
				return
			}
		}
		var payload []byte
		if payloadLen > 0 {
			if data, ok := mem.Read(payloadPtr, payloadLen); ok {
				payload = append([]byte(nil), data...)
			} else {
				stack[0] = api.EncodeI32(int32(PublishErrRuntime))
				return
			}
		}
		if binding.Publish == nil {
			stack[0] = api.EncodeI32(int32(PublishErrRuntime))
			return
		}
		if err := binding.Publish(subject, payload); err != nil {
			stack[0] = api.EncodeI32(int32(PublishErrRuntime))
			logger.Error("skill publish failed", slog.String("subject", subject), slog.String("error", err.Error()))
			return
		}
		if binding.RecordAudit != nil {
			binding.RecordAudit(AuditEvent{Type: "skill.publish", Data: map[string]any{
				"subject":       subject,
				"payload_bytes": payloadLen,
			}})
		}
		stack[0] = api.EncodeI32(int32(PublishOK))
	})
	builder.NewFunctionBuilder().
		WithGoModuleFunction(hostPublishFn, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
		WithName("host_publish").
		WithResultNames("code").
		Export("host_publish")

	_, err := builder.Instantiate(ctx)
	return err
}

const (
	PublishOK            = 0
	PublishErrNotAllowed = 1
	PublishErrRuntime    = 2
)

type HostBindings struct {
	Logger       *slog.Logger
	AllowPublish func(subject string) error
	Publish      func(subject string, payload []byte) error
	RecordAudit  func(event AuditEvent)
}

func (h HostBindings) ensure() HostBindings {
	if h.AllowPublish == nil {
		h.AllowPublish = func(string) error { return errors.New("publish disallowed") }
	}
	if h.Publish == nil {
		h.Publish = func(string, []byte) error { return errors.New("publish unsupported") }
	}
	return h
}

type AuditEvent struct {
	Type string
	Data map[string]any
}
