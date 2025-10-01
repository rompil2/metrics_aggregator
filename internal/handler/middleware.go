package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
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

func MiddlewareRequestUnzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip body", http.StatusBadRequest)
				return
			}
			defer gz.Close()

			// Заменяем тело запроса на распакованное
			r.Body = gz
			r.Header.Del("Content-Length")
		}
		next.ServeHTTP(w, r)
	})
}
func MiddlewareCheckHash(key string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hash := r.Header.Get("HashSHA256"); hash != "" {
				// Create a tee reader to compute hash while preserving the body
				var bodyBuffer bytes.Buffer
				tee := io.TeeReader(r.Body, &bodyBuffer)

				// Compute hash
				h := hmac.New(sha256.New, []byte(key))
				if _, err := io.Copy(h, tee); err != nil {
					http.Error(w, "Error computing hash", http.StatusInternalServerError)
					return
				}

				// Replace the body for subsequent handlers
				r.Body.Close()
				r.Body = io.NopCloser(&bodyBuffer)

				// Compare hashes
				serverHash := hex.EncodeToString(h.Sum(nil))
				if !hmac.Equal([]byte(serverHash), []byte(hash)) {
					// hashes are not equal, send error
					http.Error(w, "Invalid hash", http.StatusBadRequest)
					return
				}
			}

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// The Middleware takes a key and response body to calculate the hash and put it in the header
func MiddlewareSetHash(key string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get hash of a response body
			hw := newWriterSubstituter(w)
			next.ServeHTTP(hw, r)
			// if no error in the response than we can calculate hash
			if hw.buf.Len() > 0 {
				h := hmac.New(sha256.New, []byte(key))
				if _, err := h.Write(hw.buf.Bytes()); err != nil {
					http.Error(w, "Error computing hash", http.StatusInternalServerError)
					return
				}
				hash := hex.EncodeToString(h.Sum(nil))
				w.Header().Set("HashSHA256", hash)
				w.Write(hw.buf.Bytes())
			}
		})
	}
}

func MiddlewareResponseZip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cw := newWriterSubstituter(w)
		next.ServeHTTP(cw, r)
		doesClientSupportGZIP := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
		if doesClientSupportGZIP && shouldCompress(cw.Header().Get("Content-Type")) {
			if cw.buf.Len() > 0 {
				w.Header().Set("Content-encoding", "gzip")
				w.Header().Del("Content-Length")
				gz := gzip.NewWriter(w)
				defer gz.Close()
				_, err := gz.Write(cw.buf.Bytes())
				if err != nil {
					http.Error(w, "Error compressing response", http.StatusInternalServerError)
				} else {
					err := gz.Flush()
					if err != nil {
						http.Error(w, "Error flushing response", http.StatusInternalServerError)
					}
				}
			}
		} else {
			// send body as is
			w.Header().Del("Content-Length")
			_, err := w.Write((cw.buf.Bytes()))
			if err != nil {
				http.Error(w, "Error sending response", http.StatusInternalServerError)
			}
		}

	})
}

// Checks If the type good for compressing
func shouldCompress(contentType string) bool {
	compressibleTypes := []string{
		"text/html",
		"application/json",
	}

	for _, t := range compressibleTypes {
		if strings.HasPrefix(contentType, t) {
			return true
		}
	}
	return false
}

type WriterSubstituter struct {
	w   http.ResponseWriter
	buf *bytes.Buffer
}

func newWriterSubstituter(w http.ResponseWriter) *WriterSubstituter {
	return &WriterSubstituter{
		w:   w,
		buf: bytes.NewBuffer([]byte{}),
	}
}

func (c *WriterSubstituter) Header() http.Header {
	return c.w.Header()
}

func (c *WriterSubstituter) Write(p []byte) (int, error) {
	return c.buf.Write(p)
}

func (c *WriterSubstituter) WriteHeader(statusCode int) {
	c.w.WriteHeader(statusCode)
}
