package metric_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/riandyrn/otelchi/metric"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestServerRequestDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	baseCfg := metric.NewBaseConfig(
		"test-server",
		metric.WithMeterProvider(provider),
		metric.WithAttributesFunc(testAttributes),
	)

	router := chi.NewRouter()
	router.Use(metric.NewServerRequestDuration(baseCfg))
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))

	m := findMetric(t, collectMetrics(t, reader), "http.server.request.duration")
	assert.Equal(t, "s", m.Unit)
	assert.Equal(t, "Duration of HTTP server requests.", m.Description)

	histogram, ok := m.Data.(metricdata.Histogram[float64])
	require.True(t, ok)
	require.Len(t, histogram.DataPoints, 1)

	dp := histogram.DataPoints[0]
	assert.Greater(t, dp.Sum, 0.0)
	assert.Equal(t, uint64(1), dp.Count)
	assertHasAttribute(t, dp.Attributes, attribute.String("test.attr", "value"))
}
