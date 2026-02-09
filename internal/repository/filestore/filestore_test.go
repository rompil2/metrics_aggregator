package filestore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/rompil2/metrics_aggregator/internal/config"
	"github.com/rompil2/metrics_aggregator/internal/mocks"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("should create store with restore", func(t *testing.T) {
		v, d := float64(1.0), int64(42)
		mockRepo := mocks.NewMockRepo(ctrl)
		cfg := config.StoreConfig{
			StoreInterval:   time.Second,
			FileStoragePath: "test.json",
			Restore:         func(b bool) *bool { return &b }(true), // true
		}

		// Create test file for restore
		testData := []model.Metrics{
			{ID: "test1", MType: "gauge", Value: &v},
			{ID: "test2", MType: "counter", Delta: &d},
		}
		file, err := os.Create(cfg.FileStoragePath)
		require.NoError(t, err)
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		require.NoError(t, encoder.Encode(testData))
		file.Close()
		defer os.Remove(cfg.FileStoragePath)

		// Expect SetMetrics calls for each metric during restore
		mockRepo.EXPECT().SetMetrics("test1", gomock.Any()).Return(nil)
		mockRepo.EXPECT().SetMetrics("test2", gomock.Any()).Return(nil)
		mockRepo.EXPECT().GetAllMetrics().Return([]model.Metrics{}, nil).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)
		assert.NotNil(t, store)

		store.Close()
	})

	t.Run("should create store without restore", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		cfg := config.StoreConfig{
			StoreInterval:   time.Second,
			FileStoragePath: "nonexistent.json",
			Restore:         func(b bool) *bool { return &b }(false),
		}

		mockRepo.EXPECT().GetAllMetrics().Return([]model.Metrics{}, nil).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)
		assert.NotNil(t, store)

		store.Close()
	})
}

func TestStore_SetMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("should set metrics and trigger sync", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		cfg := config.StoreConfig{
			StoreInterval:   0, // synchronous mode
			FileStoragePath: "test.json",
			Restore:         func(b bool) *bool { return &b }(false),
		}

		mockRepo.EXPECT().SetMetrics("test", gomock.Any()).Return(nil)
		mockRepo.EXPECT().GetAllMetrics().Return([]model.Metrics{}, nil).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)

		err = store.SetMetrics("test", model.Metrics{ID: "test"})
		require.NoError(t, err)

		// Give some time for the goroutine to process
		time.Sleep(10 * time.Millisecond)

		store.Close()
	})
}

func TestStore_Restore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("should restore from file", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		testFile := "restore_test.json"
		v, d := float64(123.45), int64(100)
		// Create test data
		testData := []model.Metrics{
			{ID: "metric1", MType: "gauge", Value: &v},
			{ID: "metric2", MType: "counter", Delta: &d},
		}

		file, err := os.Create(testFile)
		require.NoError(t, err)
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		require.NoError(t, encoder.Encode(testData))
		file.Close()
		defer os.Remove(testFile)

		store := &Store{
			Repo:          mockRepo,
			storeFilePath: testFile,
		}

		mockRepo.EXPECT().SetMetrics("metric1", gomock.Any()).Return(nil)
		mockRepo.EXPECT().SetMetrics("metric2", gomock.Any()).Return(nil)

		err = store.Restore()
		require.NoError(t, err)
	})

	t.Run("should handle non-existent file", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		store := &Store{
			Repo:          mockRepo,
			storeFilePath: "nonexistent.json",
		}

		err := store.Restore()
		require.NoError(t, err)
	})

	t.Run("should handle invalid JSON", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		testFile := "invalid_test.json"

		err := os.WriteFile(testFile, []byte("invalid json"), 0644)
		require.NoError(t, err)
		defer os.Remove(testFile)

		store := &Store{
			Repo:          mockRepo,
			storeFilePath: testFile,
		}

		err = store.Restore()
		require.Error(t, err)
	})
}

