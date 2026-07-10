#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODE="auto"
DOCKER_CONTEXT_NAME="${MAINTENANCE_DOCKER_CONTEXT:-default}"

usage() {
  cat <<'EOF'
Usage: ./deploy/maintenance/test.sh [--static|--docker]

  --static  Run deterministic source/configuration checks only.
  --docker  Require a local Docker daemon and run the HTTP contract tests.

Without an option, static checks always run. Docker checks run only when the
selected Docker context uses a local unix/npipe endpoint and its daemon works.
Remote tcp/ssh Docker contexts are refused.
EOF
}

case "${1:-}" in
  "") ;;
  --static) MODE="static" ;;
  --docker) MODE="docker" ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local literal="$2"
  grep -Fq -- "$literal" "$file" || fail "$file is missing required contract: $literal"
}

run_static_checks() {
  local dockerfile="$ROOT_DIR/Dockerfile"
  local config="$ROOT_DIR/nginx.conf"
  local page="$ROOT_DIR/index.html"

  for file in "$dockerfile" "$config" "$page"; do
    [[ -s "$file" ]] || fail "required artifact is missing or empty: $file"
  done

  grep -Eq '^FROM nginx:[^@[:space:]]+@sha256:[0-9a-f]{64}$' "$dockerfile" \
    || fail "Dockerfile base image must be pinned by registry digest"
  assert_contains "$dockerfile" "USER nginx"
  assert_contains "$dockerfile" "http://127.0.0.1:8080/__maintenance_health"
  assert_contains "$config" "listen 8080 default_server;"
  assert_contains "$config" "access_log off;"
  assert_contains "$config" "location = /__maintenance_health"
  assert_contains "$config" "(?:api|v1|v1beta|antigravity|gpt-image|responses|chat|images|setup)"
  assert_contains "$config" "location ~ ^/(?:health|livez|readyz)$"
  assert_contains "$config" "default_type application/json;"
  assert_contains "$config" "return 503"
  assert_contains "$config" 'Retry-After "300" always;'
  assert_contains "$config" 'Cache-Control "no-store, no-cache, must-revalidate" always;'
  assert_contains "$config" '"code":"maintenance"'
  assert_contains "$page" "北京时间（UTC+8）"
  assert_contains "$page" "2026 年 7 月 11 日 01:00–05:00"
  assert_contains "$page" '<meta name="robots" content="noindex,nofollow,noarchive" />'

  if grep -Eqi '(BEGIN[[:space:]]+(RSA |EC |OPENSSH )?PRIVATE KEY|password[[:space:]]*[:=]|api[_-]?key[[:space:]]*[:=]|token[[:space:]]*[:=]|https?://(10\.|127\.|169\.254\.|172\.(1[6-9]|2[0-9]|3[01])\.|192\.168\.))' "$page"; then
    fail "maintenance page contains a credential-like value or internal address"
  fi

  if grep -Eqi '<script([[:space:]>])' "$page"; then
    fail "maintenance page must remain static and script-free"
  fi

  python3 "$ROOT_DIR/test_route_switch.py"

  printf 'PASS: static maintenance artifact contract\n'
}

local_docker_endpoint() {
  command -v docker >/dev/null 2>&1 || return 1

  local endpoint
  endpoint="$(docker context inspect "$DOCKER_CONTEXT_NAME" --format '{{ (index .Endpoints "docker").Host }}' 2>/dev/null)" || return 1
  case "$endpoint" in
    unix://*|npipe://*) printf '%s\n' "$endpoint" ;;
    *)
      printf 'REFUSED: Docker context %s is not local (%s).\n' "$DOCKER_CONTEXT_NAME" "$endpoint" >&2
      return 2
      ;;
  esac
}

header_value() {
  local file="$1"
  local name="$2"
  awk -v wanted="$name" '
    BEGIN { FS=":" }
    {
      key=tolower($1)
      if (key == tolower(wanted)) {
        sub(/^[^:]*:[[:space:]]*/, "")
        sub(/\r$/, "")
        value=$0
      }
    }
    END { print value }
  ' "$file"
}

assert_no_store_header() {
  local path="$1"
  local value="$2"
  printf '%s' "$value" | grep -Eqi '(^|[[:space:],])no-store([[:space:],]|$)' \
    || fail "$path response is cacheable: $value"
}

assert_no_sensitive_response_data() {
  local file="$1"
  if grep -Eqi '(BEGIN[[:space:]]+(RSA |EC |OPENSSH )?PRIVATE KEY|password[[:space:]]*[:=]|api[_-]?key[[:space:]]*[:=]|token[[:space:]]*[:=]|https?://(10\.|127\.|169\.254\.|172\.(1[6-9]|2[0-9]|3[01])\.|192\.168\.))' "$file"; then
    fail "response contains a credential-like value or internal address"
  fi
}

