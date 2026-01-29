// Package agent provides a system metrics collector and sender for the metrics aggregator.
// path: internal/agent/collector.go
package agent

import (
	"context"
	"log"
	"math/rand/v2"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// Collector gathers system and runtime metrics at regular intervals,
// including Go runtime statistics, memory usage, CPU utilization per core, and a random value.
type Collector struct {
	pollCount    int64
	randomValue  float64
	pollInterval time.Duration
}

// NewCollector creates a new Collector instance that polls metrics every pollInterval duration.
func NewCollector(pollInterval time.Duration) *Collector {
	return &Collector{
		pollInterval: pollInterval,
	}
}

// Run starts the metric collection loop, sending gathered metrics to the provided channel
// at each polling interval until the context is cancelled. It is intended to be run in a separate goroutine.
func (r *Collector) Run(ctx context.Context, ch chan map[string]any) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()
LOOP:
	for {
		select {
		case <-ctx.Done():
			break LOOP
		case <-ticker.C:
			ch <- r.Poll()
		}
	}
}

// Poll collects a snapshot of current system and runtime metrics,
// including Go memory statistics, total/free memory, per-core CPU utilization, and a random value.
// It is safe to call concurrently and returns a new map on each invocation.
func (r *Collector) Poll() map[string]any {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats) // Get memory stats

	metrics := make(map[string]interface{})
	r.randomValue = rand.Float64()
	metrics["RandomValue"] = r.randomValue
	// Acquire gauge metrics from runtime
	metrics["Alloc"] = float64(memStats.Alloc)
	metrics["BuckHashSys"] = float64(memStats.BuckHashSys)
	metrics["Frees"] = float64(memStats.Frees)
	metrics["GCCPUFraction"] = memStats.GCCPUFraction
	metrics["GCSys"] = float64(memStats.GCSys)
	metrics["HeapAlloc"] = float64(memStats.HeapAlloc)
	metrics["HeapIdle"] = float64(memStats.HeapIdle)
	metrics["HeapInuse"] = float64(memStats.HeapInuse)
	metrics["HeapObjects"] = float64(memStats.HeapObjects)
	metrics["HeapReleased"] = float64(memStats.HeapReleased)
	metrics["HeapSys"] = float64(memStats.HeapSys)
	metrics["LastGC"] = float64(memStats.LastGC)
	metrics["Lookups"] = float64(memStats.Lookups)
	metrics["MCacheInuse"] = float64(memStats.MCacheInuse)
	metrics["MCacheSys"] = float64(memStats.MCacheSys)
	metrics["MSpanInuse"] = float64(memStats.MSpanInuse)
	metrics["MSpanSys"] = float64(memStats.MSpanSys)
	metrics["Mallocs"] = float64(memStats.Mallocs)
	metrics["NextGC"] = float64(memStats.NextGC)
	metrics["NumForcedGC"] = float64(memStats.NumForcedGC)
	metrics["NumGC"] = float64(memStats.NumGC)
	metrics["OtherSys"] = float64(memStats.OtherSys)
	metrics["PauseTotalNs"] = float64(memStats.PauseTotalNs)
	metrics["StackInuse"] = float64(memStats.StackInuse)
	metrics["StackSys"] = float64(memStats.StackSys)
	metrics["Sys"] = float64(memStats.Sys)
	metrics["TotalAlloc"] = float64(memStats.TotalAlloc)

	// Extra metrics
	atomic.AddInt64(&r.pollCount, 1)
	metrics["PollCount"] = atomic.LoadInt64(&r.pollCount)

	if v, err := mem.VirtualMemory(); err != nil {
		log.Printf("Failed to get memory stats: %v", err)
	} else {
		metrics["TotalMemory"] = float64(v.Total)
		metrics["FreeMemory"] = float64(v.Free)
		cs, _ := cpu.Percent(r.pollInterval, true)
		for i, utilPerCore := range cs {
			key := "CPUutilization" + strconv.Itoa(i)
			metrics[key] = utilPerCore
		}
	}
	return metrics
}
