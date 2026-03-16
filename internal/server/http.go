package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/EricNguyen1206/erionn-ember/internal/pubsub"
	"github.com/EricNguyen1206/erionn-ember/internal/store"
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
	handler := &httpHandler{store: s, hub: h}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.handleHealth)
	mux.HandleFunc("GET /ready", handler.handleReady)
	return mux
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

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
}
