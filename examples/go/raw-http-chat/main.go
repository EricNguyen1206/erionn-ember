package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	httpBaseURL = "http://localhost:8080"
	prompt      = "How do I explain Go channels to a teammate?"
	ttlSeconds  = 3600
)

type getRequest struct {
	Prompt              string  `json:"prompt"`
	SimilarityThreshold float32 `json:"similarity_threshold,omitempty"`
}

type getResponse struct {
	Hit        bool    `json:"hit"`
	Response   string  `json:"response,omitempty"`
	Similarity float32 `json:"similarity"`
	ExactMatch bool    `json:"exact_match"`
}

type setRequest struct {
	Prompt   string `json:"prompt"`
	Response string `json:"response"`
	TTL      int    `json:"ttl,omitempty"`
}

type setResponse struct {
	ID string `json:"id"`
}

type healthResponse struct {
	Status string `json:"status"`
}

type statsResponse struct {
	CacheHits    int64 `json:"cache_hits"`
	CacheMisses  int64 `json:"cache_misses"`
	TotalQueries int64 `json:"total_queries"`
}

func main() {
	client := &http.Client{Timeout: 10 * time.Second}

	var health healthResponse
	requestJSON(client, http.MethodGet, httpBaseURL+"/ready", nil, &health)
	if health.Status != "ready" {
		log.Fatalf("service not ready: status=%q", health.Status)
	}

	var firstLookup getResponse
	requestJSON(client, http.MethodPost, httpBaseURL+"/v1/cache/get", getRequest{
		Prompt:              prompt,
		SimilarityThreshold: 0.85,
	}, &firstLookup)
	printLookup("first lookup", firstLookup)

	if !firstLookup.Hit {
		llmResponse := generateLLMResponse(prompt)

		var setResp setResponse
		requestJSON(client, http.MethodPost, httpBaseURL+"/v1/cache/set", setRequest{
			Prompt:   prompt,
			Response: llmResponse,
			TTL:      ttlSeconds,
		}, &setResp)

		fmt.Printf("cache miss -> placeholder LLM response stored with id=%s\n", setResp.ID)
	}

	var secondLookup getResponse
	requestJSON(client, http.MethodPost, httpBaseURL+"/v1/cache/get", getRequest{
		Prompt:              prompt,
		SimilarityThreshold: 0.85,
	}, &secondLookup)
	printLookup("second lookup", secondLookup)

	var stats statsResponse
	requestJSON(client, http.MethodGet, httpBaseURL+"/v1/stats", nil, &stats)
	fmt.Printf("stats: hits=%d misses=%d total_queries=%d\n", stats.CacheHits, stats.CacheMisses, stats.TotalQueries)
}

func requestJSON(client *http.Client, method, url string, body any, dst any) {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			log.Fatalf("marshal request for %s %s: %v", method, url, err)
		}
		payload = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		log.Fatalf("build request for %s %s: %v", method, url, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("send request for %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Fatalf("read error response for %s %s: %v", method, url, readErr)
		}
		log.Fatalf("%s %s failed: status=%s body=%q", method, url, resp.Status, string(data))
	}

	if dst == nil {
		return
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		log.Fatalf("decode response for %s %s: %v", method, url, err)
	}
}

func generateLLMResponse(prompt string) string {
	return fmt.Sprintf("[placeholder LLM] A simple answer for %q is: channels let goroutines synchronize and pass work safely.", prompt)
}

func printLookup(label string, resp getResponse) {
	if resp.Hit {
		fmt.Printf("%s: hit=true exact_match=%t similarity=%.2f response=%q\n", label, resp.ExactMatch, resp.Similarity, resp.Response)
		return
	}

	fmt.Printf("%s: hit=false -> call your LLM and then cache the answer\n", label)
}
