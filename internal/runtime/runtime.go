package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ambiware-labs/loqa-core/internal/bus"
	"github.com/ambiware-labs/loqa-core/internal/capability"
	"github.com/ambiware-labs/loqa-core/internal/config"
)

type Runtime struct {
	cfg         config.Config
	logger      *slog.Logger
	httpServer  *http.Server
	tracerClose func(context.Context) error
	busClient   *bus.Client
	registry    *capability.Registry
	ready       atomic.Bool
	wg          sync.WaitGroup
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

	shutdownTelemetry, err := setupTelemetry(r.cfg, r.logger)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", r.handleHealth)
	mux.HandleFunc("/readyz", r.handleReady)

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
	if r.ready.Load() && r.busClient != nil && r.busClient.Healthy() && (r.registry == nil || r.registry.Healthy()) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("not ready"))
}
