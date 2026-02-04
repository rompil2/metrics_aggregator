// Package filestore provides a file-based storage implementation for metrics data.
// path: internal/repository/filestore
package filestore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/config"
	"github.com/rompil2/metrics_aggregator/internal/logger"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/internal/repository"
)

// Repo is an alias for the repository.Repo interface, used to embed storage operations.
type Repo = repository.Repo

// Store implements a file-backed metrics repository that periodically persists data to disk.
// It supports both interval-based and event-driven saving strategies, and ensures safe concurrent access.
type Store struct {
	Repo
	cancel        context.CancelFunc
	synchroCh     chan struct{}
	storeFilePath string
	wg            sync.WaitGroup
	interval      time.Duration
	mu            sync.RWMutex
}

var log = logger.Get()

// NewFileStore creates a new file-backed metrics store with the given repository and configuration.
// If Restore is enabled in the config, it attempts to load existing metrics from the storage file.
// The store starts a background goroutine for periodic or synchronous saving based on the interval setting.
func NewFileStore(repo Repo, cfg config.StoreConfig) (*Store, error) {
	st := &Store{
		Repo:          repo,
		interval:      cfg.StoreInterval,
		storeFilePath: cfg.FileStoragePath,
		synchroCh:     make(chan struct{}),
	}

	if cfg.Restore {
		if err := st.Restore(); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	st.cancel = cancel

	st.wg.Add(1)
	go st.Save(ctx)

	return st, nil
}

// SetMetrics stores a metric by ID and triggers an asynchronous save if the store is configured for event-driven persistence.
// It is safe for concurrent use.
func (st *Store) SetMetrics(ID string, value model.Metrics) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	err := st.Repo.SetMetrics(ID, value)
	if err != nil {
		return err
	}

	select {
	case st.synchroCh <- struct{}{}:
	default:
		// to avoid blocking
	}

	return nil
}

// Restore loads all metrics from the configured file path into the underlying repository.
// It expects a JSON array of metrics. If the file does not exist, it returns nil (no error).
// This method is typically called during initialization when Restore is enabled.
func (st *Store) Restore() error {
	file, err := os.OpenFile(st.storeFilePath, os.O_RDONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // does nothing
		}
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// array beginning '['
	if _, err = decoder.Token(); err != nil {
		return err
	}
	locallog := log.Info()
	for decoder.More() {
		var metric model.Metrics
		if err := decoder.Decode(&metric); err != nil {
			return err
		}
		if err := st.SetMetrics(metric.ID, metric); err != nil {
			return err
		}
		locallog.Str(metric.ID, "restored")
	}
	locallog.Send()
	// array ending ']'
	if _, err := decoder.Token(); err != nil {
		return err
	}

	return nil
}

// Close stops the background saving goroutine and waits for it to finish.
// It also triggers a final save of all metrics to ensure durability on shutdown.
func (st *Store) Close() {
	st.cancel()
	st.wg.Wait()
}

// Save runs a background loop that persists metrics to disk either periodically (if interval > 0)
// or in response to write events (if interval == 0). It uses atomic file replacement via a temporary file
// to ensure data integrity. This method is started automatically by NewFileStore and should not be called directly.
func (st *Store) Save(ctx context.Context) {
	defer st.wg.Done()

	save := func() error {
		st.mu.RLock()
		defer st.mu.RUnlock()

		allMetrics, err := st.GetAllMetrics()
		if err != nil {
			return err
		}

		// Write to a temp file every time in different one.
		dir := filepath.Dir(st.storeFilePath) // use the same folder as the file
		tmpFile := filepath.Base(st.storeFilePath) + ".*.tmp"
		file, err := os.CreateTemp(dir, tmpFile)
		if err != nil {
			return err
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(allMetrics); err != nil {
			file.Close()
			os.Remove(tmpFile)
			return err
		}

		if err := file.Close(); err != nil {
			os.Remove(tmpFile)
			return err
		}

		// Replace a regular file with a temp one
		return os.Rename(file.Name(), st.storeFilePath)
	}

	defer func() {
		if err := save(); err != nil {
			// Логируем ошибку, но не паникуем
			log.Printf("Failed to save metrics on exit: %v", err)
		}
	}()

	if st.interval == 0 {
		// Syncronious saving
		for {
			select {
			case <-ctx.Done():
				return
			case <-st.synchroCh:
				if err := save(); err != nil {
					log.Printf("Failed to save metrics: %v", err)
				}
			}
		}
	} else {
		// Interval saving
		ticker := time.NewTicker(st.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := save(); err != nil {
					log.Error().Err(err).Msg("Failed to save metrics")
				}
			}
		}
	}
}
