// Package dbstore provides a PostgreSQL-backed metrics repository with retry logic for transient database errors.
package dbstore

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/migrations"
)

var retryDelays = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

const (
	maxAttempts      = 3
	updateCounterSQL = `
			INSERT INTO metrics (id, m_type, delta, value, hash)
			VALUES ($1, $2, $3, NULL, $4)
			ON CONFLICT (id) 
			DO UPDATE SET 
				delta = metrics.delta + EXCLUDED.delta,
				hash = EXCLUDED.hash,
				updated_at = CURRENT_TIMESTAMP
		`
	updateGaugeSQL = `
		INSERT INTO metrics (id, m_type, delta, value, hash)
		VALUES ($1, $2, NULL, $3, $4)
		ON CONFLICT (id) 
		DO UPDATE SET 
			value = EXCLUDED.value,
			hash = EXCLUDED.hash,
			updated_at = CURRENT_TIMESTAMP
	`
	allMetricsSQL = `SELECT id, m_type, delta, value, hash, updated_at FROM metrics ORDER BY updated_at DESC`
)

// retriableErrorCodes contains PostgreSQL error codes that should trigger retries
var retriableErrorCodes = map[string]bool{
	pgerrcode.ConnectionException:                     true,
	pgerrcode.ConnectionDoesNotExist:                  true,
	pgerrcode.ConnectionFailure:                       true,
	pgerrcode.TransactionRollback:                     true,
	pgerrcode.SerializationFailure:                    true,
	pgerrcode.DeadlockDetected:                        true,
	pgerrcode.CannotConnectNow:                        true,
	pgerrcode.AdminShutdown:                           true,
	pgerrcode.CrashShutdown:                           true,
	pgerrcode.TooManyConnections:                      true,
	pgerrcode.InvalidAuthorizationSpecification:       true,
	pgerrcode.SQLClientUnableToEstablishSQLConnection: true,
}

// DBStore represents a PostgreSQL-backed metrics repository with built-in retry logic for transient database errors.
type DBStore struct {
	db *sql.DB
}

// NewDBStore creates and initializes a new DBStore instance by validating the database connection and applying pending migrations.
func NewDBStore(db *sql.DB) (DBStore, error) {
	dbs := DBStore{db}
	// Check connection
	if err := dbs.Ping(); err != nil {
		return DBStore{}, err
	}
	if err := dbs.Migrate(); err != nil {
		return DBStore{}, err
	}
	return dbs, nil
}

// SetAllMetrics updates multiple metrics in a single atomic transaction, supporting both counter and gauge types with input validation.
func (s DBStore) SetAllMetrics(metrics []model.Metrics) error {
	return s.withRetry(func() error {
		if len(metrics) == 0 {
			return nil // Nothing to update
		}

		// Start transaction
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				panic(p)
			}
		}()

		// Prepare statements for counter and gauge updates
		counterStmt, err := tx.Prepare(updateCounterSQL)
		if err != nil {
			tx.Rollback()
			return err
		}
		defer counterStmt.Close()

		gaugeStmt, err := tx.Prepare(updateGaugeSQL)
		if err != nil {
			tx.Rollback()
			return err
		}
		defer gaugeStmt.Close()

		// Process each metric in the batch
		for _, metric := range metrics {
			if metric.MType == model.Counter && metric.Delta != nil {
				_, err := counterStmt.Exec(metric.ID, metric.MType, *metric.Delta, metric.Hash)
				if err != nil {
					tx.Rollback()
					return err
				}
			} else if metric.MType == model.Gauge && metric.Value != nil {
				_, err := gaugeStmt.Exec(metric.ID, metric.MType, *metric.Value, metric.Hash)
				if err != nil {
					tx.Rollback()
					return err
				}
			} else {
				tx.Rollback()
				return errors.New("invalid metric data in batch: ID=" + metric.ID)
			}
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return err
		}

		return nil
	})
}

// Ping checks the availability of the database connection and retries on transient failures.
func (s DBStore) Ping() error {
	return s.withRetry(s.db.Ping)
}

