package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loqalabs/loqa-core/internal/bus"
	"github.com/loqalabs/loqa-core/internal/capability"
	"github.com/loqalabs/loqa-core/internal/config"
	"github.com/loqalabs/loqa-core/internal/eventstore"
	"github.com/loqalabs/loqa-core/internal/llm"
	"github.com/loqalabs/loqa-core/internal/router"
	skillservice "github.com/loqalabs/loqa-core/internal/skills/service"
	"github.com/loqalabs/loqa-core/internal/stt"
	"github.com/loqalabs/loqa-core/internal/tts"
)

type Runtime struct {
	cfg           config.Config
	logger        *slog.Logger
	httpServer    *http.Server
	tracerClose   func(context.Context) error
	busClient     *bus.Client
	registry      *capability.Registry
	eventStore    *eventstore.Store
	sttService    *stt.Service
	llmService    *llm.Service
	ttsService    *tts.Service
	skillsService *skillservice.Service
	routerService *router.Service
	metricsServer *http.Server
	ready         atomic.Bool
	wg            sync.WaitGroup
}

func New(cfg config.Config, logger *slog.Logger) *Runtime {
	return &Runtime{
		cfg:    cfg,
		logger: logger,
	}
}

func (r *Runtime) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	shutdownTelemetry, metricsHandler, err := setupTelemetry(r.cfg, r.logger)
	if err != nil {
		return fmt.Errorf("failed to setup telemetry: %w", err)
	}
	r.tracerClose = shutdownTelemetry

	busClient, err := bus.Connect(ctx, r.cfg.Bus, r.logger)
	if err != nil {
		return fmt.Errorf("failed to connect to message bus: %w", err)
	}
	r.busClient = busClient
	registry, err := capability.NewRegistry(ctx, r.cfg.Node, r.busClient, r.logger)
	if err != nil {
		return fmt.Errorf("failed to start capability registry: %w", err)
	}
	r.registry = registry
	eventStore, err := eventstore.Open(ctx, r.cfg.EventStore, r.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize event store: %w", err)
	}
	r.eventStore = eventStore

	if r.cfg.Skills.Enabled {
		svc, err := skillservice.New(ctx, r.cfg.Skills, r.busClient, r.eventStore, r.logger)
		if err != nil {
			return fmt.Errorf("start skills service: %w", err)
		}
		r.skillsService = svc
	}

	if r.cfg.STT.Enabled {
		var recognizer stt.Recognizer
		var err error
		switch r.cfg.STT.Mode {
		case "exec":
			recognizer, err = stt.NewExecRecognizer(r.cfg.STT)
			if err != nil {
				return fmt.Errorf("failed to configure exec recognizer: %w", err)
			}
		case "mock", "":
			recognizer = stt.NewMockRecognizer()
		default:
			return fmt.Errorf("unsupported STT mode %q", r.cfg.STT.Mode)
		}
		service := stt.NewService(ctx, r.cfg.STT, r.busClient, recognizer)
		if err := service.Start(); err != nil {
			return fmt.Errorf("start STT service: %w", err)
		}
		r.sttService = service
	}

	if r.cfg.LLM.Enabled {
		var generator llm.Generator
		var err error
		switch r.cfg.LLM.Mode {
		case "ollama":
			generator = llm.NewOllamaGenerator(r.cfg.LLM.Endpoint, r.cfg.LLM.ModelFast, r.cfg.LLM.ModelBalanced)
		case "exec":
			generator, err = llm.NewExecGenerator(r.cfg.LLM.Command)
		case "mock", "":
			generator = llm.NewMockGenerator()
		default:
			return fmt.Errorf("unsupported LLM mode %q", r.cfg.LLM.Mode)
		}
		if err != nil {
			return fmt.Errorf("failed to configure LLM generator: %w", err)
		}
		service := llm.NewService(ctx, r.cfg.LLM, r.busClient, generator, r.logger)
		if err := service.Start(); err != nil {
			return fmt.Errorf("start LLM service: %w", err)
		}
		r.llmService = service
	}

	if r.cfg.TTS.Enabled {
		var synth tts.Synthesizer
		var err error
		switch r.cfg.TTS.Mode {
		case "exec":
			synth, err = tts.NewExecSynth(r.cfg.TTS.Command, r.cfg.TTS.SampleRate, r.cfg.TTS.Channels)
		case "mock", "":
			synth = tts.NewMockSynth(r.cfg.TTS.SampleRate, r.cfg.TTS.Channels)
		default:
			return fmt.Errorf("unsupported TTS mode %q", r.cfg.TTS.Mode)
		}
		if err != nil {
			return fmt.Errorf("failed to configure TTS synthesizer: %w", err)
		}
		service := tts.NewService(ctx, r.cfg.TTS, r.busClient, synth, r.logger)
		if err := service.Start(); err != nil {
			return fmt.Errorf("start TTS service: %w", err)
		}
		r.ttsService = service
	}

	if r.cfg.Router.Enabled {
		service := router.NewService(ctx, r.cfg.Router, r.busClient, r.logger)
		if err := service.Start(); err != nil {
			return fmt.Errorf("start router service: %w", err)
		}
		r.routerService = service
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", r.handleHealth)
	mux.HandleFunc("/readyz", r.handleReady)
	if metricsHandler != nil && r.cfg.Telemetry.PrometheusBind != "" {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", metricsHandler)
		r.metricsServer = &http.Server{
			Addr:              r.cfg.Telemetry.PrometheusBind,
			Handler:           metricsMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			if err := r.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				r.logger.Error("metrics server failed", slog.String("error", err.Error()))
			}
		}()
		r.logger.Info("metrics endpoint ready", slog.String("addr", r.cfg.Telemetry.PrometheusBind))
	}

	addr := fmt.Sprintf("%s:%d", r.cfg.HTTP.Bind, r.cfg.HTTP.Port)
	r.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			r.logger.Error("http server failed", slog.String("error", err.Error()))
		}
	}()

	r.ready.Store(true)
	r.logger.Info("runtime started", slog.String("addr", addr))

	<-ctx.Done()
	r.logger.Info("runtime stopping")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := r.httpServer.Shutdown(shutdownCtx); err != nil {
		r.logger.Error("http shutdown error", slog.String("error", err.Error()))
	}
	if r.registry != nil {
		r.registry.Close()
	}
	if r.sttService != nil {
		r.sttService.Close()
	}
	if r.llmService != nil {
		r.llmService.Close()
	}
	if r.ttsService != nil {
		r.ttsService.Close()
	}
	if r.routerService != nil {
		r.routerService.Close()
	}
	if r.skillsService != nil {
		r.skillsService.Close()
	}
	if r.metricsServer != nil {
		if err := r.metricsServer.Shutdown(shutdownCtx); err != nil {
			r.logger.Warn("metrics server shutdown error", slog.String("error", err.Error()))
		}
	}
	if r.eventStore != nil {
		if err := r.eventStore.Close(); err != nil {
			r.logger.Warn("event store close error", slog.String("error", err.Error()))
		}
	}
	if r.busClient != nil {
		r.busClient.Close()
	}
	r.wg.Wait()

	if r.tracerClose != nil {
		if err := r.tracerClose(shutdownCtx); err != nil {
			r.logger.Error("telemetry shutdown error", slog.String("error", err.Error()))
		}
	}

	return nil
}

func (r *Runtime) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (r *Runtime) handleReady(w http.ResponseWriter, _ *http.Request) {
	sttHealthy := r.sttService == nil || r.sttService.Healthy()
	llmHealthy := r.llmService == nil || r.llmService.Healthy()
	ttsHealthy := r.ttsService == nil || r.ttsService.Healthy()
	routerHealthy := r.routerService == nil || r.routerService.Healthy()
	skillsHealthy := r.skillsService == nil || r.skillsService.Healthy()
	if r.ready.Load() && r.busClient != nil && r.busClient.Healthy() && (r.registry == nil || r.registry.Healthy()) && sttHealthy && llmHealthy && ttsHealthy && routerHealthy && skillsHealthy {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("not ready"))
}
