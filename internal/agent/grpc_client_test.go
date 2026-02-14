// Path: nternal/agent/grpc_client_test.go
package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/rompil2/metrics_aggregator/api"
	"github.com/rompil2/metrics_aggregator/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

// MockMetricsClientInterface implements MetricsClientInterface for testing purposes.
// Using testify/mock to simplify mock creation.
type MockMetricsClientInterface struct {
	mock.Mock
}

// UpdateMetrics implements the interface method. It calls testify.mock.
func (m *MockMetricsClientInterface) UpdateMetrics(ctx context.Context, in *api.UpdateMetricsRequest, opts ...grpc.CallOption) (*api.UpdateMetricsResponse, error) {
	// Args: context, *UpdateMetricsRequest, ...opts
	args := m.Called(ctx, in, opts)
	// Return results configured in the test
	return args.Get(0).(*api.UpdateMetricsResponse), args.Error(1)
}

func TestGRPCClient_SendMetrics_Success(t *testing.T) {
	// 1. Setup
	mockClient := new(MockMetricsClientInterface)
	conn := &grpc.ClientConn{} // Can use nil or a real conn, but it's not used directly here.
	client := NewGRPCClientWithInterface(mockClient, conn, "some_addr")

	expectedResponse := &api.UpdateMetricsResponse{}
	// Configure mock: on UpdateMetrics call with any args, return expectedResponse and nil error.
	mockClient.On("UpdateMetrics", mock.Anything, mock.AnythingOfType("*api.UpdateMetricsRequest"), mock.Anything).Return(expectedResponse, nil)

	metricsToSend := []model.Metrics{
		{ID: "test_counter", MType: model.Counter, Delta: ptr(int64(42))},
		{ID: "test_gauge", MType: model.Gauge, Value: ptr(3.14159)},
	}

	// 2. Execution
	err := client.SendMetrics(context.Background(), metricsToSend)

	// 3. Verification
	assert.NoError(t, err)
	// Assert that the mock was called as expected
	mockClient.AssertExpectations(t)
}

func TestGRPCClient_SendMetrics_ServerError(t *testing.T) {
	// 1. Setup
	mockClient := new(MockMetricsClientInterface)
	conn := &grpc.ClientConn{}
	client := NewGRPCClientWithInterface(mockClient, conn, "some_addr")

	expectedError := errors.New("server error")
	// Configure mock: on UpdateMetrics call, return nil Response and the expected error
	mockClient.On("UpdateMetrics", mock.Anything, mock.AnythingOfType("*api.UpdateMetricsRequest"), mock.Anything).Return((*api.UpdateMetricsResponse)(nil), expectedError)

	metricsToSend := []model.Metrics{
		{ID: "test_counter", MType: model.Counter, Delta: ptr(int64(1))},
	}

	// 2. Execution
	err := client.SendMetrics(context.Background(), metricsToSend)

	// 3. Verification
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	// Assert that the mock was called as expected
	mockClient.AssertExpectations(t)
}

// ptr is a helper function to get a pointer
func ptr[T any](v T) *T {
	return &v
}
