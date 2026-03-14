package server

import (
	"context"
	"fmt"
	"net"
	"time"

	emberv1 "github.com/EricNguyen1206/erion-ember/proto/ember/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

// GRPCServer exposes SemanticCache over gRPC.
type GRPCServer struct {
	listener net.Listener
	server   *grpc.Server
}

func NewGRPCServer(addr string, sc *cache.SemanticCache) (*GRPCServer, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	emberv1.RegisterSemanticCacheServiceServer(grpcServer, &semanticCacheService{cache: sc})

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

type semanticCacheService struct {
	emberv1.UnimplementedSemanticCacheServiceServer
	cache *cache.SemanticCache
}

func (s *semanticCacheService) ready() bool {
	return s != nil && s.cache != nil
}

func (s *semanticCacheService) Get(ctx context.Context, req *emberv1.GetRequest) (*emberv1.GetResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}

	if req == nil || !hasText(req.Prompt) {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}

	result, hit := s.cache.Get(ctx, req.Prompt, req.SimilarityThreshold)
	resp := &emberv1.GetResponse{Hit: hit}
	if hit && result != nil {
		resp.Response = result.Response
		resp.Similarity = result.Similarity
		resp.ExactMatch = result.ExactMatch
	}

	return resp, nil
}

func (s *semanticCacheService) Set(ctx context.Context, req *emberv1.SetRequest) (*emberv1.SetResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}

	if req == nil || !hasText(req.Prompt) || !hasText(req.Response) {
		return nil, status.Error(codes.InvalidArgument, "prompt and response are required")
	}
	if req.TtlSeconds < 0 {
		return nil, status.Error(codes.InvalidArgument, "ttl must be non-negative")
	}

	id, err := s.cache.Set(ctx, req.Prompt, req.Response, time.Duration(req.TtlSeconds)*time.Second)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set cache entry: %v", err)
	}

	return &emberv1.SetResponse{Id: id}, nil
}

func (s *semanticCacheService) Delete(ctx context.Context, req *emberv1.DeleteRequest) (*emberv1.DeleteResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}

	if req == nil || !hasText(req.Prompt) {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}

	return &emberv1.DeleteResponse{Deleted: s.cache.Delete(req.Prompt)}, nil
}

func (s *semanticCacheService) Stats(ctx context.Context, req *emberv1.StatsRequest) (*emberv1.StatsResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}

	st := s.cache.Stats()
	return &emberv1.StatsResponse{
		TotalEntries: int64(st.TotalEntries),
		CacheHits:    st.CacheHits,
		CacheMisses:  st.CacheMisses,
		TotalQueries: st.TotalQueries,
		HitRate:      st.HitRate,
	}, nil
}

func (s *semanticCacheService) Health(ctx context.Context, req *emberv1.HealthRequest) (*emberv1.HealthResponse, error) {
	if !s.ready() {
		return nil, status.Error(codes.Unavailable, "service not ready")
	}

	return &emberv1.HealthResponse{Status: "ready"}, nil
}
