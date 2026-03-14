package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 15 * time.Second
	httpWriteTimeout      = 15 * time.Second
	httpIdleTimeout       = 60 * time.Second
)

// HTTPServer exposes SemanticCache over REST/JSON.
type HTTPServer struct {
	server *http.Server
}

type httpHandler struct {
	cache *cache.SemanticCache
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
	h := &httpHandler{cache: sc}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/cache/get", h.handleGet)
	mux.HandleFunc("POST /v1/cache/set", h.handleSet)
	mux.HandleFunc("POST /v1/cache/delete", h.handleDelete)
	mux.HandleFunc("GET /v1/stats", h.handleStats)
	mux.HandleFunc("GET /health", h.handleHealth)
	return mux
}

func (s *HTTPServer) ListenAndServe() error              { return s.server.ListenAndServe() }
func (s *HTTPServer) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

type getReq struct {
	Namespace           httpNamespace `json:"namespace"`
	Prompt              string        `json:"prompt"`
	SimilarityThreshold float32       `json:"similarity_threshold,omitempty"`
}

type getResp struct {
	Hit        bool    `json:"hit"`
	Response   string  `json:"response,omitempty"`
	Similarity float32 `json:"similarity"`
	ExactMatch bool    `json:"exact_match"`
}

type setReq struct {
	Namespace httpNamespace `json:"namespace"`
	Prompt    string        `json:"prompt"`
	Response  string        `json:"response"`
	TTL       int           `json:"ttl,omitempty"`
}

type setResp struct {
	ID string `json:"id"`
}

type deleteReq struct {
	Namespace httpNamespace `json:"namespace"`
	Prompt    string        `json:"prompt"`
}

type httpNamespace struct {
	Model            string `json:"model"`
	TenantID         string `json:"tenant_id"`
	SystemPromptHash string `json:"system_prompt_hash"`
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
	var req getReq
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "namespace and prompt are required", http.StatusBadRequest)
		return
	}
	if !req.Namespace.valid() || !hasText(req.Prompt) {
		http.Error(w, "namespace and prompt are required", http.StatusBadRequest)
		return
	}

	result, hit := h.cache.GetInNamespace(r.Context(), req.Namespace.cacheNamespace(), req.Prompt, req.SimilarityThreshold)
	resp := getResp{Hit: hit}
	if hit && result != nil {
		resp.Response = result.Response
		resp.Similarity = result.Similarity
		resp.ExactMatch = result.ExactMatch
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *httpHandler) handleSet(w http.ResponseWriter, r *http.Request) {
	var req setReq
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "namespace, prompt, and response are required", http.StatusBadRequest)
		return
	}
	if !req.Namespace.valid() || !hasText(req.Prompt) || !hasText(req.Response) {
		http.Error(w, "namespace, prompt, and response are required", http.StatusBadRequest)
		return
	}
	if req.TTL < 0 {
		http.Error(w, "ttl must be non-negative", http.StatusBadRequest)
		return
	}

	id, err := h.cache.SetInNamespace(r.Context(), req.Namespace.cacheNamespace(), req.Prompt, req.Response, time.Duration(req.TTL)*time.Second)
	if err != nil {
		slog.Error("cache.SetInNamespace failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, setResp{ID: id})
}

func (h *httpHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req deleteReq
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "namespace and prompt are required", http.StatusBadRequest)
		return
	}
	if !req.Namespace.valid() || !hasText(req.Prompt) {
		http.Error(w, "namespace and prompt are required", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, deleteResp{Deleted: h.cache.DeleteInNamespace(req.Namespace.cacheNamespace(), req.Prompt)})
}

func (h *httpHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	st := h.cache.Stats()
	writeJSON(w, http.StatusOK, statsResp{
		TotalEntries: int64(st.TotalEntries),
		CacheHits:    st.CacheHits,
		CacheMisses:  st.CacheMisses,
		TotalQueries: st.TotalQueries,
		HitRate:      st.HitRate,
	})
}

func (h *httpHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

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

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
}

func (n httpNamespace) valid() bool {
	return hasText(n.Model) && hasText(n.TenantID) && hasText(n.SystemPromptHash)
}

func (n httpNamespace) cacheNamespace() cache.Namespace {
	return cache.Namespace{
		Model:            n.Model,
		TenantID:         n.TenantID,
		SystemPromptHash: n.SystemPromptHash,
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
