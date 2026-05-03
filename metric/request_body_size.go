package metric

import (
	"fmt"
	"io"
	"net/http"

	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	metricNameServerRequestBodySize = "http.server.request.body.size"
	metricUnitServerRequestBodySize = "By"
	metricDescServerRequestBodySize = "Size of HTTP server request bodies."
)

type recordingRequestBody struct {
	body      io.ReadCloser
	readBytes int64
}

func (rb *recordingRequestBody) Read(p []byte) (int, error) {
	n, err := rb.body.Read(p)
	rb.readBytes += int64(n)
	return n, err
}

func (rb *recordingRequestBody) Close() error {
	return rb.body.Close()
}

// NewServerRequestBodySize records the size of HTTP server request bodies in bytes.
func NewServerRequestBodySize(cfg BaseConfig) func(next http.Handler) http.Handler {
	histogram, err := cfg.Meter.Int64Histogram(
		metricNameServerRequestBodySize,
		otelmetric.WithDescription(metricDescServerRequestBodySize),
		otelmetric.WithUnit(metricUnitServerRequestBodySize),
	)
	if err != nil {
		panic(fmt.Sprintf("unable to create %s histogram: %v", metricNameServerRequestBodySize, err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var rb *recordingRequestBody
			if r.Body != nil && r.Body != http.NoBody {
				rb = &recordingRequestBody{body: r.Body}
				r.Body = rb
			}

			next.ServeHTTP(w, r)

			var readBytes int64
			if rb != nil {
				readBytes = rb.readBytes
			}
			histogram.Record(
				r.Context(),
				readBytes,
				otelmetric.WithAttributes(
					cfg.AttributesFunc(r)...,
				),
			)
		})
	}
}
