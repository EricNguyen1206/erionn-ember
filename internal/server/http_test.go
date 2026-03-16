package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EricNguyen1206/erionn-ember/internal/pubsub"
	"github.com/EricNguyen1206/erionn-ember/internal/store"
)

func TestHTTPHandlerOnlyExposesAdminRoutes(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(store.New(), pubsub.New(4)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, err = http.Post(ts.URL+"/v1/cache/get", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST legacy route: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHTTPHandlerReadinessRequiresInitializedDependencies(t *testing.T) {
	ts := httptest.NewServer(NewHTTPHandler(nil, nil))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "not_ready" {
		t.Fatalf("got readiness status %q", body["status"])
	}
}
