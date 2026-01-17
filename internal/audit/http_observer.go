package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// HTTPAuditObserver handles audit events by sending to an HTTP endpoint
type HTTPAuditObserver struct {
	url string
}

// NewHTTPAuditObserver creates a new HTTPAuditObserver
func NewHTTPAuditObserver(url string) *HTTPAuditObserver {
	return &HTTPAuditObserver{
		url: url,
	}
}

// Notify sends the audit event via HTTP POST
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
