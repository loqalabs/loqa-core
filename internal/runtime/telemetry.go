package runtime

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ambiware-labs/loqa-core/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

func setupTelemetry(cfg config.Config, logger *slog.Logger) (func(context.Context) error, http.Handler, error) {
	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.RuntimeName),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	traceProvider, traceShutdown, err := initTracer(ctx, cfg, res, logger)
	if err != nil {
		return nil, nil, err
	}
	otel.SetTracerProvider(traceProvider)

	meterProvider, metricHandler, err := initMetrics(cfg, res, logger)
	if err != nil {
		return nil, nil, err
	}
	otel.SetMeterProvider(meterProvider)

	shutdown := func(ctx context.Context) error {
		var errs []error
		if err := meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := traceShutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}

	return shutdown, metricHandler, nil
}

func initTracer(ctx context.Context, cfg config.Config, res *resource.Resource, logger *slog.Logger) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	if endpoint := strings.TrimSpace(cfg.Telemetry.OTLPEndpoint); endpoint != "" {
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
		if cfg.Telemetry.OTLPInsecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, nil, err
		}
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		logger.Info("telemetry initialized", slog.String("exporter", "otlp"), slog.String("endpoint", endpoint))
		return tp, tp.Shutdown, nil
	}

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	logger.Info("telemetry initialized", slog.String("exporter", "stdout"))
	return tp, tp.Shutdown, nil
}

func initMetrics(cfg config.Config, res *resource.Resource, logger *slog.Logger) (*sdkmetric.MeterProvider, http.Handler, error) {
	promExporter, err := prometheus.New()
	if err != nil {
		logger.Warn("failed to initialize prometheus exporter", slog.String("error", err.Error()))
		meter := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
		return meter, nil, nil
	}
	meter := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExporter),
		sdkmetric.WithResource(res),
	)
	return meter, promhttp.Handler(), nil
}
