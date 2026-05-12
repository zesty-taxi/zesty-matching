package otel

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string // otel-collector:4318
}
type Providers struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
}

func Init(ctx context.Context, cfg Config) (*Providers, func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentName(cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	tp, err := newTracerProvider(ctx, res, cfg.OTLPEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create tracer provider: %w", err)
	}

	mp, err := newMeterProvider(ctx, res)
	if err != nil {
		if err := tp.Shutdown(ctx); err != nil {
			log.Warn().Err(err).Msg("failed to shutdown meter provider")
		}
		return nil, nil, fmt.Errorf("failed to create meter provider: %w", err)
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	otel.SetMeterProvider(mp)

	shutdown := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("tracer shutdown: %w", err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("meter shutdown: %w", err)
		}
		return nil
	}

	return &Providers{
		TracerProvider: tp,
		MeterProvider:  mp,
	}, shutdown, nil
}

func newTracerProvider(ctx context.Context, res *resource.Resource, endpoint string) (*trace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(), // TO DO with TLS
	)
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()), // TO DO Ratio Based
		// 		trace.WithSampler(
		//     trace.ParentBased(
		//         trace.TraceIDRatioBased(0.5),
		//     ),
		// )
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	return tp, nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource) (*metric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)

	return mp, nil
}
