package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	emberv1 "github.com/EricNguyen1206/erion-ember/proto/ember/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/EricNguyen1206/erion-ember/internal/pubsub"
	"github.com/EricNguyen1206/erion-ember/internal/store"
)

type GRPCServer struct {
	listener net.Listener
	server   *grpc.Server
}

func NewGRPCServer(addr string, s *store.Store, h *pubsub.Hub) (*GRPCServer, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	emberv1.RegisterCacheServiceServer(grpcServer, &cacheService{store: s, hub: h})

	return &GRPCServer{listener: listener, server: grpcServer}, nil
}

func (s *GRPCServer) Addr() net.Addr { return s.listener.Addr() }

func (s *GRPCServer) Serve() error {
	return s.server.Serve(s.listener)
}

func (s *GRPCServer) GracefulStop() {
	s.server.GracefulStop()
}

func (s *GRPCServer) Stop() {
	s.server.Stop()
}

type cacheService struct {
	emberv1.UnimplementedCacheServiceServer
	store *store.Store
	hub   *pubsub.Hub
}

func (s *cacheService) ready() bool {
	return s != nil && s.store != nil && s.hub != nil
}

func (s *cacheService) Get(ctx context.Context, req *emberv1.GetRequest) (*emberv1.GetResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	value, found, err := s.store.GetString(req.GetKey())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.GetResponse{Found: found, Value: value}, nil
}

func (s *cacheService) Set(ctx context.Context, req *emberv1.SetRequest) (*emberv1.SetResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}
	if req.GetTtlSeconds() < 0 {
		return nil, status.Error(codes.InvalidArgument, "ttl must be non-negative")
	}

	if err := s.store.SetString(req.GetKey(), req.GetValue(), time.Duration(req.GetTtlSeconds())*time.Second); err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.SetResponse{}, nil
}

func (s *cacheService) Del(ctx context.Context, req *emberv1.DelRequest) (*emberv1.DelResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	return &emberv1.DelResponse{Deleted: s.store.Del(req.GetKey())}, nil
}

func (s *cacheService) Exists(ctx context.Context, req *emberv1.ExistsRequest) (*emberv1.ExistsResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	return &emberv1.ExistsResponse{Exists: s.store.Exists(req.GetKey())}, nil
}

func (s *cacheService) Type(ctx context.Context, req *emberv1.TypeRequest) (*emberv1.TypeResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	return &emberv1.TypeResponse{Type: string(s.store.Type(req.GetKey()))}, nil
}

func (s *cacheService) Expire(ctx context.Context, req *emberv1.ExpireRequest) (*emberv1.ExpireResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}
	if req.GetTtlSeconds() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "ttl must be positive")
	}

	updated := s.store.Expire(req.GetKey(), time.Duration(req.GetTtlSeconds())*time.Second)
	return &emberv1.ExpireResponse{Updated: updated}, nil
}

func (s *cacheService) Ttl(ctx context.Context, req *emberv1.TtlRequest) (*emberv1.TtlResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	ttl, hasTTL, found := s.store.TTL(req.GetKey())
	if !found {
		return &emberv1.TtlResponse{Found: false}, nil
	}
	if !hasTTL {
		return &emberv1.TtlResponse{Found: true, HasExpiration: false}, nil
	}

	return &emberv1.TtlResponse{
		Found:         true,
		HasExpiration: true,
		TtlSeconds:    durationToSeconds(ttl),
	}, nil
}

func (s *cacheService) HSet(ctx context.Context, req *emberv1.HSetRequest) (*emberv1.HSetResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	added, err := s.store.HSet(req.GetKey(), req.GetFields())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.HSetResponse{Added: int64(added)}, nil
}

func (s *cacheService) HGet(ctx context.Context, req *emberv1.HGetRequest) (*emberv1.HGetResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	value, found, err := s.store.HGet(req.GetKey(), req.GetField())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.HGetResponse{Found: found, Value: value}, nil
}

func (s *cacheService) HDel(ctx context.Context, req *emberv1.HDelRequest) (*emberv1.HDelResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	removed, err := s.store.HDel(req.GetKey(), req.GetFields())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.HDelResponse{Removed: int64(removed)}, nil
}

func (s *cacheService) HGetAll(ctx context.Context, req *emberv1.HGetAllRequest) (*emberv1.HGetAllResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	fields, found, err := s.store.HGetAll(req.GetKey())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.HGetAllResponse{Found: found, Fields: fields}, nil
}

func (s *cacheService) LPush(ctx context.Context, req *emberv1.LPushRequest) (*emberv1.LPushResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	length, err := s.store.LPush(req.GetKey(), req.GetValues())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.LPushResponse{Length: int64(length)}, nil
}

func (s *cacheService) RPush(ctx context.Context, req *emberv1.RPushRequest) (*emberv1.RPushResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	length, err := s.store.RPush(req.GetKey(), req.GetValues())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.RPushResponse{Length: int64(length)}, nil
}

