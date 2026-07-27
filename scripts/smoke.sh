#!/bin/sh
set -eu

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SMOKE_ROOT=$(mktemp -d)
SMOKE_STATE="$SMOKE_ROOT/state"
SMOKE_SITE="$SMOKE_ROOT/site"
SMOKE_DROP="$SMOKE_ROOT/drop"
SMOKE_DROP_RESTORED="$SMOKE_ROOT/drop-restored"
SMOKE_BACKUP="$SMOKE_ROOT/drop.spare-backup"
SMOKE_UPLOAD="$SMOKE_ROOT/upload.txt"

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
mkdir -p "$SMOKE_DROP"
printf '<!doctype html><title>Spare smoke test</title><h1>Spare works</h1>\n' > "$SMOKE_SITE/index.html"
printf 'Drop works\n' > "$SMOKE_UPLOAD"

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

if command -v pgrep >/dev/null 2>&1; then
  daemon_pid=$(sed -n 's/.*"pid":\([0-9][0-9]*\).*/\1/p' "$SMOKE_STATE/endpoint.json")
  worker_pid=$(pgrep -P "$daemon_pid" | head -n 1 || true)
  if [ -n "$worker_pid" ]; then
    kill -9 "$worker_pid"
    attempt=0
    until curl -fsS "http://127.0.0.1:7398/" | grep -q "Spare works" &&
      "$PROJECT_ROOT/bin/spare" status --json | grep -q '"status": "healthy"'
    do
      attempt=$((attempt + 1))
      if [ "$attempt" -ge 60 ]; then
        echo "Site did not recover after its worker stopped" >&2
        exit 1
      fi
      sleep 0.2
    done
  fi
fi

"$PROJECT_ROOT/bin/spare" stop site >/dev/null
"$PROJECT_ROOT/bin/spare" start site >/dev/null
"$PROJECT_ROOT/bin/spare" remove site --yes >/dev/null
test -f "$SMOKE_SITE/index.html"

"$PROJECT_ROOT/bin/spare" install drop --path "$SMOKE_DROP" --port 7397 --max-file-size 1MB >/dev/null
attempt=0
until curl -fsS "http://127.0.0.1:7397/" | grep -q "Drop is ready"
do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "Drop did not become healthy" >&2
    "$PROJECT_ROOT/bin/spare" doctor >&2 || true
    exit 1
  fi
  sleep 0.2
done
curl -fsS -F "file=@$SMOKE_UPLOAD;filename=hello.txt" "http://127.0.0.1:7397/api/upload" | grep -q '"name":"hello.txt"'
curl -fsS "http://127.0.0.1:7397/files/hello.txt" | grep -q "Drop works"
"$PROJECT_ROOT/bin/spare" stop drop >/dev/null
"$PROJECT_ROOT/bin/spare" start drop >/dev/null
"$PROJECT_ROOT/bin/spare" export drop --output "$SMOKE_BACKUP" >/dev/null
"$PROJECT_ROOT/bin/spare" remove drop --yes >/dev/null
test -f "$SMOKE_DROP/hello.txt"
"$PROJECT_ROOT/bin/spare" import "$SMOKE_BACKUP" --path "$SMOKE_DROP_RESTORED" --port 7397 >/dev/null
attempt=0
until curl -fsS "http://127.0.0.1:7397/files/hello.txt" | grep -q "Drop works"
do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "Restored Drop did not become healthy" >&2
    "$PROJECT_ROOT/bin/spare" doctor >&2 || true
    exit 1
  fi
  sleep 0.2
done
"$PROJECT_ROOT/bin/spare" remove drop --yes >/dev/null
test -f "$SMOKE_DROP_RESTORED/hello.txt"

"$PROJECT_ROOT/bin/spare" install hook --port 7396 >/dev/null
attempt=0
until curl -fsS "http://127.0.0.1:7396/" | grep -q "See every request"
do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "Hook did not become healthy" >&2
    "$PROJECT_ROOT/bin/spare" doctor >&2 || true
    exit 1
  fi
  sleep 0.2
done
hook_capture=$(curl -fsS -X POST -H "Content-Type: application/json" \
  -d '{"event":"smoke"}' "http://127.0.0.1:7396/hook/smoke?source=test")
hook_id=$(printf '%s' "$hook_capture" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
test -n "$hook_id"
curl -fsS "http://127.0.0.1:7396/api/requests/$hook_id" | grep -q '\\"event\\":\\"smoke\\"'
curl -fsS -X POST -H "Content-Type: application/json" \
  -d '{"targetUrl":"http://127.0.0.1:7396/hook/replayed"}' \
  "http://127.0.0.1:7396/api/requests/$hook_id/replay" | grep -q '"status":"completed"'
curl -fsS "http://127.0.0.1:7396/api/requests" | grep -q '"path":"/hook/replayed"'
"$PROJECT_ROOT/bin/spare" remove hook --yes >/dev/null

"$PROJECT_ROOT/bin/spare" uninstall --yes >/dev/null
test -f "$SMOKE_SITE/index.html"
test -f "$SMOKE_DROP/hello.txt"
test -f "$SMOKE_DROP_RESTORED/hello.txt"

echo "Spare Site, Drop, and Hook smoke test passed"
