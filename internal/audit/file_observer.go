package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FileAuditObserver handles audit events by writing to a file
type FileAuditObserver struct {
	filePath string
	mutex    sync.Mutex
}

// NewFileAuditObserver creates a new FileAuditObserver
func NewFileAuditObserver(filePath string) *FileAuditObserver {
	return &FileAuditObserver{
		filePath: filePath,
	}
}

// Notify writes the audit event to the file
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
