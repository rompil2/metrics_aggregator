package agent

import (
	"context"

	"sync"
)

const (
	METRICS_CH_SIZE = 10
)

type Runner interface {
	Run(ctx context.Context, ch chan map[string]any)
}

type MetricsCollector interface {
	Runner
}

type MetricsSender interface {
	Runner
}

type Agent struct {
	collector MetricsCollector
	client    MetricsSender
}

func New(collector MetricsCollector, client MetricsSender) *Agent {
	return &Agent{
		collector: collector,
		client:    client,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	metricsCh := make(chan map[string]any, METRICS_CH_SIZE)
	defer close(metricsCh)
	var wg sync.WaitGroup
	wg.Add(2)

	// Collect metrics
	go func() {
		defer wg.Done()
		a.collector.Run(ctx, metricsCh)
	}()

	// Send them to the server
	go func() {
		defer wg.Done()
		a.client.Run(ctx, metricsCh)
	}()

	wg.Wait()
	return nil
}
