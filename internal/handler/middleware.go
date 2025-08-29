package handler

import (
	"net/http"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/logger"
)

// берём структуру для хранения сведений об ответе
type responseData struct {
	status int
	size   int
}

// добавляем реализацию http.ResponseWriter
type loggingResponseWriter struct {
	http.ResponseWriter  // встраиваем оригинальный http.ResponseWriter
	responseData         *responseData
	statusHasBeenChanged bool
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	// записываем ответ, используя оригинальный http.ResponseWriter
	// Оказалось, что, порой, метод WriteHeader может вообще не вызываться
	// и это интерпретируется как статус-код 200
	// Хотя и метод Write может быть не вызванн и тоже это 200, Ок.
	if !lrw.statusHasBeenChanged {
		lrw.responseData.status = http.StatusOK
	}
	size, err := lrw.ResponseWriter.Write(b)
	lrw.responseData.size += size // захватываем размер
	return size, err
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	// записываем код статуса, используя оригинальный http.ResponseWriter
	if lrw.statusHasBeenChanged {
		return // Уже был вызван, избегаем дублирования
	}
	lrw.ResponseWriter.WriteHeader(statusCode)
	lrw.responseData.status = statusCode // захватываем код статуса
	lrw.statusHasBeenChanged = true
}

func NaiveLoggerMiddleware(next http.Handler) http.Handler {
	NaiveLogger := func(w http.ResponseWriter, r *http.Request) {

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		log := logger.Get()
		lw := loggingResponseWriter{
			ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
			responseData:   responseData,
		}

		start := time.Now()
		ctx := logger.WithLogger(r.Context(), log)
		next.ServeHTTP(&lw, r.WithContext(ctx))

		duration := time.Since(start)

		log.Info().
			Str("uri", r.RequestURI).
			Str("method", r.Method).
			Dur("duration", duration).
			Int("status", lw.responseData.status). // получаем перехваченный код статуса ответа
			Int("size", lw.responseData.size).     // получаем перехваченный размер ответа
			Send()

	}
	return http.HandlerFunc(NaiveLogger)
}
