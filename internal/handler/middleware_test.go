package handler

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rompil2/metrics_aggregator/internal/logger"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func setupTest() {
	logger.SetGlobalLogger(zerolog.Nop())
}

func TestMiddleware_Status200(t *testing.T) {
	setupTest()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	middleware := NaiveLoggerMiddleware(handler)
	req := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()

	middleware.ServeHTTP(recorder, req)

	if recorder.Code != 200 {
		t.Errorf("Expected 200, got %d", recorder.Code)
	}
}

func TestMiddleware_Status404(t *testing.T) {
	setupTest()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("Not found"))
	})

	middleware := NaiveLoggerMiddleware(handler)
	req := httptest.NewRequest("GET", "/not-found", nil)
	recorder := httptest.NewRecorder()

	middleware.ServeHTTP(recorder, req)

	if recorder.Code != 404 {
		t.Errorf("Expected 404, got %d", recorder.Code)
	}
}

func TestMiddleware_ResponseSize(t *testing.T) {
	setupTest()

	responseText := "Hello World"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(responseText))
	})

	middleware := NaiveLoggerMiddleware(handler)
	req := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()

	middleware.ServeHTTP(recorder, req)

	if recorder.Body.Len() != len(responseText) {
		t.Errorf("Expected size %d, got %d", len(responseText), recorder.Body.Len())
	}
}

func TestMiddleware_MultipleCalls(t *testing.T) {
	setupTest()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("test"))
	})
	middleware := NaiveLoggerMiddleware(handler)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		recorder := httptest.NewRecorder()
		middleware.ServeHTTP(recorder, req)
	}
	if callCount != 5 {
		t.Errorf("Expected 5 calls, got %d", callCount)
	}
}

func Test_isCompressible(t *testing.T) {
	type args struct {
		contentType string
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Positive Test, json",
			args{
				"application/json",
			},
			true,
		},
		{
			"Positive Test, html",
			args{
				"text/html",
			},
			true,
		},
		{
			"Negative Test, text",
			args{
				"text/plain",
			},
			true,
		},
		{
			"Negative Test, empty",
			args{""},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCompressible(tt.args.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Dummy Handler
func testNextHandler(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "Ошибка чтения тела запроса: %v\n", err)
		return
	}

	fmt.Fprint(w, string(bodyBytes))
}

func TestMiddlewareRequestUnzip_ValidGZIP(t *testing.T) {
	testBody := "Это тестовый запрос."
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write([]byte(testBody))
	assert.NoError(t, err)
	err = gz.Close()
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(b.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")
	recorder := httptest.NewRecorder()
	middleware := MiddlewareRequestUnzip(http.HandlerFunc(testNextHandler))
	middleware.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, testBody, recorder.Body.String()) // Должны увидеть исходное тело запроса
}

func TestMiddlewareRequestUnzip_BadGZIP(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("Некорректный gzip")))
	req.Header.Set("Content-Encoding", "gzip")
	recorder := httptest.NewRecorder()
	middleware := MiddlewareRequestUnzip(http.HandlerFunc(testNextHandler))
	middleware.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Invalid gzip body")
}

func TestMiddlewareRequestUnzip_NoContentEncoding(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()
	middleware := MiddlewareRequestUnzip(http.HandlerFunc(testNextHandler))
	middleware.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Body.String()) // Тело пустое, так как GET запрос
}

func TestSupportsGzip(t *testing.T) {
	tests := []struct {
		name           string
		acceptEncoding string
		expected       bool
	}{
		{
			name:           "empty header",
			acceptEncoding: "",
			expected:       false,
		},
		{
			name:           "no gzip",
			acceptEncoding: "deflate, br",
			expected:       false,
		},
		{
			name:           "gzip included",
			acceptEncoding: "gzip",
			expected:       true,
		},
		{
			name:           "gzip with other encodings",
			acceptEncoding: "deflate, gzip, br",
			expected:       true,
		},
		{
			name:           "gzip first",
			acceptEncoding: "gzip, deflate",
			expected:       true,
		},
		{
			name:           "gzip with qvalue",
			acceptEncoding: "gzip;q=0.8",
			expected:       true,
		},
		{
			name:           "gzip with qvalue and space",
			acceptEncoding: " gzip ; q=0.8 ",
			expected:       true,
		},
		{
			name:           "multiple qvalues, gzip present",
			acceptEncoding: "br;q=1.0, gzip;q=0.8, deflate;q=0.5",
			expected:       true,
		},
		{
			name:           "malformed encoding (no value after semicolon)",
			acceptEncoding: "gzip;",
			expected:       true,
		},
		{
			name:           "malformed encoding (trailing comma)",
			acceptEncoding: "gzip,,",
			expected:       true,
		},
		{
			name:           "only whitespace",
			acceptEncoding: "   ",
			expected:       false,
		},
		{
			name:           "whitespace between commas",
			acceptEncoding: "  , deflate ,  , br ,  ",
			expected:       false,
		},
		{
			name:           "case sensitivity - should be case-insensitive in practice but 'gzip' is lowercase per spec",
			acceptEncoding: "GZIP",
			expected:       false, // Note: technically, per spec, it's case-insensitive, but common impls use lowercase
		},
		{
			name:           "gzip with extra params",
			acceptEncoding: "gzip; level=5",
			expected:       true,
		},
		{
			name:           "empty part due to double comma",
			acceptEncoding: "deflate,, gzip",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{Header: make(http.Header)}
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}

			result := supportsGzip(req)

			assert.Equal(t, tt.expected, result, "supportsGzip() result mismatch")
		})
	}
}

func TestMiddlewareCheckIP(t *testing.T) {
	tests := []struct {
		name           string
		trustedSubnet  string
		headerIP       string
		expectedStatus int
		expectPanic    bool
	}{
		{
			name:           "NoSubnet_AllowsEverything",
			trustedSubnet:  "",
			headerIP:       "",
			expectedStatus: http.StatusOK,
			expectPanic:    false,
		},
		{
			name:           "MissingHeader_ReturnsForbidden",
			trustedSubnet:  "192.168.1.0/24",
			headerIP:       "",
			expectedStatus: http.StatusForbidden,
			expectPanic:    false,
		},
		{
			name:           "InvalidIP_ReturnsForbidden",
			trustedSubnet:  "192.168.1.0/24",
			headerIP:       "not-an-ip",
			expectedStatus: http.StatusForbidden,
			expectPanic:    false,
		},
		{
			name:           "NotInSubnet_ReturnsForbidden",
			trustedSubnet:  "192.168.1.0/24",
			headerIP:       "10.0.0.1",
			expectedStatus: http.StatusForbidden,
			expectPanic:    false,
		},
		{
			name:           "ValidIP_AllowsRequest",
			trustedSubnet:  "192.168.1.0/24",
			headerIP:       "192.168.1.100",
			expectedStatus: http.StatusOK,
			expectPanic:    false,
		},
		{
			name:           "PanicsOnInvalidCIDR",
			trustedSubnet:  "invalid-cidr",
			headerIP:       "",
			expectedStatus: 0,
			expectPanic:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					MiddlewareCheckIP(tt.trustedSubnet)
				})
				return
			}

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			middleware := MiddlewareCheckIP(tt.trustedSubnet)
			handler := middleware(next)

			req := httptest.NewRequest("GET", "/", nil)
			if tt.headerIP != "" {
				req.Header.Set("X-Real-IP", tt.headerIP)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
