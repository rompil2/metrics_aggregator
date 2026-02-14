// Package server also implements the gRPC Metrics service.
// It handles incoming metric updates and stores them using the underlying service.
// Path: internal/server/grpc_server.go
package server

import (
	"context"
	"net"
	"net/netip"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/rompil2/metrics_aggregator/api"
	"github.com/rompil2/metrics_aggregator/internal/model"
)

type MetricService interface {
	UpdateAllMetrics(metrics []model.Metrics) error
}

// GRPCServer implements the gRPC Metrics service for handling metric updates.
type GRPCServer struct {
	api.UnimplementedMetricsServer
	service       MetricService
	trustedSubnet string
}

// NewGRPCServer creates a new instance of GRPCServer with the provided service and trusted subnet.
func NewGRPCServer(service MetricService, trustedSubnet string) *GRPCServer {
	return &GRPCServer{
		service:       service,
		trustedSubnet: trustedSubnet,
	}
}

// UpdateMetrics receives a batch of metrics via gRPC and stores them using the underlying service.
// It verifies the client's IP address against the trusted subnet if configured.
func (s *GRPCServer) UpdateMetrics(ctx context.Context, req *api.UpdateMetricsRequest) (*api.UpdateMetricsResponse, error) {
	// Проверка IP-адреса через метаданные
	if err := s.checkIP(ctx); err != nil {
		return nil, err
	}

	// Преобразование proto-метрик в модель
	metrics := make([]model.Metrics, 0, len(req.Metrics))
	for _, m := range req.Metrics {
		modelMetric := model.Metrics{
			ID:    m.Id,
			MType: mTypeToString(m.Type),
		}

		switch m.Type {
		case api.Metric_COUNTER:
			modelMetric.Delta = &m.Delta
		case api.Metric_GAUGE:
			modelMetric.Value = &m.Value
		}

		metrics = append(metrics, modelMetric)
	}

	// Обновление метрик
	if err := s.service.UpdateAllMetrics(metrics); err != nil {
		return nil, status.Error(codes.Internal, "failed to update metrics")
	}

	return &api.UpdateMetricsResponse{}, nil
}

func (s *GRPCServer) checkIP(ctx context.Context) error {
	if s.trustedSubnet == "" {
		return nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.PermissionDenied, "metadata required")
	}

	ipList := md["x-real-ip"]
	if len(ipList) == 0 {
		return status.Error(codes.PermissionDenied, "x-real-ip header required")
	}

	ipStr := ipList[0]
	_, err := netip.ParseAddr(ipStr)
	if err != nil {
		return status.Error(codes.PermissionDenied, "invalid IP address")
	}

	_, trustedNet, err := net.ParseCIDR(s.trustedSubnet)
	if err != nil {
		return status.Error(codes.Internal, "invalid trusted subnet")
	}

	if !trustedNet.Contains(net.ParseIP(ipStr)) {
		return status.Error(codes.PermissionDenied, "IP is not in trusted subnet")
	}

	return nil
}

func mTypeToString(mType api.Metric_MType) string {
	if mType == api.Metric_COUNTER {
		return model.Counter
	}
	return model.Gauge
}

// StartGRPCServer initializes and starts a gRPC server listening on the specified address.
// It applies an IP check interceptor if a trusted subnet is provided.
func StartGRPCServer(addr, trustedSubnet string, svc MetricService) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(IPCheckUnaryInterceptor(trustedSubnet)))
	api.RegisterMetricsServer(grpcServer, NewGRPCServer(svc, trustedSubnet))

	return grpcServer.Serve(lis)
}

// IPCheckUnaryInterceptor returns a gRPC unary interceptor that checks if the client's IP address
// is within the trusted subnet specified. If the subnet is empty, the check is skipped.
// The IP address must be provided in the 'x-real-ip' metadata key.
func IPCheckUnaryInterceptor(trustedSubnet string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if trustedSubnet == "" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "metadata required")
		}

		ipList := md["x-real-ip"]
		if len(ipList) == 0 {
			return nil, status.Error(codes.PermissionDenied, "x-real-ip header required")
		}

		ipStr := ipList[0]
		_, err := netip.ParseAddr(ipStr)
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, "invalid IP address")
		}

		_, trustedNet, err := net.ParseCIDR(trustedSubnet)
		if err != nil {
			return nil, status.Error(codes.Internal, "invalid trusted subnet")
		}

		if !trustedNet.Contains(net.ParseIP(ipStr)) {
			return nil, status.Error(codes.PermissionDenied, "IP is not in trusted subnet")
		}

		return handler(ctx, req)
	}
}
