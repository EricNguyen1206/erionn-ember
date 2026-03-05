#!/bin/bash
# Erion Ember Docker Integration Test
# Run from project root: bash scripts/test-docker.sh

set -e

BASE_URL="http://localhost:8080"
PASS=0
FAIL=0

check() {
  local name="$1" expected="$2" got="$3"
  if echo "$got" | grep -q "$expected"; then
    echo "  ✅ PASS: $name"
    ((PASS++))
  else
    echo "  ❌ FAIL: $name"
    echo "     expected: $expected"
    echo "     got:      $got"
    ((FAIL++))
  fi
}

echo "⏳ Waiting for server..."
for i in $(seq 1 10); do
  if curl -sf "$BASE_URL/health" > /dev/null 2>&1; then
    echo "✅ Server ready"
    break
  fi
  sleep 1
done

echo ""
echo "── Health ──────────────────────────────────"
RESP=$(curl -sf "$BASE_URL/health")
check "health returns ok" '"ok"' "$RESP"

echo ""
echo "── Cache Set ───────────────────────────────"
RESP=$(curl -sf -XPOST "$BASE_URL/v1/cache/set" \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"What is Go?","response":"Go is a compiled, statically typed language."}')
check "set returns id" '"id"' "$RESP"

echo ""
echo "── Cache Get (exact hit) ───────────────────"
RESP=$(curl -sf -XPOST "$BASE_URL/v1/cache/get" \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"What is Go?"}')
check "hit=true"        '"hit":true'        "$RESP"
check "exact_match=true" '"exact_match":true' "$RESP"
check "response text"  'compiled'           "$RESP"

echo ""
echo "── Cache Get (miss) ────────────────────────"
RESP=$(curl -sf -XPOST "$BASE_URL/v1/cache/get" \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"something completely different xyz999"}')
check "hit=false" '"hit":false' "$RESP"

echo ""
echo "── Stats ───────────────────────────────────"
RESP=$(curl -sf "$BASE_URL/v1/stats")
check "stats has cache_hits"    '"cache_hits"'    "$RESP"
check "stats has total_queries" '"total_queries"' "$RESP"

echo ""
echo "── Cache Delete ────────────────────────────"
RESP=$(curl -sf -XPOST "$BASE_URL/v1/cache/delete" \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"What is Go?"}')
check "deleted=true" '"deleted":true' "$RESP"

RESP=$(curl -sf -XPOST "$BASE_URL/v1/cache/get" \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"What is Go?"}')
check "after delete: hit=false" '"hit":false' "$RESP"

echo ""
echo "═══════════════════════════════════════════"
echo "  Results: ${PASS} passed, ${FAIL} failed"
echo "═══════════════════════════════════════════"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
