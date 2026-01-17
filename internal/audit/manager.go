package audit

import (
	"context"
	"log"
	"sync"
)

// AuditManager manages audit observers
type AuditManager struct {
	observers []AuditObserver
}

// NewAuditManager creates a new AuditManager
func NewAuditManager(observers []AuditObserver) *AuditManager {
	return &AuditManager{
		observers: observers,
	}
}

// AddObserver adds a new observer
func (am *AuditManager) AddObserver(observer AuditObserver) {
	am.observers = append(am.observers, observer)
}

// NotifyAll notifies all observers
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
