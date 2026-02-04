// Package audit provides functionality for auditing successful metric operations.
// path: internal/audit/file_observer.go
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FileAuditObserver handles audit events by appending them as JSON lines to a specified file.
// It is safe for concurrent use and ensures atomic writes per event using a mutex.
type FileAuditObserver struct {
	filePath string
	mutex    sync.Mutex
}

// NewFileAuditObserver creates a new FileAuditObserver that writes audit events to the given file path.
// The file will be created if it does not exist, and events are appended as newline-delimited JSON records.
func NewFileAuditObserver(filePath string) *FileAuditObserver {
	return &FileAuditObserver{
		filePath: filePath,
	}
}

// Notify appends the provided audit event as a JSON-encoded line to the configured file.
// It uses a mutex to ensure thread-safe access to the file and handles file opening/closing on each call.
// Returns an error if file operations or JSON marshaling fail.
func (f *FileAuditObserver) Notify(ctx context.Context, event *AuditEvent) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	file, err := os.OpenFile(f.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}
	defer file.Close()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	_, err = file.WriteString(string(eventJSON) + "\n")
	if err != nil {
		return fmt.Errorf("failed to write audit event to file: %w", err)
	}

	return nil
}
