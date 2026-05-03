package metric

import (
	"fmt"
	"net/http"

	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	metricNameServerResponseBodySize = "http.server.response.body.size"
	metricUnitServerResponseBodySize = "By"
	metricDescServerResponseBodySize = "Size of HTTP server response bodies."
)

// NewServerResponseBodySize records the size of HTTP server response bodies in bytes.
func NewServerResponseBodySize(cfg BaseConfig) func(next http.Handler) http.Handler {
	histogram, err := cfg.Meter.Int64Histogram(
		metricNameServerResponseBodySize,
		otelmetric.WithDescription(metricDescServerResponseBodySize),
		otelmetric.WithUnit(metricUnitServerResponseBodySize),
	)
	if err != nil {
		panic(fmt.Sprintf("unable to create %s histogram: %v", metricNameServerResponseBodySize, err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rrw := getRRW(w)
			defer putRRW(rrw)

			next.ServeHTTP(rrw.writer, r)

			histogram.Record(
				r.Context(),
				rrw.writtenBytes,
				otelmetric.WithAttributes(
					cfg.AttributesFunc(r)...,
				),
			)
		})
	}
}
