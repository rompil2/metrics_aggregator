package agent

import (
	"context"
	"math/rand/v2"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type Collector struct {
	pollCount    int64
	randomValue  float64
	pollInterval time.Duration
}

func NewCollector(pollInterval time.Duration) *Collector {
	return &Collector{
		pollInterval: pollInterval,
	}
}

// Run implements agent.Collector.
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
	v, _ := mem.VirtualMemory()
	metrics["TotalMemory"] = float64(v.Total)
	metrics["FreeMemory"] = float64(v.Free)
	cs, _ := cpu.Percent(r.pollInterval, true)
	for i, utilPerCore := range cs {
		key := "CPUutilization" + strconv.Itoa(i)
		metrics[key] = utilPerCore
	}
	return metrics
}
