package dbstore

import (
	"database/sql"
	"fmt"
	"time"

	"errors"
	"log"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/migrations"
)

const maxAttempts = 3
const updateCounterSQL = `
		INSERT INTO metrics (id, m_type, delta, value, hash)
		VALUES ($1, $2, $3, NULL, $4)
		ON CONFLICT (id) 
		DO UPDATE SET 
			delta = metrics.delta + EXCLUDED.delta,
			hash = EXCLUDED.hash,
			updated_at = CURRENT_TIMESTAMP
	`
const updateGaugeSQL = `
		INSERT INTO metrics (id, m_type, delta, value, hash)
		VALUES ($1, $2, NULL, $3, $4)
		ON CONFLICT (id) 
		DO UPDATE SET 
			value = EXCLUDED.value,
			hash = EXCLUDED.hash,
			updated_at = CURRENT_TIMESTAMP
	`

var retryDelays = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

type DBStore struct {
	db *sql.DB
}

type Void struct{}

func NewDBStore(db *sql.DB) (DBStore, error) {
	if db != nil {
		return DBStore{db}, nil
	}

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

// SetAllMetrics updates multiple metrics in a single transaction with batch processing
func (s DBStore) SetAllMetrics(metrics []model.Metrics) error {
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
}

func (s DBStore) Ping() error {
	return s.withRetry(func() error {
		return s.db.Ping()
	})
}

func (s DBStore) SetMetrics(ID string, metric model.Metrics) error {
	return s.withRetry(func() error {
		return s.setMetrics(ID, metric)
	})
}

func (s DBStore) setMetrics(ID string, metric model.Metrics) error {
	if metric.MType == "counter" && metric.Delta != nil {

		_, err := s.db.Exec(updateCounterSQL, ID, metric.MType, *metric.Delta, metric.Hash)
		return err
	} else if metric.MType == "gauge" && metric.Value != nil {

		_, err := s.db.Exec(updateGaugeSQL, ID, metric.MType, *metric.Value, metric.Hash)
		return err
	}

	return errors.New("invalid metric data")
}

func (s DBStore) GetMetrics(metricID string) (model.Metrics, error) {
	var result model.Metrics
	var err error
	err = s.withRetry(func() error {
		result, err = s.getMetrics(metricID)
		return err
	})

	return result, err
}

func (s DBStore) getMetrics(metricID string) (model.Metrics, error) {
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

func (s DBStore) GetAllMetrics() ([]model.Metrics, error) {
	var result []model.Metrics
	var err error
	err = s.withRetry(func() error {
		result, err = s.getAllMetrics()
		return err
	})

	return result, err
}

func (s DBStore) getAllMetrics() ([]model.Metrics, error) {
	query := `SELECT id, m_type, delta, value, hash, updated_at FROM metrics ORDER BY updated_at DESC`

	rows, err := s.db.Query(query)
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

func (s DBStore) Close() error {
	return s.db.Close()
}

// Error retry wrapper
func (s *DBStore) withRetry(operation func() error) error {

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

			select {
			case <-time.After(retryDelays[attempt]):
				// Continue with next attempt
			default:
			}
		}
	}

	return fmt.Errorf("all %d attempts failed, last error: %w", maxAttempts, lastErr)
}

func (s *DBStore) isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// PostgreSQL error check
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Retriable
		retriableCodes := map[string]Void{
			pgerrcode.ConnectionException:                     {},
			pgerrcode.ConnectionDoesNotExist:                  {},
			pgerrcode.ConnectionFailure:                       {},
			pgerrcode.TransactionRollback:                     {},
			pgerrcode.SerializationFailure:                    {},
			pgerrcode.DeadlockDetected:                        {},
			pgerrcode.CannotConnectNow:                        {},
			pgerrcode.AdminShutdown:                           {},
			pgerrcode.CrashShutdown:                           {},
			pgerrcode.TooManyConnections:                      {},
			pgerrcode.InvalidAuthorizationSpecification:       {},
			pgerrcode.SQLClientUnableToEstablishSQLConnection: {},
		}

		if _, exist := retriableCodes[pgErr.Code]; exist {
			return true
		}

	}

	// Network errors are retriable
	var netErr interface {
		Timeout() bool
		Temporary() bool
	}
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	// Non-retriable by default
	return false
}
