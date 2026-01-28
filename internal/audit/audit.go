// path: internal/audit/audit.go
package audit

import (
	"context"
)

// AuditEvent represents a structured record of a successful metrics operation,
// including the Unix timestamp, list of affected metric IDs, and the client's IP address.
// It is serialized as JSON for audit logging to files or HTTP endpoints.
type AuditEvent struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// AuditObserver defines the interface for components that handle audit events.
// Implementations may write to files, send HTTP requests, or integrate with external audit systems.
// The Notify method must be safe for concurrent use and respect the provided context for cancellation.
type AuditObserver interface {
	Notify(ctx context.Context, event *AuditEvent) error
}
