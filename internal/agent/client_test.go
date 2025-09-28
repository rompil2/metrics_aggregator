package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient(t *testing.T) {
	t.Parallel()

	// Тестируем с batchEnabled = false
	client := NewHTTPClient(10*time.Second, "localhost", 8080, false, "")

	assert.Equal(t, 10*time.Second, client.reportInterval)
	assert.Equal(t, "http://localhost:8080", client.socket)
	assert.NotNil(t, client.client)
	assert.False(t, client.batchEnabled)
	assert.Equal(t, 30*time.Second, client.client.Timeout)

	// Тестируем с batchEnabled = true
	clientWithBatch := NewHTTPClient(5*time.Second, "example.com", 9090, true, "")
	assert.True(t, clientWithBatch.batchEnabled)
}

func TestHTTPClient_Run_ContextCancellation(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(100*time.Millisecond, "localhost", 8080, false, "")
	ctx, cancel := context.WithCancel(context.Background())
	metricsCh := make(chan map[string]any)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		client.Run(ctx, metricsCh)
	}()

	// Отправляем тестовые метрики
	metricsCh <- map[string]interface{}{"test": int64(42)}

	// Даем время на обработку
	time.Sleep(50 * time.Millisecond)

	// Отменяем контекст
	cancel()

	// Ждем завершения
	wg.Wait()
}

func TestHTTPClient_Run_MetricsProcessing_Individual(t *testing.T) {
	t.Parallel()

	// Создаем тестовый сервер для индивидуальных запросов
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(100*time.Millisecond, "localhost", 8080, false, "")
	// Подменяем адрес сервера на тестовый
	client.socket = server.URL

	metricsCh := make(chan map[string]interface{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		client.Run(ctx, metricsCh)
	}()

	// Отправляем тестовые метрики
	testMetrics := map[string]interface{}{
		"counter1": int64(42),
		"gauge1":   3.14,
	}
	metricsCh <- testMetrics

	// Даем время на обработку
	time.Sleep(150 * time.Millisecond)

	// Проверяем, что метрики были сохранены
	client.mu.RLock()
	assert.Equal(t, testMetrics, client.lastMetrics)
	client.mu.RUnlock()

	// Должно быть 2 запроса (по одному на каждую метрику)
	assert.GreaterOrEqual(t, requestCount, 2)

	cancel()
	wg.Wait()
}

func TestHTTPClient_Run_MetricsProcessing_Batch(t *testing.T) {
	t.Parallel()

	// Создаем тестовый сервер для batch запросов
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		assert.Equal(t, "/updates/", r.URL.Path) // Проверяем batch endpoint
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(100*time.Millisecond, "localhost", 8080, true, "")
	// Подменяем адрес сервера на тестовый
	client.socket = server.URL

	metricsCh := make(chan map[string]interface{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		client.Run(ctx, metricsCh)
	}()

	// Отправляем тестовые метрики
	testMetrics := map[string]interface{}{
		"counter1": int64(42),
		"gauge1":   3.14,
		"counter2": int64(100),
	}
	metricsCh <- testMetrics

	// Даем время на обработку
	time.Sleep(150 * time.Millisecond)

	// Проверяем, что метрики были сохранены
	client.mu.RLock()
	assert.Equal(t, testMetrics, client.lastMetrics)
	client.mu.RUnlock()

	// Должен быть 1 batch запрос
	assert.Equal(t, 1, requestCount)

	cancel()
	wg.Wait()
}

func TestHTTPClient_SendMetrics_Success(t *testing.T) {
	t.Parallel()

	// Создаем тестовый сервер
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/update/", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(1*time.Second, "localhost", 8080, false, "")
	// Подменяем адрес сервера на тестовый
	client.socket = server.URL

	metrics := map[string]interface{}{
		"counter1": int64(42),
		"gauge1":   3.14,
	}

	err := client.SendMetrics(context.Background(), metrics)
	assert.NoError(t, err)
	// Должно быть 2 запроса (по одному на каждую метрику)
	assert.Equal(t, 2, requestCount)
}

func TestHTTPClient_SendMetricsBatch_Success(t *testing.T) {
	t.Parallel()

	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/updates/", r.URL.Path)
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))

		// Можно распаковать и проверить данные
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(1*time.Second, "localhost", 8080, true, "")
	client.socket = server.URL

	metrics := map[string]interface{}{
		"counter1": int64(42),
		"gauge1":   3.14,
		"counter2": int64(100),
	}

	err := client.SendMetricsBatch(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestHTTPClient_SendMetricsBatch_WithInvalidMetrics(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(1*time.Second, "localhost", 8080, true, "")
	client.socket = server.URL

	// Смесь валидных и невалидных метрик
	metrics := map[string]interface{}{
		"counter1": int64(42),      // валидная
		"invalid":  "string_value", // невалидная
		"gauge1":   3.14,           // валидная
	}

	err := client.SendMetricsBatch(context.Background(), metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metric type")
}

func TestHTTPClient_SendMetricsBatch_EmptyMetrics(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Should not make request for empty metrics")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(1*time.Second, "localhost", 8080, true, "")
	client.socket = server.URL

	// Пустые метрики
	metrics := map[string]interface{}{}

	err := client.SendMetricsBatch(context.Background(), metrics)
	assert.NoError(t, err) // Не должно быть ошибки для пустых метрик
}

func TestHTTPClient_SendMetrics_ErrorCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		serverHandler http.HandlerFunc
		metrics       map[string]any
		expectError   bool
	}{
		{
			name: "ServerError",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			metrics:     map[string]any{"test": int64(1)},
			expectError: true,
		},
		{
			name: "InvalidMetricType",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			metrics:     map[string]any{"invalid": "string"},
			expectError: true,
		},
		{
			name: "RequestError",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// Сервер закрывает соединение сразу
				hj, ok := w.(http.Hijacker)
				require.True(t, ok)
				conn, _, _ := hj.Hijack()
				conn.Close()
			},
			metrics:     map[string]any{"test": int64(1)},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.serverHandler)
			defer server.Close()

			client := NewHTTPClient(1*time.Second, "localhost", 8080, false, "")
			client.socket = server.URL

			err := client.SendMetrics(context.Background(), tc.metrics)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_SendMetricsBatch_ErrorCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		serverHandler http.HandlerFunc
		metrics       map[string]any
		expectError   bool
	}{
		{
			name: "ServerError",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			metrics:     map[string]any{"test": int64(1)},
			expectError: true,
		},
		{
			name: "RequestError",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				hj, ok := w.(http.Hijacker)
				require.True(t, ok)
				conn, _, _ := hj.Hijack()
				conn.Close()
			},
			metrics:     map[string]any{"test": int64(1)},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.serverHandler)
			defer server.Close()

			client := NewHTTPClient(1*time.Second, "localhost", 8080, true, "")
			client.socket = server.URL

			err := client.SendMetricsBatch(context.Background(), tc.metrics)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_SendMetrics_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Сервер, который долго обрабатывает запрос
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(1*time.Second, "localhost", 8080, false, "")
	client.socket = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	metrics := map[string]interface{}{"test": int64(1)}
	err := client.SendMetrics(ctx, metrics)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled), "Expected context error")
}

