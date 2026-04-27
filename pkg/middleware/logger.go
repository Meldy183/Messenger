package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/fyodor/messenger/pkg/logger"
)

// Logger is an HTTP middleware that:
//  1. Reads X-Request-ID from the incoming request (or generates one).
//  2. Reads X-Trace-ID from the incoming request (set by ingress/upstream).
//  3. Builds a child logger with those fields pre-attached.
//  4. Injects it into the request context — handlers retrieve it via logger.L(ctx).
//  5. Writes X-Request-ID back to the response so clients can correlate.
//  6. Logs the completed request (method, path, status, latency).
//
// base is the root logger created in main(). Every request gets its own child.
func Logger(base *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.NewString()
			}
			traceID := r.Header.Get("X-Trace-ID") // may be empty, that's fine

			// With() creates a child logger that always includes these fields.
			// Every log line from this request will carry request_id and trace_id
			// without the handler needing to pass them explicitly.
			reqLogger := base.With(
				zap.String("request_id", requestID),
				zap.String("trace_id", traceID),
			)

			// Expose the request ID to the caller for client-side correlation.
			w.Header().Set("X-Request-ID", requestID)

			// Wrap the ResponseWriter so we can capture the status code.
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			start := time.Now()
			next.ServeHTTP(rw, r.WithContext(logger.WithContext(r.Context(), reqLogger)))

			reqLogger.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rw.status),
				zap.Duration("latency", time.Since(start)),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the written status code.
// The standard ResponseWriter doesn't expose it after WriteHeader is called.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack proxies websocket/connection upgrades to the underlying writer.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

// Flush proxies streaming flushes when supported by the underlying writer.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Push proxies HTTP/2 server push when supported by the underlying writer.
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	p, ok := rw.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}
