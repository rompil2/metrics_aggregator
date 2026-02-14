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

type GRPCClient struct {
	conn   *grpc.ClientConn
	client api.MetricsClient
	addr   string
}

func NewGRPCClient(addr string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &GRPCClient{
		conn:   conn,
		client: api.NewMetricsClient(conn),
		addr:   addr,
	}, nil
}

func (c *GRPCClient) Close() error {
	return c.conn.Close()
}

func (c *GRPCClient) SendMetrics(ctx context.Context, metrics []model.Metrics) error {
	// Преобразование модели в proto
	protoMetrics := make([]*api.Metric, 0, len(metrics))
	for _, m := range metrics {
		protoMetric := &api.Metric{
			Id: m.ID,
			// Преобразование типа метрики
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

	// Добавление IP-адреса в метаданные
	localIP, err := getLocalIP()
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "x-real-ip", localIP)

	_, err = c.client.UpdateMetrics(ctx, req)
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
