package server

import (
	"context"
	"net"
	"reflect"
	"strings"
	"testing"

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
	namespaceA := testNamespace()
	namespaceB := &Namespace{Model: namespaceA.Model, TenantId: "tenant-b", SystemPromptHash: namespaceA.SystemPromptHash}

	setResponse, err := client.Set(ctx, &SetRequest{
		Namespace: namespaceA,
		Prompt:    "What is Go?",
		Response:  "Go is a compiled language.",
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if setResponse.Id == "" {
		t.Fatal("expected non-empty cache id")
	}

	if _, err := client.Set(ctx, &SetRequest{
		Namespace: namespaceB,
		Prompt:    "What is Go?",
		Response:  "Go is a systems language.",
	}); err != nil {
		t.Fatalf("Set namespace B failed: %v", err)
	}

	getResponse, err := client.Get(ctx, &GetRequest{Namespace: namespaceA, Prompt: "What is Go?"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !getResponse.Hit {
		t.Fatal("expected cache hit")
	}
	if !getResponse.ExactMatch {
		t.Fatal("expected exact match")
	}
	if getResponse.Response != "Go is a compiled language." {
		t.Fatalf("namespace A response = %q, want %q", getResponse.Response, "Go is a compiled language.")
	}

	getResponse, err = client.Get(ctx, &GetRequest{Namespace: namespaceB, Prompt: "What is Go?"})
	if err != nil {
		t.Fatalf("Get namespace B failed: %v", err)
	}
	if !getResponse.Hit {
		t.Fatal("expected namespace B hit")
	}
	if getResponse.Response != "Go is a systems language." {
		t.Fatalf("namespace B response = %q, want %q", getResponse.Response, "Go is a systems language.")
	}

	statsResponse, err := client.Stats(ctx, &StatsRequest{})
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if statsResponse.TotalEntries != 2 {
		t.Fatalf("got %d entries, want 2", statsResponse.TotalEntries)
	}

	deleteResponse, err := client.Delete(ctx, &DeleteRequest{Namespace: namespaceA, Prompt: "What is Go?"})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleteResponse.Deleted {
		t.Fatal("expected delete success")
	}

	getResponse, err = client.Get(ctx, &GetRequest{Namespace: namespaceA, Prompt: "What is Go?"})
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if getResponse.Hit {
		t.Fatal("expected miss after delete")
	}

	getResponse, err = client.Get(ctx, &GetRequest{Namespace: namespaceB, Prompt: "What is Go?"})
	if err != nil {
		t.Fatalf("Get namespace B after delete failed: %v", err)
	}
	if !getResponse.Hit {
		t.Fatal("expected namespace B entry to remain after namespace A delete")
	}
	if getResponse.Response != "Go is a systems language." {
		t.Fatalf("namespace B response after delete = %q, want %q", getResponse.Response, "Go is a systems language.")
	}

	healthResponse, err := client.Health(ctx, &HealthRequest{})
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if healthResponse.Status != "ok" {
		t.Fatalf("got health status %q", healthResponse.Status)
	}
}

func TestGRPCServiceValidation(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	namespace := testNamespace()

	_, err := client.Set(context.Background(), &SetRequest{Namespace: namespace, Prompt: " ", Response: "value"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.InvalidArgument)
	}

	_, err = client.Set(context.Background(), &SetRequest{Namespace: namespace, Prompt: "prompt", Response: "value", TtlSeconds: -1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.InvalidArgument)
	}

	if _, err := client.Set(context.Background(), &SetRequest{Namespace: namespace, Prompt: "prompt", Response: "value"}); err != nil {
		t.Fatalf("Set with namespace failed: %v", err)
	}

	t.Run("set requires namespace", func(t *testing.T) {
		_, err := client.Set(context.Background(), &SetRequest{Prompt: "prompt", Response: "value"})
		assertInvalidNamespace(t, err)
	})

	t.Run("set requires complete namespace", func(t *testing.T) {
		_, err := client.Set(context.Background(), &SetRequest{Namespace: &Namespace{Model: namespace.Model}, Prompt: "prompt", Response: "value"})
		assertInvalidNamespace(t, err)
	})

	t.Run("get requires namespace", func(t *testing.T) {
		_, err := client.Get(context.Background(), &GetRequest{Prompt: "prompt"})
		assertInvalidNamespace(t, err)
	})

	t.Run("get requires complete namespace", func(t *testing.T) {
		_, err := client.Get(context.Background(), &GetRequest{Namespace: &Namespace{Model: namespace.Model}, Prompt: "prompt"})
		assertInvalidNamespace(t, err)
	})

	t.Run("delete requires namespace", func(t *testing.T) {
		_, err := client.Delete(context.Background(), &DeleteRequest{Prompt: "prompt"})
		assertInvalidNamespace(t, err)
	})

	t.Run("delete requires complete namespace", func(t *testing.T) {
		_, err := client.Delete(context.Background(), &DeleteRequest{Namespace: &Namespace{Model: namespace.Model}, Prompt: "prompt"})
		assertInvalidNamespace(t, err)
	})

	t.Run("stats does not require namespace", func(t *testing.T) {
		resp, err := client.Stats(context.Background(), &StatsRequest{})
		if err != nil {
			t.Fatalf("Stats without namespace failed: %v", err)
		}
		if resp == nil {
			t.Fatal("expected stats response")
		}
	})
}

func TestGRPCServiceRequiresNamespace(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		message interface{}
	}{
		{name: "GetRequest", message: &GetRequest{}},
		{name: "SetRequest", message: &SetRequest{}},
		{name: "DeleteRequest", message: &DeleteRequest{}},
		{name: "StatsRequest", message: &StatsRequest{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			typeOf := reflect.TypeOf(tc.message).Elem()

			structField, ok := typeOf.FieldByName("Namespace")
			if !ok {
				t.Fatalf("%s missing Namespace field", typeOf.Name())
			}

			namespaceType := reflect.TypeOf((*Namespace)(nil))
			if structField.Type != namespaceType {
				t.Fatalf("%s.Namespace type = %s, want %s", typeOf.Name(), structField.Type, namespaceType)
			}
		})
	}
}

func assertInvalidNamespace(t *testing.T, err error) {
	t.Helper()

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.InvalidArgument)
	}
	if !strings.Contains(status.Convert(err).Message(), "namespace") {
		t.Fatalf("got message %q, want namespace validation", status.Convert(err).Message())
	}
}

func testNamespace() *Namespace {
	return &Namespace{
		Model:            "llama3.1-8b",
		TenantId:         "tenant-a",
		SystemPromptHash: "sys-123",
	}
}

func newBufconnClient(t *testing.T) (SemanticCacheServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	RegisterSemanticCacheServiceServer(grpcServer, &semanticCacheService{cache: newServerTestCache()})

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

	return NewSemanticCacheServiceClient(conn), cleanup
}
