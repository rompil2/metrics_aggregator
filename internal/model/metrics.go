// path: internal/model
package model

// Counter represents the metric type for monotonically increasing integer counters.
const Counter = "counter"

// Gauge represents the metric type for arbitrary floating-point values that can increase or decrease.
const Gauge = "gauge"

// Metrics represents a single metric entity with an ID, type, and value.
// It supports two types: "counter" (with Delta as *int64) and "gauge" (with Value as *float64).
// The Hash field is used for integrity validation in distributed scenarios.
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}
