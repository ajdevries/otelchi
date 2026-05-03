package metric

import (
	"fmt"
	"net/http"

	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	metricNameResponseSizeBytes = "response_size_bytes"
	metricUnitResponseSizeBytes = "By"
	metricDescResponseSizeBytes = "Measures the size of the response in bytes."

	metricNameServerResponseBodySize = "http.server.response.body.size"
	metricUnitServerResponseBodySize = "By"
	metricDescServerResponseBodySize = "Size of HTTP server response bodies."
)

// Deprecated: use NewServerResponseBodySize instead.
func NewResponseSizeBytes(cfg BaseConfig) func(next http.Handler) http.Handler {
	// init metric, here we are using histogram for capturing response size
	histogram, err := cfg.Meter.Int64Histogram(
		metricNameResponseSizeBytes,
		otelmetric.WithDescription(metricDescResponseSizeBytes),
		otelmetric.WithUnit(metricUnitResponseSizeBytes),
	)
	if err != nil {
		panic(fmt.Sprintf("unable to create %s histogram: %v", metricNameResponseSizeBytes, err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// get recording response writer
			rrw := getRRW(w)
			defer putRRW(rrw)

			// execute next http handler
			next.ServeHTTP(rrw.writer, r)

			// record the response size
			histogram.Record(
				r.Context(),
				int64(rrw.writtenBytes),
				otelmetric.WithAttributes(
					cfg.AttributesFunc(r)...,
				),
			)
		})
	}
}

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
