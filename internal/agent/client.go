package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/http"
	"sync"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/model"
)

const (
	errChSize            = 1
	lenOfEmptyCollection = 0
	updatePath           = "/update/"
)

type Metrics = map[string]any

type HTTPClient struct {
	mu             sync.RWMutex
	lastMetrics    Metrics
	reportInterval time.Duration
	socket         string
	client         *http.Client
}

func NewHTTPClient(reportInterval time.Duration, host string, port uint) *HTTPClient {
	return &HTTPClient{
		reportInterval: reportInterval,
		socket:         fmt.Sprintf("http://%s:%v", host, port),
		client:         &http.Client{},
	}
}

func (h *HTTPClient) Run(ctx context.Context, ch chan map[string]any) {
	ticker := time.NewTicker(h.reportInterval)
	defer ticker.Stop()

	errCh := make(chan error, errChSize) //Buffer is to avoid stacking
	defer close(errCh)

	var wg sync.WaitGroup

	go func() {
		for err := range errCh { // read errors
			log.Printf("HTTP client error: %v", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			wg.Wait() // wait when all goroutines are completed
			return

		case m, ok := <-ch:
			if !ok {
				continue //the channel is closed
			}
			h.mu.Lock()
			h.lastMetrics = m
			h.mu.Unlock()

		case <-ticker.C: // Time to send metrics to the server
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
				if err := h.SendMetrics(ctx, metrics); err != nil {
					select {
					case errCh <- err:
					default: // avoid blocking if errCh is full
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
			// prepare data
			mp := HTTPMetricProcessor{}
			metric, err := mp.CreateMetric(key, value)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			buf, err := mp.MarshalMetric(metric)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			// compress data
			zbuf := bytes.NewBuffer(nil)
			zb := gzip.NewWriter(zbuf)
			_, err = zb.Write(buf)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("compressing data  for %s: %w", key, err))
				mu.Unlock()
				return
			}
			defer zb.Close()
			err = zb.Flush()
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("flushing data  for %s: %w", key, err))
				mu.Unlock()
				return
			}
			// Send compressed data
			url := h.socket + updatePath
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, zbuf)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("create request for %s: %w", key, err))
				mu.Unlock()
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")

			resp, err := h.client.Do(req)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("send metric %s: %w", key, err))
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= http.StatusBadRequest {
				mu.Lock()
				errs = append(errs, fmt.Errorf("bad status for %s: %d", key, resp.StatusCode))
				mu.Unlock()
				return
			}
		}(k, v)
	}

	wg.Wait()

	if len(errs) > lenOfEmptyCollection {
		return fmt.Errorf("%d errors occurred, first one: %w", len(errs), errs[0])
	}
	return nil
}

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
		return m, fmt.Errorf("unknown metric type for key %s", key)
	}

	return m, nil
}

func (p *HTTPMetricProcessor) MarshalMetric(metric model.Metrics) ([]byte, error) {
	return json.Marshal(metric)
}
