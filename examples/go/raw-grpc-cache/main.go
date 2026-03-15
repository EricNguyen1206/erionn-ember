package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	emberv1 "github.com/EricNguyen1206/erion-ember/proto/ember/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultGRPCAddress = "localhost:9090"

func main() {
	client, conn, err := dialCache()
	if err != nil {
		log.Fatalf("dial gRPC server: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Health(ctx, &emberv1.HealthRequest{}); err != nil {
		log.Fatalf("service health check failed: %v", err)
	}
	if _, err := client.Set(ctx, &emberv1.SetRequest{Key: "language:go", Value: "Go is a compiled language."}); err != nil {
		log.Fatalf("Set failed: %v", err)
	}

	resp, err := client.Get(ctx, &emberv1.GetRequest{Key: "language:go"})
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}

	fmt.Printf("found=%t value=%q\n", resp.GetFound(), resp.GetValue())
}

func dialCache() (emberv1.CacheServiceClient, *grpc.ClientConn, error) {
	addr := os.Getenv("GRPC_ADDRESS")
	if addr == "" {
		addr = defaultGRPCAddress
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return emberv1.NewCacheServiceClient(conn), conn, nil
}
