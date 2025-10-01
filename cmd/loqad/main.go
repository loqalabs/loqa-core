package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/loqalabs/loqa-core/internal/config"
	"github.com/loqalabs/loqa-core/internal/runtime"
)

var version = "0.1.0-dev"

func main() {
	var (
		configPath  string
		showVersion bool
	)

	flag.StringVar(&configPath, "config", "loqa.yaml", "Path to configuration file")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	rt := runtime.New(cfg, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := rt.Start(ctx); err != nil {
		logger.Error("runtime exited with error", slog.String("error", err.Error()))
		time.Sleep(1 * time.Second)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
