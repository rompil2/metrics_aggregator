package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"sync"
	"time"
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

	errCh := make(chan error, 1) //Buffer is to avoid stacking
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
					default: // adoid blocking if errCh is full
					}
				}
			}()
		}
	}
}

func (h *HTTPClient) SendMetrics(ctx context.Context, metrics Metrics) error {
	const pathTemplate = "/update/%s/%s/%v"
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup

	for k, v := range metrics {
		wg.Add(1)

		go func(key string, value any) {
			defer wg.Done()

			var path string
			switch val := value.(type) {
			case int64:
				path = fmt.Sprintf(pathTemplate, "counter", key, val)
			case float64:
				path = fmt.Sprintf(pathTemplate, "gauge", key, val)
			default:
				return //Unknown metrics type
			}

			url := h.socket + path
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("create request for %s: %w", key, err))
				mu.Unlock()
				return
			}

			resp, err := h.client.Do(req)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("send metric %s: %w", key, err))
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("read response for %s: %w", key, err))
				mu.Unlock()
			}

			if resp.StatusCode >= http.StatusBadRequest {
				mu.Lock()
				errs = append(errs, fmt.Errorf("bad status for %s: %d", key, resp.StatusCode))
				mu.Unlock()
			}
		}(k, v)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("%d errors occurred, first one: %w", len(errs), errs[0])
	}
	return nil
}
