package store

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/config"
	"github.com/rompil2/metrics_aggregator/internal/logger"
	"github.com/rompil2/metrics_aggregator/internal/model"
)

type Repo interface {
	SetMetrics(ID string, value any) error
	GetMetrics(ID string) (any, error)
	AllMetrics() ([]any, error)
}

type Store struct {
	Repo
	storeFilePath string
	interval      time.Duration
	cancel        context.CancelFunc
	synchroCh     chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
}

var log = logger.Get()

func NewStore(repo Repo, cfg config.StoreConfig) (*Store, error) {
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

func (st *Store) SetMetrics(ID string, value any) error {
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

	for decoder.More() {
		var metric model.Metrics
		if err := decoder.Decode(&metric); err != nil {
			return err
		}
		if err := st.SetMetrics(metric.ID, &metric); err != nil {
			return err
		}
	}

	// array ending ']'
	if _, err := decoder.Token(); err != nil {
		return err
	}

	return nil
}

func (st *Store) Close() {
	st.cancel()
	st.wg.Wait()
}

func (st *Store) Save(ctx context.Context) {
	defer st.wg.Done()

	save := func() error {
		st.mu.RLock()
		defer st.mu.RUnlock()

		allMetrics, err := st.AllMetrics()
		if err != nil {
			return err
		}

		// Write to a temp file every time in different one.
		tmpFile := st.storeFilePath + "*.tmp"
		file, err := os.CreateTemp(os.TempDir(), tmpFile)
		if err != nil {
			return err
		}

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
		return os.Rename(tmpFile, st.storeFilePath)
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
					log.Printf("Failed to save metrics: %v", err)
				}
			}
		}
	}
}
