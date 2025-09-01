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

func Test_shouldCompress(t *testing.T) {
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
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCompress(tt.args.contentType)
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