func (s *cacheService) LPop(ctx context.Context, req *emberv1.LPopRequest) (*emberv1.LPopResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	value, found, err := s.store.LPop(req.GetKey())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.LPopResponse{Found: found, Value: value}, nil
}

func (s *cacheService) RPop(ctx context.Context, req *emberv1.RPopRequest) (*emberv1.RPopResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	value, found, err := s.store.RPop(req.GetKey())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.RPopResponse{Found: found, Value: value}, nil
}

func (s *cacheService) LRange(ctx context.Context, req *emberv1.LRangeRequest) (*emberv1.LRangeResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	values, found, err := s.store.LRange(req.GetKey(), req.GetStart(), req.GetStop())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.LRangeResponse{Found: found, Values: values}, nil
}

func (s *cacheService) SAdd(ctx context.Context, req *emberv1.SAddRequest) (*emberv1.SAddResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	added, err := s.store.SAdd(req.GetKey(), req.GetMembers())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.SAddResponse{Added: int64(added)}, nil
}

func (s *cacheService) SRem(ctx context.Context, req *emberv1.SRemRequest) (*emberv1.SRemResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	removed, err := s.store.SRem(req.GetKey(), req.GetMembers())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.SRemResponse{Removed: int64(removed)}, nil
}

func (s *cacheService) SMembers(ctx context.Context, req *emberv1.SMembersRequest) (*emberv1.SMembersResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	members, found, err := s.store.SMembers(req.GetKey())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.SMembersResponse{Found: found, Members: members}, nil
}

func (s *cacheService) SIsMember(ctx context.Context, req *emberv1.SIsMemberRequest) (*emberv1.SIsMemberResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if err := validateKeyStatus(req.GetKey()); err != nil {
		return nil, err
	}

	isMember, err := s.store.SIsMember(req.GetKey(), req.GetMember())
	if err != nil {
		return nil, statusForStoreError(err)
	}

	return &emberv1.SIsMemberResponse{IsMember: isMember}, nil
}

func (s *cacheService) Publish(ctx context.Context, req *emberv1.PublishRequest) (*emberv1.PublishResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}
	if !hasText(req.GetChannel()) {
		return nil, status.Error(codes.InvalidArgument, "channel is required")
	}

	delivered := s.hub.Publish(req.GetChannel(), req.GetPayload())
	return &emberv1.PublishResponse{Delivered: int64(delivered)}, nil
}

func (s *cacheService) Subscribe(req *emberv1.SubscribeRequest, stream grpc.ServerStreamingServer[emberv1.SubscribeMessage]) error {
	if !s.ready() {
		return status.Error(codes.Unavailable, "service not ready")
	}

	channels := sanitizeChannels(req.GetChannels())
	if len(channels) == 0 {
		return status.Error(codes.InvalidArgument, "at least one channel is required")
	}

	sub := s.hub.Subscribe(channels)
	defer s.hub.Remove(sub.ID)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case msg, ok := <-sub.Messages:
			if !ok {
				return nil
			}
			if err := stream.Send(&emberv1.SubscribeMessage{
				Channel:         msg.Channel,
				Payload:         msg.Payload,
				PublishedAtUnix: msg.PublishedAt.Unix(),
			}); err != nil {
				return err
			}
		}
	}
}

func (s *cacheService) Stats(ctx context.Context, req *emberv1.StatsRequest) (*emberv1.StatsResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}

	storeStats := s.store.Stats()
	hubStats := s.hub.Stats()
	return &emberv1.StatsResponse{
		TotalKeys:   storeStats.TotalKeys,
		StringKeys:  storeStats.StringKeys,
		HashKeys:    storeStats.HashKeys,
		ListKeys:    storeStats.ListKeys,
		SetKeys:     storeStats.SetKeys,
		Channels:    hubStats.Channels,
		Subscribers: hubStats.Subscribers,
	}, nil
}

func (s *cacheService) Health(ctx context.Context, req *emberv1.HealthRequest) (*emberv1.HealthResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}

	return &emberv1.HealthResponse{Status: "ready"}, nil
}

func durationToSeconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	seconds := value / time.Second
	if value%time.Second != 0 {
		seconds++
	}
	return int64(seconds)
}

func sanitizeChannels(channels []string) []string {
	seen := make(map[string]struct{}, len(channels))
	result := make([]string, 0, len(channels))
	for _, channel := range channels {
		channel = strings.TrimSpace(channel)
		if channel == "" {
			continue
		}
		if _, exists := seen[channel]; exists {
			continue
		}
		seen[channel] = struct{}{}
		result = append(result, channel)
	}
	return result
}

func validateKeyStatus(key string) error {
	if !hasText(key) {
		return status.Error(codes.InvalidArgument, store.ErrEmptyKey.Error())
	}
	return nil
}

func statusForStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrEmptyKey) || errors.Is(err, store.ErrEmptyFieldSet) || errors.Is(err, store.ErrEmptyValues) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, store.ErrWrongType) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, store.ErrInvalidValue) {
		return status.Error(codes.Internal, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}
