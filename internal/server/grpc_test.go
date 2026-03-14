package server

import (
	"context"
	"net"
	"testing"

	emberv1 "github.com/EricNguyen1206/erion-ember/proto/ember/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGRPCServiceCRUDFlow(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	ctx := context.Background()

	setResponse, err := client.Set(ctx, &emberv1.SetRequest{
		Prompt:   "What is Go?",
		Response: "Go is a compiled language.",
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if setResponse.Id == "" {
		t.Fatal("expected non-empty cache id")
	}

	getResponse, err := client.Get(ctx, &emberv1.GetRequest{Prompt: "What is Go?"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !getResponse.Hit {
		t.Fatal("expected cache hit")
	}
	if !getResponse.ExactMatch {
		t.Fatal("expected exact match")
	}

	statsResponse, err := client.Stats(ctx, &emberv1.StatsRequest{})
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if statsResponse.TotalEntries != 1 {
		t.Fatalf("got %d entries, want 1", statsResponse.TotalEntries)
	}

	deleteResponse, err := client.Delete(ctx, &emberv1.DeleteRequest{Prompt: "What is Go?"})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleteResponse.Deleted {
		t.Fatal("expected delete success")
	}

	getResponse, err = client.Get(ctx, &emberv1.GetRequest{Prompt: "What is Go?"})
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if getResponse.Hit {
		t.Fatal("expected miss after delete")
	}

	healthResponse, err := client.Health(ctx, &emberv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if healthResponse.Status != "ready" {
		t.Fatalf("got health status %q", healthResponse.Status)
	}
}

func TestGRPCServiceValidation(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	_, err := client.Set(context.Background(), &emberv1.SetRequest{Prompt: " ", Response: "value"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.InvalidArgument)
	}

	_, err = client.Set(context.Background(), &emberv1.SetRequest{Prompt: "prompt", Response: "value", TtlSeconds: -1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestGRPCServiceHealthReportsReadiness(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	resp, err := client.Health(context.Background(), &emberv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if resp.Status != "ready" {
		t.Fatalf("got health status %q, want %q", resp.Status, "ready")
	}
}

func TestGRPCServiceHealthRequiresInitializedCache(t *testing.T) {
	service := &semanticCacheService{}

	_, err := service.Health(context.Background(), &emberv1.HealthRequest{})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.Unavailable)
	}
}

func TestGRPCServiceGetRequiresInitializedCache(t *testing.T) {
	service := &semanticCacheService{}

	_, err := service.Get(context.Background(), &emberv1.GetRequest{Prompt: "What is Go?"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.Unavailable)
	}
}

func newBufconnClient(t *testing.T) (emberv1.SemanticCacheServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	emberv1.RegisterSemanticCacheServiceServer(grpcServer, &semanticCacheService{cache: newServerTestCache()})

	go func() {
		_ = grpcServer.Serve(listener)
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		listener.Close()
		grpcServer.Stop()
		t.Fatalf("NewClient: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		grpcServer.Stop()
		_ = listener.Close()
	}

	return emberv1.NewSemanticCacheServiceClient(conn), cleanup
}
