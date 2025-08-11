package service

import (
	"errors"
	"fmt"

	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rompil2/metrics_aggregator/internal/mocks"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestMetricService_UpdateMetrics(t *testing.T) {

	type args struct {
		metric *model.Metrics
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		errString string
	}{
		{
			name: "Positive test. Add a new counter",
			args: args{
				metric: &model.Metrics{
					ID:    "testCounter",
					MType: model.Counter,
					Delta: new(int64),
				},
			},
			wantErr:   true,
			errString: "unknown metrics ID, created the new one",
		},
		{
			name: "Positive test. Add a new gauge",
			args: args{
				metric: &model.Metrics{
					ID:    "testGauge",
					MType: model.Gauge,
					Value: new(float64),
				},
			},
			wantErr:   true,
			errString: "unknown metrics ID, created the new one",
		},
	}
	// Create a new Gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)
			mockRepository.EXPECT().SetMetrics(tt.args.metric.ID, tt.args.metric).Return(nil).Times(1)
			mockRepository.EXPECT().GetMetrics(tt.args.metric.ID).Return(nil, fmt.Errorf("requested value for %s does not exist\n", tt.args.metric.ID)).Times(1)
			s := NewMetricService(mockRepository)
			err := s.UpdateMetrics(tt.args.metric)
			if tt.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.errString)
			}

		})
	}
}

func TestMetricService_UpdateMetrics_SecondTime(t *testing.T) {

	type args struct {
		metric *model.Metrics
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		errString string
	}{
		{
			name: "Positive test. Add a new counter",
			args: args{
				metric: &model.Metrics{
					ID:    "testCounter",
					MType: model.Counter,
					Delta: new(int64),
				},
			},
			wantErr:   false,
			errString: "Unknown metrics ID, created the new one",
		},
		{
			name: "Positive test. Add a new gauge",
			args: args{
				metric: &model.Metrics{
					ID:    "testGauge",
					MType: model.Gauge,
					Value: new(float64),
				},
			},
			wantErr:   false,
			errString: "Unknown metrics ID, created the new one",
		},
	}
	// Create a new Gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)
			updatedMetrics := *tt.args.metric
			if tt.args.metric.MType == model.Counter {
				*(tt.args.metric.Delta) = 1
				updatedMetrics.Delta = new(int64)
				*updatedMetrics.Delta = 2
			}
			mockRepository.EXPECT().SetMetrics(tt.args.metric.ID, &updatedMetrics).Return(nil).Times(1)
			mockRepository.EXPECT().GetMetrics(tt.args.metric.ID).Return(tt.args.metric, nil).Times(1)
			s := NewMetricService(mockRepository)
			err := s.UpdateMetrics(tt.args.metric)
			assert.NoError(t, err)
		})
	}
}

func TestMetricService_GetMetrics(t *testing.T) {

	type args struct {
		metric *model.Metrics
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		errString string
	}{
		{
			name: "Negative test. No counter",
			args: args{
				metric: &model.Metrics{
					ID:    "testCounter",
					MType: model.Counter,
					Delta: new(int64),
				},
			},
			wantErr:   true,
			errString: "unknown metrics ID",
		},
		{
			name: "Negative test. No gauge",
			args: args{
				metric: &model.Metrics{
					ID:    "testGauge",
					MType: model.Gauge,
					Value: new(float64),
				},
			},
			wantErr:   true,
			errString: "unknown metrics ID",
		},
		{
			name: "Positive test. Counter exists",
			args: args{
				metric: &model.Metrics{
					ID:    "testCounter",
					MType: model.Counter,
					Delta: new(int64),
				},
			},
			wantErr: false,
		},
		{
			name: "Positive test. Gauge exists",
			args: args{
				metric: &model.Metrics{
					ID:    "testGauge",
					MType: model.Gauge,
					Value: new(float64),
				},
			},
			wantErr: false,
		},
	}
	// Create a new Gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)
			if tt.wantErr {
				mockRepository.EXPECT().GetMetrics(tt.args.metric.ID).Return(nil, fmt.Errorf("requested value for %s does not exist\n", tt.args.metric.ID)).Times(1)
			} else {
				mockRepository.EXPECT().GetMetrics(tt.args.metric.ID).Return(tt.args.metric, nil).Times(1)
			}
			s := NewMetricService(mockRepository)
			val, err := s.GetMetrics(tt.args.metric.ID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.errString)
			} else {
				assert.EqualValues(t, tt.args.metric, &val)
			}
		})
	}
}

func TestMetricService_AllMetrics(t *testing.T) {

	tests := []struct {
		name      string
		args      []any
		wantErr   bool
		errString string
	}{
		{
			name: "Positive test. There some metrics",
			args: []any{
				&model.Metrics{
					ID:    "testCounter",
					MType: model.Counter,
					Delta: new(int64),
				},
				&model.Metrics{
					ID:    "testGauge",
					MType: model.Gauge,
					Value: new(float64),
				},
			},
			wantErr: false,
		},
		{
			name:    "Negative test. No metrics",
			args:    nil,
			wantErr: true,
		},
	}
	// Create a new Gomock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)
			if tt.wantErr {
				mockRepository.EXPECT().AllMetrics().Return(tt.args, errors.New("Some error")).Times(1)
			} else {
				mockRepository.EXPECT().AllMetrics().Return(tt.args, nil).Times(1)
			}
			s := NewMetricService(mockRepository)
			metrics, err := s.AllMetrics()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				var want []model.Metrics
				for _, v := range tt.args {
					if value, ok := v.(*model.Metrics); ok {
						want = append(want, *value)
					}
				}
				assert.EqualValues(t, want, metrics)

			}
		})
	}
}
