package agent

import (
	"context"

	"sync"
)

type Runner interface {
	Run(ctx context.Context, ch chan map[string]any)
}

type Agent struct {
	collector Runner
	client    Runner
}

func New(collector Runner, client Runner) *Agent {
	return &Agent{
		collector: collector,
		client:    client,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	metricsCh := make(chan map[string]any, 10)
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
