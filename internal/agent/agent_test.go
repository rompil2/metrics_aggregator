package agent_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/agent"
	"github.com/stretchr/testify/assert"
)

type MockRunner struct {
	runFunc func(ctx context.Context, ch chan map[string]any)
}

func (m *MockRunner) Run(ctx context.Context, ch chan map[string]any) {
	if m.runFunc != nil {
		m.runFunc(ctx, ch)
	}
}

func TestAgent_Run_NormalExecution(t *testing.T) {
	t.Parallel()

	collectorCalled := false
	clientCalled := false

	collector := &MockRunner{
		runFunc: func(ctx context.Context, ch chan map[string]any) {
			collectorCalled = true
			ch <- map[string]any{"test": 123}
		},
	}

	client := &MockRunner{
		runFunc: func(ctx context.Context, ch chan map[string]any) {
			clientCalled = true
			metrics := <-ch
			assert.Equal(t, 123, metrics["test"])
		},
	}

	a := agent.New(collector, client)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := a.Run(ctx)
	assert.NoError(t, err)
	assert.True(t, collectorCalled, "Collector should be called")
	assert.True(t, clientCalled, "Client should be called")
}

func TestAgent_Run_ContextCancellation(t *testing.T) {
	t.Parallel()

	collector := &MockRunner{
		runFunc: func(ctx context.Context, ch chan map[string]any) {
			<-ctx.Done()
		},
	}

	client := &MockRunner{
		runFunc: func(ctx context.Context, ch chan map[string]any) {
			<-ctx.Done()
		},
	}

	a := agent.New(collector, client)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = a.Run(ctx)
	}()

	// wait for a while
	time.Sleep(50 * time.Millisecond)

	cancel()

	wg.Wait()

	assert.NoError(t, runErr)
}

func TestAgent_Run_ChannelCommunication(t *testing.T) {
	t.Parallel()

	metricsSent := []map[string]any{
		{"metric1": 1},
		{"metric2": 2},
		{"metric3": 3},
	}

	collector := &MockRunner{
		runFunc: func(ctx context.Context, ch chan map[string]any) {
			for _, m := range metricsSent {
				select {
				case ch <- m:
				case <-ctx.Done():
					return
				}
			}
		},
	}

	var metricsReceived []map[string]any
	var mu sync.Mutex

	client := &MockRunner{
		runFunc: func(ctx context.Context, ch chan map[string]any) {
			for {
				select {
				case m, ok := <-ch:
					if !ok {
						return
					}
					mu.Lock()
					metricsReceived = append(metricsReceived, m)
					mu.Unlock()
				case <-ctx.Done():
					return
				}
			}
		},
	}

	a := agent.New(collector, client)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := a.Run(ctx)
	assert.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, metricsSent, metricsReceived)
}

func TestAgent_Run_WithRealComponents(t *testing.T) {
	t.Parallel()

	collector := &MockRunner{
		runFunc: func(ctx context.Context, ch chan map[string]any) {
			ch <- map[string]any{"cpu": 42.5}
			ch <- map[string]any{"memory": 1024}
		},
	}

	var receivedMetrics []map[string]any
	var mu sync.Mutex

	client := &MockRunner{
		runFunc: func(ctx context.Context, ch chan map[string]any) {
			for {
				select {
				case m, ok := <-ch:
					if !ok {
						return
					}
					mu.Lock()
					receivedMetrics = append(receivedMetrics, m)
					mu.Unlock()
				case <-ctx.Done():
					return
				}
			}
		},
	}

	a := agent.New(collector, client)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := a.Run(ctx)
	assert.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, receivedMetrics, 2)
	assert.Contains(t, receivedMetrics[0], "cpu")
	assert.Contains(t, receivedMetrics[1], "memory")
}
