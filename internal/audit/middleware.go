// Package audit provides auditing functionality for metrics operations.
// path: internal/audit/middleware.go
package audit

import (
	"net/http"
	"strings"
	"time"

	"context"

	"github.com/go-chi/chi/v5/middleware"
)

type contextKey string

const auditMetricsKey contextKey = "audit_metrics"

// WithAuditMetrics associates a list of metric IDs with the given context for audit logging purposes.
// The stored IDs are later used by AuditMiddleware to generate audit events for successful metric operations.
func WithAuditMetrics(ctx context.Context, metricIDs []string) context.Context {
	return context.WithValue(ctx, auditMetricsKey, metricIDs)
}

// GetAuditMetrics retrieves the list of metric IDs from the context that were marked for auditing.
// Returns nil if no metrics were associated with the context.
func GetAuditMetrics(ctx context.Context) []string {
	if val := ctx.Value(auditMetricsKey); val != nil {
		if ids, ok := val.([]string); ok {
			return ids
		}
	}
	return nil
}

// GetClientIP extracts the client IP address from the request,
// respecting X-Forwarded-For and X-Real-IP headers before falling back to RemoteAddr.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Use RemoteAddr as fallback
	ipPort := r.RemoteAddr
	ip := strings.Split(ipPort, ":")[0]
	return ip
}

// AuditMiddleware creates a middleware that logs successful metric update operations to all configured audit sinks.
// It captures metric IDs from the request context (via WithAuditMetrics), client IP, and timestamp,
// and sends an audit event only for 2xx responses on known metrics endpoints (/update/, /updates/, etc.).
func AuditMiddleware(manager *AuditManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Просто проксируем запрос
			next.ServeHTTP(ww, r)

			// Аудит только успешных операций с метриками
			if ww.Status() >= 200 && ww.Status() < 300 {
				if isMetricsEndpoint(r.URL.Path) {
					metrics := GetAuditMetrics(r.Context())
					if len(metrics) > 0 {
						event := &AuditEvent{
							Timestamp: time.Now().Unix(),
							Metrics:   metrics,
							IPAddress: GetClientIP(r),
						}
						manager.NotifyAll(r.Context(), event)
					}
				}
			}
		})
	}
}

// isMetricsEndpoint checks if the URL path corresponds to a metrics operation
func isMetricsEndpoint(path string) bool {
	metricsPaths := []string{
		"/update/",
		"/updates/",
		"/update", // For paths like /update/counter/id/123
	}

	for _, p := range metricsPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// InitializeAuditManager creates and configures an AuditManager with file and/or HTTP audit observers
// based on the provided audit file path and audit URL. If both are empty, returns a manager with no observers.
func InitializeAuditManager(auditFile, auditURL string) *AuditManager {

	var observers []AuditObserver

	if auditFile != "" {
		observers = append(observers, NewFileAuditObserver(auditFile))
	}

	if auditURL != "" {
		observers = append(observers, NewHTTPAuditObserver(auditURL))
	}

	return NewAuditManager(observers)
}
