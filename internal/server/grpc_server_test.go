// internal/server/grpc_server_test.go

package server

import (
	"context"
	"errors"
	"testing"

	"github.com/rompil2/metrics_aggregator/api"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ✅ MockMetricService реализует интерфейс service.Service для целей тестирования.
type MockMetricService struct {
	mock.Mock
}

func (m *MockMetricService) UpdateMetrics(metric *model.Metrics) error {
	args := m.Called(metric)
	return args.Error(0)
}

func (m *MockMetricService) GetMetrics(ID string) (model.Metrics, error) {
	args := m.Called(ID)
	return args.Get(0).(model.Metrics), args.Error(1)
}

func (m *MockMetricService) UpdateAllMetrics(ms []model.Metrics) error {
	args := m.Called(ms)
	return args.Error(0)
}

func (m *MockMetricService) GetAllMetrics() ([]model.Metrics, error) {
	args := m.Called()
	return args.Get(0).([]model.Metrics), args.Error(1)
}

func (m *MockMetricService) Ping() error {
	args := m.Called()
	return args.Error(0)
}

// TestGRPCServer_UpdateMetrics_Success тестирует успешное обновление метрик.
func TestGRPCServer_UpdateMetrics_Success(t *testing.T) {
	// 1. Подготовка
	mockService := new(MockMetricService)
	server := NewGRPCServer(mockService, "") // ✅ Передаём интерфейс

	req := &api.UpdateMetricsRequest{
		Metrics: []*api.Metric{
			{
				Id:    "test_counter",
				Type:  api.Metric_COUNTER,
				Delta: 42,
			},
			{
				Id:    "test_gauge",
				Type:  api.Metric_GAUGE,
				Value: 3.14,
			},
		},
	}

	// Ожидаем, что UpdateAllMetrics будет вызван с правильным списком метрик
	expectedModelMetrics := []model.Metrics{
		{ID: "test_counter", MType: model.Counter, Delta: ptr(int64(42))},
		{ID: "test_gauge", MType: model.Gauge, Value: ptr(3.14)},
	}
	mockService.On("UpdateAllMetrics", expectedModelMetrics).Return(nil)

	// 2. Выполнение
	ctx := context.Background()
	// Добавляем метаданные, так как UpdateMetrics вызывает checkIP, который проверит их при наличии trustedSubnet.
	// Но в данном случае trustedSubnet пустой, проверка не пройдёт.
	// Однако, если trustedSubnet не пустой, нам нужно добавить x-real-ip.
	// Для этого теста (с пустым trustedSubnet) метаданные не нужны.
	resp, err := server.UpdateMetrics(ctx, req)

	// 3. Проверка
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	mockService.AssertExpectations(t)
}

// TestGRPCServer_UpdateMetrics_ServiceError тестирует сценарий, когда сервис возвращает ошибку.
func TestGRPCServer_UpdateMetrics_ServiceError(t *testing.T) {
	// 1. Подготовка
	mockService := new(MockMetricService)
	server := NewGRPCServer(mockService, "") // ✅ Передаём интерфейс

	req := &api.UpdateMetricsRequest{
		Metrics: []*api.Metric{
			{
				Id:    "test_counter",
				Type:  api.Metric_COUNTER,
				Delta: 1,
			},
		},
	}

	// Ожидаем, что UpdateAllMetrics вернет ошибку
	expectedError := errors.New("service error")
	mockService.On("UpdateAllMetrics", mock.AnythingOfType("[]model.Metrics")).Return(expectedError)

	// 2. Выполнение
	ctx := context.Background()
	resp, err := server.UpdateMetrics(ctx, req)

	// 3. Проверка
	assert.Error(t, err)
	assert.Nil(t, resp)
	// Проверяем, что ошибка является gRPC-ошибкой с кодом Internal
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, s.Code())
	assert.Contains(t, s.Message(), "failed to update metrics")
	mockService.AssertExpectations(t)
}

// TestGRPCServer_checkIP_NoSubnet тестирует, что проверка IP пропускается, если подсеть не установлена.
func TestGRPCServer_checkIP_NoSubnet(t *testing.T) {
	server := NewGRPCServer(nil, "") // trustedSubnet пустой
	err := server.checkIP(context.Background())
	assert.NoError(t, err)
}

// TestGRPCServer_checkIP_NoMetadata_WithSubnet тестирует, что возвращается ошибка, если метаданные отсутствуют и подсеть установлена.
func TestGRPCServer_checkIP_NoMetadata_WithSubnet(t *testing.T) {
	server := NewGRPCServer(nil, "192.168.1.0/24") // trustedSubnet НЕ пустой
	ctx := context.Background()                    // Без метаданных
	err := server.checkIP(ctx)
	assert.Error(t, err)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, s.Code())
	assert.Equal(t, "metadata required", s.Message())
}

// TestGRPCServer_checkIP_NoIPHeader_WithSubnet тестирует, что возвращается ошибка, если заголовок x-real-ip отсутствует.
func TestGRPCServer_checkIP_NoIPHeader_WithSubnet(t *testing.T) {
	server := NewGRPCServer(nil, "192.168.1.0/24")                          // trustedSubnet НЕ пустой
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{}) // Пустые метаданные
	err := server.checkIP(ctx)
	assert.Error(t, err)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, s.Code())
	assert.Equal(t, "x-real-ip header required", s.Message())
}

