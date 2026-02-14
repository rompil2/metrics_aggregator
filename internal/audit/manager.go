// Package audit provides functionality for managing audit observers and broadcasting audit events.
// path: internal/audit/manager.go
package audit

import (
	"context"
	"log"
	"sync"
)

// AuditManager manages a list of audit observers and broadcasts audit events to them concurrently.
type AuditManager struct {
	observers []AuditObserver
}

// NewAuditManager creates a new AuditManager instance with the provided list of observers.
// The manager will notify all observers in parallel when an audit event occurs.
func NewAuditManager(observers []AuditObserver) *AuditManager {
	return &AuditManager{
		observers: observers,
	}
}

// AddObserver appends a new audit observer to the manager's list.
// This method is not thread-safe and should be called during initialization only.
func (am *AuditManager) AddObserver(observer AuditObserver) {
	am.observers = append(am.observers, observer)
}

// NotifyAll sends the given audit event to all registered observers concurrently.
// Errors from individual observers are logged but do not stop notification of others.
// This method is safe to call from multiple goroutines.
func (am *AuditManager) NotifyAll(ctx context.Context, event *AuditEvent) {
	var wg sync.WaitGroup
	errCh := make(chan error, len(am.observers))

	for _, observer := range am.observers {
		wg.Add(1)
		go func(obs AuditObserver) {
			defer wg.Done()
			if err := obs.Notify(ctx, event); err != nil {
				errCh <- err
			}
		}(observer)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		log.Printf("Audit error: %v", err)
	}
}
