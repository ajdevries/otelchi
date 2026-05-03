package metric

import (
	"fmt"
	"net/http"

	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	metricNameRequestInFlight = "requests_inflight"
	metricUnitRequestInFlight = "{count}"
	metricDescRequestInFlight = "Measures the number of requests currently being processed by the server."

	metricNameServerActiveRequests = "http.server.active_requests"
	metricUnitServerActiveRequests = "{request}"
	metricDescServerActiveRequests = "Number of active HTTP server requests."
)

// [RequestInFlight] is a metrics recorder for recording the number of requests in flight.
//
// Deprecated: use NewServerActiveRequests instead.
func NewRequestInFlight(cfg BaseConfig) func(next http.Handler) http.Handler {
	// init metric, here we are using counter for capturing request in flight
	counter, err := cfg.Meter.Int64UpDownCounter(
		metricNameRequestInFlight,
		otelmetric.WithDescription(metricDescRequestInFlight),
		otelmetric.WithUnit(metricUnitRequestInFlight),
	)
	if err != nil {
		panic(fmt.Sprintf("unable to create %s counter: %v", metricNameRequestInFlight, err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// define metric attributes
			attrs := otelmetric.WithAttributes(cfg.AttributesFunc(r)...)

			// increase the number of requests in flight
			counter.Add(r.Context(), 1, attrs)

			// execute next http handler
			next.ServeHTTP(w, r)

			// decrease the number of requests in flight
			counter.Add(r.Context(), -1, attrs)
		})
	}
}

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
