package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

// HTTPServer exposes SemanticCache over REST/JSON.
type HTTPServer struct {
	cache  *cache.SemanticCache
	server *http.Server
}

func NewHTTPServer(addr string, sc *cache.SemanticCache) *HTTPServer {
	s := &HTTPServer{cache: sc}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/cache/get", s.handleGet)
	mux.HandleFunc("POST /v1/cache/set", s.handleSet)
	mux.HandleFunc("POST /v1/cache/delete", s.handleDelete)
	mux.HandleFunc("GET /v1/stats", s.handleStats)
	mux.HandleFunc("GET /health", s.handleHealth)
	s.server = &http.Server{Addr: addr, Handler: mux}
	return s
}

func (s *HTTPServer) ListenAndServe() error              { return s.server.ListenAndServe() }
func (s *HTTPServer) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

// ─── Request / Response types ────────────────────────────────────────────────

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
	TTL      int    `json:"ttl,omitempty"` // seconds, 0 = default
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

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (s *HTTPServer) handleGet(w http.ResponseWriter, r *http.Request) {
	var req getReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}
	result, hit := s.cache.Get(r.Context(), req.Prompt, req.SimilarityThreshold)
	resp := getResp{Hit: hit}
	if hit && result != nil {
		resp.Response = result.Response
		resp.Similarity = result.Similarity
		resp.ExactMatch = result.ExactMatch
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *HTTPServer) handleSet(w http.ResponseWriter, r *http.Request) {
	var req setReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Prompt == "" || req.Response == "" {
		http.Error(w, "prompt and response are required", http.StatusBadRequest)
		return
	}
	id, err := s.cache.Set(r.Context(), req.Prompt, req.Response, time.Duration(req.TTL)*time.Second)
	if err != nil {
		slog.Error("cache.Set failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, setResp{ID: id})
}

func (s *HTTPServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req deleteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, deleteResp{Deleted: s.cache.Delete(req.Prompt)})
}

func (s *HTTPServer) handleStats(w http.ResponseWriter, r *http.Request) {
	st := s.cache.Stats()
	writeJSON(w, http.StatusOK, statsResp{
		TotalEntries: int64(st.TotalEntries),
		CacheHits:    st.CacheHits,
		CacheMisses:  st.CacheMisses,
		TotalQueries: st.TotalQueries,
		HitRate:      st.HitRate,
	})
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
