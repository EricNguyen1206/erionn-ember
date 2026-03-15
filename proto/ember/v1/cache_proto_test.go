package emberv1_test

import (
	"testing"

	emberv1 "github.com/EricNguyen1206/erion-ember/proto/ember/v1"
	"google.golang.org/grpc"
)

func TestProtoTypesExist(t *testing.T) {
	_ = &emberv1.GetRequest{Key: "alpha"}
	_ = &emberv1.HSetRequest{Key: "users", Fields: map[string]string{"name": "eric"}}
	_ = &emberv1.SubscribeRequest{Channels: []string{"news"}}

	var conn grpc.ClientConnInterface
	_ = emberv1.NewCacheServiceClient(conn)

	type testServer struct {
		emberv1.UnimplementedCacheServiceServer
	}

	var _ emberv1.CacheServiceServer = (*testServer)(nil)

	if emberv1.CacheService_Get_FullMethodName == "" {
		t.Fatal("expected generated full method name for Get")
	}
}
