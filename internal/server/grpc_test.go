package server

import (
	"context"
	"net"
	"testing"

	"github.com/EricNguyen1206/erionn-ember/internal/pubsub"
	"github.com/EricNguyen1206/erionn-ember/internal/store"
	emberv1 "github.com/EricNguyen1206/erionn-ember/proto/ember/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGRPCServiceStringFlow(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	ctx := context.Background()

	if _, err := client.Set(ctx, &emberv1.SetRequest{Key: "name", Value: "ember"}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	getResponse, err := client.Get(ctx, &emberv1.GetRequest{Key: "name"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !getResponse.Found {
		t.Fatal("expected value to exist")
	}
	if getResponse.Value != "ember" {
		t.Fatalf("got value %q, want %q", getResponse.Value, "ember")
	}

	existsResponse, err := client.Exists(ctx, &emberv1.ExistsRequest{Key: "name"})
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !existsResponse.Exists {
		t.Fatal("expected key to exist")
	}

	typeResponse, err := client.Type(ctx, &emberv1.TypeRequest{Key: "name"})
	if err != nil {
		t.Fatalf("Type failed: %v", err)
	}
	if typeResponse.Type != "string" {
		t.Fatalf("got type %q, want %q", typeResponse.Type, "string")
	}

	statsResponse, err := client.Stats(ctx, &emberv1.StatsRequest{})
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if statsResponse.TotalKeys != 1 {
		t.Fatalf("got %d keys, want 1", statsResponse.TotalKeys)
	}

	delResponse, err := client.Del(ctx, &emberv1.DelRequest{Key: "name"})
	if err != nil {
		t.Fatalf("Del failed: %v", err)
	}
	if !delResponse.Deleted {
		t.Fatal("expected delete success")
	}

	getResponse, err = client.Get(ctx, &emberv1.GetRequest{Key: "name"})
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if getResponse.Found {
		t.Fatal("expected key to be missing after delete")
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

	_, err := client.Set(context.Background(), &emberv1.SetRequest{Key: " ", Value: "value"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.InvalidArgument)
	}

	_, err = client.Set(context.Background(), &emberv1.SetRequest{Key: "key", Value: "value", TtlSeconds: -1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.InvalidArgument)
	}

	_, err = client.HSet(context.Background(), &emberv1.HSetRequest{Key: "hash", Fields: nil})
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
	service := &cacheService{}

	_, err := service.Health(context.Background(), &emberv1.HealthRequest{})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.Unavailable)
	}
}

func TestGRPCServiceGetRequiresInitializedCache(t *testing.T) {
	service := &cacheService{}

	_, err := service.Get(context.Background(), &emberv1.GetRequest{Key: "name"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.Unavailable)
	}
}

func newBufconnClient(t *testing.T) (emberv1.CacheServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	emberv1.RegisterCacheServiceServer(grpcServer, &cacheService{store: store.New(), hub: pubsub.New(4)})

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

	return emberv1.NewCacheServiceClient(conn), cleanup
}
