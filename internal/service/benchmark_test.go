package service_test

import (
	"testing"

	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/internal/repository/memstore"
	"github.com/rompil2/metrics_aggregator/internal/service"
)

func BenchmarkService_UpdateMetric(b *testing.B) {
	repo := memstore.NewMemStore()
	svc := service.NewMetricService(repo)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			svc.UpdateMetrics(&model.Metrics{
				ID:    "testCounter",
				MType: "counter",
				Delta: func() *int64 { v := int64(1); return &v }(),
			})
		}
	})
}

func BenchmarkService_GetMetric(b *testing.B) {
	repo := memstore.NewMemStore()
	svc := service.NewMetricService(repo)

	// Подготовка данных
	for i := 0; i < 100; i++ {
		svc.UpdateMetrics(&model.Metrics{
			ID:    "testCounter",
			MType: "counter",
			Delta: func() *int64 { v := int64(1); return &v }(),
		})
	}

	for b.Loop() {
		svc.GetMetrics("testCounter")
	}
}
