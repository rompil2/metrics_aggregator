// Package service provides business logic for managing metrics.
// path: internal/service
package service

import (
	"fmt"

	"github.com/rompil2/metrics_aggregator/internal/model"
)

// ErrMetricNotFound indicates that a requested metric does not exist in the repository.
var ErrMetricNotFound = fmt.Errorf("metric not found")

// ErrMetricCreated is returned when a new metric is successfully created during an update operation.
var ErrMetricCreated = fmt.Errorf("metric created")

// Repo defines the interface for a metrics repository, supporting basic CRUD operations and health checks.
type Repo interface {
	SetMetrics(ID string, value model.Metrics) error
	GetMetrics(ID string) (model.Metrics, error)
	GetAllMetrics() ([]model.Metrics, error)
	SetAllMetrics([]model.Metrics) error
	Ping() error
}

// MetricService provides business logic for managing metrics, including creation, retrieval, and batch updates,
// while abstracting the underlying storage implementation via the Repo interface.
type MetricService struct {
	repository Repo
}

// NewMetricService creates a new MetricService instance with the provided repository backend.
func NewMetricService(repository Repo) *MetricService {
	return &MetricService{
		repository: repository,
	}
}

// GetAllMetrics returns all available metrics from the underlying repository.
// It wraps any repository error with additional context for easier debugging.
func (s *MetricService) GetAllMetrics() ([]model.Metrics, error) {
	metrics, err := s.repository.GetAllMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to get all metrics: %w", err)
	}
	return metrics, nil
}

// GetMetrics retrieves a metric by its ID from the repository.
// Returns ErrMetricNotFound wrapped with the ID if the metric does not exist.
func (s *MetricService) GetMetrics(ID string) (model.Metrics, error) {
	storedData, err := s.repository.GetMetrics(ID)
	if err != nil {
		return model.Metrics{}, fmt.Errorf("%w: %s", ErrMetricNotFound, ID)
	}
	return storedData, nil
}

// UpdateMetrics updates an existing metric or creates a new one if it doesn't exist.
// Returns ErrMetricCreated when a new metric is created; otherwise returns nil on successful update or an error.
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

// UpdateAllMetrics performs a batch update of multiple metrics using the repository's SetAllMetrics method.
// It returns an error if the underlying operation fails, or nil if successful (including when the input slice is empty).
func (s *MetricService) UpdateAllMetrics(metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	if err := s.repository.SetAllMetrics(metrics); err != nil {
		return fmt.Errorf("failed to update all metrics: %w", err)
	}
	return nil
}

// Ping checks the availability and connectivity of the underlying repository.
// It is typically used for health checks and readiness probes.
func (s *MetricService) Ping() error {
	if err := s.repository.Ping(); err != nil {
		return fmt.Errorf("repository ping failed: %w", err)
	}
	return nil
}
