#!/usr/bin/env bash
# Day 5 load-test script. Requires hey (https://github.com/rakyll/hey).
# Run against `docker compose up` stack, or a local `go run ./cmd/server`.
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
DURATION="${DURATION:-30s}"
CONCURRENCY="${CONCURRENCY:-50}"

command -v hey >/dev/null || { echo "install hey first: https://github.com/rakyll/hey"; exit 1; }

echo ">> creating a short link"
RESP=$(curl -sS -X POST "$BASE/shorten" \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}')
echo "$RESP"
CODE=$(printf '%s' "$RESP" | sed -E 's/.*"short_code":"([^"]+)".*/\1/')
[ -n "$CODE" ] || { echo "failed to obtain a short code"; exit 1; }
echo ">> benchmarking redirect (warm cache)  code=$CODE"
hey -c "$CONCURRENCY" -z "$DURATION" "$BASE/s/$CODE"

echo ">> benchmarking redirect (penetration / 404 path)"
hey -c "$CONCURRENCY" -z 10s "$BASE/s/zzzzzz"

echo ">> benchmarking POST /shorten"
echo "   note: raise RATE_LIMIT_RPS / RATE_LIMIT_BURST when measuring raw write throughput"
hey -c 10 -z 10s -m POST -T application/json \
  -d '{"url":"https://example.com"}' "$BASE/shorten" || true
