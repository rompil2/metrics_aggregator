package audit

import (
	"context"
)

// AuditEvent represents the structure of an audit event
type AuditEvent struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// AuditObserver defines the interface for audit event handlers
type AuditObserver interface {
	Notify(ctx context.Context, event *AuditEvent) error
}
