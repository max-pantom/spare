#!/bin/sh
set -eu

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SMOKE_ROOT=$(mktemp -d)
SMOKE_STATE="$SMOKE_ROOT/state"
SMOKE_SITE="$SMOKE_ROOT/site"

cleanup() {
  if [ -f "$SMOKE_STATE/endpoint.json" ]; then
    endpoint_pid=$(sed -n 's/.*"pid":\([0-9][0-9]*\).*/\1/p' "$SMOKE_STATE/endpoint.json")
    if [ -n "$endpoint_pid" ]; then
      kill "$endpoint_pid" 2>/dev/null || true
    fi
  fi
  rm -rf "$SMOKE_ROOT"
}
trap cleanup EXIT INT TERM

mkdir -p "$SMOKE_SITE"
printf '<!doctype html><title>Spare smoke test</title><h1>Spare works</h1>\n' > "$SMOKE_SITE/index.html"

export SPARE_HOME="$SMOKE_STATE"
export SPARED_PATH="$PROJECT_ROOT/bin/spared"
export SPARE_NO_SERVICE=1

"$PROJECT_ROOT/bin/spare" init >/dev/null
"$PROJECT_ROOT/bin/spare" install site --path "$SMOKE_SITE" --port 7398 >/dev/null

attempt=0
until curl -fsS "http://127.0.0.1:7398/" | grep -q "Spare works" &&
  "$PROJECT_ROOT/bin/spare" status --json | grep -q '"status": "healthy"'
do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "Site did not become healthy" >&2
    "$PROJECT_ROOT/bin/spare" doctor >&2 || true
    exit 1
  fi
  sleep 0.2
done
"$PROJECT_ROOT/bin/spare" stop site >/dev/null
"$PROJECT_ROOT/bin/spare" start site >/dev/null
"$PROJECT_ROOT/bin/spare" remove site --yes >/dev/null
test -f "$SMOKE_SITE/index.html"
"$PROJECT_ROOT/bin/spare" uninstall --yes >/dev/null
test -f "$SMOKE_SITE/index.html"

echo "Spare smoke test passed"
