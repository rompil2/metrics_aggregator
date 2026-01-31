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
		args      args
		name      string
		errString string
		wantErr   bool
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
			errString: ErrMetricCreated.Error(),
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
			errString: ErrMetricCreated.Error(),
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)

			// Ожидаем вызов GetMetrics сначала (вернет ошибку - метрика не найдена)
			mockRepository.EXPECT().
				GetMetrics(tt.args.metric.ID).
				Return(model.Metrics{}, errors.New("not found")).
				Times(1)

			// Затем ожидаем вызов SetMetrics для создания новой метрики
			mockRepository.EXPECT().
				SetMetrics(tt.args.metric.ID, *tt.args.metric).
				Return(nil).
				Times(1)

			s := NewMetricService(mockRepository)
			err := s.UpdateMetrics(tt.args.metric)

			if tt.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.errString)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricService_UpdateMetrics_SecondTime(t *testing.T) {
	type args struct {
		metric *model.Metrics
	}
	tests := []struct {
		args      args
		name      string
		errString string
		wantErr   bool
	}{
		{
			name: "Positive test. Update existing counter",
			args: args{
				metric: &model.Metrics{
					ID:    "testCounter",
					MType: model.Counter,
					Delta: func() *int64 { v := int64(2); return &v }(),
				},
			},
			wantErr: false,
		},
		{
			name: "Positive test. Update existing gauge",
			args: args{
				metric: &model.Metrics{
					ID:    "testGauge",
					MType: model.Gauge,
					Value: func() *float64 { v := float64(3.14); return &v }(),
				},
			},
			wantErr: false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)

			// Ожидаем вызов GetMetrics (вернет существующую метрику)
			existingMetric := model.Metrics{
				ID:    tt.args.metric.ID,
				MType: tt.args.metric.MType,
			}

			if tt.args.metric.MType == model.Counter {
				existingMetric.Delta = func() *int64 { v := int64(1); return &v }()
			} else {
				existingMetric.Value = func() *float64 { v := float64(2.71); return &v }()
			}

			mockRepository.EXPECT().
				GetMetrics(tt.args.metric.ID).
				Return(existingMetric, nil).
				Times(1)

			// Ожидаем вызов SetMetrics для обновления существующей метрики
			mockRepository.EXPECT().
				SetMetrics(tt.args.metric.ID, *tt.args.metric).
				Return(nil).
				Times(1)

			s := NewMetricService(mockRepository)
			err := s.UpdateMetrics(tt.args.metric)

			assert.NoError(t, err)
		})
	}
}

func TestMetricService_GetMetrics(t *testing.T) {
	type args struct {
		metric   *model.Metrics
		metricID string
	}
	tests := []struct {
		args      args
		name      string
		errString string
		wantErr   bool
	}{
		{
			name: "Negative test. No counter",
			args: args{
				metricID: "testCounter",
				metric:   nil,
			},
			wantErr:   true,
			errString: "metric not found: testCounter",
		},
		{
			name: "Negative test. No gauge",
			args: args{
				metricID: "testGauge",
				metric:   nil,
			},
			wantErr:   true,
			errString: "metric not found: testGauge",
		},
		{
			name: "Positive test. Counter exists",
			args: args{
				metricID: "testCounter",
				metric: &model.Metrics{
					ID:    "testCounter",
					MType: model.Counter,
					Delta: func() *int64 { v := int64(10); return &v }(),
				},
			},
			wantErr: false,
		},
		{
			name: "Positive test. Gauge exists",
			args: args{
				metricID: "testGauge",
				metric: &model.Metrics{
					ID:    "testGauge",
					MType: model.Gauge,
					Value: func() *float64 { v := float64(3.14); return &v }(),
				},
			},
			wantErr: false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)

			if tt.wantErr {
				// Метрика не найдена - возвращаем ошибку
				mockRepository.EXPECT().
					GetMetrics(tt.args.metricID).
					Return(model.Metrics{}, errors.New("not found")).
					Times(1)
			} else {
				// Метрика найдена - возвращаем ее
				mockRepository.EXPECT().
					GetMetrics(tt.args.metricID).
					Return(*tt.args.metric, nil).
					Times(1)
			}

			s := NewMetricService(mockRepository)
			val, err := s.GetMetrics(tt.args.metricID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tt.args.metric, &val)
			}
		})
	}
}

func TestMetricService_GetAllMetrics(t *testing.T) {
	tests := []struct {
		name      string
		errString string
		metrics   []model.Metrics
		wantErr   bool
	}{
		{
			name: "Positive test. There are some metrics",
			metrics: []model.Metrics{
				{
					ID:    "testCounter",
					MType: model.Counter,
					Delta: func() *int64 { v := int64(10); return &v }(),
				},
				{
					ID:    "testGauge",
					MType: model.Gauge,
					Value: func() *float64 { v := float64(3.14); return &v }(),
				},
			},
			wantErr: false,
		},
		{
			name:      "Negative test. Error from repository",
			metrics:   nil,
			wantErr:   true,
			errString: "failed to get all metrics",
		},
		{
			name:    "Positive test. Empty metrics list",
			metrics: []model.Metrics{},
			wantErr: false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)

			if tt.wantErr {
				mockRepository.EXPECT().
					GetAllMetrics().
					Return(nil, errors.New("repository error")).
					Times(1)
			} else {
				mockRepository.EXPECT().
					GetAllMetrics().
					Return(tt.metrics, nil).
					Times(1)
			}

			s := NewMetricService(mockRepository)
			metrics, err := s.GetAllMetrics()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tt.metrics, metrics)
			}
		})
	}
}

