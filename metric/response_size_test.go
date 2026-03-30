package metric_test

import (
	"context"
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

func TestResponseSizeBytes(t *testing.T) {
	// setup environment
	responseMsg := "Hello, World!"

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	baseCfg := metric.NewBaseConfig("test-server", metric.WithMeterProvider(provider))
	middleware := metric.NewResponseSizeBytes(baseCfg)

	router := chi.NewRouter()
	router.Use(middleware)
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(responseMsg))
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// read the recorded metrics
	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)
	require.Len(t, rm.ScopeMetrics, 1)

	metrics := rm.ScopeMetrics[0].Metrics
	require.Len(t, metrics, 1)

	hist, ok := metrics[0].Data.(metricdata.Histogram[int64])
	require.True(t, ok)
	require.Len(t, hist.DataPoints, 1)

	dp := hist.DataPoints[0]
	assert.Equal(t, int64(len(responseMsg)), dp.Sum)
	assert.Equal(t, uint64(1), dp.Count)
}

func TestResponseSizeBytesAccumulationIssue(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	cfg := metric.NewBaseConfig(
		"test-server",
		metric.WithMeterProvider(meterProvider),
		metric.WithAttributesFunc(func(r *http.Request) []attribute.KeyValue {
			return []attribute.KeyValue{}
		}),
	)

	// handler writes response in 3 separate Write calls
	// total expected bytes: 5 + 6 + 4 = 15
	chunks := []string{"hello", " world", " bye"}
	totalExpected := int64(0)
	for _, c := range chunks {
		totalExpected += int64(len(c))
	}

	handler := metric.NewResponseSizeBytes(cfg)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, chunk := range chunks {
				n, err := w.Write([]byte(chunk))
				if err != nil {
					t.Errorf("unexpected write error: %v", err)
					return
				}
				if n != len(chunk) {
					t.Errorf("expected to write %d bytes, wrote %d", len(chunk), n)
					return
				}
			}
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// collect metrics
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(req.Context(), &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	histogram := findHistogram(t, rm, "response_size_bytes")
	if histogram == nil {
		t.Fatal("response_size_bytes histogram not found")
	}

	if len(histogram.DataPoints) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(histogram.DataPoints))
	}

	got := histogram.DataPoints[0].Sum
	if got != totalExpected {
		t.Errorf(
			"regression: response size bytes accumulation is broken.\n"+
				"wrote %d bytes across %d Write calls, but metric recorded %d bytes.\n"+
				"this indicates only the first Write call was counted",
			totalExpected,
			len(chunks),
			got,
		)
	}
}

// findHistogram is a helper that looks up a specific Int64Histogram
// by name from collected ResourceMetrics.
func findHistogram(
	t *testing.T,
	rm metricdata.ResourceMetrics,
	name string,
) *metricdata.Histogram[int64] {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			histogram, ok := m.Data.(metricdata.Histogram[int64])
			if !ok {
				t.Fatalf("metric %q is not a Histogram[int64]", name)
			}
			return &histogram
		}
	}
	return nil
}