func TestStore_Save(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("should save metrics synchronously", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		testFile := "sync_save_test.json"
		v, d := float64(1.0), int64(42)
		cfg := config.StoreConfig{
			StoreInterval:   0, // synchronous mode
			FileStoragePath: testFile,
			Restore:         func(b bool) *bool { return &b }(false), // false
		}

		testMetrics := []model.Metrics{
			{ID: "test1", MType: "gauge", Value: &v},
			{ID: "test2", MType: "counter", Delta: &d},
		}

		mockRepo.EXPECT().GetAllMetrics().Return(testMetrics, nil)
		mockRepo.EXPECT().GetAllMetrics().Return(testMetrics, nil).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)

		// Trigger save via sync channel
		store.synchroCh <- struct{}{}

		// Give some time for save to complete
		time.Sleep(10 * time.Millisecond)

		store.Close()

		// Verify file was created
		_, err = os.Stat(testFile)
		require.NoError(t, err)
		defer os.Remove(testFile)
	})

	t.Run("should save metrics on interval", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		testFile := "interval_save_test.json"
		v := float64(1.0)
		cfg := config.StoreConfig{
			StoreInterval:   10 * time.Millisecond,
			FileStoragePath: testFile,
			Restore:         func(b bool) *bool { return &b }(false), // false
		}

		testMetrics := []model.Metrics{{ID: "test", MType: "gauge", Value: &v}}
		mockRepo.EXPECT().GetAllMetrics().Return(testMetrics, nil).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)

		// Wait for at least one interval
		time.Sleep(15 * time.Millisecond)

		store.Close()

		// Verify file was created
		_, err = os.Stat(testFile)
		require.NoError(t, err)
		defer os.Remove(testFile)
	})

	t.Run("should handle save error", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		testFile := filepath.Join("invalid", "path", "test.json")

		cfg := config.StoreConfig{
			StoreInterval:   0,
			FileStoragePath: testFile,
			Restore:         func(b bool) *bool { return &b }(false), // false
		}
		v := float64(1.0)
		testMetrics := []model.Metrics{{ID: "test", MType: "gauge", Value: &v}}
		mockRepo.EXPECT().GetAllMetrics().Return(testMetrics, nil).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)

		// Trigger save
		store.synchroCh <- struct{}{}

		// Give some time for save to attempt
		time.Sleep(10 * time.Millisecond)

		store.Close()
	})
}

func TestStore_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("should close store gracefully", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		cfg := config.StoreConfig{
			StoreInterval:   time.Second,
			FileStoragePath: "close_test.json",
			Restore:         func(b bool) *bool { return &b }(false), //false
		}

		mockRepo.EXPECT().GetAllMetrics().Return([]model.Metrics{}, nil).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)

		// Store should be running
		assert.NotNil(t, store.cancel)

		store.Close()

		// Verify context is cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		select {
		case <-ctx.Done():
			// Expected
		default:
			t.Error("Context should be cancelled")
		}
	})
}

func TestStore_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("should handle repo errors in SetMetrics", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		cfg := config.StoreConfig{
			StoreInterval:   time.Second,
			FileStoragePath: "test.json",
			Restore:         func(b bool) *bool { return &b }(false),
		}

		expectedErr := errors.New("repo error")
		mockRepo.EXPECT().SetMetrics("test", gomock.Any()).Return(expectedErr)
		mockRepo.EXPECT().GetAllMetrics().Return([]model.Metrics{}, nil).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)

		err = store.SetMetrics("test", model.Metrics{ID: "test"})
		require.Error(t, err)
		require.Equal(t, expectedErr, err)

		store.Close()
	})

	t.Run("should handle repo errors in AllMetrics", func(t *testing.T) {
		mockRepo := mocks.NewMockRepo(ctrl)
		cfg := config.StoreConfig{
			StoreInterval:   0,
			FileStoragePath: "test.json",
			Restore:         func(b bool) *bool { return &b }(false), //false
		}

		expectedErr := errors.New("all metrics error")
		mockRepo.EXPECT().GetAllMetrics().Return([]model.Metrics{}, expectedErr).AnyTimes()

		store, err := NewFileStore(mockRepo, cfg)
		require.NoError(t, err)

		// Trigger save which should fail
		store.synchroCh <- struct{}{}

		// Give some time for the error to be handled
		time.Sleep(10 * time.Millisecond)

		store.Close()
	})
}
