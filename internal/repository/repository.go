package repository

import "github.com/rompil2/metrics_aggregator/internal/model"

type Repo interface {
	SetMetrics(ID string, value model.Metrics) error
	GetMetrics(ID string) (model.Metrics, error)
	GetAllMetrics() ([]model.Metrics, error)
	SetAllMetrics([]model.Metrics) error
	Ping() error
}
