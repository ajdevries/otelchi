package metric

import (
	"fmt"
	"net/http"
	"time"

	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	metricNameServerRequestDuration = "http.server.request.duration"
	metricUnitServerRequestDuration = "s"
	metricDescServerRequestDuration = "Duration of HTTP server requests."
)

// NewServerRequestDuration records the duration of HTTP server requests in seconds.
func NewServerRequestDuration(cfg BaseConfig) func(next http.Handler) http.Handler {
	histogram, err := cfg.Meter.Float64Histogram(
		metricNameServerRequestDuration,
		otelmetric.WithDescription(metricDescServerRequestDuration),
		otelmetric.WithUnit(metricUnitServerRequestDuration),
		otelmetric.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.075, 0.1,
			0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10,
		),
	)
	if err != nil {
		panic(fmt.Sprintf("unable to create %s histogram: %v", metricNameServerRequestDuration, err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			next.ServeHTTP(w, r)

			duration := time.Since(startTime)
			histogram.Record(
				r.Context(),
				float64(duration)/float64(time.Second),
				otelmetric.WithAttributes(
					cfg.AttributesFunc(r)...,
				),
			)
		})
	}
}
