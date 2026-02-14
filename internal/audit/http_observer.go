// Package audit provides functionality for auditing successful metric operations.
// path: internal/audit/http_observer.go
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// HTTPAuditObserver handles audit events by sending them to a remote HTTP endpoint via POST requests.
// It serializes each AuditEvent as JSON and expects a 2xx response status to consider the notification successful.
type HTTPAuditObserver struct {
	url string
}

// NewHTTPAuditObserver creates a new HTTPAuditObserver that sends audit events to the specified URL.
// The URL must be a valid absolute HTTP or HTTPS endpoint.
func NewHTTPAuditObserver(url string) *HTTPAuditObserver {
	return &HTTPAuditObserver{
		url: url,
	}
}

// Notify sends the given audit event to the configured HTTP endpoint as a JSON payload.
// It uses the provided context for request cancellation and timeout control.
// Returns an error if serialization, network transmission, or HTTP status indicates failure.
func (h *HTTPAuditObserver) Notify(ctx context.Context, event *AuditEvent) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.url, strings.NewReader(string(eventJSON)))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send audit event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received non-success status code: %d", resp.StatusCode)
	}

	return nil
}
