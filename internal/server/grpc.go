package server

import (
	"context"
	"fmt"
	"net"
	"time"

	oldproto "github.com/golang/protobuf/proto"
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

// SemanticCacheServiceServer defines the gRPC semantic cache contract.
type SemanticCacheServiceServer interface {
	Get(context.Context, *GetRequest) (*GetResponse, error)
	Set(context.Context, *SetRequest) (*SetResponse, error)
	Delete(context.Context, *DeleteRequest) (*DeleteResponse, error)
	Stats(context.Context, *StatsRequest) (*StatsResponse, error)
	Health(context.Context, *HealthRequest) (*HealthResponse, error)
}

// SemanticCacheServiceClient defines the gRPC semantic cache client.
type SemanticCacheServiceClient interface {
	Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetResponse, error)
	Set(ctx context.Context, in *SetRequest, opts ...grpc.CallOption) (*SetResponse, error)
	Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error)
	Stats(ctx context.Context, in *StatsRequest, opts ...grpc.CallOption) (*StatsResponse, error)
	Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error)
}

type semanticCacheServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSemanticCacheServiceClient(cc grpc.ClientConnInterface) SemanticCacheServiceClient {
	return &semanticCacheServiceClient{cc: cc}
}

func (c *semanticCacheServiceClient) Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetResponse, error) {
	out := new(GetResponse)
	err := c.cc.Invoke(ctx, "/ember.v1.SemanticCacheService/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *semanticCacheServiceClient) Set(ctx context.Context, in *SetRequest, opts ...grpc.CallOption) (*SetResponse, error) {
	out := new(SetResponse)
	err := c.cc.Invoke(ctx, "/ember.v1.SemanticCacheService/Set", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *semanticCacheServiceClient) Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error) {
	out := new(DeleteResponse)
	err := c.cc.Invoke(ctx, "/ember.v1.SemanticCacheService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *semanticCacheServiceClient) Stats(ctx context.Context, in *StatsRequest, opts ...grpc.CallOption) (*StatsResponse, error) {
	out := new(StatsResponse)
	err := c.cc.Invoke(ctx, "/ember.v1.SemanticCacheService/Stats", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *semanticCacheServiceClient) Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error) {
	out := new(HealthResponse)
	err := c.cc.Invoke(ctx, "/ember.v1.SemanticCacheService/Health", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func RegisterSemanticCacheServiceServer(registrar grpc.ServiceRegistrar, srv SemanticCacheServiceServer) {
	registrar.RegisterService(&semanticCacheServiceDesc, srv)
}

type semanticCacheService struct {
	cache *cache.SemanticCache
}

func (s *semanticCacheService) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	if req == nil || !hasText(req.Prompt) {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}

	result, hit := s.cache.Get(ctx, req.Prompt, req.SimilarityThreshold)
	resp := &GetResponse{Hit: hit}
	if hit && result != nil {
		resp.Response = result.Response
		resp.Similarity = result.Similarity
		resp.ExactMatch = result.ExactMatch
	}

	return resp, nil
}

func (s *semanticCacheService) Set(ctx context.Context, req *SetRequest) (*SetResponse, error) {
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

	return &SetResponse{Id: id}, nil
}

func (s *semanticCacheService) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	if req == nil || !hasText(req.Prompt) {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}

	return &DeleteResponse{Deleted: s.cache.Delete(req.Prompt)}, nil
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

type GetRequest struct {
	Prompt              string  `protobuf:"bytes,1,opt,name=prompt,proto3" json:"prompt,omitempty"`
	SimilarityThreshold float32 `protobuf:"fixed32,2,opt,name=similarity_threshold,json=similarityThreshold,proto3" json:"similarity_threshold,omitempty"`
}

func (m *GetRequest) Reset()         { *m = GetRequest{} }
func (m *GetRequest) String() string { return oldproto.CompactTextString(m) }
func (*GetRequest) ProtoMessage()    {}

type GetResponse struct {
	Hit        bool    `protobuf:"varint,1,opt,name=hit,proto3" json:"hit,omitempty"`
	Response   string  `protobuf:"bytes,2,opt,name=response,proto3" json:"response,omitempty"`
	Similarity float32 `protobuf:"fixed32,3,opt,name=similarity,proto3" json:"similarity,omitempty"`
	ExactMatch bool    `protobuf:"varint,4,opt,name=exact_match,json=exactMatch,proto3" json:"exact_match,omitempty"`
}

func (m *GetResponse) Reset()         { *m = GetResponse{} }
func (m *GetResponse) String() string { return oldproto.CompactTextString(m) }
func (*GetResponse) ProtoMessage()    {}

type SetRequest struct {
	Prompt     string `protobuf:"bytes,1,opt,name=prompt,proto3" json:"prompt,omitempty"`
	Response   string `protobuf:"bytes,2,opt,name=response,proto3" json:"response,omitempty"`
	TtlSeconds int64  `protobuf:"varint,3,opt,name=ttl_seconds,json=ttlSeconds,proto3" json:"ttl_seconds,omitempty"`
}

func (m *SetRequest) Reset()         { *m = SetRequest{} }
func (m *SetRequest) String() string { return oldproto.CompactTextString(m) }
func (*SetRequest) ProtoMessage()    {}

type SetResponse struct {
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (m *SetResponse) Reset()         { *m = SetResponse{} }
func (m *SetResponse) String() string { return oldproto.CompactTextString(m) }
func (*SetResponse) ProtoMessage()    {}

type DeleteRequest struct {
	Prompt string `protobuf:"bytes,1,opt,name=prompt,proto3" json:"prompt,omitempty"`
}

func (m *DeleteRequest) Reset()         { *m = DeleteRequest{} }
func (m *DeleteRequest) String() string { return oldproto.CompactTextString(m) }
func (*DeleteRequest) ProtoMessage()    {}

type DeleteResponse struct {
	Deleted bool `protobuf:"varint,1,opt,name=deleted,proto3" json:"deleted,omitempty"`
}

func (m *DeleteResponse) Reset()         { *m = DeleteResponse{} }
func (m *DeleteResponse) String() string { return oldproto.CompactTextString(m) }
func (*DeleteResponse) ProtoMessage()    {}

type StatsRequest struct{}

func (m *StatsRequest) Reset()         { *m = StatsRequest{} }
func (m *StatsRequest) String() string { return oldproto.CompactTextString(m) }
func (*StatsRequest) ProtoMessage()    {}

type StatsResponse struct {
	TotalEntries int64   `protobuf:"varint,1,opt,name=total_entries,json=totalEntries,proto3" json:"total_entries,omitempty"`
	CacheHits    int64   `protobuf:"varint,2,opt,name=cache_hits,json=cacheHits,proto3" json:"cache_hits,omitempty"`
	CacheMisses  int64   `protobuf:"varint,3,opt,name=cache_misses,json=cacheMisses,proto3" json:"cache_misses,omitempty"`
	TotalQueries int64   `protobuf:"varint,4,opt,name=total_queries,json=totalQueries,proto3" json:"total_queries,omitempty"`
	HitRate      float64 `protobuf:"fixed64,5,opt,name=hit_rate,json=hitRate,proto3" json:"hit_rate,omitempty"`
}

func (m *StatsResponse) Reset()         { *m = StatsResponse{} }
func (m *StatsResponse) String() string { return oldproto.CompactTextString(m) }
func (*StatsResponse) ProtoMessage()    {}

type HealthRequest struct{}

func (m *HealthRequest) Reset()         { *m = HealthRequest{} }
func (m *HealthRequest) String() string { return oldproto.CompactTextString(m) }
func (*HealthRequest) ProtoMessage()    {}

type HealthResponse struct {
	Status string `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
}

func (m *HealthResponse) Reset()         { *m = HealthResponse{} }
func (m *HealthResponse) String() string { return oldproto.CompactTextString(m) }
func (*HealthResponse) ProtoMessage()    {}

func _SemanticCacheService_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SemanticCacheServiceServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ember.v1.SemanticCacheService/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SemanticCacheServiceServer).Get(ctx, req.(*GetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SemanticCacheService_Set_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SemanticCacheServiceServer).Set(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ember.v1.SemanticCacheService/Set",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SemanticCacheServiceServer).Set(ctx, req.(*SetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SemanticCacheService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SemanticCacheServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ember.v1.SemanticCacheService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SemanticCacheServiceServer).Delete(ctx, req.(*DeleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SemanticCacheService_Stats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SemanticCacheServiceServer).Stats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ember.v1.SemanticCacheService/Stats",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SemanticCacheServiceServer).Stats(ctx, req.(*StatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SemanticCacheService_Health_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HealthRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SemanticCacheServiceServer).Health(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ember.v1.SemanticCacheService/Health",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SemanticCacheServiceServer).Health(ctx, req.(*HealthRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var semanticCacheServiceDesc = grpc.ServiceDesc{
	ServiceName: "ember.v1.SemanticCacheService",
	HandlerType: (*SemanticCacheServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Get", Handler: _SemanticCacheService_Get_Handler},
		{MethodName: "Set", Handler: _SemanticCacheService_Set_Handler},
		{MethodName: "Delete", Handler: _SemanticCacheService_Delete_Handler},
		{MethodName: "Stats", Handler: _SemanticCacheService_Stats_Handler},
		{MethodName: "Health", Handler: _SemanticCacheService_Health_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "ember.v1.SemanticCacheService",
}
