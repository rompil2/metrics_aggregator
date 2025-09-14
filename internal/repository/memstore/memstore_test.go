package memstore

import (
	"testing"

	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemStorage(t *testing.T) {
	storage := NewMemStore()
	assert.NotNil(t, storage)
	assert.NotNil(t, &storage.storage)
}

func TestMemStorage_SetMetrics(t *testing.T) {
	tt := []struct {
		name    string
		id      string
		metric  model.Metrics
		wantErr bool
		errMsg  string
		setup   func(*MemStorage)
	}{
		{
			name: "Set gauge metric",
			id:   "test_gauge",
			metric: model.Metrics{
				ID:    "test_gauge",
				MType: model.Gauge,
				Value: func() *float64 { v := 1.5; return &v }(),
			},
			wantErr: false,
		},
		{
			name: "Set counter metric",
			id:   "test_counter",
			metric: model.Metrics{
				ID:    "test_counter",
				MType: model.Counter,
				Delta: func() *int64 { v := int64(10); return &v }(),
			},
			wantErr: false,
		},
		{
			name: "Set counter without delta - should error",
			id:   "test_counter_err",
			metric: model.Metrics{
				ID:    "test_counter_err",
				MType: model.Counter,
				Delta: nil,
			},
			wantErr: true,
			errMsg:  "counter metric must have Delta value",
		},
		{
			name: "Set gauge without value - should error",
			id:   "test_gauge_err",
			metric: model.Metrics{
				ID:    "test_gauge_err",
				MType: model.Gauge,
				Value: nil,
			},
			wantErr: true,
			errMsg:  "gauge metric must have Value",
		},
		{
			name: "Update existing counter",
			id:   "existing_counter",
			metric: model.Metrics{
				ID:    "existing_counter",
				MType: model.Counter,
				Delta: func() *int64 { v := int64(5); return &v }(),
			},
			setup: func(ms *MemStorage) {
				initial := model.Metrics{
					ID:    "existing_counter",
					MType: model.Counter,
					Delta: func() *int64 { v := int64(10); return &v }(),
				}
				ms.storage.Store("existing_counter", initial)
			},
			wantErr: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			mm := NewMemStore()

			if tc.setup != nil {
				tc.setup(mm)
			}

			err := mm.SetMetrics(tc.id, tc.metric)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify the metric was stored correctly
				got, err := mm.GetMetrics(tc.id)
				assert.NoError(t, err)

				switch tc.metric.MType {
				case model.Gauge:
					assert.Equal(t, *tc.metric.Value, *got.Value)
				case model.Counter:
					if tc.setup != nil {
						// For updated counters, verify the sum
						existing, _ := mm.GetMetrics(tc.id)
						if existing.Delta != nil && tc.metric.Delta != nil {
							assert.Equal(t, int64(15), *existing.Delta) // 10 + 5
						}
					} else {
						assert.Equal(t, *tc.metric.Delta, *got.Delta)
					}
				}
			}
		})
	}
}

func TestMemStorage_GetMetrics(t *testing.T) {
	tt := []struct {
		name    string
		id      string
		setup   func(*MemStorage)
		want    model.Metrics
		wantErr bool
		errMsg  string
	}{
		{
			name: "Get existing metric",
			id:   "test_metric",
			setup: func(ms *MemStorage) {
				metric := model.Metrics{
					ID:    "test_metric",
					MType: model.Gauge,
					Value: func() *float64 { v := 42.0; return &v }(),
				}
				ms.storage.Store("test_metric", metric)
			},
			want: model.Metrics{
				ID:    "test_metric",
				MType: model.Gauge,
				Value: func() *float64 { v := 42.0; return &v }(),
			},
			wantErr: false,
		},
		{
			name:    "Get non-existent metric",
			id:      "non_existent",
			setup:   func(ms *MemStorage) {},
			wantErr: true,
			errMsg:  "metric non_existent not found",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			mm := NewMemStore()
			tc.setup(mm)

			got, err := mm.GetMetrics(tc.id)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				assert.Equal(t, model.Metrics{}, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want.ID, got.ID)
				assert.Equal(t, tc.want.MType, got.MType)

				switch tc.want.MType {
				case model.Gauge:
					assert.Equal(t, *tc.want.Value, *got.Value)
				case model.Counter:
					assert.Equal(t, *tc.want.Delta, *got.Delta)
				}
			}
		})
	}
}

