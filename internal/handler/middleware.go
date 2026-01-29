// Package handler provides HTTP request and response logging middleware.
// path: internal/handler
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
	"sync"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/logger"
)

var (
	bufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	gzipWriterPool = sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(io.Discard)
		},
	}
)

func getBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func putBuffer(b *bytes.Buffer) {
	if b != nil {
		b.Reset()
		bufferPool.Put(b)
	}
}

// responseData holds HTTP response metadata captured during request handling.
type responseData struct {
	status int
	size   int
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code and response size.
type loggingResponseWriter struct {
	http.ResponseWriter
	responseData         *responseData
	statusHasBeenChanged bool
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if !lrw.statusHasBeenChanged {
		lrw.responseData.status = http.StatusOK
	}
	n, err := lrw.ResponseWriter.Write(b)
	lrw.responseData.size += n
	return n, err
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	if lrw.statusHasBeenChanged {
		return
	}
	lrw.ResponseWriter.WriteHeader(statusCode)
	lrw.responseData.status = statusCode
	lrw.statusHasBeenChanged = true
}

// NaiveLoggerMiddleware logs HTTP request and response metadata including URI, method,
// duration, status code, and response size. It captures the actual status code even if
// WriteHeader is not explicitly called by the handler.
func NaiveLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get()
		ctx := logger.WithLogger(r.Context(), log)
		rd := &responseData{status: 0, size: 0}
		lrw := &loggingResponseWriter{
			ResponseWriter:       w,
			responseData:         rd,
			statusHasBeenChanged: false,
		}

		start := time.Now()
		next.ServeHTTP(lrw, r.WithContext(ctx))
		duration := time.Since(start)

		log.Info().
			Str("uri", r.RequestURI).
			Str("method", r.Method).
			Dur("duration", duration).
			Int("status", rd.status).
			Int("size", rd.size).
			Send()
	})
}

// MiddlewareRequestUnzip decompresses gzipped HTTP request bodies when the
// Content-Encoding header indicates gzip compression. It replaces the original
// request body with a decompressed reader and removes the Content-Length header.
func MiddlewareRequestUnzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Create a new reader — don't use pool for simplicity and safety
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Invalid gzip body", http.StatusBadRequest)
			return
		}
		defer gr.Close()

		// Replace request body
		r.Body = gr
		r.Header.Del("Content-Length")
		next.ServeHTTP(w, r)
	})
}

// MiddlewareCheckHash validates the integrity of the request body using an HMAC-SHA256 hash
// provided in the HashSHA256 header. It recomputes the hash using the provided key and
// compares it with the client-provided value. If hashes don't match, it returns a 400 error.
func MiddlewareCheckHash(key string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hash := r.Header.Get("HashSHA256")
			if hash == "" {
				next.ServeHTTP(w, r)
				return
			}

			bodyBuf := getBuffer()
			defer putBuffer(bodyBuf)

			tee := io.TeeReader(r.Body, bodyBuf)
			h := hmac.New(sha256.New, []byte(key))
			if _, err := io.Copy(h, tee); err != nil {
				http.Error(w, "Error computing hash", http.StatusInternalServerError)
				return
			}

			r.Body.Close()
			r.Body = io.NopCloser(bodyBuf)

			serverHash := hex.EncodeToString(h.Sum(nil))
			if !hmac.Equal([]byte(serverHash), []byte(hash)) {
				http.Error(w, "Invalid hash", http.StatusBadRequest)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareSetHash computes an HMAC-SHA256 hash of the response body and sets it
// in the HashSHA256 response header. It requires capturing the full response body,
// so it should be placed early in the middleware chain, before any streaming responses.
func MiddlewareSetHash(key string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cw := newCapturingResponseWriter(w)
			defer cw.releaseBuffer()

			next.ServeHTTP(cw, r)

			if cw.buf.Len() == 0 {
				return
			}

			h := hmac.New(sha256.New, []byte(key))
			if _, err := h.Write(cw.buf.Bytes()); err != nil {
				http.Error(w, "Error computing hash", http.StatusInternalServerError)
				return
			}

			hash := hex.EncodeToString(h.Sum(nil))
			w.Header().Set("HashSHA256", hash)

			status := cw.statusCode
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			w.Write(cw.buf.Bytes())
		})
	}
}

// MiddlewareResponseZip compresses HTTP responses with gzip encoding
// when the client indicates support via the Accept-Encoding header and
// the response Content-Type is compressible (e.g., JSON, HTML, plain text).
// It automatically sets Content-Encoding: gzip and Vary: Accept-Encoding headers.
func MiddlewareResponseZip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientSupportsGzip := supportsGzip(r)

		cw := newCapturingResponseWriter(w)
		defer cw.releaseBuffer()

		next.ServeHTTP(cw, r)

		contentType := cw.Header().Get("Content-Type")
		shouldCompress := clientSupportsGzip && isCompressible(contentType)

		w.Header().Add("Vary", "Accept-Encoding")

		status := cw.statusCode
		if status == 0 {
			status = http.StatusOK
		}

		if shouldCompress && cw.buf.Len() > 0 {
			w.Header().Del("Content-Length")
			w.Header().Set("Content-Encoding", "gzip")
			w.WriteHeader(status)

			gz := gzipWriterPool.Get().(*gzip.Writer)
			gz.Reset(w)
			defer func() {
				gz.Close()
				gzipWriterPool.Put(gz)
			}()

			if _, err := gz.Write(cw.buf.Bytes()); err != nil {
				return // can't recover after headers sent
			}
			gz.Flush()
		} else {
			w.Header().Del("Content-Length")
			w.WriteHeader(status)
			if _, err := w.Write(cw.buf.Bytes()); err != nil {
				return
			}
		}
	})
}

// supportsGzip checks if the client explicitly accepts gzip encoding.
func supportsGzip(r *http.Request) bool {
	encodings := strings.Split(r.Header.Get("Accept-Encoding"), ",")
	for _, part := range encodings {
		encoding := strings.TrimSpace(strings.Split(part, ";")[0])
		if encoding == "gzip" {
			return true
		}
	}
	return false
}

// isCompressible returns true for text-based or structured compressible content types.
func isCompressible(contentType string) bool {
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[:i]
	}
	contentType = strings.TrimSpace(strings.ToLower(contentType))

	switch contentType {
	case
		"text/html",
		"text/plain",
		"text/css",
		"text/javascript",
		"text/xml",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/xhtml+xml",
		"application/rss+xml",
		"application/atom+xml":
		return true
	default:
		return false
	}
}

// capturingResponseWriter captures status code, headers, and response body.
type capturingResponseWriter struct {
	w           http.ResponseWriter
	buf         *bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func newCapturingResponseWriter(w http.ResponseWriter) *capturingResponseWriter {
	return &capturingResponseWriter{
		w:   w,
		buf: getBuffer(),
	}
}

func (cw *capturingResponseWriter) Header() http.Header {
	return cw.w.Header()
}

func (cw *capturingResponseWriter) Write(data []byte) (int, error) {
	if !cw.wroteHeader {
		cw.statusCode = http.StatusOK
		cw.wroteHeader = true
	}
	return cw.buf.Write(data)
}

func (cw *capturingResponseWriter) WriteHeader(statusCode int) {
	if !cw.wroteHeader {
		cw.statusCode = statusCode
		cw.wroteHeader = true
	}
}

func (cw *capturingResponseWriter) releaseBuffer() {
	putBuffer(cw.buf)
	cw.buf = nil
}
