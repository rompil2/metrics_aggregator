package handler_test

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/rompil2/metrics_aggregator/internal/handler"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/internal/repository/memstore"
	"github.com/rompil2/metrics_aggregator/internal/service"
)

func BenchmarkHandler_UpdateJSON(b *testing.B) {
	repo := memstore.NewMemStore()
	svc := service.NewMetricService(repo)
	h := handler.NewHandlerMux(svc, nil, "", "", "", nil)

	jsonData := `{"id":"testCounter","type":"counter","delta":1}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/update/", bytes.NewBufferString(jsonData))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateWithJSON(rr, req)
	}
}

func BenchmarkHandler_GetMetricJSON(b *testing.B) {
	repo := memstore.NewMemStore()
	svc := service.NewMetricService(repo)
	h := handler.NewHandlerMux(svc, nil, "", "", "", nil)

	// Подготовка данных
	svc.UpdateMetrics(&model.Metrics{
		ID:    "testCounter",
		MType: "counter",
		Delta: func() *int64 { v := int64(1); return &v }(),
	})

	jsonData := `{"id":"testCounter","type":"counter"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/value/", bytes.NewBufferString(jsonData))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.GetMetricsJSON(rr, req)
	}
}
