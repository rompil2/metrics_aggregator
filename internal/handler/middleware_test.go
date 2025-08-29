package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rompil2/metrics_aggregator/internal/logger"
	"github.com/rs/zerolog"
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

	// Вызываем несколько раз
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		recorder := httptest.NewRecorder()
		middleware.ServeHTTP(recorder, req)
	}

	if callCount != 5 {
		t.Errorf("Expected 5 calls, got %d", callCount)
	}
}
