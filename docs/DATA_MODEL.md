# Data Model & Data Structures

gomemkv is an in-memory key-value store. Every value has a specific type — using the wrong command for the wrong type returns a `WRONGTYPE` error.

Each entry contains: `Key`, `Type`, `Value`, `ExpiresAt`, `CreatedAt`, `UpdatedAt`.

---

## 1. Strings

The most basic type for raw text, tokens, sessions, or cached HTML.

```bash
SET app:theme dark_mode
GET app:theme          # "dark_mode"
INCR visit:count       # (integer) 1
SET session:abc tok123 EX 3600  # with TTL
```

---

## 2. Hashes (Maps / Objects)

Uses `map[string]string` — good for objects like user profiles, avoiding JSON parsing.

```bash
HSET user:123 name Eric age 25 active true
HGET user:123 name    # "Eric"
HGETALL user:123      # name Eric age 25 active true
HDEL user:123 active  # (integer) 1
```

---

## 3. Lists

An ordered `[]string`. Good for logs, queues, recent-activity history.

```bash
RPUSH cart:999 item_A item_B   # (integer) 2
LPUSH cart:999 item_X          # (integer) 3
LRANGE cart:999 0 -1           # item_X item_A item_B
LPOP cart:999                  # "item_X"
```

---

## 4. Sets (Unique Collections)

Backed by `map[string]struct{}` — O(1) membership checks. Good for tags, IP bans, unique views.

```bash
SADD article:101:likes user_A user_B user_C  # (integer) 3
SISMEMBER article:101:likes user_D           # (integer) 0
SMEMBERS article:101:likes                   # user_A user_B user_C
SCARD article:101:likes                      # (integer) 3
```

---

## TTL & Expiration

gomemkv uses **lazy eviction** — expired keys are deleted on access, not by a background timer.

```bash
SET key value EX 60    # expires in 60 seconds
TTL key                 # seconds remaining
EXPIRE key 120          # reset TTL to 120s
DEL key                 # immediate delete
```

```
TTL returns:
  ≥ 0  — seconds remaining
  -1   — no expiration set
  -2   — key not found
```
