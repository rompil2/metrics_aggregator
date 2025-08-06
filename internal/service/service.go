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
	if data, err := (*s.repository).AllMetrics(); err != nil {
		return nil, err
	} else {
		var metrics []model.Metrics
		for _, v := range data {
			metrics = append(metrics, (v).(model.Metrics))
		}
		return metrics, nil
	}
}

// Returns metrics with the given ID.
func (s *MetricService) GetMetrics(ID string) (model.Metrics, error) {
	storedData, err := (*s.repository).GetMetrics(ID)
	if err != nil {
		return model.Metrics{}, errors.New("unknown metrics ID")
	}
	return *(storedData).(*model.Metrics), nil
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
		return errors.New("unknown metrics ID, created the new one")
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
		err := (*s.repository).SetMetrics(id, metric)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewMetricService(repository Repo) MetricService {
	return MetricService{
		repository: &repository,
	}
}
