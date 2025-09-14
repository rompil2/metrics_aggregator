package dbstore

import (
	"database/sql"

	"errors"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/rompil2/metrics_aggregator/migrations"
)

var db *sql.DB

type DBStore struct {
	db *sql.DB
}

// SetAllMetrics implements service.Repo.
func (s DBStore) SetAllMetrics([]model.Metrics) error {
	panic("unimplemented")
}

func NewDBStore(connStr string) (DBStore, error) {
	if db != nil {
		return DBStore{db}, nil
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return DBStore{}, err
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

func (s DBStore) Ping() error {
	return s.db.Ping()
}

func (s DBStore) SetMetrics(ID string, metric model.Metrics) error {
	if metric.MType == "counter" && metric.Delta != nil {
		query := `
			INSERT INTO metrics (id, m_type, delta, value, hash)
			VALUES ($1, $2, $3, NULL, $4)
			ON CONFLICT (id) 
			DO UPDATE SET 
				delta = metrics.delta + EXCLUDED.delta,
				hash = EXCLUDED.hash,
				updated_at = CURRENT_TIMESTAMP
		`
		_, err := s.db.Exec(query, ID, metric.MType, *metric.Delta, metric.Hash)
		return err
	} else if metric.MType == "gauge" && metric.Value != nil {
		query := `
			INSERT INTO metrics (id, m_type, delta, value, hash)
			VALUES ($1, $2, NULL, $3, $4)
			ON CONFLICT (id) 
			DO UPDATE SET 
				value = EXCLUDED.value,
				hash = EXCLUDED.hash,
				updated_at = CURRENT_TIMESTAMP
		`
		_, err := s.db.Exec(query, ID, metric.MType, *metric.Value, metric.Hash)
		return err
	}

	return errors.New("invalid metric data")
}

func (s DBStore) GetMetrics(metricID string) (model.Metrics, error) {
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

func (s DBStore) GetMetricsCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM metrics").Scan(&count)
	return count, err
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