func TestHTTPClient_SendMetricsBatch_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Сервер, который долго обрабатывает запрос
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(1*time.Second, "localhost", 8080, true, "")
	client.socket = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	metrics := map[string]interface{}{"test": int64(1)}
	err := client.SendMetricsBatch(ctx, metrics)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled), "Expected context error")
}

func TestHTTPClient_Run_ChannelClosed(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(100*time.Millisecond, "localhost", 8080, false, "")
	metricsCh := make(chan map[string]interface{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		client.Run(ctx, metricsCh)
	}()

	// Закрываем канал
	close(metricsCh)

	// Даем время на обработку
	time.Sleep(150 * time.Millisecond)

	// Проверяем, что Run не завершился с ошибкой
	cancel()
	wg.Wait()
}

func TestHTTPMetricProcessor_CreateMetric(t *testing.T) {
	type args struct {
		key   string
		value any
	}
	tests := []struct {
		name    string
		args    args
		want    model.Metrics
		wantErr bool
		errMsg  string
	}{
		{
			name: "Positive test. new counter",
			args: args{
				key:   "counter",
				value: int64(42),
			},
			want: model.Metrics{
				ID:    "counter",
				MType: model.Counter,
				Delta: func() *int64 { v := int64(42); return &v }(),
			},
			wantErr: false,
		},
		{
			name: "Positive test. new gauge",
			args: args{
				key:   "gauge",
				value: float64(3.14),
			},
			want: model.Metrics{
				ID:    "gauge",
				MType: model.Gauge,
				Value: func() *float64 { v := float64(3.14); return &v }(),
			},
			wantErr: false,
		},
		{
			name: "Negative test. unknown type string",
			args: args{
				key:   "invalid",
				value: "string_value",
			},
			want: model.Metrics{
				ID: "invalid",
			},
			wantErr: true,
			errMsg:  "unknown metric type for key invalid: string",
		},
	}

	p := &HTTPMetricProcessor{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.CreateMetric(tt.args.key, tt.args.value)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want.ID, got.ID)
			assert.Equal(t, tt.want.MType, got.MType)

			if tt.want.Delta != nil {
				assert.NotNil(t, got.Delta)
				assert.Equal(t, *tt.want.Delta, *got.Delta)
			}

			if tt.want.Value != nil {
				assert.NotNil(t, got.Value)
				assert.Equal(t, *tt.want.Value, *got.Value)
			}
		})
	}
}

func TestHTTPClient_compressData(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(1*time.Second, "localhost", 8080, false, "")

	testData := []byte("test data for compression")
	compressed, err := client.compressData(testData)

	assert.NoError(t, err)
	assert.NotNil(t, compressed)
	assert.True(t, compressed.Len() > 0)
}

func TestHTTPClient_handleErrors(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(1*time.Second, "localhost", 8080, false, "")

	// Нет ошибок
	err := client.handleErrors(nil)
	assert.NoError(t, err)

	// Пустой массив ошибок
	err = client.handleErrors([]error{})
	assert.NoError(t, err)

	// Одна ошибка
	singleErr := errors.New("single error")
	err = client.handleErrors([]error{singleErr})
	assert.EqualError(t, err, "single error")

	// Несколько ошибок
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err = client.handleErrors([]error{err1, err2})
	assert.Contains(t, err.Error(), "2 errors occurred")
	assert.Contains(t, err.Error(), "error 1")
}
