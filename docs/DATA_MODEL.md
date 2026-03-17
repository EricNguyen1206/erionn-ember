# Data Model & Data Structures

Ember is a completely in-memory (RAM) Key-Value Store. Every value is categorized into specific structural types, and you cannot use commands intended for the wrong data type (e.g., calling `HGet` on a `String` key will result in an error).

Each entry in the system consists of: `Key`, `Type`, `Value`, `ExpiresAt`, `CreatedAt`, and `UpdatedAt`.

The currently supported structures include:

## 1. Strings
The most basic and commonly used data type for storing raw bytes or simple text (Tokens, Sessions, HTML Caching).

- **`Set(Key, Value, TTL)`**: Assigns a new string (Time-to-Live can be set).
- **`Get(Key)`**: Retrieves a string value, returning `found=true/false` along with the data.

```go
// Store Data
// proto/ember/v1/cache.proto: SetRequest
client.Set(ctx, &pb.SetRequest{
	Key:        "app:config:theme",
	Value:      "dark_mode",
	TtlSeconds: nil, // Never expires
})
```

## 2. Hashes (Maps / Objects)
Uses `map[string]string` to store objects, such as a user profile. This is highly convenient to avoid parsing JSON on the Client side.

- **`HSet(Key, Fields(Map))`**: Overwrites or appends fields to a local Hash map.
- **`HGet(Key, Field)`**: Reads the value of a single field within the Hash object.
- **`HGetAll(Key)`**: Retrieves all keys and values from that Hash object.
- **`HDel(Key, Fields(Array))`**: Removes one or more specific fields from the Hash object.

```go
// Update 'age' and 'is_active' for user 123
client.HSet(ctx, &pb.HSetRequest{
	Key: "user:123",
	Fields: map[string]string{
		"age":       "25",
		"is_active": "true",
	},
})
```

## 3. Lists (Sequential Arrays)
An ordered list of `[]string`. Typically used for storing Logs or access histories.

- **`LPush/RPush(Key, Values(Array))`**: Inserts multiple items into the head (L) or tail (R) of the list.
- **`LPop/RPop(Key)`**: Removes and returns an item from the head / tail of the list.
- **`LRange(Key, Start, Stop)`**: Allows fetching sub-sections of the array, supporting pagination (Supports negative indexing).

```go
// Add 2 items to the end of the cart
client.RPush(ctx, &pb.RPushRequest{
	Key:    "cart:999",
	Values: []string{"item_A", "item_B"},
})
```

## 4. Sets (Unique Collections)
Stored using `map[string]struct{}` within the Go engine, allowing for ultra-fast existence queries (O(1)). Useful for Tags, IP Bans, or unique views.

- **`SAdd(Key, Members(Array))`**: Adds a list of members (skips if they already exist).
- **`SRem(Key, Members(Array))`**: Removes specified members from the set.
- **`SMembers(Key)`**: Retrieves all members as an Array.
- **`SIsMember(Key, Member)`**: A utility function for the system to quickly answer Yes/No (`is_member=bool`).

```go
client.SAdd(ctx, &pb.SAddRequest{
	Key:     "article:101:likes",
	Members: []string{"user_A", "user_B", "user_C"},
})

// Quick check to see if user_D has liked it
res, _ := client.SIsMember(ctx, &pb.SIsMemberRequest{
	Key:    "article:101:likes",
	Member: "user_D",
})
// res.IsMember == false
```

## Lifecycle & Eviction (TTL)
Ember uses a Lazy Eviction mechanism: Keys are not immediately deleted by a cron-worker at the exact second they expire. Instead, the server checks `ExpiresAt` in two scenarios:
- When someone "Touches" / Reads / Updates an already expired Key. The server silently deletes it instead of returning a read.
- You can modify the TTL (extend) of a living Key using the `Expire` command (this resets the lifespan). Use the `Ttl` command to check the remaining lifespan.

Global Delete Command: `Del(Key)` will remove Data from Memory immediately. Use with caution.
