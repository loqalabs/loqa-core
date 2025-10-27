package natsserver

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/loqalabs/loqa-core/internal/config"
	"github.com/nats-io/nats-server/v2/server"
)

// EmbeddedServer wraps a NATS server instance for zero-dependency deployment.
type EmbeddedServer struct {
	ns  *server.Server
	log *slog.Logger
}

// Start creates and starts an embedded NATS server with JetStream enabled.
func Start(cfg config.BusConfig, log *slog.Logger) (*EmbeddedServer, error) {
	if !cfg.Embedded {
		return nil, nil
	}

	opts := &server.Options{
		Host:      "0.0.0.0",
		Port:      cfg.Port,
		JetStream: true,
		StoreDir:  "./data/nats",
		LogFile:   "",     // Use stdout/stderr
		Trace:     false,
		Debug:     false,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("create embedded NATS server: %w", err)
	}

	// Start the server in a goroutine
	go ns.Start()

	// Wait for server to be ready
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		return nil, fmt.Errorf("embedded NATS server failed to start within 5 seconds")
	}

	log.Info("embedded NATS server started",
		slog.Int("port", cfg.Port),
		slog.String("store_dir", "./data/nats"))

	return &EmbeddedServer{
		ns:  ns,
		log: log,
	}, nil
}

// Shutdown gracefully shuts down the embedded NATS server.
func (e *EmbeddedServer) Shutdown() {
	if e == nil || e.ns == nil {
		return
	}
	e.log.Info("shutting down embedded NATS server")
	e.ns.Shutdown()
	e.ns.WaitForShutdown()
}
