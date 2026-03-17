# Quickstart Guide

Welcome to **Erionn Ember** - A miniature, in-memory cache server written in Go that communicates via gRPC. 

This guide will help you start the server and connect your first application to Ember.

## 1. Running the Ember Server

First, you need to build and run the server locally.

```bash
# Compile code
make build

# Run the server
./bin/erionn-ember
```

The server will start and listen on a single gRPC port:
- `grpc://localhost:9090`: The main port for data communication and `grpc.health.v1` health checks.

Verify that the server is running using `grpc_health_probe`:
```bash
grpc_health_probe -addr=localhost:9090
# Should return: status: SERVING
```

## 2. Preparing the Client

Ember communicates exclusively via gRPC protobuf. In your client project (e.g., Golang), you need to include Ember's protobuf file `proto/ember/v1/cache.proto` and install the gRPC package.

```bash
go get google.golang.org/grpc
go get google.golang.org/grpc/credentials/insecure
```

## 3. Your First Code (Hello World)

Let's try connecting to the Server and performing basic `Set` and `Get` operations.

```go
package main

import (
	"context"
	"fmt"
	"log"

	pb "github.com/ericnguyen1206/erionn-ember/proto/ember/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Connect to local port 9090 (Insecure for local dev)
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewCacheServiceClient(conn)
	ctx := context.Background()

	// 1. Store Data (String)
	ttl := int32(3600) // Live for 1 hour
	_, err = client.Set(ctx, &pb.SetRequest{
		Key:        "hello",
		Value:      "world",
		TtlSeconds: &ttl,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Successfully stored key 'hello'.")

	// 2. Retrieve Data
	res, err := client.Get(ctx, &pb.GetRequest{Key: "hello"})
	if err != nil {
		log.Fatal(err)
	}
	
	if res.Found {
		fmt.Printf("Value of 'hello' is: %s\n", res.Value)
	} else {
		fmt.Println("Data not found!")
	}
}
```

## What's Next?

Now that you know how to connect, check out these guides to master Ember:
- [Data Model & Structures](DATA_MODEL.md): How to use Hashes, Lists, and Sets.
- [Pub/Sub Guide](PUB_SUB.md): Using Ember as a real-time Message Broker.
- [API Reference](API_REFERENCE.md): Detailed reference of all gRPC commands.
