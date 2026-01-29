// Package memstore implements an in-memory metrics storage using sync.Map.
// path: internal/repository/memstore
package memstore

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rompil2/metrics_aggregator/internal/model"
)

// MemStorage is a concurrent-safe in-memory metrics store based on sync.Map.
// It supports both Counter and Gauge metric types with atomic updates for counters.
type MemStorage struct {
	storage sync.Map
}

// NewMemStore creates and returns a new instance of MemStorage.
// The store is safe for concurrent use from multiple goroutines.
func NewMemStore() *MemStorage {
	return &MemStorage{
		storage: sync.Map{},
	}
}

// SetMetrics stores or updates a metric by ID in the in-memory store.
// For Counter metrics, it atomically adds the provided Delta value to the existing counter.
// For Gauge metrics, it replaces the current value.
// Returns an error if the metric type is unknown or required fields are missing.
func (mem *MemStorage) SetMetrics(id string, metric model.Metrics) error {
	switch metric.MType {
	case model.Counter:
		if metric.Delta == nil {
			return errors.New("counter metric must have Delta value")
		}
		storedValue, loaded := mem.storage.LoadOrStore(id, metric)
		if loaded {
			atomic.AddInt64(storedValue.(model.Metrics).Delta, *metric.Delta)
		}

	case model.Gauge:
		if metric.Value == nil {
			return errors.New("gauge metric must have Value")
		}
		mem.storage.Store(id, metric)

	default:
		return fmt.Errorf("unknown metric type: %s", metric.MType)
	}

	return nil
}

// GetMetrics retrieves a metric by its ID from the store.
// Returns an error if the metric does not exist or has an invalid type.
func (mem *MemStorage) GetMetrics(ID string) (model.Metrics, error) {
	val, ok := mem.storage.Load(ID)
	if !ok {
		return model.Metrics{}, fmt.Errorf("metric %s not found", ID)
	}

	metric, ok := val.(model.Metrics)
	if !ok {
		return model.Metrics{}, fmt.Errorf("invalid metric type for %s", ID)
	}

	return metric, nil
}

// GetAllMetrics returns a snapshot of all metrics currently stored in memory.
// The returned slice is a copy and safe to modify; however, the operation may be expensive
// under high load as it iterates over the entire map.
func (mem *MemStorage) GetAllMetrics() ([]model.Metrics, error) {
	var result []model.Metrics

	mem.storage.Range(func(key, value any) bool {
		if metric, ok := value.(model.Metrics); ok {
			result = append(result, metric)
		}
		return true
	})

	return result, nil
}

// SetAllMetrics stores multiple metrics in a single batch operation.
// It applies each metric sequentially using SetMetrics and stops on the first error.
// This method is not atomic—partial updates may occur if an error happens midway.
func (mem *MemStorage) SetAllMetrics(metrics []model.Metrics) error {
	for _, metric := range metrics {
		if err := mem.SetMetrics(metric.ID, metric); err != nil {
			return fmt.Errorf("failed to set metric %s: %w", metric.ID, err)
		}
	}
	return nil
}

// Ping checks the availability of the storage backend.
// Currently returns an error because health-check logic is not implemented for in-memory storage.
// For compatibility with the Storage interface.
func (mem *MemStorage) Ping() error {
	// Для in-memory хранилища всегда доступно
	return errors.New("not implemented yet")
}
