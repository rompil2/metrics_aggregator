package service

import (
	"fmt"

	"github.com/rompil2/metrics_aggregator/internal/model"
)

// Service-level errors
var (
	ErrMetricNotFound = fmt.Errorf("metric not found")
	ErrMetricCreated  = fmt.Errorf("metric created")
)

type Repo interface {
	SetMetrics(ID string, value model.Metrics) error
	GetMetrics(ID string) (model.Metrics, error)
	GetAllMetrics() ([]model.Metrics, error)
	SetAllMetrics([]model.Metrics) error
	Ping() error
}

type MetricService struct {
	repository Repo
}

// NewMetricService creates a new MetricService instance
func NewMetricService(repository Repo) *MetricService {
	return &MetricService{
		repository: repository,
	}
}

// GetAllMetrics returns all available metrics from the repository
func (s *MetricService) GetAllMetrics() ([]model.Metrics, error) {
	metrics, err := s.repository.GetAllMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to get all metrics: %w", err)
	}
	return metrics, nil
}

// GetMetrics returns metrics with the given ID
func (s *MetricService) GetMetrics(ID string) (model.Metrics, error) {
	storedData, err := s.repository.GetMetrics(ID)
	if err != nil {
		return model.Metrics{}, fmt.Errorf("%w: %s", ErrMetricNotFound, ID)
	}
	return storedData, nil
}

// UpdateMetrics updates or creates metrics with the given ID
func (s *MetricService) UpdateMetrics(metric *model.Metrics) error {
	_, err := s.repository.GetMetrics(metric.ID)
	if err != nil {
		// Metric doesn't exist, create new one
		if err := s.repository.SetMetrics(metric.ID, *metric); err != nil {
			return fmt.Errorf("failed to create metric: %w", err)
		}
		return ErrMetricCreated
	}

	// Metric exists, update it
	if err := s.repository.SetMetrics(metric.ID, *metric); err != nil {
		return fmt.Errorf("failed to update metric: %w", err)
	}
	return nil
}

// UpdateAllMetrics updates multiple metrics at once
func (s *MetricService) UpdateAllMetrics(metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	if err := s.repository.SetAllMetrics(metrics); err != nil {
		return fmt.Errorf("failed to update all metrics: %w", err)
	}
	return nil
}

// Ping checks the repository connection
func (s *MetricService) Ping() error {
	if err := s.repository.Ping(); err != nil {
		return fmt.Errorf("repository ping failed: %w", err)
	}
	return nil
}
