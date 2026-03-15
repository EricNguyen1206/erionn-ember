package server

import (
	"context"
	"testing"
	"time"

	emberv1 "github.com/EricNguyen1206/erion-ember/proto/ember/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCServiceDataTypesAndTTL(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	ctx := context.Background()

	if _, err := client.Set(ctx, &emberv1.SetRequest{Key: "name", Value: "ember", TtlSeconds: 2}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	ttlResp, err := client.Ttl(ctx, &emberv1.TtlRequest{Key: "name"})
	if err != nil {
		t.Fatalf("Ttl failed: %v", err)
	}
	if !ttlResp.Found || !ttlResp.HasExpiration || ttlResp.TtlSeconds <= 0 {
		t.Fatalf("unexpected ttl response: %+v", ttlResp)
	}

	hsetResp, err := client.HSet(ctx, &emberv1.HSetRequest{Key: "user:1", Fields: map[string]string{"name": "eric", "role": "owner"}})
	if err != nil || hsetResp.Added != 2 {
		t.Fatalf("unexpected HSet result: resp=%+v err=%v", hsetResp, err)
	}
	hgetResp, err := client.HGet(ctx, &emberv1.HGetRequest{Key: "user:1", Field: "name"})
	if err != nil || !hgetResp.Found || hgetResp.Value != "eric" {
		t.Fatalf("unexpected HGet result: resp=%+v err=%v", hgetResp, err)
	}

	lpushResp, err := client.LPush(ctx, &emberv1.LPushRequest{Key: "jobs", Values: []string{"b", "a"}})
	if err != nil || lpushResp.Length != 2 {
		t.Fatalf("unexpected LPush result: resp=%+v err=%v", lpushResp, err)
	}
	rpushResp, err := client.RPush(ctx, &emberv1.RPushRequest{Key: "jobs", Values: []string{"c"}})
	if err != nil || rpushResp.Length != 3 {
		t.Fatalf("unexpected RPush result: resp=%+v err=%v", rpushResp, err)
	}
	lrangeResp, err := client.LRange(ctx, &emberv1.LRangeRequest{Key: "jobs", Start: 0, Stop: 2})
	if err != nil || !lrangeResp.Found || len(lrangeResp.Values) != 3 {
		t.Fatalf("unexpected LRange result: resp=%+v err=%v", lrangeResp, err)
	}

	saddResp, err := client.SAdd(ctx, &emberv1.SAddRequest{Key: "tags", Members: []string{"go", "grpc", "go"}})
	if err != nil || saddResp.Added != 2 {
		t.Fatalf("unexpected SAdd result: resp=%+v err=%v", saddResp, err)
	}
	smembersResp, err := client.SMembers(ctx, &emberv1.SMembersRequest{Key: "tags"})
	if err != nil || !smembersResp.Found || len(smembersResp.Members) != 2 {
		t.Fatalf("unexpected SMembers result: resp=%+v err=%v", smembersResp, err)
	}
}

func TestGRPCServiceSubscribeDeliversPublishedMessage(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Subscribe(ctx, &emberv1.SubscribeRequest{Channels: []string{"news"}})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		statsResp, err := client.Stats(context.Background(), &emberv1.StatsRequest{})
		if err == nil && statsResp.Subscribers == 1 {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("subscriber did not register in time")
	}

	pubResp, err := client.Publish(context.Background(), &emberv1.PublishRequest{Channel: "news", Payload: []byte("hello")})
	if err != nil || pubResp.Delivered != 1 {
		t.Fatalf("unexpected Publish result: resp=%+v err=%v", pubResp, err)
	}

	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}
	if msg.Channel != "news" || string(msg.Payload) != "hello" {
		t.Fatalf("unexpected subscribe message: %+v", msg)
	}
}

func TestGRPCServiceWrongTypeReturnsFailedPrecondition(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	ctx := context.Background()
	if _, err := client.Set(ctx, &emberv1.SetRequest{Key: "name", Value: "ember"}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	_, err := client.HGet(ctx, &emberv1.HGetRequest{Key: "name", Field: "field"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}
