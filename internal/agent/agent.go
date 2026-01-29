// Package agent provides the core functionality for running a metrics collection and transmission agent.
// path: internal/agent/agent.go
package agent

import (
	"context"

	"sync"
)

const (
	metricsChSize = 10
)

// Runner defines the interface for components that can run in a background goroutine
// and process metrics from a channel until the context is cancelled.
type Runner interface {
	Run(ctx context.Context, ch chan map[string]any)
}

// MetricsCollector is an alias for Runner, representing a component that gathers system or application metrics
// and sends them to a shared channel for further processing or transmission.
type MetricsCollector interface {
	Runner
}

// MetricsSender is an alias for Runner, representing a component that consumes metrics from a channel
// and transmits them to a remote server (e.g., via HTTP).
type MetricsSender interface {
	Runner
}

// Agent orchestrates metric collection and transmission by running a collector and a sender concurrently.
// It connects the two components via an internal buffered channel and manages their lifecycle using a WaitGroup.
type Agent struct {
	collector MetricsCollector
	client    MetricsSender
}

// New creates a new Agent instance with the provided collector and sender.
// The collector gathers metrics, while the sender transmits them to a remote endpoint.
func New(collector MetricsCollector, client MetricsSender) *Agent {
	return &Agent{
		collector: collector,
		client:    client,
	}
}

// Run starts the agent's main loop, launching collector and sender goroutines connected by a metrics channel.
// It blocks until both components finish (typically when the context is cancelled) and returns nil on completion.
func (a *Agent) Run(ctx context.Context) error {
	metricsCh := make(chan map[string]any, metricsChSize)
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
