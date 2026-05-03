package metric_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestServerActiveRequests(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	baseCfg := metric.NewBaseConfig(
		"test-server",
		metric.WithMeterProvider(provider),
		metric.WithAttributesFunc(testAttributes),
	)

	router := chi.NewRouter()
	router.Use(metric.NewServerActiveRequests(baseCfg))
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		m := findMetric(t, collectMetrics(t, reader), "http.server.active_requests")
		assert.Equal(t, "{request}", m.Unit)
		assert.Equal(t, "Number of active HTTP server requests.", m.Description)

		sum, ok := m.Data.(metricdata.Sum[int64])
		require.True(t, ok)
		require.Len(t, sum.DataPoints, 1)
		assert.Equal(t, int64(1), sum.DataPoints[0].Value)
		assertHasAttribute(t, sum.DataPoints[0].Attributes, attribute.String("test.attr", "value"))

		w.WriteHeader(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))

	m := findMetric(t, collectMetrics(t, reader), "http.server.active_requests")
	sum, ok := m.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, sum.DataPoints, 1)
	assert.Equal(t, int64(0), sum.DataPoints[0].Value)
}

func TestServerRequestBodySize(t *testing.T) {
	const requestBody = "hello request body"

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	baseCfg := metric.NewBaseConfig(
		"test-server",
		metric.WithMeterProvider(provider),
		metric.WithAttributesFunc(testAttributes),
	)

	router := chi.NewRouter()
	router.Use(metric.NewServerRequestBodySize(baseCfg))
	router.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(requestBody))
	router.ServeHTTP(httptest.NewRecorder(), req)

	m := findMetric(t, collectMetrics(t, reader), "http.server.request.body.size")
	assert.Equal(t, "By", m.Unit)
	assert.Equal(t, "Size of HTTP server request bodies.", m.Description)

	histogram, ok := m.Data.(metricdata.Histogram[int64])
	require.True(t, ok)
	require.Len(t, histogram.DataPoints, 1)

	dp := histogram.DataPoints[0]
	assert.Equal(t, int64(len(requestBody)), dp.Sum)
	assert.Equal(t, uint64(1), dp.Count)
	assertHasAttribute(t, dp.Attributes, attribute.String("test.attr", "value"))
}

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

func testAttributes(*http.Request) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("test.attr", "value"),
	}
}

func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()

	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)
	return rm
}

func findMetric(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}
	t.Fatalf("metric %q not found", name)
	return metricdata.Metrics{}
}

func assertHasAttribute(t *testing.T, attrs attribute.Set, want attribute.KeyValue) {
	t.Helper()

	got, ok := attrs.Value(want.Key)
	require.True(t, ok)
	assert.Equal(t, want.Value.AsString(), got.AsString())
}