// TestGRPCServer_checkIP_InvalidIP_WithSubnet тестирует, что возвращается ошибка, если IP-адрес недействителен.
func TestGRPCServer_checkIP_InvalidIP_WithSubnet(t *testing.T) {
	server := NewGRPCServer(nil, "192.168.1.0/24") // trustedSubnet НЕ пустой
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"x-real-ip": []string{"invalid_ip"}})
	err := server.checkIP(ctx)
	assert.Error(t, err)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, s.Code())
	assert.Equal(t, "invalid IP address", s.Message())
}

// TestGRPCServer_checkIP_NotInSubnet_WithSubnet тестирует, что возвращается ошибка, если IP не входит в доверенную подсеть.
func TestGRPCServer_checkIP_NotInSubnet_WithSubnet(t *testing.T) {
	server := NewGRPCServer(nil, "192.168.1.0/24") // trustedSubnet НЕ пустой
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"x-real-ip": []string{"10.0.0.1"}})
	err := server.checkIP(ctx)
	assert.Error(t, err)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, s.Code())
	assert.Equal(t, "IP is not in trusted subnet", s.Message())
}

// TestGRPCServer_checkIP_Valid_WithSubnet тестирует успешную проверку IP.
func TestGRPCServer_checkIP_Valid_WithSubnet(t *testing.T) {
	server := NewGRPCServer(nil, "192.168.1.0/24") // trustedSubnet НЕ пустой
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"x-real-ip": []string{"192.168.1.100"}})
	err := server.checkIP(ctx)
	assert.NoError(t, err)
}

// TestIPCheckUnaryInterceptor_NoSubnet тестирует, что интерцептор пропускает вызов, если подсеть не установлена.
func TestIPCheckUnaryInterceptor_NoSubnet(t *testing.T) {
	interceptor := IPCheckUnaryInterceptor("")

	// Простой обработчик, который ничего не делает
	testHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	resp, err := interceptor(context.Background(), nil, nil, testHandler)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp)
}

// TestIPCheckUnaryInterceptor_NoMetadata_WithSubnet тестирует, что интерцептор возвращает ошибку, если метаданные отсутствуют.
func TestIPCheckUnaryInterceptor_NoMetadata_WithSubnet(t *testing.T) {
	interceptor := IPCheckUnaryInterceptor("192.168.1.0/24")

	// Простой обработчик, который не должен быть вызван
	testHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	resp, err := interceptor(context.Background(), nil, nil, testHandler) // Контекст без метаданных
	assert.Error(t, err)
	assert.Nil(t, resp)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, s.Code())
	assert.Equal(t, "metadata required", s.Message())
}

// TestIPCheckUnaryInterceptor_NoIPHeader_WithSubnet тестирует, что интерцептор возвращает ошибку, если заголовок x-real-ip отсутствует.
func TestIPCheckUnaryInterceptor_NoIPHeader_WithSubnet(t *testing.T) {
	interceptor := IPCheckUnaryInterceptor("192.168.1.0/24")

	// Простой обработчик, который не должен быть вызван
	testHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{}) // Пустые метаданные
	resp, err := interceptor(ctx, nil, nil, testHandler)
	assert.Error(t, err)
	assert.Nil(t, resp)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, s.Code())
	assert.Equal(t, "x-real-ip header required", s.Message())
}

// TestIPCheckUnaryInterceptor_InvalidIP_WithSubnet тестирует, что интерцептор возвращает ошибку, если IP-адрес недействителен.
func TestIPCheckUnaryInterceptor_InvalidIP_WithSubnet(t *testing.T) {
	interceptor := IPCheckUnaryInterceptor("192.168.1.0/24")

	// Простой обработчик, который не должен быть вызван
	testHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"x-real-ip": []string{"invalid_ip"}})
	resp, err := interceptor(ctx, nil, nil, testHandler)
	assert.Error(t, err)
	assert.Nil(t, resp)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, s.Code())
	assert.Equal(t, "invalid IP address", s.Message())
}

// TestIPCheckUnaryInterceptor_NotInSubnet_WithSubnet тестирует, что интерцептор возвращает ошибку, если IP не входит в доверенную подсеть.
func TestIPCheckUnaryInterceptor_NotInSubnet_WithSubnet(t *testing.T) {
	interceptor := IPCheckUnaryInterceptor("192.168.1.0/24")

	// Простой обработчик, который не должен быть вызван
	testHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"x-real-ip": []string{"10.0.0.1"}})
	resp, err := interceptor(ctx, nil, nil, testHandler)
	assert.Error(t, err)
	assert.Nil(t, resp)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, s.Code())
	assert.Equal(t, "IP is not in trusted subnet", s.Message())
}

// TestIPCheckUnaryInterceptor_Valid_WithSubnet тестирует успешную проверку IP в интерцепторе.
func TestIPCheckUnaryInterceptor_Valid_WithSubnet(t *testing.T) {
	interceptor := IPCheckUnaryInterceptor("192.168.1.0/24")

	// Простой обработчик, который должен быть вызван
	testHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"x-real-ip": []string{"192.168.1.100"}})
	resp, err := interceptor(ctx, nil, nil, testHandler)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp)
}

// ptr вспомогательная функция для получения указателя
func ptr[T any](v T) *T {
	return &v
}
