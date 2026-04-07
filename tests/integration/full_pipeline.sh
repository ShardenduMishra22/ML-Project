#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

payload='{"latitude":12.9716,"longitude":77.5946}'
host="${API_HOST:-127.0.0.1}"

echo "Starting integration validation against running stack..."

for i in {1..30}; do
  if curl -fsS "http://${host}:${BACKEND_PORT:-8080}/health" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

response="$(curl -fsS -X POST "http://${host}:${BACKEND_PORT:-8080}/analyze" -H "Content-Type: application/json" -d "$payload")"
report_id="$(echo "$response" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -n1)"

if [[ -z "$report_id" ]]; then
  echo "Failed: report id not found in analyze response"
  echo "$response"
  exit 1
fi

echo "Report ID: $report_id"

curl -fsS "http://${host}:${BACKEND_PORT:-8080}/report/$report_id" >/dev/null
curl -fsS "http://${host}:${BACKEND_PORT:-8080}/trace/$report_id" >/dev/null
curl -fsS "http://${host}:${BACKEND_PORT:-8080}/report/$report_id?format=pdf" >/dev/null

echo "Integration pipeline check passed."
