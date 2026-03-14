package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
	embv1 "github.com/EricNguyen1206/erion-ember/internal/gen/ember/v1"
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
	RegisterSemanticCacheServiceServer(grpcServer, &semanticCacheService{cache: sc})

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

type SemanticCacheServiceServer = embv1.SemanticCacheServiceServer
type SemanticCacheServiceClient = embv1.SemanticCacheServiceClient

type Namespace = embv1.Namespace
type GetRequest = embv1.GetRequest
type GetResponse = embv1.GetResponse
type SetRequest = embv1.SetRequest
type SetResponse = embv1.SetResponse
type DeleteRequest = embv1.DeleteRequest
type DeleteResponse = embv1.DeleteResponse
type StatsRequest = embv1.StatsRequest
type StatsResponse = embv1.StatsResponse
type HealthRequest = embv1.HealthRequest
type HealthResponse = embv1.HealthResponse

var RegisterSemanticCacheServiceServer = embv1.RegisterSemanticCacheServiceServer
var NewSemanticCacheServiceClient = embv1.NewSemanticCacheServiceClient

type semanticCacheService struct {
	embv1.UnimplementedSemanticCacheServiceServer
	cache *cache.SemanticCache
}

func (s *semanticCacheService) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	if err := validateNamespace(req); err != nil {
		return nil, err
	}
	if req == nil || !hasText(req.Prompt) {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}

	result, hit := s.cache.GetInNamespace(ctx, cacheNamespace(req.Namespace), req.Prompt, req.SimilarityThreshold)
	resp := &GetResponse{Hit: hit}
	if hit && result != nil {
		resp.Response = result.Response
		resp.Similarity = result.Similarity
		resp.ExactMatch = result.ExactMatch
	}

	return resp, nil
}

func (s *semanticCacheService) Set(ctx context.Context, req *SetRequest) (*SetResponse, error) {
	if err := validateNamespace(req); err != nil {
		return nil, err
	}
	if req == nil || !hasText(req.Prompt) || !hasText(req.Response) {
		return nil, status.Error(codes.InvalidArgument, "prompt and response are required")
	}
	if req.TtlSeconds < 0 {
		return nil, status.Error(codes.InvalidArgument, "ttl must be non-negative")
	}

	id, err := s.cache.SetInNamespace(ctx, cacheNamespace(req.Namespace), req.Prompt, req.Response, time.Duration(req.TtlSeconds)*time.Second)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set cache entry: %v", err)
	}

	return &SetResponse{Id: id}, nil
}

func (s *semanticCacheService) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	if err := validateNamespace(req); err != nil {
		return nil, err
	}
	if req == nil || !hasText(req.Prompt) {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}

	return &DeleteResponse{Deleted: s.cache.DeleteInNamespace(cacheNamespace(req.Namespace), req.Prompt)}, nil
}

func (s *semanticCacheService) Stats(ctx context.Context, req *StatsRequest) (*StatsResponse, error) {
	st := s.cache.Stats()
	return &StatsResponse{
		TotalEntries: int64(st.TotalEntries),
		CacheHits:    st.CacheHits,
		CacheMisses:  st.CacheMisses,
		TotalQueries: st.TotalQueries,
		HitRate:      st.HitRate,
	}, nil
}

func (s *semanticCacheService) Health(ctx context.Context, req *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok"}, nil
}

type namespaceRequest interface {
	GetNamespace() *Namespace
}

func validateNamespace(req namespaceRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "namespace is required")
	}

	ns := req.GetNamespace()
	if ns == nil || !hasText(ns.GetModel()) || !hasText(ns.GetTenantId()) || !hasText(ns.GetSystemPromptHash()) {
		return status.Error(codes.InvalidArgument, "namespace is required")
	}

	return nil
}

func cacheNamespace(ns *Namespace) cache.Namespace {
	if ns == nil {
		return cache.Namespace{}
	}

	return cache.Namespace{
		Model:            ns.GetModel(),
		TenantID:         ns.GetTenantId(),
		SystemPromptHash: ns.GetSystemPromptHash(),
	}
}
