package dbstore

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgconn"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBStore_Ping(t *testing.T) {
	t.Run("successful ping", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectPing()

		store := DBStore{db: db}
		err = store.Ping()
		assert.NoError(t, err)
	})

	t.Run("ping with retriable error succeeds after retry", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()

		// First attempt fails with retriable error
		mock.ExpectPing().WillReturnError(&pgconn.PgError{Code: "08000"})
		// Second attempt succeeds
		mock.ExpectPing()

		store := DBStore{db: db}
		err = store.Ping()
		assert.NoError(t, err)
	})
}

func TestDBStore_SetMetrics(t *testing.T) {
	t.Run("successful counter update", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO metrics").WithArgs(
			"test_counter", "counter", int64(42), "",
		).WillReturnResult(sqlmock.NewResult(1, 1))

		store := DBStore{db: db}
		metric := model.Metrics{
			ID:    "test_counter",
			MType: "counter",
			Delta: func() *int64 { v := int64(42); return &v }(),
		}

		err = store.SetMetrics("test_counter", metric)
		assert.NoError(t, err)
	})

	t.Run("successful gauge update", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("INSERT INTO metrics").WithArgs(
			"test_gauge", "gauge", 3.14, "",
		).WillReturnResult(sqlmock.NewResult(1, 1))

		store := DBStore{db: db}
		metric := model.Metrics{
			ID:    "test_gauge",
			MType: "gauge",
			Value: func() *float64 { v := 3.14; return &v }(),
		}

		err = store.SetMetrics("test_gauge", metric)
		assert.NoError(t, err)
	})
}

func TestDBStore_GetMetrics(t *testing.T) {
	t.Run("successful metric retrieval", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		rows := sqlmock.NewRows([]string{"id", "m_type", "delta", "value", "hash"}).
			AddRow("test_counter", "counter", int64(42), nil, "")
		mock.ExpectQuery("SELECT id, m_type, delta, value, hash FROM metrics WHERE id = ?").
			WithArgs("test_counter").
			WillReturnRows(rows)

		store := DBStore{db: db}
		metric, err := store.GetMetrics("test_counter")
		assert.NoError(t, err)
		assert.Equal(t, "test_counter", metric.ID)
		assert.Equal(t, int64(42), *metric.Delta)
	})

	t.Run("metric not found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery("SELECT id, m_type, delta, value, hash FROM metrics WHERE id = ?").
			WithArgs("nonexistent").
			WillReturnError(sql.ErrNoRows)

		store := DBStore{db: db}
		_, err = store.GetMetrics("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metric not found")
	})
}

func TestDBStore_GetAllMetrics(t *testing.T) {
	t.Run("successful retrieval of all metrics", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		rows := sqlmock.NewRows([]string{"id", "m_type", "delta", "value", "hash", "updated_at"}).
			AddRow("counter1", "counter", int64(10), nil, "", time.Now()).
			AddRow("gauge1", "gauge", nil, 3.14, "", time.Now())
		mock.ExpectQuery("SELECT id, m_type, delta, value, hash, updated_at FROM metrics ORDER BY updated_at DESC").
			WillReturnRows(rows)

		store := DBStore{db: db}
		metrics, err := store.GetAllMetrics()
		assert.NoError(t, err)
		assert.Len(t, metrics, 2)
	})
}

func TestDBStore_RetryLogic(t *testing.T) {
	t.Run("retry on retriable error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()

		// First attempt fails with retriable error
		mock.ExpectExec("INSERT INTO metrics").WithArgs(
			"test_counter", "counter", int64(42), "",
		).WillReturnError(&pgconn.PgError{Code: "08000"})
		// Second attempt succeeds
		mock.ExpectExec("INSERT INTO metrics").WithArgs(
			"test_counter", "counter", int64(42), "",
		).WillReturnResult(sqlmock.NewResult(1, 1))

		store := DBStore{db: db}
		metric := model.Metrics{
			ID:    "test_counter",
			MType: "counter",
			Delta: func() *int64 { v := int64(42); return &v }(),
		}

		err = store.SetMetrics("test_counter", metric)
		assert.NoError(t, err)
	})
}

func TestDBStore_IsRetriableError(t *testing.T) {
	store := DBStore{}

	tests := []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "retriable PostgreSQL error",
			err:      &pgconn.PgError{Code: "08000"},
			expected: true,
		},
		{
			name:     "non-retriable PostgreSQL error",
			err:      &pgconn.PgError{Code: "23505"},
			expected: false,
		},
		{
			name:     "connection done error",
			err:      sql.ErrConnDone,
			expected: true,
		},
		{
			name:     "regular error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := store.isRetriableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDBStore_Close(t *testing.T) {
	t.Run("successful close", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)

		mock.ExpectClose()

		store := DBStore{db: db}
		err = store.Close()
		assert.NoError(t, err)
	})
}

func TestDBStore_SetAllMetrics(t *testing.T) {
	t.Run("successful batch update", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT INTO metrics")
		mock.ExpectPrepare("INSERT INTO metrics")
		mock.ExpectExec("INSERT INTO metrics").WithArgs(
			"counter1", "counter", int64(10), "",
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO metrics").WithArgs(
			"gauge1", "gauge", 3.14, "",
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		store := DBStore{db: db}
		metrics := []model.Metrics{
			{
				ID:    "counter1",
				MType: "counter",
				Delta: func() *int64 { v := int64(10); return &v }(),
			},
			{
				ID:    "gauge1",
				MType: "gauge",
				Value: func() *float64 { v := 3.14; return &v }(),
			},
		}

		err = store.SetAllMetrics(metrics)
		assert.NoError(t, err)
	})

	t.Run("empty batch", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		store := DBStore{db: db}
		err = store.SetAllMetrics([]model.Metrics{})
		assert.NoError(t, err)
	})

	t.Run("retry on retriable error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		// First attempt fails with retriable error during begin transaction
		mock.ExpectBegin().WillReturnError(&pgconn.PgError{Code: "08000"})
		// Second attempt succeeds
		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT INTO metrics")
		mock.ExpectPrepare("INSERT INTO metrics")
		mock.ExpectExec("INSERT INTO metrics").WithArgs(
			"counter1", "counter", int64(10), "",
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		store := DBStore{db: db}
		metrics := []model.Metrics{
			{
				ID:    "counter1",
				MType: "counter",
				Delta: func() *int64 { v := int64(10); return &v }(),
			},
		}

		err = store.SetAllMetrics(metrics)
		assert.NoError(t, err)
	})
}