run_http_contract() {
  local port="$1"
  local tmpdir="$2"
  local headers body status content_type retry_after cache_control

  for host in laoshirenai.com www.laoshirenai.com; do
    headers="$tmpdir/web-${host}.headers"
    body="$tmpdir/web-${host}.body"
    status="$(curl --silent --show-error --max-time 5 \
      -H "Host: $host" -D "$headers" -o "$body" -w '%{http_code}' \
      "http://127.0.0.1:${port}/dashboard?ignored=value")"
    [[ "$status" == "503" ]] || fail "$host browser route returned HTTP $status, expected 503"
    grep -Fq '<h1>系统正在维护中</h1>' "$body" || fail "$host did not return the maintenance HTML"
    grep -Fq '北京时间（UTC+8）' "$body" || fail "$host HTML omitted the Beijing timezone"
    content_type="$(header_value "$headers" Content-Type)"
    retry_after="$(header_value "$headers" Retry-After)"
    cache_control="$(header_value "$headers" Cache-Control)"
    [[ "$content_type" == text/html* ]] || fail "$host browser content type is not HTML: $content_type"
    [[ "$retry_after" == "300" ]] || fail "$host browser Retry-After is not 300: $retry_after"
    assert_no_store_header "$host browser" "$cache_control"
    assert_no_sensitive_response_data "$body"
  done

  local paths=(
    /api
    /api/v1/auth/login
    /v1/models
    /v1beta/models
    /antigravity/v1/messages
    /gpt-image/v1/images/generations
    /responses
    /chat/completions
    /images/generations
    /setup/status
    /health
    /livez
    /readyz
  )

  local path safe_path
  for path in "${paths[@]}"; do
    safe_path="${path//\//_}"
    headers="$tmpdir/api-${safe_path}.headers"
    body="$tmpdir/api-${safe_path}.body"
    status="$(curl --silent --show-error --max-time 5 --request POST \
      -H 'Host: api.laoshirenai.com' -H 'Content-Type: application/json' \
      --data '{}' -D "$headers" -o "$body" -w '%{http_code}' \
      "http://127.0.0.1:${port}${path}")"
    [[ "$status" == "503" ]] || fail "$path returned HTTP $status, expected 503"

    content_type="$(header_value "$headers" Content-Type)"
    retry_after="$(header_value "$headers" Retry-After)"
    cache_control="$(header_value "$headers" Cache-Control)"
    [[ "$content_type" == application/json* ]] || fail "$path content type is not JSON: $content_type"
    [[ "$retry_after" == "300" ]] || fail "$path Retry-After is not 300: $retry_after"
    assert_no_store_header "$path" "$cache_control"

    python3 - "$body" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
error = payload.get("error", {})
assert error.get("code") == "maintenance", payload
assert error.get("type") == "service_unavailable", payload
assert error.get("retry_after_seconds") == 300, payload
PY
    assert_no_sensitive_response_data "$body"
  done

  status="$(curl --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' \
    "http://127.0.0.1:${port}/__maintenance_health")"
  [[ "$status" == "204" ]] || fail "container-local health returned HTTP $status, expected 204"
}

run_docker_checks() {
  command -v curl >/dev/null 2>&1 || fail "curl is required for Docker HTTP checks"
  command -v python3 >/dev/null 2>&1 || fail "python3 is required for JSON validation"

  local endpoint
  if endpoint="$(local_docker_endpoint)"; then
    :
  else
    local result=$?
    if [[ "$MODE" == "docker" || "$result" -eq 2 ]]; then
      fail "a working local-only Docker context is required for --docker"
    fi
    printf 'SKIP: local Docker endpoint unavailable; runtime HTTP contract is not proven.\n' >&2
    return 0
  fi

  if ! docker --context "$DOCKER_CONTEXT_NAME" info >/dev/null 2>&1; then
    if [[ "$MODE" == "docker" ]]; then
      fail "local Docker daemon is unavailable for context $DOCKER_CONTEXT_NAME"
    fi
    printf 'SKIP: local Docker daemon unavailable; runtime HTTP contract is not proven.\n' >&2
    return 0
  fi

  local suffix image container tmpdir port status
  suffix="$$-$(date +%s)"
  image="laoshirenai-maintenance-contract:${suffix}"
  container="laoshirenai-maintenance-contract-${suffix}"
  tmpdir="$(mktemp -d)"
  mkdir -p "$tmpdir/docker-config"

  # The image base is public. Use an empty client config so this test neither
  # reads a credential helper nor forwards registry credentials to the daemon.
  local docker_cmd=(docker --config "$tmpdir/docker-config" --host "$endpoint")

  cleanup_docker_check() {
    "${docker_cmd[@]}" rm -f "$container" >/dev/null 2>&1 || true
    "${docker_cmd[@]}" image rm -f "$image" >/dev/null 2>&1 || true
    rm -rf "$tmpdir"
  }
  trap cleanup_docker_check EXIT INT TERM

  printf 'INFO: using local Docker context %s (%s)\n' "$DOCKER_CONTEXT_NAME" "$endpoint"
  "${docker_cmd[@]}" build --tag "$image" "$ROOT_DIR"
  "${docker_cmd[@]}" run --detach --name "$container" \
    --publish 127.0.0.1::8080 "$image" >/dev/null

  port="$("${docker_cmd[@]}" port "$container" 8080/tcp | awk -F: 'END { print $NF }')"
  [[ "$port" =~ ^[0-9]+$ ]] || fail "could not determine the local published port"

  status=""
  for _ in $(seq 1 30); do
    status="$("${docker_cmd[@]}" inspect \
      --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' "$container")"
    [[ "$status" == "healthy" ]] && break
    [[ "$status" == "unhealthy" ]] && break
    sleep 1
  done
  [[ "$status" == "healthy" ]] || fail "maintenance container health is $status"

  run_http_contract "$port" "$tmpdir"
  printf 'PASS: Docker maintenance HTTP contract\n'

  cleanup_docker_check
  trap - EXIT INT TERM
}

run_static_checks
if [[ "$MODE" != "static" ]]; then
  run_docker_checks
else
  printf 'SKIP: Docker runtime checks disabled by --static; runtime HTTP contract is not proven.\n'
fi
