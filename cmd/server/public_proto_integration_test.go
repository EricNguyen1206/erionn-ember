package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	emberv1 "github.com/EricNguyen1206/erion-ember/proto/ember/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestPublicProtoClientAgainstRunningServer(t *testing.T) {
	t.Parallel()

	httpPort := freePort(t)
	grpcPort := freePort(t)
	serverDir := filepath.Join(repoRoot(t), "cmd", "server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", ".")
	cmd.Dir = serverDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HTTP_PORT=%s", httpPort),
		fmt.Sprintf("GRPC_PORT=%s", grpcPort),
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_, _ = cmd.Process.Wait()
	}()

	waitForReady(t, "http://127.0.0.1:"+httpPort+"/ready")

	conn, err := grpc.NewClient(
		"127.0.0.1:"+grpcPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial gRPC server: %v", err)
	}
	defer conn.Close()

	client := emberv1.NewCacheServiceClient(conn)
	grpcCtx, grpcCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer grpcCancel()

	_, err = client.Set(grpcCtx, &emberv1.SetRequest{Key: "language:go", Value: "Go is a compiled language."})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	getResp, err := client.Get(grpcCtx, &emberv1.GetRequest{Key: "language:go"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !getResp.GetFound() || getResp.GetValue() != "Go is a compiled language." {
		t.Fatalf("expected stored value, got found=%t value=%q", getResp.GetFound(), getResp.GetValue())
	}

	healthResp, err := client.Health(grpcCtx, &emberv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if healthResp.GetStatus() != "ready" {
		t.Fatalf("got health status %q, want %q", healthResp.GetStatus(), "ready")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	return filepath.Dir(filepath.Dir(wd))
}

func freePort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	defer listener.Close()

	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func waitForReady(t *testing.T, readyURL string) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(readyURL)
		if err == nil {
			var body struct {
				Status string `json:"status"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&body)
			_ = resp.Body.Close()
			if decodeErr == nil && resp.StatusCode == http.StatusOK && body.Status == "ready" {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("server did not become ready at %s", readyURL)
}
