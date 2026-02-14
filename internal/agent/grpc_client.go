// Package agent provides methods for sending metrics to a gRPC server.
// It implements the client-side logic for interacting with the gRPC service.
// Path: internal/agent/grpc_client.go
package agent

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/rompil2/metrics_aggregator/api"
	"github.com/rompil2/metrics_aggregator/internal/model"
)

// MetricsClientInterface defines the gRPC client interface for sending metrics.
// This allows the GRPCClient to depend on an abstraction rather than a concrete implementation.
type MetricsClientInterface interface {
	UpdateMetrics(ctx context.Context, in *api.UpdateMetricsRequest, opts ...grpc.CallOption) (*api.UpdateMetricsResponse, error)
}

// GRPCClient provides methods for sending metrics to a gRPC server.
// It depends on the MetricsClientInterface to allow for easier testing and flexibility.
type GRPCClient struct {
	conn   *grpc.ClientConn
	client MetricsClientInterface // Now depends on interface
	addr   string
}

// NewGRPCClient creates a new gRPC client connected to the specified address.
// It uses the generated api.MetricsClient as the concrete implementation.
func NewGRPCClient(addr string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// Create the concrete client implementing MetricsClientInterface
	concreteClient := api.NewMetricsClient(conn)

	return &GRPCClient{
		conn:   conn,
		client: concreteClient, // Pass the concrete implementation
		addr:   addr,
	}, nil
}

// NewGRPCClientWithInterface creates a new gRPC client using a provided MetricsClientInterface.
// This constructor is useful for testing where you might want to inject a mock implementation.
func NewGRPCClientWithInterface(client MetricsClientInterface, conn *grpc.ClientConn, addr string) *GRPCClient {
	return &GRPCClient{
		conn:   conn,
		client: client, // Use the passed interface
		addr:   addr,
	}
}

// Close closes the underlying gRPC connection.
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendMetrics sends a batch of metrics to the gRPC server.
// It includes the client's IP address in the metadata for server-side verification.
func (c *GRPCClient) SendMetrics(ctx context.Context, metrics []model.Metrics) error {
	// Convert model to proto
	protoMetrics := make([]*api.Metric, 0, len(metrics))
	for _, m := range metrics {
		protoMetric := &api.Metric{
			Id: m.ID,
			// Convert metric type
			Type: mTypeToProto(m.MType),
		}

		if m.Delta != nil {
			protoMetric.Delta = *m.Delta
		}
		if m.Value != nil {
			protoMetric.Value = *m.Value
		}

		protoMetrics = append(protoMetrics, protoMetric)
	}

	req := &api.UpdateMetricsRequest{
		Metrics: protoMetrics,
	}

	// Add IP address to metadata
	localIP, err := getLocalIP()
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "x-real-ip", localIP)

	_, err = c.client.UpdateMetrics(ctx, req) // Call through interface
	if err != nil {
		return err
	}

	return nil
}

func mTypeToProto(mType string) api.Metric_MType {
	if mType == model.Counter {
		return api.Metric_COUNTER
	}
	return api.Metric_GAUGE
}

func getLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
