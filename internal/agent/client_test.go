package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(10*time.Second, "localhost", 8080)

	assert.Equal(t, 10*time.Second, client.reportInterval)
	assert.Equal(t, "http://localhost:8080", client.socket)
	assert.NotNil(t, client.client)
}

func TestHTTPClient_Run_ContextCancellation(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(100*time.Millisecond, "localhost", 8080)
	ctx, cancel := context.WithCancel(context.Background())
	metricsCh := make(chan map[string]interface{})

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

func TestHTTPClient_Run_MetricsProcessing(t *testing.T) {
	t.Parallel()

	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(100*time.Millisecond, "localhost", 8080)
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

	cancel()
	wg.Wait()
}

func TestHTTPClient_SendMetrics_Success(t *testing.T) {
	t.Parallel()

	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(1*time.Second, "localhost", 8080)
	// Подменяем адрес сервера на тестовый
	client.socket = server.URL

	metrics := map[string]interface{}{
		"counter1": int64(42),
		"gauge1":   3.14,
	}

	err := client.SendMetrics(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestHTTPClient_SendMetrics_ErrorCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		serverHandler http.HandlerFunc
		metrics       map[string]interface{}
		expectError   bool
	}{
		{
			name: "ServerError",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			metrics:     map[string]interface{}{"test": int64(1)},
			expectError: true,
		},
		{
			name: "InvalidMetricType",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			metrics:     map[string]interface{}{"invalid": "string"},
			expectError: false, // Неподдерживаемые типы просто игнорируются
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
			metrics:     map[string]interface{}{"test": int64(1)},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.serverHandler)
			defer server.Close()

			client := NewHTTPClient(1*time.Second, "localhost", 8080)
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

func TestHTTPClient_SendMetrics_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Сервер, который долго обрабатывает запрос
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(1*time.Second, "localhost", 8080)
	client.socket = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	metrics := map[string]interface{}{"test": int64(1)}
	err := client.SendMetrics(ctx, metrics)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "Expected context.DeadlineExceeded")
}

func TestHTTPClient_Run_ChannelClosed(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(100*time.Millisecond, "localhost", 8080)
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
