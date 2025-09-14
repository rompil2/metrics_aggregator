package memstore

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rompil2/metrics_aggregator/internal/model"
)

type MemStorage struct {
	storage sync.Map
}

// NewMemStore создает и возвращает новый MemStorage
func NewMemStore() *MemStorage {
	return &MemStorage{
		storage: sync.Map{},
	}
}

// SetMetrics устанавливает или обновляет метрику в хранилище
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

// GetMetrics возвращает метрику по ID
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

// GetAllMetrics возвращает все метрики из хранилища
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

// SetAllMetrics устанавливает несколько метрик
func (mem *MemStorage) SetAllMetrics(metrics []model.Metrics) error {
	for _, metric := range metrics {
		if err := mem.SetMetrics(metric.ID, metric); err != nil {
			return fmt.Errorf("failed to set metric %s: %w", metric.ID, err)
		}
	}
	return nil
}

// Ping проверяет доступность хранилища
func (mem *MemStorage) Ping() error {
	// Для in-memory хранилища всегда доступно
	return errors.New("Not implemented")
}
