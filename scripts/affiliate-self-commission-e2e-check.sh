#!/usr/bin/env bash
set -euo pipefail
set +x

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly PROJECT_NAME="${AFFILIATE_STAGING_PROJECT_NAME:-laoshirenai-affiliate-v2-staging}"
readonly APP_CONTAINER="${PROJECT_NAME}-app-1"
readonly CONTEXT_FILE="$(mktemp "${TMPDIR:-/tmp}/affiliate-self-e2e-context.XXXXXX")"
readonly HELPER_DIR="$ROOT/backend/cmd/affiliate-self-e2e-helper-$$"
readonly HELPER_BIN="${TMPDIR:-/tmp}/affiliate-self-e2e-helper-$$"

cleanup() {
  rm -rf "$HELPER_DIR" "$HELPER_BIN" "$CONTEXT_FILE"
  docker exec "$APP_CONTAINER" rm -f /tmp/affiliate-self-e2e-helper >/dev/null 2>&1 || true
}
trap cleanup EXIT

[[ "${AFFILIATE_STAGING_URL:-}" == http://127.0.0.1:* ]] || {
  echo "self commission acceptance is restricted to isolated localhost Stage" >&2
  exit 1
}
docker inspect "$APP_CONTAINER" >/dev/null
mkdir -p "$HELPER_DIR"
chmod 700 "$HELPER_DIR"
export AFFILIATE_SELF_E2E_CONTEXT="$CONTEXT_FILE"

"$ROOT/scripts/affiliate-self-commission-e2e-check.py" prepare

cat >"$HELPER_DIR/main.go" <<'GO'
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/repository"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func mustEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic("missing required environment: " + name)
	}
	return value
}

func mustInt64(name string) int64 {
	value, err := strconv.ParseInt(mustEnv(name), 10, 64)
	if err != nil || value <= 0 {
		panic("invalid positive integer environment: " + name)
	}
	return value
}

func main() {
	if mustEnv("DATABASE_DBNAME") != "affiliate_staging" {
		panic("refusing to run outside isolated affiliate_staging database")
	}
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		mustEnv("DATABASE_HOST"),
		mustEnv("DATABASE_PORT"),
		mustEnv("DATABASE_USER"),
		mustEnv("DATABASE_PASSWORD"),
		mustEnv("DATABASE_DBNAME"),
		mustEnv("DATABASE_SSLMODE"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	userID := mustInt64("AFFILIATE_SELF_E2E_USER_ID")
	apiKeyID := mustInt64("AFFILIATE_SELF_E2E_API_KEY_ID")
	expectedFinalBalanceMicros, err := strconv.ParseInt(
		mustEnv("AFFILIATE_SELF_E2E_BALANCE_BEFORE_MICROS"),
		10,
		64,
	)
	if err != nil || expectedFinalBalanceMicros < 0 {
		panic("invalid non-negative AFFILIATE_SELF_E2E_BALANCE_BEFORE_MICROS")
	}
	usageLogID := time.Now().UnixMicro()
	result, err := repository.NewUsageBillingRepository(nil, db).Apply(
		ctx,
		&service.UsageBillingCommand{
			RequestID:  "stage-self-e2e:" + uuid.NewString(),
			APIKeyID:   apiKeyID,
			UsageLogID: usageLogID,
			UserID:     userID,
			BalanceCost: 3,
		},
	)
	if err != nil {
		panic(err)
	}
	if !result.Applied ||
		result.AffiliateCustomerRebateMicros != 0 ||
		result.AffiliateAgentCommissionMicros != 300_000 {
		panic(fmt.Sprintf("unexpected billing result: %+v", result))
	}

	var (
		balanceMicros int64
		lotPolicy     string
		eventPolicy   string
		cashPolicy    string
		rewardCount   int
		cashCount     int
	)
	err = db.QueryRowContext(ctx, `
		SELECT
			ROUND(u.balance * 1000000)::bigint,
			MAX(bl.affiliate_policy),
			MAX(pe.affiliate_policy),
			MAX(ce.metadata ->> 'attribution_policy'),
			COUNT(DISTINCT re.id),
			COUNT(DISTINCT ce.id)
		FROM users u
		JOIN balance_lots bl
		  ON bl.user_id = u.id
		 AND bl.affiliate_policy = 'PARTNER_SELF_USAGE'
		JOIN affiliate_performance_events pe
		  ON pe.user_id = u.id
		 AND pe.source_type = 'balance_usage'
		 AND pe.source_id = $2
		LEFT JOIN affiliate_reward_entries re
		  ON re.consumer_user_id = u.id
		 AND re.source_type = 'confirmed_consumption'
		 AND re.source_id = pe.id
		LEFT JOIN agent_cash_commission_entries ce
		  ON ce.consumer_user_id = u.id
		 AND ce.source_type = 'confirmed_consumption'
		 AND ce.source_id = pe.id
		WHERE u.id = $1
		GROUP BY u.id
	`, userID, usageLogID).Scan(
		&balanceMicros,
		&lotPolicy,
		&eventPolicy,
		&cashPolicy,
		&rewardCount,
		&cashCount,
	)
	if err != nil {
		panic(err)
	}
	if balanceMicros != expectedFinalBalanceMicros ||
		lotPolicy != service.AffiliateSourcePolicyPartnerSelfUsage ||
		eventPolicy != service.AffiliateSourcePolicyPartnerSelfUsage ||
		cashPolicy != service.AffiliateSourcePolicyPartnerSelfUsage ||
		rewardCount != 0 ||
		cashCount != 1 {
		panic(fmt.Sprintf(
			"unexpected persisted settlement balance=%d lot=%s event=%s cash=%s rewards=%d cash_entries=%d",
			balanceMicros, lotPolicy, eventPolicy, cashPolicy, rewardCount, cashCount,
		))
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"applied": true,
		"consumed_micros": 3_000_000,
		"cash_commission_micros": result.AffiliateAgentCommissionMicros,
		"customer_rebate_micros": result.AffiliateCustomerRebateMicros,
	})
}
GO

(
  cd "$ROOT/backend"
  case "$(docker exec "$APP_CONTAINER" uname -m)" in
    aarch64|arm64) helper_arch=arm64 ;;
    x86_64|amd64) helper_arch=amd64 ;;
    *)
      echo "unsupported Stage container architecture" >&2
      exit 1
      ;;
  esac
  CGO_ENABLED=0 GOOS=linux GOARCH="$helper_arch" \
    go build -o "$HELPER_BIN" "./cmd/$(basename "$HELPER_DIR")"
)
docker cp "$HELPER_BIN" "$APP_CONTAINER:/tmp/affiliate-self-e2e-helper"

read -r user_id api_key_id balance_before_micros < <(
  python3 - "$CONTEXT_FILE" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as f:
    value = json.load(f)
print(value["user_id"], value["api_key_id"], value["balance_before_micros"])
PY
)

docker exec \
  -e AFFILIATE_SELF_E2E_USER_ID="$user_id" \
  -e AFFILIATE_SELF_E2E_API_KEY_ID="$api_key_id" \
  -e AFFILIATE_SELF_E2E_BALANCE_BEFORE_MICROS="$balance_before_micros" \
  "$APP_CONTAINER" /tmp/affiliate-self-e2e-helper

"$ROOT/scripts/affiliate-self-commission-e2e-check.py" verify