func TestMemStorage_GetAllMetrics(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MemStorage)
		wantLen int
		wantErr bool
	}{
		{
			name:    "Get all metrics from empty storage",
			setup:   func(ms *MemStorage) {},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "Get all metrics with multiple entries",
			setup: func(ms *MemStorage) {
				ms.storage.Store("gauge1", model.Metrics{
					ID:    "gauge1",
					MType: model.Gauge,
					Value: func() *float64 { v := 1.0; return &v }(),
				})
				ms.storage.Store("counter1", model.Metrics{
					ID:    "counter1",
					MType: model.Counter,
					Delta: func() *int64 { v := int64(10); return &v }(),
				})
				ms.storage.Store("gauge2", model.Metrics{
					ID:    "gauge2",
					MType: model.Gauge,
					Value: func() *float64 { v := 2.5; return &v }(),
				})
			},
			wantLen: 3,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mm := NewMemStore()
			tc.setup(mm)

			metrics, err := mm.GetAllMetrics()

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, metrics, tc.wantLen)
			}
		})
	}
}

func TestMemStorage_SetAllMetrics(t *testing.T) {
	t.Run("Set multiple metrics", func(t *testing.T) {
		mm := NewMemStore()

		metrics := []model.Metrics{
			{
				ID:    "gauge1",
				MType: model.Gauge,
				Value: func() *float64 { v := 1.0; return &v }(),
			},
			{
				ID:    "counter1",
				MType: model.Counter,
				Delta: func() *int64 { v := int64(5); return &v }(),
			},
			{
				ID:    "gauge2",
				MType: model.Gauge,
				Value: func() *float64 { v := 2.5; return &v }(),
			},
		}

		err := mm.SetAllMetrics(metrics)
		assert.NoError(t, err)

		// Verify all metrics were stored
		allMetrics, err := mm.GetAllMetrics()
		assert.NoError(t, err)
		assert.Len(t, allMetrics, 3)

		// Verify individual metrics
		gauge1, err := mm.GetMetrics("gauge1")
		assert.NoError(t, err)
		assert.Equal(t, 1.0, *gauge1.Value)

		counter1, err := mm.GetMetrics("counter1")
		assert.NoError(t, err)
		assert.Equal(t, int64(5), *counter1.Delta)
	})

	t.Run("Set metrics with error should stop", func(t *testing.T) {
		mm := NewMemStore()

		metrics := []model.Metrics{
			{
				ID:    "valid_gauge",
				MType: model.Gauge,
				Value: func() *float64 { v := 1.0; return &v }(),
			},
			{
				ID:    "invalid_counter",
				MType: model.Counter,
				Delta: nil, // This should cause error
			},
			{
				ID:    "another_gauge",
				MType: model.Gauge,
				Value: func() *float64 { v := 2.0; return &v }(),
			},
		}

		err := mm.SetAllMetrics(metrics)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set metric invalid_counter")

		// Only the first valid metric should be stored
		allMetrics, err := mm.GetAllMetrics()
		assert.NoError(t, err)
		assert.Len(t, allMetrics, 1) // Only the first valid metric
	})
}

func TestMemStorage_Ping(t *testing.T) {
	t.Run("Ping should return not implemented error", func(t *testing.T) {
		mm := NewMemStore()
		err := mm.Ping()
		assert.Error(t, err)
		assert.Equal(t, "not implemented yet", err.Error())
	})
}

func TestMemStorage_CounterIncrement(t *testing.T) {
	t.Run("Counter should increment properly", func(t *testing.T) {
		mm := NewMemStore()
		counterID := "test_counter"

		// First increment
		err := mm.SetMetrics(counterID, model.Metrics{
			ID:    counterID,
			MType: model.Counter,
			Delta: func() *int64 { v := int64(5); return &v }(),
		})
		require.NoError(t, err)

		// Second increment
		err = mm.SetMetrics(counterID, model.Metrics{
			ID:    counterID,
			MType: model.Counter,
			Delta: func() *int64 { v := int64(3); return &v }(),
		})
		require.NoError(t, err)

		// Verify total
		result, err := mm.GetMetrics(counterID)
		require.NoError(t, err)
		assert.Equal(t, int64(8), *result.Delta)
	})
}
