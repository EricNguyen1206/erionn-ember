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
	client, conn, err := dialPubSub()
	if err != nil {
		log.Fatalf("dial gRPC server: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Subscribe(ctx, &emberv1.SubscribeRequest{Channels: []string{"news"}})
	if err != nil {
		log.Fatalf("Subscribe failed: %v", err)
	}
	if err := waitForSubscriber(ctx, client); err != nil {
		log.Fatalf("subscriber did not register: %v", err)
	}

	if _, err := client.Publish(context.Background(), &emberv1.PublishRequest{Channel: "news", Payload: []byte("hello")}); err != nil {
		log.Fatalf("Publish failed: %v", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		log.Fatalf("Recv failed: %v", err)
	}

	fmt.Printf("channel=%s payload=%s\n", msg.GetChannel(), string(msg.GetPayload()))
}

func dialPubSub() (emberv1.CacheServiceClient, *grpc.ClientConn, error) {
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

func waitForSubscriber(ctx context.Context, client emberv1.CacheServiceClient) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		stats, err := client.Stats(ctx, &emberv1.StatsRequest{})
		if err == nil && stats.GetSubscribers() > 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
