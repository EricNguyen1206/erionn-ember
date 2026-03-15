package main_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRawGRPCCacheExampleAgainstRunningServer(t *testing.T) {
	output := runExampleAgainstServer(t, filepath.Join("examples", "go", "raw-grpc-cache"))
	if !strings.Contains(output, `found=true value="Go is a compiled language."`) {
		t.Fatalf("unexpected example output: %s", output)
	}
}

func TestRawGRPCPubSubExampleAgainstRunningServer(t *testing.T) {
	output := runExampleAgainstServer(t, filepath.Join("examples", "go", "raw-grpc-pubsub"))
	if !strings.Contains(output, "channel=news payload=hello") {
		t.Fatalf("unexpected example output: %s", output)
	}
}

func runExampleAgainstServer(t *testing.T, exampleDir string) string {
	t.Helper()

	httpPort := freePort(t)
	grpcPort := freePort(t)
	repoDir := repoRoot(t)
	serverDir := filepath.Join(repoDir, "cmd", "server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverCmd := exec.CommandContext(ctx, "go", "run", ".")
	serverCmd.Dir = serverDir
	serverCmd.Env = append(os.Environ(),
		fmt.Sprintf("HTTP_PORT=%s", httpPort),
		fmt.Sprintf("GRPC_PORT=%s", grpcPort),
	)
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		_ = serverCmd.Process.Signal(os.Interrupt)
		_, _ = serverCmd.Process.Wait()
	}()

	waitForReady(t, "http://127.0.0.1:"+httpPort+"/ready")

	exampleCmd := exec.CommandContext(ctx, "go", "run", ".")
	exampleCmd.Dir = filepath.Join(repoDir, exampleDir)
	exampleCmd.Env = append(os.Environ(), fmt.Sprintf("GRPC_ADDRESS=127.0.0.1:%s", grpcPort))
	output, err := exampleCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run example %s: %v\n%s", exampleDir, err, string(output))
	}

	return string(output)
}