// SetMetrics stores or updates a single metric in the database, handling counters and gauges with appropriate SQL queries.
func (s DBStore) SetMetrics(ID string, metric model.Metrics) error {
	return s.withRetry(func() error {
		if metric.MType == "counter" && metric.Delta != nil {
			_, err := s.db.Exec(updateCounterSQL, ID, metric.MType, *metric.Delta, metric.Hash)
			return err
		} else if metric.MType == "gauge" && metric.Value != nil {
			_, err := s.db.Exec(updateGaugeSQL, ID, metric.MType, *metric.Value, metric.Hash)
			return err
		}

		return errors.New("invalid metric data")
	})
}

// GetMetrics retrieves a metric by its ID from the database, returning an error if not found or on database failure.
func (s DBStore) GetMetrics(metricID string) (model.Metrics, error) {
	var result model.Metrics
	err := s.withRetry(func() error {
		var innerErr error
		result, innerErr = s.getMetricsInternal(metricID)
		return innerErr
	})
	return result, err
}

func (s DBStore) getMetricsInternal(metricID string) (model.Metrics, error) {
	var metric model.Metrics
	var delta sql.NullInt64
	var value sql.NullFloat64

	query := `SELECT id, m_type, delta, value, hash FROM metrics WHERE id = $1`

	err := s.db.QueryRow(query, metricID).Scan(
		&metric.ID,
		&metric.MType,
		&delta,
		&value,
		&metric.Hash,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Metrics{}, errors.New("metric not found")
		}
		return model.Metrics{}, err
	}

	// the actual values might be null
	if delta.Valid {
		metric.Delta = &delta.Int64
	}
	if value.Valid {
		metric.Value = &value.Float64
	}

	return metric, nil
}

// GetAllMetrics fetches all stored metrics from the database, ordered by most recently updated.
func (s DBStore) GetAllMetrics() ([]model.Metrics, error) {
	var result []model.Metrics
	err := s.withRetry(func() error {
		var innerErr error
		result, innerErr = s.getAllMetricsInternal()
		return innerErr
	})
	return result, err
}

func (s DBStore) getAllMetricsInternal() ([]model.Metrics, error) {
	rows, err := s.db.Query(allMetricsSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []model.Metrics

	for rows.Next() {
		var metric model.Metrics
		var delta sql.NullInt64
		var value sql.NullFloat64
		var createdAt sql.NullTime

		err := rows.Scan(
			&metric.ID,
			&metric.MType,
			&delta,
			&value,
			&metric.Hash,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		if delta.Valid {
			metric.Delta = &delta.Int64
		}
		if value.Valid {
			metric.Value = &value.Float64
		}

		metrics = append(metrics, metric)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return metrics, nil
}

// Error retry wrapper
func (s DBStore) withRetry(operation func() error) error {

	var lastErr error

	for attempt := range maxAttempts {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		if !s.isRetriableError(err) {
			return err
		}

		if attempt < maxAttempts-1 {
			log.Printf("Database operation failed (attempt %d/%d), retrying in %v: %v",
				attempt+1, maxAttempts, retryDelays[attempt], err)

			time.Sleep(retryDelays[attempt])
		}
	}

	return fmt.Errorf("all %d attempts failed, last error: %w", maxAttempts, lastErr)
}

func (s DBStore) isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// PostgreSQL error check
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return retriableErrorCodes[pgErr.Code]
	}

	// Network/timeout errors are retriable
	var netErr interface {
		Timeout() bool
		Temporary() bool
	}
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	// Connection errors are retriable
	if errors.Is(err, sql.ErrConnDone) {
		return true
	}

	// Non-retriable by default
	return false
}

// Migrate applies all pending database schema migrations using the goose framework and embedded migration files.
func (s DBStore) Migrate() error {
	files, err := migrations.GetMigrationFiles()
	if err != nil {
		log.Fatal(err)
	}
	for file := range files {
		log.Println(file)
	}
	goose.SetBaseFS(migrations.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.Up(s.db, "."); err != nil {
		return err
	}
	log.Print("Migration completed successfully")
	return nil
}

// Close terminates the underlying database connection and releases associated resources.
func (s DBStore) Close() error {
	return s.db.Close()
}
