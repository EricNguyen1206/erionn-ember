package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

type logRecord struct {
	Level   slog.Level
	Message string
}

type captureHandler struct {
	mu      sync.Mutex
	records []logRecord
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, logRecord{Level: record.Level, Message: record.Message})
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }

func (h *captureHandler) count(level slog.Level, message string) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	count := 0
	for _, record := range h.records {
		if record.Level == level && record.Message == message {
			count++
		}
	}

	return count
}

func newServerTestCache() *cache.SemanticCache {
	return cache.New(cache.Config{
		MaxElements:         128,
		SimilarityThreshold: 0.85,
		DefaultTTL:          time.Hour,
	})
}

func TestHTTPHandlerCRUDFlow(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(newServerTestCache()))
	defer ts.Close()

	var setResponse setResp
	postJSON(t, ts.URL+"/v1/cache/set", setReq{
		Prompt:   "What is Go?",
		Response: "Go is a compiled language.",
	}, http.StatusOK, &setResponse)
	if setResponse.ID == "" {
		t.Fatal("expected non-empty cache id")
	}

	var getResponse getResp
	postJSON(t, ts.URL+"/v1/cache/get", getReq{Prompt: "What is Go?"}, http.StatusOK, &getResponse)
	if !getResponse.Hit {
		t.Fatal("expected cache hit")
	}
	if !getResponse.ExactMatch {
		t.Fatal("expected exact match")
	}
	if getResponse.Response != "Go is a compiled language." {
		t.Fatalf("got response %q", getResponse.Response)
	}

	var stats statsResp
	getJSON(t, ts.URL+"/v1/stats", http.StatusOK, &stats)
	if stats.TotalEntries != 1 {
		t.Fatalf("got %d entries, want 1", stats.TotalEntries)
	}

	var deleteResponse deleteResp
	postJSON(t, ts.URL+"/v1/cache/delete", deleteReq{Prompt: "What is Go?"}, http.StatusOK, &deleteResponse)
	if !deleteResponse.Deleted {
		t.Fatal("expected delete success")
	}

	postJSON(t, ts.URL+"/v1/cache/get", getReq{Prompt: "What is Go?"}, http.StatusOK, &getResponse)
	if getResponse.Hit {
		t.Fatal("expected cache miss after delete")
	}

	var health map[string]string
	getJSON(t, ts.URL+"/health", http.StatusOK, &health)
	if health["status"] != "ok" {
		t.Fatalf("got health status %q", health["status"])
	}
}

func TestHTTPHandlerValidation(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(newServerTestCache()))
	defer ts.Close()

	postJSONStatus(t, ts.URL+"/v1/cache/set", setReq{Prompt: " ", Response: "value"}, http.StatusBadRequest)
	postJSONStatus(t, ts.URL+"/v1/cache/set", setReq{Prompt: "prompt", Response: "value", TTL: -1}, http.StatusBadRequest)
	postJSONStatus(t, ts.URL+"/v1/cache/delete", deleteReq{Prompt: ""}, http.StatusBadRequest)

	resp, err := http.Post(ts.URL+"/v1/cache/get", "application/json", bytes.NewBufferString(`{"prompt":"ok","extra":true}`))
	if err != nil {
		t.Fatalf("POST get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHTTPHandlerRejectsOversizedRequestBody(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(newServerTestCache()))
	defer ts.Close()

	body := `{"prompt":"` + strings.Repeat("a", maxRequestBodyBytes+1) + `"}`
	resp, err := http.Post(ts.URL+"/v1/cache/get", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST oversized get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestHTTPHandlerAcceptsLargeButReasonableRequestBody(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(newServerTestCache()))
	defer ts.Close()

	body := setReq{
		Prompt:   "large-prompt",
		Response: strings.Repeat("a", 2<<20),
	}

	var respBody setResp
	postJSON(t, ts.URL+"/v1/cache/set", body, http.StatusOK, &respBody)
	if respBody.ID == "" {
		t.Fatal("expected non-empty cache id")
	}
}

func TestHTTPHandlerHealthAndReadinessAreDistinct(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(newServerTestCache()))
	defer ts.Close()

	var health map[string]string
	getJSON(t, ts.URL+"/health", http.StatusOK, &health)
	if health["status"] != "ok" {
		t.Fatalf("got health status %q", health["status"])
	}

	var readiness map[string]string
	getJSON(t, ts.URL+"/ready", http.StatusOK, &readiness)
	if readiness["status"] != "ready" {
		t.Fatalf("got readiness status %q", readiness["status"])
	}
}

func TestHTTPHandlerReadinessRequiresInitializedCache(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(nil))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	var readiness map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&readiness); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if readiness["status"] != "not_ready" {
		t.Fatalf("got readiness status %q", readiness["status"])
	}
}

func TestHTTPHandlerRejectsRequestsWhenNotReady(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(nil))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/v1/cache/get", "application/json", strings.NewReader(`{"prompt":"What is Go?"}`))
	if err != nil {
		t.Fatalf("POST /v1/cache/get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestHTTPHandlerExposesMetrics(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(newServerTestCache()))
	defer ts.Close()

	postJSONStatus(t, ts.URL+"/v1/cache/set", setReq{
		Prompt:   "What is Go?",
		Response: "Go is a compiled language.",
	}, http.StatusOK)
	postJSONStatus(t, ts.URL+"/v1/cache/get", getReq{Prompt: "What is Go?"}, http.StatusOK)
	postJSONStatus(t, ts.URL+"/v1/cache/get", getReq{Prompt: "What is Rust?"}, http.StatusOK)

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}

	text := string(body)
	for _, want := range []string{
		`erion_ember_http_requests_total{method="POST",path="/v1/cache/get",status="200"} 2`,
		`erion_ember_http_request_duration_seconds_count{method="POST",path="/v1/cache/get",status="200"} 2`,
		`erion_ember_cache_hits_total 1`,
		`erion_ember_cache_misses_total 1`,
		`erion_ember_cache_queries_total 2`,
		`erion_ember_cache_entries 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics output missing %q\n%s", want, text)
		}
	}
}

func TestHTTPHandlerLogsRequestsAtDebugLevel(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(newServerTestCache()))
	defer ts.Close()

	handler := &captureHandler{}
	logger := slog.New(handler)
	previous := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(previous)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if handler.count(slog.LevelInfo, "http request completed") != 0 {
		t.Fatal("expected no info-level request logs")
	}
	if handler.count(slog.LevelDebug, "http request completed") == 0 {
		t.Fatal("expected debug-level request log")
	}
}

func postJSON(t *testing.T, url string, body any, wantStatus int, out any) {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		t.Fatalf("got status %d, want %d", resp.StatusCode, wantStatus)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}

func postJSONStatus(t *testing.T, url string, body any, wantStatus int) {
	t.Helper()
	postJSON(t, url, body, wantStatus, nil)
}

func getJSON(t *testing.T, url string, wantStatus int, out any) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		t.Fatalf("got status %d, want %d", resp.StatusCode, wantStatus)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
