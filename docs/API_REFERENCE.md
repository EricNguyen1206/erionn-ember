# API Reference

## gRPC

Proto source: `proto/ember/v1/cache.proto`

Service: `ember.v1.CacheService`

### Generic Commands

- `Del(DelRequest) -> DelResponse`
  - Request: `key`
  - Response: `deleted`

- `Exists(ExistsRequest) -> ExistsResponse`
  - Request: `key`
  - Response: `exists`

- `Type(TypeRequest) -> TypeResponse`
  - Request: `key`
  - Response: `type` in `none|string|hash|list|set`

- `Expire(ExpireRequest) -> ExpireResponse`
  - Request: `key`, `ttl_seconds`
  - Response: `updated`

- `Ttl(TtlRequest) -> TtlResponse`
  - Request: `key`
  - Response: `found`, `has_expiration`, `ttl_seconds`

- `Stats(StatsRequest) -> StatsResponse`
  - Response fields: `total_keys`, `string_keys`, `hash_keys`, `list_keys`, `set_keys`, `channels`, `subscribers`

- `Health(HealthRequest) -> HealthResponse`
  - Response: `status`

### Strings

- `Get(GetRequest) -> GetResponse`
  - Request: `key`
  - Response: `found`, `value`

- `Set(SetRequest) -> SetResponse`
  - Request: `key`, `value`, optional `ttl_seconds`

### Hashes

- `HSet(HSetRequest) -> HSetResponse`
  - Request: `key`, `fields`
  - Response: `added`

- `HGet(HGetRequest) -> HGetResponse`
  - Request: `key`, `field`
  - Response: `found`, `value`

- `HDel(HDelRequest) -> HDelResponse`
  - Request: `key`, `fields`
  - Response: `removed`

- `HGetAll(HGetAllRequest) -> HGetAllResponse`
  - Request: `key`
  - Response: `found`, `fields`

### Lists

- `LPush(LPushRequest) -> LPushResponse`
- `RPush(RPushRequest) -> RPushResponse`
  - Request: `key`, `values`
  - Response: `length`

- `LPop(LPopRequest) -> LPopResponse`
- `RPop(RPopRequest) -> RPopResponse`
  - Request: `key`
  - Response: `found`, `value`

- `LRange(LRangeRequest) -> LRangeResponse`
  - Request: `key`, `start`, `stop`
  - Response: `found`, `values`
  - Range is inclusive and supports negative indexes

### Sets

- `SAdd(SAddRequest) -> SAddResponse`
  - Request: `key`, `members`
  - Response: `added`

- `SRem(SRemRequest) -> SRemResponse`
  - Request: `key`, `members`
  - Response: `removed`

- `SMembers(SMembersRequest) -> SMembersResponse`
  - Request: `key`
  - Response: `found`, `members`

- `SIsMember(SIsMemberRequest) -> SIsMemberResponse`
  - Request: `key`, `member`
  - Response: `is_member`

### Pub/Sub

- `Publish(PublishRequest) -> PublishResponse`
  - Request: `channel`, `payload`
  - Response: `delivered`

- `Subscribe(SubscribeRequest) -> stream SubscribeMessage`
  - Request: `channels`
  - Stream message fields: `channel`, `payload`, `published_at_unix`

## Error Mapping

- Invalid request data -> `codes.InvalidArgument`
- Wrong type for key -> `codes.FailedPrecondition`
- Corrupt in-memory value -> `codes.Internal`

Missing data is usually represented in the response body rather than as an error:

- `Get`, `HGet`, `LPop`, `RPop` use `found=false`
- `SMembers` and `HGetAll` use `found=false`
- `SIsMember` returns `is_member=false`
- `Type` returns `none`

## Standard gRPC Health Check

Ember implements the standard `grpc.health.v1.Health` service natively.

You can verify the server health using standard tooling like `grpc_health_probe`:

```bash
grpc_health_probe -addr=localhost:9090
```
