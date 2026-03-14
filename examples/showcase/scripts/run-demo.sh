#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHOWCASE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${SHOWCASE_DIR}/docker-compose.yml"
BASE_URL="http://127.0.0.1:18080"
PROMPT="How should a chat backend explain cache warming during a release demo?"
RESPONSE="Cache warming means the first request misses, the backend calls the LLM once, and the second request is served directly from Erion Ember."

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

print_json() {
  if command -v jq >/dev/null 2>&1; then
    jq .
    return
  fi

  cat
}

extract_metric() {
  local metric_name="$1"
  while IFS= read -r line; do
    if [[ "$line" == "$metric_name "* ]]; then
      printf '%s\n' "$line"
      return 0
    fi
  done

  return 1
}

post_json() {
  local path="$1"
  local payload="$2"
  curl --silent --show-error --fail \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "${BASE_URL}${path}"
}

wait_for_ready() {
  printf 'Waiting for Erion Ember on %s' "$BASE_URL"
  for _ in $(seq 1 60); do
    if curl --silent --fail "${BASE_URL}/ready" >/dev/null 2>&1; then
      printf ' ready\n'
      return 0
    fi
    printf '.'
    sleep 1
  done
  printf '\nservice did not become ready in time\n' >&2
  exit 1
}

show_stats() {
  local label="$1"
  printf '\n%s stats\n' "$label"
  curl --silent --show-error --fail "${BASE_URL}/v1/stats" | print_json
}

show_metrics() {
  local label="$1"
  local metrics
  metrics="$(curl --silent --show-error --fail "${BASE_URL}/metrics")"

  printf '\n%s metrics\n' "$label"
  printf '%s\n' "$metrics" | extract_metric "erion_ember_cache_entries"
  printf '%s\n' "$metrics" | extract_metric "erion_ember_cache_hits_total"
  printf '%s\n' "$metrics" | extract_metric "erion_ember_cache_misses_total"
  printf '%s\n' "$metrics" | extract_metric "erion_ember_cache_queries_total"
  printf '%s\n' "$metrics" | extract_metric "erion_ember_cache_hit_rate"
}

show_lookup() {
  local label="$1"
  local payload
  payload="{\"prompt\":\"${PROMPT}\",\"similarity_threshold\":0.85}"

  printf '\n%s\n' "$label"
  post_json "/v1/cache/get" "$payload" | print_json
}

require_command docker
require_command curl

printf 'Starting showcase stack from %s\n' "$COMPOSE_FILE"
docker build -t erion-ember:local "${SHOWCASE_DIR}/../.."
docker compose -f "$COMPOSE_FILE" up -d --force-recreate --remove-orphans

wait_for_ready

show_stats "Before demo"
show_metrics "Before demo"
show_lookup "First lookup: expect miss"

printf '\nStoring placeholder LLM response\n'
post_json "/v1/cache/set" "{\"prompt\":\"${PROMPT}\",\"response\":\"${RESPONSE}\",\"ttl\":3600}" | print_json

show_lookup "Second lookup: expect hit"
show_stats "After demo"
show_metrics "After demo"

printf '\nShowcase complete. The container is still running for follow-up demos.\n'
printf 'Stop it with: docker compose -f %s down\n' "$COMPOSE_FILE"
