package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/pubsub"
	"github.com/EricNguyen1206/erion-ember/internal/store"
)

const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 15 * time.Second
	httpWriteTimeout      = 15 * time.Second
	httpIdleTimeout       = 60 * time.Second
)

type HTTPServer struct {
	server *http.Server
}

type httpHandler struct {
	store *store.Store
	hub   *pubsub.Hub
	stats *httpMetrics
}

type httpMetrics struct {
	mu       sync.Mutex
	requests map[requestMetricKey]*requestMetric
}

type requestMetricKey struct {
	Method string
	Path   string
	Status int
}

type requestMetric struct {
	Count       int64
	DurationSum time.Duration
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (h *httpHandler) ready() bool {
	return h != nil && h.store != nil && h.hub != nil
}

func NewHTTPServer(addr string, s *store.Store, h *pubsub.Hub) *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr:              addr,
			Handler:           NewHTTPHandler(s, h),
			ReadHeaderTimeout: httpReadHeaderTimeout,
			ReadTimeout:       httpReadTimeout,
			WriteTimeout:      httpWriteTimeout,
			IdleTimeout:       httpIdleTimeout,
		},
	}
}

func NewHTTPHandler(s *store.Store, h *pubsub.Hub) http.Handler {
	handler := &httpHandler{store: s, hub: h, stats: newHTTPMetrics()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.handleHealth)
	mux.HandleFunc("GET /ready", handler.handleReady)
	mux.HandleFunc("GET /metrics", handler.handleMetrics)
	return handler.withObservability(mux)
}

func (s *HTTPServer) ListenAndServe() error              { return s.server.ListenAndServe() }
func (s *HTTPServer) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

func (h *httpHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *httpHandler) handleReady(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *httpHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	if h.stats != nil {
		_, _ = io.WriteString(w, h.stats.render())
	}
	if !h.ready() {
		return
	}

	storeStats := h.store.Stats()
	hubStats := h.hub.Stats()
	_, _ = fmt.Fprintf(w, "erion_ember_keys_total %d\n", storeStats.TotalKeys)
	_, _ = fmt.Fprintf(w, "erion_ember_string_keys_total %d\n", storeStats.StringKeys)
	_, _ = fmt.Fprintf(w, "erion_ember_hash_keys_total %d\n", storeStats.HashKeys)
	_, _ = fmt.Fprintf(w, "erion_ember_list_keys_total %d\n", storeStats.ListKeys)
	_, _ = fmt.Fprintf(w, "erion_ember_set_keys_total %d\n", storeStats.SetKeys)
	_, _ = fmt.Fprintf(w, "erion_ember_pubsub_channels %d\n", hubStats.Channels)
	_, _ = fmt.Fprintf(w, "erion_ember_pubsub_subscribers %d\n", hubStats.Subscribers)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func newHTTPMetrics() *httpMetrics {
	return &httpMetrics{requests: make(map[requestMetricKey]*requestMetric)}
}

func (h *httpHandler) withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		duration := time.Since(startedAt)
		if h.stats != nil {
			h.stats.record(r.Method, r.URL.Path, recorder.status, duration)
		}

		slog.Debug("http request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration", duration,
		)
	})
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (m *httpMetrics) record(method, path string, status int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := requestMetricKey{Method: method, Path: path, Status: status}
	metric := m.requests[key]
	if metric == nil {
		metric = &requestMetric{}
		m.requests[key] = metric
	}

	metric.Count++
	metric.DurationSum += duration
}

func (m *httpMetrics) render() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys := make([]requestMetricKey, 0, len(m.requests))
	for key := range m.requests {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Path != keys[j].Path {
			return keys[i].Path < keys[j].Path
		}
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		return keys[i].Status < keys[j].Status
	})

	var b strings.Builder
	for _, key := range keys {
		metric := m.requests[key]
		labels := fmt.Sprintf("method=%q,path=%q,status=%q", key.Method, key.Path, strconv.Itoa(key.Status))
		_, _ = fmt.Fprintf(&b, "erion_ember_http_requests_total{%s} %d\n", labels, metric.Count)
		_, _ = fmt.Fprintf(&b, "erion_ember_http_request_duration_seconds_sum{%s} %.9f\n", labels, metric.DurationSum.Seconds())
		_, _ = fmt.Fprintf(&b, "erion_ember_http_request_duration_seconds_count{%s} %d\n", labels, metric.Count)
	}

	return b.String()
}

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
}
