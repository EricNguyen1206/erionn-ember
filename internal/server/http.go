package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 15 * time.Second
	httpWriteTimeout      = 15 * time.Second
	httpIdleTimeout       = 60 * time.Second
	maxRequestBodyBytes   = 8 << 20
)

// HTTPServer exposes SemanticCache over REST/JSON.
type HTTPServer struct {
	server *http.Server
}

type httpHandler struct {
	cache *cache.SemanticCache
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
	return h != nil && h.cache != nil
}

func NewHTTPServer(addr string, sc *cache.SemanticCache) *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr:              addr,
			Handler:           NewHTTPHandler(sc),
			ReadHeaderTimeout: httpReadHeaderTimeout,
			ReadTimeout:       httpReadTimeout,
			WriteTimeout:      httpWriteTimeout,
			IdleTimeout:       httpIdleTimeout,
		},
	}
}

func NewHTTPHandler(sc *cache.SemanticCache) http.Handler {
	h := &httpHandler{cache: sc, stats: newHTTPMetrics()}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/cache/get", h.handleGet)
	mux.HandleFunc("POST /v1/cache/set", h.handleSet)
	mux.HandleFunc("POST /v1/cache/delete", h.handleDelete)
	mux.HandleFunc("GET /v1/stats", h.handleStats)
	mux.HandleFunc("GET /metrics", h.handleMetrics)
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("GET /ready", h.handleReady)
	return h.withObservability(mux)
}

func (s *HTTPServer) ListenAndServe() error              { return s.server.ListenAndServe() }
func (s *HTTPServer) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

type getReq struct {
	Prompt              string  `json:"prompt"`
	SimilarityThreshold float32 `json:"similarity_threshold,omitempty"`
}

type getResp struct {
	Hit        bool    `json:"hit"`
	Response   string  `json:"response,omitempty"`
	Similarity float32 `json:"similarity"`
	ExactMatch bool    `json:"exact_match"`
}

type setReq struct {
	Prompt   string `json:"prompt"`
	Response string `json:"response"`
	TTL      int    `json:"ttl,omitempty"`
}

type setResp struct {
	ID string `json:"id"`
}

type deleteReq struct {
	Prompt string `json:"prompt"`
}

type deleteResp struct {
	Deleted bool `json:"deleted"`
}

type statsResp struct {
	TotalEntries int64   `json:"total_entries"`
	CacheHits    int64   `json:"cache_hits"`
	CacheMisses  int64   `json:"cache_misses"`
	TotalQueries int64   `json:"total_queries"`
	HitRate      float64 `json:"hit_rate"`
}

func (h *httpHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}

	var req getReq
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err, "prompt is required")
		return
	}
	if !hasText(req.Prompt) {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	result, hit := h.cache.Get(r.Context(), req.Prompt, req.SimilarityThreshold)
	resp := getResp{Hit: hit}
	if hit && result != nil {
		resp.Response = result.Response
		resp.Similarity = result.Similarity
		resp.ExactMatch = result.ExactMatch
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *httpHandler) handleSet(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}

	var req setReq
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err, "prompt and response are required")
		return
	}
	if !hasText(req.Prompt) || !hasText(req.Response) {
		http.Error(w, "prompt and response are required", http.StatusBadRequest)
		return
	}
	if req.TTL < 0 {
		http.Error(w, "ttl must be non-negative", http.StatusBadRequest)
		return
	}

	id, err := h.cache.Set(r.Context(), req.Prompt, req.Response, time.Duration(req.TTL)*time.Second)
	if err != nil {
		slog.Error("cache.Set failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, setResp{ID: id})
}

func (h *httpHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}

	var req deleteReq
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err, "prompt is required")
		return
	}
	if !hasText(req.Prompt) {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, deleteResp{Deleted: h.cache.Delete(req.Prompt)})
}

func (h *httpHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}

	st := h.cache.Stats()
	writeJSON(w, http.StatusOK, statsResp{
		TotalEntries: int64(st.TotalEntries),
		CacheHits:    st.CacheHits,
		CacheMisses:  st.CacheMisses,
		TotalQueries: st.TotalQueries,
		HitRate:      st.HitRate,
	})
}

func (h *httpHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	if h.stats != nil {
		_, _ = io.WriteString(w, h.stats.render())
	}

	if h.cache == nil {
		return
	}

	st := h.cache.Stats()
	_, _ = fmt.Fprintf(w, "erion_ember_cache_entries %d\n", st.TotalEntries)
	_, _ = fmt.Fprintf(w, "erion_ember_cache_hits_total %d\n", st.CacheHits)
	_, _ = fmt.Fprintf(w, "erion_ember_cache_misses_total %d\n", st.CacheMisses)
	_, _ = fmt.Fprintf(w, "erion_ember_cache_queries_total %d\n", st.TotalQueries)
	_, _ = fmt.Fprintf(w, "erion_ember_cache_hit_rate %g\n", st.HitRate)
}

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

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}

	if err := dec.Decode(new(struct{})); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	return errors.New("request body must contain a single JSON object")
}

func writeDecodeError(w http.ResponseWriter, err error, fallback string) {
	if isRequestTooLarge(err) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	http.Error(w, fallback, http.StatusBadRequest)
}

func isRequestTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
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
