package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "messenger_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "path", "status"})

	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "messenger_http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"service", "method", "path", "status"})
)

// Metrics records request duration and count per service/method/path/status.
// Use chi.RouteContext to get the matched route pattern (avoids UUID cardinality).
func Metrics(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			// Use matched route pattern to avoid per-UUID cardinality.
			path := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				path = rctx.RoutePattern()
			}

			status := strconv.Itoa(rw.status)
			httpDuration.WithLabelValues(serviceName, r.Method, path, status).Observe(time.Since(start).Seconds())
			httpRequests.WithLabelValues(serviceName, r.Method, path, status).Inc()
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// Hijack proxies WebSocket/connection upgrades to the underlying writer.
// Without this, gorilla/websocket's Upgrade call fails because it asserts
// http.Hijacker on the ResponseWriter it receives.
func (sr *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := sr.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

// Flush proxies streaming flushes when supported by the underlying writer.
func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Push proxies HTTP/2 server push when supported by the underlying writer.
func (sr *statusRecorder) Push(target string, opts *http.PushOptions) error {
	p, ok := sr.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}
