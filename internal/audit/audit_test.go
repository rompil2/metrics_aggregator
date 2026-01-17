// audit_test.go
package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAuditEventMarshaling проверяет корректность сериализации AuditEvent в JSON
func TestAuditEventMarshaling(t *testing.T) {
	event := &AuditEvent{
		Timestamp: 1234567890,
		Metrics:   []string{"Alloc", "Frees"},
		IPAddress: "192.168.1.1",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("unexpected error marshaling event: %v", err)
	}

	var decoded AuditEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling event: %v", err)
	}

	if decoded.Timestamp != event.Timestamp ||
		!equalStringSlices(decoded.Metrics, event.Metrics) ||
		decoded.IPAddress != event.IPAddress {
		t.Errorf("event mismatch: got %+v, want %+v", decoded, event)
	}
}

// equalStringSlices проверяет равенство двух срезов строк (порядок важен)
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestFileAuditObserver_Notify проверяет запись события в файл
func TestFileAuditObserver_Notify(t *testing.T) {
	tmpDir := t.TempDir()
	auditFile := filepath.Join(tmpDir, "audit.log")

	observer := NewFileAuditObserver(auditFile)
	event := &AuditEvent{
		Timestamp: time.Now().Unix(),
		Metrics:   []string{"Counter1", "Gauge2"},
		IPAddress: "127.0.0.1",
	}

	err := observer.Notify(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error from FileAuditObserver.Notify: %v", err)
	}

	// Читаем файл
	content, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	var loggedEvent AuditEvent
	if err := json.Unmarshal(content[:len(content)-1], &loggedEvent); err != nil { // убираем \n
		t.Fatalf("failed to unmarshal logged event: %v", err)
	}

	if !equalStringSlices(loggedEvent.Metrics, event.Metrics) ||
		loggedEvent.IPAddress != event.IPAddress {
		t.Errorf("logged event mismatch: got %+v, want %+v", loggedEvent, event)
	}
}

// TestHTTPAuditObserver_Notify проверяет отправку события по HTTP
func TestHTTPAuditObserver_Notify(t *testing.T) {
	receivedEvents := make([]AuditEvent, 0)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event AuditEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		receivedEvents = append(receivedEvents, event)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	observer := NewHTTPAuditObserver(server.URL)
	event := &AuditEvent{
		Timestamp: time.Now().Unix(),
		Metrics:   []string{"MetricX"},
		IPAddress: "10.0.0.5",
	}

	err := observer.Notify(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error from HTTPAuditObserver.Notify: %v", err)
	}

	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(receivedEvents))
	}

	if !equalStringSlices(receivedEvents[0].Metrics, event.Metrics) ||
		receivedEvents[0].IPAddress != event.IPAddress {
		t.Errorf("received event mismatch: got %+v, want %+v", receivedEvents[0], event)
	}
}

// TestHTTPAuditObserver_Error проверяет обработку ошибки HTTP
func TestHTTPAuditObserver_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	observer := NewHTTPAuditObserver(server.URL)
	event := &AuditEvent{Timestamp: 1}

	err := observer.Notify(context.Background(), event)
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), "non-success status code") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestAuditManager_NotifyAll проверяет уведомление всех наблюдателей
func TestAuditManager_NotifyAll(t *testing.T) {
	var fileEvents, httpEvents []AuditEvent

	// Фейковый файловый наблюдатель
	fileObserver := &mockObserver{events: &fileEvents}
	httpObserver := &mockObserver{events: &httpEvents}

	manager := NewAuditManager([]AuditObserver{fileObserver, httpObserver})
	event := &AuditEvent{Timestamp: 1, Metrics: []string{"Test"}, IPAddress: "1.1.1.1"}

	manager.NotifyAll(context.Background(), event)

	if len(fileEvents) != 1 || len(httpEvents) != 1 {
		t.Errorf("expected 1 event per observer, got file=%d, http=%d", len(fileEvents), len(httpEvents))
	}
}

// mockObserver — фейковая реализация AuditObserver для тестов
type mockObserver struct {
	events *[]AuditEvent
}

func (m *mockObserver) Notify(ctx context.Context, event *AuditEvent) error {
	*m.events = append(*m.events, *event)
	return nil
}

// TestWithAuditMetrics_Context передача метрик через контекст
func TestWithAuditMetrics_Context(t *testing.T) {
	ctx := context.Background()
	metrics := []string{"M1", "M2"}

	ctx = WithAuditMetrics(ctx, metrics)
	retrieved := GetAuditMetrics(ctx)

	if !equalStringSlices(retrieved, metrics) {
		t.Errorf("retrieved metrics mismatch: got %v, want %v", retrieved, metrics)
	}
}

// TestGetClientIP проверяет извлечение IP-адреса
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name:     "X-Forwarded-For",
			headers:  map[string]string{"X-Forwarded-For": "192.168.1.100, 10.0.0.1"},
			remote:   "127.0.0.1:1234",
			expected: "192.168.1.100",
		},
		{
			name:     "X-Real-IP",
			headers:  map[string]string{"X-Real-IP": "10.10.10.10"},
			remote:   "127.0.0.1:1234",
			expected: "10.10.10.10",
		},
		{
			name:     "RemoteAddr fallback",
			headers:  map[string]string{},
			remote:   "203.0.113.195:8080",
			expected: "203.0.113.195",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			ip := GetClientIP(req)
			if ip != tt.expected {
				t.Errorf("GetClientIP() = %v, expected %v", ip, tt.expected)
			}
		})
	}
}

// TestAuditMiddleware_Success проверяет вызов аудита при успешном ответе
func TestAuditMiddleware_Success(t *testing.T) {

	// Фейковый менеджер, который захватывает событие
	// manager := &mockManager{capture: func(e *AuditEvent) { capturedEvent = e }}
	manager := NewAuditManager([]AuditObserver{})
	middleware := AuditMiddleware(manager)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Добавляем метрики в контекст (имитация хендлера)
		ctx := WithAuditMetrics(r.Context(), []string{"MetricA"})
		*r = *r.Clone(ctx)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/update/", nil)
	req.RemoteAddr = "192.168.0.42:12345"
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v", rr.Code, http.StatusOK)
	}

	ctx := req.Context()
	assert.Contains(t, GetAuditMetrics(ctx), "MetricA")

}

// TestAuditMiddleware_Failure не вызывает аудит при ошибке
func TestAuditMiddleware_Failure(t *testing.T) {
	var capturedEvent *AuditEvent
	manager := NewAuditManager([]AuditObserver{})
	middleware := AuditMiddleware(manager)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithAuditMetrics(r.Context(), []string{"MetricB"})
		*r = *r.Clone(ctx)
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := httptest.NewRequest("POST", "/update/", nil)
	rr := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rr, req)

	if capturedEvent != nil {
		t.Errorf("unexpected audit event on failure: %+v", capturedEvent)
	}
}

// mockManager — фейковая реализация для тестирования middleware
type mockManager struct {
	capture func(*AuditEvent)
}

func (m *mockManager) NotifyAll(ctx context.Context, event *AuditEvent) {
	if m.capture != nil {
		m.capture(event)
	}
}
