package repository_test

import (
	"testing"

	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/internal/repository/memstore"
)

func BenchmarkMemStore_Set(b *testing.B) {
	repo := memstore.NewMemStore()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			repo.SetMetrics("testCounter", model.Metrics{
				ID:    "testCounter",
				MType: "counter",
				Delta: func() *int64 { v := int64(1); return &v }(),
			})
		}
	})
}

func BenchmarkMemStore_Get(b *testing.B) {
	repo := memstore.NewMemStore()

	// Подготовка данных
	for i := 0; i < 1000; i++ {
		repo.SetMetrics("counter", model.Metrics{
			ID:    "counter",
			MType: "counter",
			Delta: func() *int64 { v := int64(i); return &v }(),
		})
	}

	for b.Loop() {
		repo.GetMetrics("counter")
	}
}

func BenchmarkMemStore_GetAll(b *testing.B) {
	repo := memstore.NewMemStore()

	// Подготовка 1000 метрик
	for i := range 1000 {
		repo.SetMetrics("counter", model.Metrics{
			ID:    "counter",
			MType: "counter",
			Delta: func() *int64 { v := int64(i); return &v }(),
		})
	}

	for b.Loop() {
		repo.GetAllMetrics()
	}
}