func TestMetricService_Ping(t *testing.T) {
	tests := []struct {
		name      string
		errString string
		wantErr   bool
	}{
		{
			name:    "Positive test. Ping successful",
			wantErr: false,
		},
		{
			name:      "Negative test. Ping failed",
			wantErr:   true,
			errString: "repository ping failed",
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)

			if tt.wantErr {
				mockRepository.EXPECT().
					Ping().
					Return(errors.New("connection failed")).
					Times(1)
			} else {
				mockRepository.EXPECT().
					Ping().
					Return(nil).
					Times(1)
			}

			s := NewMetricService(mockRepository)
			err := s.Ping()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricService_UpdateAllMetrics(t *testing.T) {
	tests := []struct {
		name      string
		errString string
		metrics   []model.Metrics
		wantErr   bool
	}{
		{
			name: "Positive test. Update multiple metrics successfully",
			metrics: []model.Metrics{
				{
					ID:    "counter1",
					MType: model.Counter,
					Delta: func() *int64 { v := int64(100); return &v }(),
				},
				{
					ID:    "gauge1",
					MType: model.Gauge,
					Value: func() *float64 { v := float64(3.14); return &v }(),
				},
				{
					ID:    "counter2",
					MType: model.Counter,
					Delta: func() *int64 { v := int64(200); return &v }(),
				},
			},
			wantErr: false,
		},
		{
			name:    "Positive test. Update empty metrics list",
			metrics: []model.Metrics{},
			wantErr: false,
		},
		{
			name: "Negative test. Repository returns error",
			metrics: []model.Metrics{
				{
					ID:    "counter1",
					MType: model.Counter,
					Delta: func() *int64 { v := int64(100); return &v }(),
				},
			},
			wantErr:   true,
			errString: "failed to update all metrics",
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepository := mocks.NewMockRepo(ctrl)

			if tt.wantErr {
				// Ожидаем ошибку от репозитория
				mockRepository.EXPECT().
					SetAllMetrics(tt.metrics).
					Return(errors.New("database error")).AnyTimes()
			} else {
				// Ожидаем успешное выполнение
				var expectedMetrics []model.Metrics
				if tt.metrics != nil {
					expectedMetrics = tt.metrics
				} else {
					expectedMetrics = []model.Metrics{} // nil преобразуется в пустой slice
				}

				mockRepository.EXPECT().
					SetAllMetrics(expectedMetrics).
					Return(nil).AnyTimes()
			}

			s := NewMetricService(mockRepository)
			err := s.UpdateAllMetrics(tt.metrics)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricService_UpdateAllMetrics_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("Test with metrics containing nil pointers", func(t *testing.T) {
		mockRepository := mocks.NewMockRepo(ctrl)

		metricsWithNil := []model.Metrics{
			{
				ID:    "counterNil",
				MType: model.Counter,
				Delta: nil, // nil pointer
			},
			{
				ID:    "gaugeNil",
				MType: model.Gauge,
				Value: nil, // nil pointer
			},
		}

		mockRepository.EXPECT().
			SetAllMetrics(metricsWithNil).
			Return(nil).
			Times(1)

		s := NewMetricService(mockRepository)
		err := s.UpdateAllMetrics(metricsWithNil)

		assert.NoError(t, err)
	})

	t.Run("Test with large batch of metrics", func(t *testing.T) {
		mockRepository := mocks.NewMockRepo(ctrl)

		// Создаем большое количество метрик для тестирования производительности
		var largeBatch []model.Metrics
		for i := 0; i < 1000; i++ {
			if i%2 == 0 {
				largeBatch = append(largeBatch, model.Metrics{
					ID:    fmt.Sprintf("counter%d", i),
					MType: model.Counter,
					Delta: func() *int64 { v := int64(i); return &v }(),
				})
			} else {
				largeBatch = append(largeBatch, model.Metrics{
					ID:    fmt.Sprintf("gauge%d", i),
					MType: model.Gauge,
					Value: func() *float64 { v := float64(i) + 0.5; return &v }(),
				})
			}
		}

		mockRepository.EXPECT().
			SetAllMetrics(largeBatch).
			Return(nil).
			Times(1)

		s := NewMetricService(mockRepository)
		err := s.UpdateAllMetrics(largeBatch)

		assert.NoError(t, err)
	})
}

func TestMetricService_UpdateAllMetrics_IntegrationWithOtherMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("Test UpdateAllMetrics followed by GetAllMetrics", func(t *testing.T) {
		mockRepository := mocks.NewMockRepo(ctrl)

		metricsToUpdate := []model.Metrics{
			{
				ID:    "testCounter",
				MType: model.Counter,
				Delta: func() *int64 { v := int64(42); return &v }(),
			},
			{
				ID:    "testGauge",
				MType: model.Gauge,
				Value: func() *float64 { v := float64(99.9); return &v }(),
			},
		}

		// Ожидаем вызов SetAllMetrics
		mockRepository.EXPECT().
			SetAllMetrics(metricsToUpdate).
			Return(nil).
			Times(1)

		// Ожидаем вызов GetAllMetrics после обновления
		mockRepository.EXPECT().
			GetAllMetrics().
			Return(metricsToUpdate, nil).
			Times(1)

		s := NewMetricService(mockRepository)

		// Обновляем метрики
		err := s.UpdateAllMetrics(metricsToUpdate)
		assert.NoError(t, err)

		// Проверяем, что метрики были обновлены
		retrievedMetrics, err := s.GetAllMetrics()
		assert.NoError(t, err)
		assert.Equal(t, metricsToUpdate, retrievedMetrics)
	})
}
