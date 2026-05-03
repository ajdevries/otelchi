package metric_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/riandyrn/otelchi/metric"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestServerResponseBodySize(t *testing.T) {
	const responseBody = "hello response body"

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	baseCfg := metric.NewBaseConfig(
		"test-server",
		metric.WithMeterProvider(provider),
		metric.WithAttributesFunc(testAttributes),
	)

	router := chi.NewRouter()
	router.Use(metric.NewServerResponseBodySize(baseCfg))
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(responseBody))
		require.NoError(t, err)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))

	m := findMetric(t, collectMetrics(t, reader), "http.server.response.body.size")
	assert.Equal(t, "By", m.Unit)
	assert.Equal(t, "Size of HTTP server response bodies.", m.Description)

	histogram, ok := m.Data.(metricdata.Histogram[int64])
	require.True(t, ok)
	require.Len(t, histogram.DataPoints, 1)

	dp := histogram.DataPoints[0]
	assert.Equal(t, int64(len(responseBody)), dp.Sum)
	assert.Equal(t, uint64(1), dp.Count)
	assertHasAttribute(t, dp.Attributes, attribute.String("test.attr", "value"))
}
