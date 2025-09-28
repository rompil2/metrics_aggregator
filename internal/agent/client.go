package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"maps"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/model"
)

const (
	errChSize       = 1
	updatePath      = "/update/"
	batchUpdatePath = "/updates/" // Добавляем отдельный путь для batch обновлений
)

type Metrics = map[string]any

type HTTPClient struct {
	mu             sync.RWMutex
	lastMetrics    Metrics
	reportInterval time.Duration
	socket         string
	client         *http.Client
	batchEnabled   bool
	hasher         *hash.Hash
}

func NewHTTPClient(reportInterval time.Duration, host string, port uint, batchEnabled bool, hashKey string) *HTTPClient {
	var h *hash.Hash
	if hashKey != "" {
		key := []byte(hashKey)
		*h = hmac.New(sha256.New, key)
	}
	return &HTTPClient{
		reportInterval: reportInterval,
		socket:         fmt.Sprintf("http://%s:%v", host, port),
		client: &http.Client{
			Timeout: 30 * time.Second, // Добавляем таймаут
		},
		batchEnabled: batchEnabled,
		hasher:       h,
	}
}

func (h *HTTPClient) Run(ctx context.Context, ch chan map[string]any) {
	ticker := time.NewTicker(h.reportInterval)
	defer ticker.Stop()

	errCh := make(chan error, errChSize)
	defer close(errCh)

	var wg sync.WaitGroup

	// Обработчик ошибок
	go func() {
		for err := range errCh {
			if err != nil {
				log.Printf("HTTP client error: %v", err)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return

		case m, ok := <-ch:
			if !ok {
				continue
			}
			h.mu.Lock()
			h.lastMetrics = m
			h.mu.Unlock()

		case <-ticker.C:
			h.mu.RLock()
			metrics := make(Metrics, len(h.lastMetrics))
			maps.Copy(metrics, h.lastMetrics)
			h.mu.RUnlock()

			if len(metrics) == 0 {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				var err error
				if h.batchEnabled {
					err = h.SendMetricsBatch(ctx, metrics)
				} else {
					err = h.SendMetrics(ctx, metrics)
				}
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
			}()
		}
	}
}

func (h *HTTPClient) SendMetrics(ctx context.Context, metrics Metrics) error {
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup

	for k, v := range metrics {
		wg.Add(1)

		go func(key string, value any) {
			defer wg.Done()

			mp := HTTPMetricProcessor{}
			metric, err := mp.CreateMetric(key, value)
			if err != nil {
				h.appendError(&mu, &errs, err)
				return
			}

			buf, err := mp.MarshalMetric(metric)
			if err != nil {
				h.appendError(&mu, &errs, err)
				return
			}
			var hashString *string
			if h.hasher != nil {
				(*h.hasher).Reset()
				if _, err := (*h.hasher).Write(buf); err != nil {
					h.appendError(&mu, &errs, fmt.Errorf("hashing data: %w", err))
					return
				}
				*hashString = fmt.Sprintf("%x", (*h.hasher).Sum(nil))
			}

			// Создаем сжатые данные
			compressedData, err := h.compressData(buf)
			if err != nil {
				h.appendError(&mu, &errs, fmt.Errorf("compressing data for %s: %w", key, err))
				return
			}

			// Отправляем запрос
			url := h.socket + updatePath
			err = h.sendRequest(ctx, url, compressedData, hashString)
			if err != nil {
				h.appendError(&mu, &errs, fmt.Errorf("sending metric %s: %w", key, err))
			}
		}(k, v)
	}

	wg.Wait()

	return h.handleErrors(errs)
}

func (h *HTTPClient) SendMetricsBatch(ctx context.Context, metrics Metrics) error {
	// Создаем batch метрик
	metricsBatch, errs := h.createMetricsBatch(metrics)
	if len(errs) > 0 {
		return h.handleErrors(errs)
	}

	if len(metricsBatch) == 0 {
		return nil // Нет валидных метрик для отправки
	}

	// Маршалим в JSON
	jsonData, err := json.Marshal(metricsBatch)
	if err != nil {
		return fmt.Errorf("marshaling metrics batch: %w", err)
	}
	var hashString *string
	if h.hasher != nil {
		(*h.hasher).Reset()
		if _, err := (*h.hasher).Write(jsonData); err != nil {
			return fmt.Errorf("hashing data: %w", err)
		}
		*hashString = fmt.Sprintf("%x", (*h.hasher).Sum(nil))
	}

	// Сжимаем данные
	compressedData, err := h.compressData(jsonData)
	if err != nil {
		return fmt.Errorf("compressing batch data: %w", err)
	}

	// Отправляем запрос
	url := h.socket + batchUpdatePath // Используем отдельный endpoint для batch
	return h.sendRequest(ctx, url, compressedData, hashString)
}

// Вспомогательные методы

func (h *HTTPClient) createMetricsBatch(metrics Metrics) ([]model.Metrics, []error) {
	var errs []error
	metricsBatch := make([]model.Metrics, 0, len(metrics))
	mp := HTTPMetricProcessor{}

	for key, value := range metrics {
		metric, err := mp.CreateMetric(key, value)
		if err != nil {
			errs = append(errs, fmt.Errorf("creating metric %s: %w", key, err))
			continue
		}
		metricsBatch = append(metricsBatch, metric)
	}

	return metricsBatch, errs
}

func (h *HTTPClient) compressData(data []byte) (*bytes.Buffer, error) {
	var zbuf bytes.Buffer
	zw := gzip.NewWriter(&zbuf)

	if _, err := zw.Write(data); err != nil {
		zw.Close()
		return nil, fmt.Errorf("writing to gzip writer: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	return &zbuf, nil
}

func (h *HTTPClient) sendRequest(ctx context.Context, url string, body *bytes.Buffer, hash *string) error {
	const maxAttempts = 3
	retryDelays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	var lastErr error

	for attempt := range maxAttempts {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")
		if hash != nil {
			req.Header.Set("HashSHA256", *hash)
		}

		resp, err := h.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("sending request: %w", err)

			// Check if error is retriable
			if !h.isRetriableError(err) || attempt == maxAttempts-1 {
				return lastErr
			}

			// Wait before retrying
			select {
			case <-time.After(retryDelays[attempt]):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			lastErr = fmt.Errorf("server returned status: %d %s", resp.StatusCode, resp.Status)

			// Check if status code is retriable (5xx errors are usually retriable)
			if !h.isRetriableStatusCode(resp.StatusCode) || attempt == maxAttempts-1 {
				return lastErr
			}

			// Wait before retrying
			select {
			case <-time.After(retryDelays[attempt]):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	}

	return fmt.Errorf("all %d attempts failed, last error: %w", maxAttempts, lastErr)
}

// isRetriableError checks if an error is retriable
func (h *HTTPClient) isRetriableError(err error) bool {
	// Network errors, timeouts, and temporary errors are retriable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	// DNS errors are retriable
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Connection errors are retriable
	if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "connection") {
		return true
	}

	return false
}

// isRetriableStatusCode checks if an HTTP status code is retriable
func (h *HTTPClient) isRetriableStatusCode(statusCode int) bool {
	// 5xx errors are server errors and usually retriable
	// 429 (Too Many Requests) and 408 (Request Timeout) are also retriable
	return statusCode >= 500 && statusCode < 600 ||
		statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusRequestTimeout
}

func (h *HTTPClient) appendError(mu *sync.Mutex, errs *[]error, err error) {
	mu.Lock()
	defer mu.Unlock()
	*errs = append(*errs, err)
}

func (h *HTTPClient) handleErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	// Для множественных ошибок можно использовать errors.Join в Go 1.20+
	if len(errs) == 1 {
		return errs[0]
	}

	return fmt.Errorf("%d errors occurred, first one: %w", len(errs), errs[0])
}

// Остальной код без изменений...
type MetricProcessor interface {
	CreateMetric(key string, value any) (model.Metrics, error)
	MarshalMetric(metric model.Metrics) ([]byte, error)
}

type HTTPMetricProcessor struct{}

func (p *HTTPMetricProcessor) CreateMetric(key string, value any) (model.Metrics, error) {
	var m model.Metrics
	m.ID = key

	switch val := value.(type) {
	case int64:
		m.MType = model.Counter
		m.Delta = &val
	case float64:
		m.MType = model.Gauge
		m.Value = &val
	default:
		return m, fmt.Errorf("unknown metric type for key %s: %T", key, value)
	}

	return m, nil
}

func (p *HTTPMetricProcessor) MarshalMetric(metric model.Metrics) ([]byte, error) {
	return json.Marshal(metric)
}
