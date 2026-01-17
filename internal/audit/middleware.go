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

// Внутри пакета audit
func WithAuditMetrics(ctx context.Context, metricIDs []string) context.Context {
	return context.WithValue(ctx, auditMetricsKey, metricIDs)
}

func GetAuditMetrics(ctx context.Context) []string {
	if val := ctx.Value(auditMetricsKey); val != nil {
		if ids, ok := val.([]string); ok {
			return ids
		}
	}
	return nil
}

// GetClientIP extracts the client IP address from the request
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

// InitializeAuditManager creates an AuditManager based on configuration
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
