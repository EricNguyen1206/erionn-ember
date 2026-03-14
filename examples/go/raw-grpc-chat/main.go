package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	emberv1 "github.com/EricNguyen1206/erion-ember/proto/ember/v1"
)

const (
	grpcAddress = "localhost:9090"
	prompt      = "How do I explain Go channels to a teammate?"
	ttlSeconds  = int64(3600)
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial gRPC server: %v", err)
	}
	defer conn.Close()

	client := emberv1.NewSemanticCacheServiceClient(conn)

	if _, err := client.Health(ctx, &emberv1.HealthRequest{}); err != nil {
		log.Fatalf("service health check failed: %v", err)
	}

	firstLookup, err := client.Get(ctx, &emberv1.GetRequest{Prompt: prompt, SimilarityThreshold: 0.85})
	if err != nil {
		log.Fatalf("initial cache lookup failed: %v", err)
	}
	printLookup("first lookup", firstLookup)

	if !firstLookup.GetHit() {
		llmResponse := generateLLMResponse(prompt)
		setResp, err := client.Set(ctx, &emberv1.SetRequest{
			Prompt:     prompt,
			Response:   llmResponse,
			TtlSeconds: ttlSeconds,
		})
		if err != nil {
			log.Fatalf("cache set failed: %v", err)
		}

		fmt.Printf("cache miss -> placeholder LLM response stored with id=%s\n", setResp.GetId())
	}

	secondLookup, err := client.Get(ctx, &emberv1.GetRequest{Prompt: prompt, SimilarityThreshold: 0.85})
	if err != nil {
		log.Fatalf("follow-up cache lookup failed: %v", err)
	}
	printLookup("second lookup", secondLookup)

	stats, err := client.Stats(ctx, &emberv1.StatsRequest{})
	if err != nil {
		log.Fatalf("stats lookup failed: %v", err)
	}

	fmt.Printf("stats: hits=%d misses=%d total_queries=%d\n", stats.GetCacheHits(), stats.GetCacheMisses(), stats.GetTotalQueries())
}

func generateLLMResponse(prompt string) string {
	return fmt.Sprintf("[placeholder LLM] A simple answer for %q is: channels let goroutines synchronize and pass work safely.", prompt)
}

func printLookup(label string, resp *emberv1.GetResponse) {
	if resp.GetHit() {
		fmt.Printf("%s: hit=true exact_match=%t similarity=%.2f response=%q\n", label, resp.GetExactMatch(), resp.GetSimilarity(), resp.GetResponse())
		return
	}

	fmt.Printf("%s: hit=false -> call your LLM and then cache the answer\n", label)
}
