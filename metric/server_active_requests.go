package metric

import (
	"fmt"
	"net/http"

	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	metricNameServerActiveRequests = "http.server.active_requests"
	metricUnitServerActiveRequests = "{request}"
	metricDescServerActiveRequests = "Number of active HTTP server requests."
)

// NewServerActiveRequests records the number of active HTTP server requests.
func NewServerActiveRequests(cfg BaseConfig) func(next http.Handler) http.Handler {
	counter, err := cfg.Meter.Int64UpDownCounter(
		metricNameServerActiveRequests,
		otelmetric.WithDescription(metricDescServerActiveRequests),
		otelmetric.WithUnit(metricUnitServerActiveRequests),
	)
	if err != nil {
		panic(fmt.Sprintf("unable to create %s counter: %v", metricNameServerActiveRequests, err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attrs := otelmetric.WithAttributes(cfg.AttributesFunc(r)...)

			counter.Add(r.Context(), 1, attrs)
			defer counter.Add(r.Context(), -1, attrs)

			next.ServeHTTP(w, r)
		})
	}
}
