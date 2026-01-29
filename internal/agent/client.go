// Package agent provides a client for sending metrics to a remote server via HTTP.
// path: internal/agent/client.go
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

// Metrics represents a map of metric names to their current values, used for internal aggregation.
type Metrics = map[string]any

// JobMetrics encapsulates compressed metric data and its optional HMAC-SHA256 hash for secure transmission.
type JobMetrics struct {
	compressedData *bytes.Buffer
	Hash           *string
}

// HTTPClient sends collected metrics to a remote server via HTTP,
// supporting both individual and batch updates with optional gzip compression and request signing.
type HTTPClient struct {
	lastMetrics    Metrics
	client         *http.Client
	hasher         *hash.Hash
	socket         string
	reportInterval time.Duration
	rateLimit      uint
	mu             sync.RWMutex
	batchEnabled   bool
}

// NewHTTPClient creates a new HTTPClient configured with the given reporting interval, server address,
// batch mode, HMAC key for request signing, and rate limit (number of concurrent workers).
// If hashKey is non-empty, all requests will include a HashSHA256 header for integrity verification.
func NewHTTPClient(
	reportInterval time.Duration,
	host string,
	port uint,
	batchEnabled bool,
	hashKey string,
	rateLimit uint,
) *HTTPClient {
	if hashKey != "" {
		key := []byte(hashKey)
		hash := hmac.New(sha256.New, key)
		return &HTTPClient{
			reportInterval: reportInterval,
			socket:         fmt.Sprintf("http://%s:%v", host, port),
			client: &http.Client{
				Timeout: 30 * time.Second, // Добавляем таймаут
			},
			batchEnabled: batchEnabled,
			hasher:       &hash,
			mu:           sync.RWMutex{},
			rateLimit:    rateLimit,
		}
	}
	return &HTTPClient{
		reportInterval: reportInterval,
		socket:         fmt.Sprintf("http://%s:%v", host, port),
		client: &http.Client{
			Timeout: 30 * time.Second, // Добавляем таймаут
		},
		batchEnabled: batchEnabled,
		hasher:       nil,
		rateLimit:    rateLimit,
	}
}

// Run starts the metrics reporting loop, sending the latest collected metrics to the server
// at each reporting interval. It reads metrics from the channel and respects context cancellation.
// Errors during transmission are logged but do not stop the loop.
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

			var err error
			if h.rateLimit < 2 {
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

		}
	}
}

// SendMetrics sends metrics individually using multiple concurrent workers (rate-limited).
// Each metric is serialized, optionally hashed, compressed with gzip, and sent to the /update/ endpoint.
// This method is used when rateLimit >= 2.
func (h *HTTPClient) SendMetrics(ctx context.Context, metrics Metrics) error {
	var errs []error
	var mu sync.Mutex

	jobQueue := make(chan JobMetrics)
	mp := HTTPMetricProcessor{}
	url := h.socket + updatePath

	var wg sync.WaitGroup

	for i := 0; i < int(h.rateLimit); i++ {
		wg.Add(1)
		// run a worker
		go func(jobQueue <-chan JobMetrics) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobQueue:
					if !ok {
						return
					}
					// Actually send a request
					err := h.sendRequest(ctx, url, job.compressedData, job.Hash)
					if err != nil {
						h.appendError(&mu, &errs, fmt.Errorf("sending metric error: %w", err))
					}

				}

			}
		}(jobQueue)
	}
producerLoop:
	for key, value := range metrics {
		// Send every metrics  to a worker goroutine
		metric, err := mp.CreateMetric(key, value)
		if err != nil {
			h.appendError(&mu, &errs, err)
			break
		}

		buf, err := mp.MarshalMetric(metric)
		if err != nil {
			h.appendError(&mu, &errs, err)
			break
		}
		hashString := new(string)
		if h.hasher != nil {
			mu.Lock()
			(*h.hasher).Reset()
			if _, err = (*h.hasher).Write(buf); err != nil {
				h.appendError(&mu, &errs, fmt.Errorf("hashing data: %w", err))
				mu.Unlock()
				break
			}
			*hashString = fmt.Sprintf("%x", (*h.hasher).Sum(nil))
			mu.Unlock()
		}

		// Создаем сжатые данные
		compressedData, err := h.compressData(buf)
		if err != nil {
			h.appendError(&mu, &errs, fmt.Errorf("compressing data for %s: %w", key, err))
			break
		}
		job := JobMetrics{
			compressedData: compressedData,
			Hash:           hashString,
		}
		select {
		case <-ctx.Done():
			break producerLoop
		case jobQueue <- job:
		}

	}
	close(jobQueue)
	wg.Wait()

	return h.handleErrors(errs)
}

// SendMetricsBatch sends all metrics in a single JSON array request to the /updates/ endpoint.
// It serializes the entire batch, optionally computes a single hash, compresses with gzip,
// and sends it in one HTTP POST. This method is used when rateLimit < 2.
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
	hashString := new(string)
	if h.hasher != nil {
		(*h.hasher).Reset()
		if _, err = (*h.hasher).Write(jsonData); err != nil {
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

			// Check if status code is retriable
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

	if len(errs) == 1 {
		return errs[0]
	}

	return fmt.Errorf("%d errors occurred, first one: %w", len(errs), errors.Join(errs...))
}

// MetricProcessor defines an interface for converting raw metric values into structured model.Metrics
// and serializing them to JSON bytes.
type MetricProcessor interface {
	CreateMetric(key string, value any) (model.Metrics, error)
	MarshalMetric(metric model.Metrics) ([]byte, error)
}

// HTTPMetricProcessor implements MetricProcessor for HTTP-based metric transmission,
// converting Go types (int64, float64) to counter or gauge metrics and marshaling to JSON.
type HTTPMetricProcessor struct{}

// CreateMetric converts a key-value pair into a typed model.Metrics instance,
// mapping int64 to Counter (Delta) and float64 to Gauge (Value).
// Returns an error for unsupported value types.
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

// MarshalMetric serializes a model.Metrics instance to JSON bytes.
func (p *HTTPMetricProcessor) MarshalMetric(metric model.Metrics) ([]byte, error) {
	return json.Marshal(metric)
}
