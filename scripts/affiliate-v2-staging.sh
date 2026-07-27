#!/usr/bin/env bash
set -euo pipefail
set +x

readonly EXPECTED_WORKTREE="/Users/fujunhao/laoshirenai/worktrees/affiliate-program-v2"
readonly EXPECTED_BRANCH="feat/affiliate-program-v2-20260726"
readonly PROJECT_NAME="laoshirenai-affiliate-v2-staging"
readonly COMPOSE_FILE="$EXPECTED_WORKTREE/deploy/staging/affiliate-v2.compose.yml"
readonly DEFAULT_URL="http://127.0.0.1:${AFFILIATE_STAGING_PORT:-18080}"

readonly SECRET_POSTGRES="LAOSHIRENAI_AFFILIATE_STAGING_POSTGRES_PASSWORD"
readonly SECRET_REDIS="LAOSHIRENAI_AFFILIATE_STAGING_REDIS_PASSWORD"
readonly SECRET_ADMIN="LAOSHIRENAI_AFFILIATE_STAGING_ADMIN_PASSWORD"
readonly SECRET_JWT="LAOSHIRENAI_AFFILIATE_STAGING_JWT_SECRET"
readonly SECRET_TOTP="LAOSHIRENAI_AFFILIATE_STAGING_TOTP_ENCRYPTION_KEY"

require_checkout() {
  local current_root current_branch
  current_root="$(git -C "$EXPECTED_WORKTREE" rev-parse --show-toplevel)"
  current_branch="$(git -C "$EXPECTED_WORKTREE" branch --show-current)"
  [[ "$current_root" == "$EXPECTED_WORKTREE" ]] || {
    echo "unexpected staging worktree: $current_root" >&2
    exit 1
  }
  [[ "$current_branch" == "$EXPECTED_BRANCH" ]] || {
    echo "unexpected staging branch: $current_branch" >&2
    exit 1
  }
  make -C "$EXPECTED_WORKTREE" checkout-validate >/dev/null
}

secret_exists() {
  agent-switch secret list | grep -Fxq "$1"
}

init_secret() {
  local name="$1" generator="$2"
  if secret_exists "$name"; then
    echo "secret exists: $name"
    return
  fi
  case "$generator" in
    hex)
      openssl rand -hex 32 | agent-switch secret set --stdin "$name" >/dev/null
      ;;
    password)
      openssl rand -base64 36 | tr -d '\n' | agent-switch secret set --stdin "$name" >/dev/null
      ;;
    *)
      echo "unknown secret generator" >&2
      exit 1
      ;;
  esac
  echo "secret created: $name"
}

read_secret_into() {
  local name="$1" variable="$2" fifo value pid status
  fifo="$(mktemp -u "${TMPDIR:-/tmp}/affiliate-v2-secret.XXXXXX")"
  mkfifo "$fifo"
  chmod 600 "$fifo"
  agent-switch secret get --fd 3 "$name" 3>"$fifo" >/dev/null &
  pid=$!
  value=""
  IFS= read -r value <"$fifo" || [[ -n "$value" ]]
  wait "$pid"
  status=$?
  rm -f "$fifo"
  [[ $status -eq 0 && -n "$value" ]] || {
    echo "failed to load staging secret: $name" >&2
    exit 1
  }
  printf -v "$variable" '%s' "$value"
  export "$variable"
}

load_secrets() {
  for name in "$SECRET_POSTGRES" "$SECRET_REDIS" "$SECRET_ADMIN" "$SECRET_JWT" "$SECRET_TOTP"; do
    secret_exists "$name" || {
      echo "missing Agent Switch secret: $name" >&2
      echo "run: $0 init-secrets" >&2
      exit 1
    }
  done
  read_secret_into "$SECRET_POSTGRES" AFFILIATE_STAGING_POSTGRES_PASSWORD
  read_secret_into "$SECRET_REDIS" AFFILIATE_STAGING_REDIS_PASSWORD
  read_secret_into "$SECRET_ADMIN" AFFILIATE_STAGING_ADMIN_PASSWORD
  read_secret_into "$SECRET_JWT" AFFILIATE_STAGING_JWT_SECRET
  read_secret_into "$SECRET_TOTP" AFFILIATE_STAGING_TOTP_ENCRYPTION_KEY
  export AFFILIATE_STAGING_ADMIN_EMAIL="${AFFILIATE_STAGING_ADMIN_EMAIL:-affiliate-staging@local.invalid}"
  export AFFILIATE_STAGING_PORT="${AFFILIATE_STAGING_PORT:-18080}"
  export GIT_SHA="$(git -C "$EXPECTED_WORKTREE" rev-parse --short=12 HEAD)"
}

