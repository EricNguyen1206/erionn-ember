package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

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
