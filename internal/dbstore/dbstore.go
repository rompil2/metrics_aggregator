package dbstore

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rompil2/metrics_aggregator/internal/model"
)

var db *sql.DB

type DBStore struct {
	db *sql.DB
}

func NewDBStore(connStr string) (DBStore, error) {
	if db != nil {
		return DBStore{db}, nil
	} else {
		db, err := sql.Open("pgx", connStr)
		if err != nil {
			return DBStore{}, err
		}
		return DBStore{db}, nil
	}

}

func (db DBStore) SetMetrics(ID string, value any) error {
	return nil
}
func (db DBStore) GetMetrics(ID string) (any, error) {
	return any(model.Metrics{}), nil
}
func (db DBStore) AllMetrics() ([]any, error) {
	return []any{any(model.Metrics{})}, nil
}