compose() {
  docker compose --project-name "$PROJECT_NAME" --file "$COMPOSE_FILE" "$@"
}

ensure_fresh_install_prerequisites() {
  local deadline=$((SECONDS + 90))
  local groups_table migration_138

  while true; do
    groups_table="$(
      compose exec -T postgres psql -U affiliate_staging -d affiliate_staging -Atc \
        "SELECT COALESCE(to_regclass('public.groups')::text, '')" 2>/dev/null || true
    )"
    if [[ "$groups_table" == "groups" ]]; then
      break
    fi
    if (( SECONDS >= deadline )); then
      echo "staging groups table was not created in time" >&2
      exit 1
    fi
    sleep 1
  done

  migration_138="$(
    compose exec -T postgres psql -U affiliate_staging -d affiliate_staging -Atc \
      "SELECT COUNT(*) FROM schema_migrations WHERE filename = '138_add_apex_monthly_card_groups.sql'" 2>/dev/null || true
  )"
  if [[ "$migration_138" == "0" ]]; then
    compose exec -T postgres psql -v ON_ERROR_STOP=1 -U affiliate_staging -d affiliate_staging >/dev/null <<'SQL'
INSERT INTO groups (name, description, status)
VALUES
  ('GPT Ultra 月卡组', '[affiliate-v2 staging bootstrap] Disabled prerequisite for immutable migration 138.', 'disabled'),
  ('Claude Ultra 月卡组', '[affiliate-v2 staging bootstrap] Disabled prerequisite for immutable migration 138.', 'disabled')
ON CONFLICT (name) WHERE deleted_at IS NULL DO NOTHING;
SQL
    # The first application boot intentionally characterizes the immutable
    # migration prerequisite. Restart immediately instead of waiting for
    # Docker's exponential restart delay.
    compose restart app >/dev/null
  fi
}

retire_fresh_install_prerequisites() {
  compose exec -T postgres psql -v ON_ERROR_STOP=1 -U affiliate_staging -d affiliate_staging >/dev/null <<'SQL'
UPDATE groups
SET deleted_at = COALESCE(deleted_at, NOW()), updated_at = NOW()
WHERE deleted_at IS NULL
  AND description = '[affiliate-v2 staging bootstrap] Disabled prerequisite for immutable migration 138.';
SQL
}

wait_ready() {
  local deadline=$((SECONDS + 300))
  until curl --fail --silent "$DEFAULT_URL/readyz" >/dev/null 2>&1; do
    if (( SECONDS >= deadline )); then
      compose ps
      compose logs --tail=120 app
      echo "staging readiness timed out" >&2
      exit 1
    fi
    sleep 2
  done
  curl --fail --silent --show-error "$DEFAULT_URL/health" >/dev/null
}

command="${1:-help}"
case "$command" in
  init-secrets)
    require_checkout
    init_secret "$SECRET_POSTGRES" password
    init_secret "$SECRET_REDIS" password
    init_secret "$SECRET_ADMIN" password
    init_secret "$SECRET_JWT" hex
    init_secret "$SECRET_TOTP" hex
    ;;
  validate)
    require_checkout
    load_secrets
    compose config --quiet
    echo "affiliate v2 staging config is valid"
    ;;
  up)
    require_checkout
    load_secrets
    compose up --detach --build --remove-orphans
    ensure_fresh_install_prerequisites
    wait_ready
    retire_fresh_install_prerequisites
    echo "affiliate v2 staging ready: $DEFAULT_URL"
    ;;
  status)
    require_checkout
    load_secrets
    compose ps
    ;;
  logs)
    require_checkout
    load_secrets
    compose logs --tail="${TAIL:-200}" app
    ;;
  down)
    require_checkout
    load_secrets
    compose down --remove-orphans
    ;;
  reset)
    require_checkout
    [[ "${2:-}" == "--confirm-staging-data-reset" ]] || {
      echo "usage: $0 reset --confirm-staging-data-reset" >&2
      exit 1
    }
    load_secrets
    compose down --volumes --remove-orphans
    echo "affiliate v2 staging data reset"
    ;;
  url)
    echo "$DEFAULT_URL"
    ;;
  *)
    cat <<USAGE
Usage: $0 <command>

Commands:
  init-secrets  Create missing staging-only secrets in Agent Switch
  validate      Validate the isolated Docker Compose configuration
  up            Build current worktree and start isolated staging
  status        Show isolated staging containers
  logs          Show application logs
  down          Stop staging without deleting data
  reset --confirm-staging-data-reset
                Delete staging-only containers and volumes
  url           Print the local staging URL
USAGE
    ;;
esac
