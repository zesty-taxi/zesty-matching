package metrics

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	HTTPRequestsTotal   metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram
	ActiveConnections   metric.Int64UpDownCounter
}

func New(serviceName string) (*Metrics, error) {
	meter := otel.Meter(serviceName)

	requestsTotal, err := meter.Int64Counter(
		"http.server.request.total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5,
		),
	)
	if err != nil {
		return nil, err
	}

	activeConns, err := meter.Int64UpDownCounter(
		"http.server.active_connections",
		metric.WithDescription("Number of active HTTP connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		HTTPRequestsTotal:   requestsTotal,
		HTTPRequestDuration: requestDuration,
		ActiveConnections:   activeConns,
	}, nil
}
