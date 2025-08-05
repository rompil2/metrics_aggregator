package service

import (
	"errors"

	"github.com/rompil2/metrics_aggregator/internal/model"
)

type Repo interface {
	SetMetrics(ID string, value any) error
	GetMetrics(ID string) (any, error)
	AllMetrics() ([]any, error)
}

type MetricService struct {
	repository *Repo
}

// Returns all available metrics in the repository.
func (s *MetricService) AllMetrics() ([]model.Metrics, error) {
	panic("unimplemented")
}

// Returns metrics with the given ID.
func (s *MetricService) GetMetrics(ID string) (model.Metrics, error) {
	panic("unimplemented")
}

// Updates metrics with the given ID. either it is Counter or Gauge.
func (s *MetricService) UpdateMetrics(metric *model.Metrics) error {
	id := metric.ID
	storedData, err := (*s.repository).GetMetrics(id)

	if err != nil { //if metric doesn't exist
		err := (*s.repository).SetMetrics(id, metric)
		if err != nil {
			return err
		}
		return errors.New("Unknown metrics ID, create a new one")
	}
	if storedData != nil {
		var existedMetrics model.Metrics
		if d, ok := (storedData).(*model.Metrics); !ok {
			panic("Unknown type of stored data") //shouldn't happen
		} else {
			existedMetrics = *d
		}
		switch metric.MType {
		case model.Counter:
			if existedMetrics.Delta != nil {
				*existedMetrics.Delta += *metric.Delta
			} else {
				existedMetrics.Delta = metric.Delta
			}
		case model.Gauge:
			*existedMetrics.Value = *metric.Value
		default:
			panic("Unknown type of metric") //shouldn't happen
		}
	}
	return nil
}

func NewMetricService(repository Repo) MetricService {
	return MetricService{
		repository: &repository,
	}
}
