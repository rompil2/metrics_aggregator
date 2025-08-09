package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCollector(t *testing.T) {
	t.Parallel()

	interval := 2 * time.Second
	collector := NewCollector(interval)

	assert.NotNil(t, collector)
	assert.Equal(t, interval, collector.pollInterval)

}

func TestCollector_Run_MetricsDelivery(t *testing.T) {
	t.Parallel()

	collector := NewCollector(100 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	outputCh := make(chan map[string]any, 10)
	go collector.Run(ctx, outputCh)

	// wait for the first acqired metrics
	select {
	case metrics := <-outputCh:
		assert.NotEmpty(t, metrics)
		assert.Contains(t, metrics, "RandomValue")
		assert.Contains(t, metrics, "Alloc")
		assert.Contains(t, metrics, "PollCount")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timeout waiting for metrics")
	}
}

func TestCollector_Poll(t *testing.T) {
	t.Parallel()

	collector := NewCollector(1 * time.Second)
	metrics := collector.Poll()

	assert.NotZero(t, metrics["RandomValue"])
	assert.NotZero(t, metrics["Alloc"])
	assert.NotZero(t, metrics["PollCount"])

	// Check for pollCounter increase
	initialCount := metrics["PollCount"].(int64)
	metrics = collector.Poll()
	assert.Equal(t, initialCount+1, metrics["PollCount"])
}

func TestCollector_Poll_MetricsTypes(t *testing.T) {
	t.Parallel()

	collector := NewCollector(1 * time.Second)
	metrics := collector.Poll()

	assert.IsType(t, float64(0), metrics["RandomValue"])
	assert.IsType(t, float64(0), metrics["Alloc"])
	assert.IsType(t, int64(0), metrics["PollCount"])
}

func TestCollector_Poll_AllMetricsPresent(t *testing.T) {
	t.Parallel()

	collector := NewCollector(1 * time.Second)
	metrics := collector.Poll()

	requiredMetrics := []string{
		"RandomValue", "Alloc", "BuckHashSys", "Frees", "GCCPUFraction",
		"GCSys", "HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects",
		"HeapReleased", "HeapSys", "LastGC", "Lookups", "MCacheInuse",
		"MCacheSys", "MSpanInuse", "MSpanSys", "Mallocs", "NextGC",
		"NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs", "StackInuse",
		"StackSys", "Sys", "TotalAlloc", "PollCount",
	}

	for _, metric := range requiredMetrics {
		assert.Contains(t, metrics, metric, "missing metric: %s", metric)
	}
}

func TestCollector_Run_MultipleMetrics(t *testing.T) {
	t.Parallel()

	collector := NewCollector(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	outputCh := make(chan map[string]interface{}, 10)
	go collector.Run(ctx, outputCh)

	// get a few metrics
	var metrics []map[string]any
	for range 3 {
		select {
		case m := <-outputCh:
			metrics = append(metrics, m)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Timeout waiting for metrics")
		}
	}

	// Check pollCounter increases every time
	assert.Equal(t, int64(1), metrics[0]["PollCount"])
	assert.Equal(t, int64(2), metrics[1]["PollCount"])
	assert.Equal(t, int64(3), metrics[2]["PollCount"])

	// Check RandomValue changres
	assert.NotEqual(t, metrics[0]["RandomValue"], metrics[1]["RandomValue"])
	assert.NotEqual(t, metrics[1]["RandomValue"], metrics[2]["RandomValue"])
}
