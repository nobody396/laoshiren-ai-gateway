package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	openAIRouteBudgetKeyPrefix             = "route:v1:budget:"
	openAIRouteBudgetReservationKeyPrefix  = "route:v1:budget-reservation:"
	openAIRouteBudgetScriptPolicyConflict  = int64(-3)
	openAIRouteBudgetScriptRequestConflict = int64(-2)
)

var reserveOpenAIRouteBudgetScript = redis.NewScript(`
local signature = ARGV[1]
local reservation_ttl_ms = tonumber(ARGV[2])
local rate = tonumber(ARGV[3])
local estimate = tonumber(ARGV[4])
local route = ARGV[5]
local n = tonumber(ARGV[6])
local existing_state = redis.call('HGET', KEYS[1], 'state')

if existing_state ~= false then
  local existing_signature = redis.call('HGET', KEYS[1], 'signature')
  if existing_state == 'reserved' and existing_signature == signature then
    redis.call('PEXPIRE', KEYS[1], reservation_ttl_ms)
    return {1, tonumber(redis.call('HGET', KEYS[1], 'emergency')) or 0, -1}
  end
  return {-2, 0, -1}
end

local offset = 7
local emergency = 0
for i = 1, n do
  local target = tonumber(ARGV[offset])
  local hard = tonumber(ARGV[offset + 1])
  local debt = tonumber(ARGV[offset + 2])
  local max_credit = tonumber(ARGV[offset + 3])
  local ttl_ms = tonumber(ARGV[offset + 4])
  local extra = tonumber(ARGV[offset + 5])
  local scope = ARGV[offset + 6]
  local policy = ARGV[offset + 7]
  local budget_key = KEYS[i + 1]
  local exists = redis.call('EXISTS', budget_key)
  local current_policy = redis.call('HGET', budget_key, 'policy')

  if exists == 1 and (current_policy == false or current_policy ~= policy) then
    return {-3, 0, i - 1}
  end

  local credit = tonumber(redis.call('HGET', budget_key, 'credit')) or 0
  local equivalent = tonumber(redis.call('HGET', budget_key, 'equivalent')) or 0
  local actual = tonumber(redis.call('HGET', budget_key, 'actual')) or 0
  if equivalent > 0 then
    local average_multiplier = (actual / equivalent) * 1000000000
    if average_multiplier > hard and rate > target then
      return {0, 0, i - 1}
    end
  end

  local available = credit + debt
  if available < 0 then
    available = 0
  end
  if extra > available then
    return {0, 0, i - 1}
  end
  if extra > math.max(0, credit) then
    emergency = 1
  end

  offset = offset + 8
end

offset = 7
for i = 1, n do
  local target = tonumber(ARGV[offset])
  local hard = tonumber(ARGV[offset + 1])
  local debt = tonumber(ARGV[offset + 2])
  local max_credit = tonumber(ARGV[offset + 3])
  local ttl_ms = tonumber(ARGV[offset + 4])
  local extra = tonumber(ARGV[offset + 5])
  local scope = ARGV[offset + 6]
  local policy = ARGV[offset + 7]
  local budget_key = KEYS[i + 1]
  local credit = tonumber(redis.call('HGET', budget_key, 'credit')) or 0
  local equivalent = tonumber(redis.call('HGET', budget_key, 'equivalent')) or 0
  local actual = tonumber(redis.call('HGET', budget_key, 'actual')) or 0

  redis.call('HSET', budget_key,
    'policy', policy,
    'target', target,
    'hard', hard,
    'debt_limit', debt,
    'max_credit', max_credit,
    'credit', credit - extra,
    'equivalent', equivalent,
    'actual', actual)
  redis.call('PEXPIRE', budget_key, ttl_ms)

  redis.call('HSET', KEYS[1],
    'window:' .. i, scope,
    'policy:' .. i, policy,
    'extra:' .. i, extra)
  offset = offset + 8
end

redis.call('HSET', KEYS[1],
  'state', 'reserved',
  'signature', signature,
  'route', route,
  'rate', rate,
  'estimate', estimate,
  'windows', n,
  'emergency', emergency)
redis.call('PEXPIRE', KEYS[1], reservation_ttl_ms)
return {1, emergency, -1}
`)

var settleOpenAIRouteBudgetScript = redis.NewScript(`
local audit_ttl_ms = tonumber(ARGV[1])
local n = tonumber(ARGV[2])
local actual_base = tonumber(ARGV[3])
local actual_account = tonumber(ARGV[4])
local route = ARGV[5]
local state = redis.call('HGET', KEYS[1], 'state')
if state == false then
  return -2
end
if tonumber(redis.call('HGET', KEYS[1], 'windows')) ~= n then
  return -2
end
if redis.call('HGET', KEYS[1], 'route') ~= route then
  return -2
end

local offset = 6
for i = 1, n do
  local scope = ARGV[offset]
  local policy = ARGV[offset + 1]
  if redis.call('HGET', KEYS[1], 'window:' .. i) ~= scope then
    return -2
  end
  if redis.call('HGET', KEYS[1], 'policy:' .. i) ~= policy then
    return -2
  end
  offset = offset + 4
end

if state == 'settled' then
  if tonumber(redis.call('HGET', KEYS[1], 'actual_base')) ~= actual_base then
    return -2
  end
  if tonumber(redis.call('HGET', KEYS[1], 'actual_account')) ~= actual_account then
    return -2
  end
  redis.call('PEXPIRE', KEYS[1], audit_ttl_ms)
  return 1
end
if state ~= 'reserved' then
  return -2
end

offset = 6
for i = 1, n do
  local policy = ARGV[offset + 1]
  if redis.call('EXISTS', KEYS[i + 1]) ~= 1 then
    return -2
  end
  if redis.call('HGET', KEYS[i + 1], 'policy') ~= policy then
    return -3
  end
  offset = offset + 4
end

offset = 6
for i = 1, n do
  local scope = ARGV[offset]
  local policy = ARGV[offset + 1]
  local ttl_ms = tonumber(ARGV[offset + 2])
  local credit_delta = tonumber(ARGV[offset + 3])
  local budget_key = KEYS[i + 1]

  local credit = tonumber(redis.call('HGET', budget_key, 'credit')) or 0
  local equivalent = tonumber(redis.call('HGET', budget_key, 'equivalent')) or 0
  local actual = tonumber(redis.call('HGET', budget_key, 'actual')) or 0
  local extra = tonumber(redis.call('HGET', KEYS[1], 'extra:' .. i)) or 0
  local max_credit = tonumber(redis.call('HGET', budget_key, 'max_credit')) or 0
  local debt = tonumber(redis.call('HGET', budget_key, 'debt_limit')) or 0
  credit = credit + extra + credit_delta
  if credit > max_credit then
    credit = max_credit
  end
  if credit < -debt then
    credit = -debt
  end

  redis.call('HSET', budget_key,
    'credit', credit,
    'equivalent', equivalent + actual_base,
    'actual', actual + actual_account)
  redis.call('PEXPIRE', budget_key, ttl_ms)
  offset = offset + 4
end

redis.call('HSET', KEYS[1],
  'state', 'settled',
  'actual_base', actual_base,
  'actual_account', actual_account)
redis.call('PEXPIRE', KEYS[1], audit_ttl_ms)
return 1
`)

var cancelOpenAIRouteBudgetScript = redis.NewScript(`
local audit_ttl_ms = tonumber(ARGV[1])
local n = tonumber(ARGV[2])
local route = ARGV[3]
local state = redis.call('HGET', KEYS[1], 'state')
if state == false then
  return -2
end
if tonumber(redis.call('HGET', KEYS[1], 'windows')) ~= n then
  return -2
end
if redis.call('HGET', KEYS[1], 'route') ~= route then
  return -2
end

local offset = 4
for i = 1, n do
  local scope = ARGV[offset]
  if redis.call('HGET', KEYS[1], 'window:' .. i) ~= scope then
    return -2
  end
  offset = offset + 2
end

if state == 'cancelled' then
  redis.call('PEXPIRE', KEYS[1], audit_ttl_ms)
  return 1
end
if state ~= 'reserved' then
  return -2
end

for i = 1, n do
  if redis.call('EXISTS', KEYS[i + 1]) ~= 1 then
    return -2
  end
end

offset = 4
for i = 1, n do
  local scope = ARGV[offset]
  local ttl_ms = tonumber(ARGV[offset + 1])
  local budget_key = KEYS[i + 1]
  local credit = tonumber(redis.call('HGET', budget_key, 'credit')) or 0
  local extra = tonumber(redis.call('HGET', KEYS[1], 'extra:' .. i)) or 0
  local max_credit = tonumber(redis.call('HGET', budget_key, 'max_credit')) or 0
  credit = credit + extra
  if credit > max_credit then
    credit = max_credit
  end
  redis.call('HSET', budget_key, 'credit', credit)
  redis.call('PEXPIRE', budget_key, ttl_ms)
  offset = offset + 2
end

redis.call('HSET', KEYS[1], 'state', 'cancelled')
redis.call('PEXPIRE', KEYS[1], audit_ttl_ms)
return 1
`)

type openAIRouteBudgetCache struct {
	rdb *redis.Client
}

// NewOpenAIRouteBudgetCache returns the shared multi-window budget ledger.
// Expired reservations are intentionally not refunded: losing a reservation is
// conservative for cost safety, and the explicit window epoch eventually
// retires the bounded loss.
func NewOpenAIRouteBudgetCache(rdb *redis.Client) service.OpenAIRouteBudgetStore {
	return &openAIRouteBudgetCache{rdb: rdb}
}

func (c *openAIRouteBudgetCache) GetLedgers(ctx context.Context, windows []service.OpenAIRouteBudgetWindowConfig) ([]service.OpenAIRouteBudgetLedger, error) {
	prepared, err := prepareOpenAIRouteBudgetWindows(windows)
	if err != nil {
		return nil, err
	}
	if c == nil || c.rdb == nil {
		return nil, service.ErrOpenAIRouteBudgetExhausted
	}

	pipe := c.rdb.Pipeline()
	commands := make([]*redis.MapStringStringCmd, len(prepared))
	for i, window := range prepared {
		commands[i] = pipe.HGetAll(ctx, window.redisKey)
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	ledgers := make([]service.OpenAIRouteBudgetLedger, len(windows))
	for i, window := range prepared {
		values, err := commands[i].Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		ledger := window.config.EmptyLedger()
		if len(values) > 0 {
			if values["policy"] != window.policyFingerprint {
				return nil, fmt.Errorf("%w: budget window %s policy changed without a new epoch", service.ErrOpenAIRouteReservationConflict, window.config.Scope.Window)
			}
			credit, parseErr := parseOpenAIRouteRedisInt(values, "credit")
			if parseErr != nil {
				return nil, parseErr
			}
			equivalent, parseErr := parseOpenAIRouteRedisInt(values, "equivalent")
			if parseErr != nil {
				return nil, parseErr
			}
			actual, parseErr := parseOpenAIRouteRedisInt(values, "actual")
			if parseErr != nil {
				return nil, parseErr
			}
			ledger.CreditUSD = service.OpenAIRouteCostUnitsToUSD(credit)
			ledger.EquivalentCostUSD = service.OpenAIRouteCostUnitsToUSD(equivalent)
			ledger.ActualAccountCostUSD = service.OpenAIRouteCostUnitsToUSD(actual)
		}
		if err := ledger.Validate(); err != nil {
			return nil, err
		}
		ledgers[window.originalIndex] = ledger
	}
	return ledgers, nil
}

func (c *openAIRouteBudgetCache) Reserve(ctx context.Context, req service.OpenAIRouteBudgetStoreReserveRequest) (service.OpenAIRouteBudgetStoreReservation, error) {
	result := service.OpenAIRouteBudgetStoreReservation{
		ReservationID:  strings.TrimSpace(req.ReservationID),
		RouteKey:       req.RouteKey,
		RejectedWindow: -1,
	}
	if c == nil || c.rdb == nil {
		return result, service.ErrOpenAIRouteBudgetExhausted
	}
	if result.ReservationID == "" || req.ReservationTTL <= 0 {
		return result, service.ErrOpenAIRouteInvalidCost
	}
	rateUnits, err := service.OpenAIRouteMultiplierToUnits(req.RateMultiplier)
	if err != nil {
		return result, err
	}
	estimateUnits, err := service.OpenAIRouteUSDToCostUnits(req.EstimatedBaseCostUSD)
	if err != nil {
		return result, err
	}
	prepared, err := prepareOpenAIRouteBudgetWindows(req.Windows)
	if err != nil {
		return result, err
	}
	routeFingerprint, err := validateOpenAIRouteBudgetRoute(req.RouteKey, prepared)
	if err != nil {
		return result, err
	}

	keys := make([]string, 1, len(prepared)+1)
	keys[0] = openAIRouteBudgetReservationRedisKey(prepared[0].domainFingerprint, result.ReservationID)
	args := []any{
		openAIRouteBudgetReservationSignature(result.ReservationID, routeFingerprint, rateUnits, estimateUnits, prepared),
		openAIRouteDurationMilliseconds(req.ReservationTTL),
		rateUnits,
		estimateUnits,
		routeFingerprint,
		len(prepared),
	}
	for i := range prepared {
		window := &prepared[i]
		extraUSD := (req.RateMultiplier - window.config.TargetAverageMultiplier) * req.EstimatedBaseCostUSD
		if extraUSD < 0 {
			extraUSD = 0
		}
		extraUnits, convertErr := service.OpenAIRouteUSDToCostUnits(extraUSD)
		if convertErr != nil {
			return result, convertErr
		}
		window.reservedExtraUnits = extraUnits
		keys = append(keys, window.redisKey)
		args = append(args,
			window.targetUnits,
			window.hardUnits,
			window.debtUnits,
			window.maxCreditUnits,
			window.ttlMilliseconds,
			window.reservedExtraUnits,
			window.scopeFingerprint,
			window.policyFingerprint,
		)
	}

	values, err := reserveOpenAIRouteBudgetScript.Run(ctx, c.rdb, keys, args...).Slice()
	if err != nil {
		return result, err
	}
	if len(values) != 3 {
		return result, fmt.Errorf("unexpected OpenAI route budget reserve response: %v", values)
	}
	code, err := openAIRouteRedisScriptInt(values[0])
	if err != nil {
		return result, err
	}
	emergency, err := openAIRouteRedisScriptInt(values[1])
	if err != nil {
		return result, err
	}
	rejected, err := openAIRouteRedisScriptInt(values[2])
	if err != nil {
		return result, err
	}

	switch code {
	case 1:
		result.Allowed = true
		result.Emergency = emergency == 1
		return result, nil
	case 0:
		if rejected >= 0 && int(rejected) < len(prepared) {
			result.RejectedWindow = prepared[rejected].originalIndex
		}
		return result, nil
	case openAIRouteBudgetScriptPolicyConflict, openAIRouteBudgetScriptRequestConflict:
		return result, service.ErrOpenAIRouteReservationConflict
	default:
		return result, fmt.Errorf("unexpected OpenAI route budget reserve code: %d", code)
	}
}

func (c *openAIRouteBudgetCache) Settle(ctx context.Context, settlement service.OpenAIRouteBudgetStoreSettlement) error {
	prepared, reservationKey, routeFingerprint, baseUnits, accountUnits, err := c.prepareSettlement(settlement)
	if err != nil {
		return err
	}
	keys := make([]string, 1, len(prepared)+1)
	keys[0] = reservationKey
	args := []any{
		openAIRouteDurationMilliseconds(settlement.AuditTTL),
		len(prepared),
		baseUnits,
		accountUnits,
		routeFingerprint,
	}
	for _, window := range prepared {
		targetCostUnits, convertErr := service.OpenAIRouteUSDToCostUnits(window.config.TargetAverageMultiplier * settlement.ActualBaseCostUSD)
		if convertErr != nil {
			return convertErr
		}
		keys = append(keys, window.redisKey)
		args = append(args,
			window.scopeFingerprint,
			window.policyFingerprint,
			window.ttlMilliseconds,
			targetCostUnits-accountUnits,
		)
	}
	code, err := settleOpenAIRouteBudgetScript.Run(ctx, c.rdb, keys, args...).Int64()
	if err != nil {
		return err
	}
	if code == openAIRouteBudgetScriptRequestConflict || code == openAIRouteBudgetScriptPolicyConflict {
		return service.ErrOpenAIRouteReservationConflict
	}
	if code != 1 {
		return fmt.Errorf("unexpected OpenAI route budget settlement code: %d", code)
	}
	return nil
}

func (c *openAIRouteBudgetCache) Cancel(ctx context.Context, settlement service.OpenAIRouteBudgetStoreSettlement) error {
	prepared, reservationKey, routeFingerprint, _, _, err := c.prepareSettlement(settlement)
	if err != nil {
		return err
	}
	keys := make([]string, 1, len(prepared)+1)
	keys[0] = reservationKey
	args := []any{
		openAIRouteDurationMilliseconds(settlement.AuditTTL),
		len(prepared),
		routeFingerprint,
	}
	for _, window := range prepared {
		keys = append(keys, window.redisKey)
		args = append(args, window.scopeFingerprint, window.ttlMilliseconds)
	}
	code, err := cancelOpenAIRouteBudgetScript.Run(ctx, c.rdb, keys, args...).Int64()
	if err != nil {
		return err
	}
	if code == openAIRouteBudgetScriptRequestConflict {
		return service.ErrOpenAIRouteReservationConflict
	}
	if code != 1 {
		return fmt.Errorf("unexpected OpenAI route budget cancellation code: %d", code)
	}
	return nil
}

func (c *openAIRouteBudgetCache) prepareSettlement(settlement service.OpenAIRouteBudgetStoreSettlement) ([]openAIRouteBudgetPreparedWindow, string, string, int64, int64, error) {
	if c == nil || c.rdb == nil {
		return nil, "", "", 0, 0, service.ErrOpenAIRouteBudgetExhausted
	}
	reservationID := strings.TrimSpace(settlement.ReservationID)
	if reservationID == "" || settlement.AuditTTL <= 0 {
		return nil, "", "", 0, 0, service.ErrOpenAIRouteInvalidCost
	}
	baseUnits, err := service.OpenAIRouteUSDToCostUnits(settlement.ActualBaseCostUSD)
	if err != nil {
		return nil, "", "", 0, 0, err
	}
	accountUnits, err := service.OpenAIRouteUSDToCostUnits(settlement.ActualAccountCostUSD)
	if err != nil {
		return nil, "", "", 0, 0, err
	}
	prepared, err := prepareOpenAIRouteBudgetWindows(settlement.Windows)
	if err != nil {
		return nil, "", "", 0, 0, err
	}
	routeFingerprint, err := validateOpenAIRouteBudgetRoute(settlement.RouteKey, prepared)
	if err != nil {
		return nil, "", "", 0, 0, err
	}
	return prepared, openAIRouteBudgetReservationRedisKey(prepared[0].domainFingerprint, reservationID), routeFingerprint, baseUnits, accountUnits, nil
}

type openAIRouteBudgetPreparedWindow struct {
	config             service.OpenAIRouteBudgetWindowConfig
	originalIndex      int
	domainFingerprint  string
	scopeFingerprint   string
	policyFingerprint  string
	redisKey           string
	targetUnits        int64
	hardUnits          int64
	debtUnits          int64
	maxCreditUnits     int64
	ttlMilliseconds    int64
	reservedExtraUnits int64
}

func prepareOpenAIRouteBudgetWindows(windows []service.OpenAIRouteBudgetWindowConfig) ([]openAIRouteBudgetPreparedWindow, error) {
	if len(windows) == 0 {
		return nil, service.ErrOpenAIRouteInvalidPolicy
	}
	groupID := windows[0].Scope.GroupID
	model := strings.TrimSpace(windows[0].Scope.Model)
	requestClass := windows[0].Scope.RequestClass
	domainFingerprint := openAIRouteBudgetDomainFingerprint(groupID, model, requestClass)
	prepared := make([]openAIRouteBudgetPreparedWindow, 0, len(windows))
	seen := make(map[string]struct{}, len(windows))
	for i, config := range windows {
		if err := config.Validate(); err != nil {
			return nil, err
		}
		if config.Scope.GroupID != groupID || strings.TrimSpace(config.Scope.Model) != model || config.Scope.RequestClass != requestClass {
			return nil, fmt.Errorf("%w: all budget windows must share group, model, and request class", service.ErrOpenAIRouteInvalidPolicy)
		}
		scopeFingerprint := config.Scope.Fingerprint()
		if _, ok := seen[scopeFingerprint]; ok {
			return nil, fmt.Errorf("%w: duplicate budget window", service.ErrOpenAIRouteInvalidPolicy)
		}
		seen[scopeFingerprint] = struct{}{}

		targetUnits, err := service.OpenAIRouteMultiplierToUnits(config.TargetAverageMultiplier)
		if err != nil {
			return nil, err
		}
		hardUnits, err := service.OpenAIRouteMultiplierToUnits(config.HardAverageMultiplier)
		if err != nil {
			return nil, err
		}
		debtUnits, err := service.OpenAIRouteUSDToCostUnits(config.EmergencyDebtLimitUSD)
		if err != nil {
			return nil, err
		}
		maxCreditUnits, err := service.OpenAIRouteUSDToCostUnits(config.MaxCreditUSD)
		if err != nil {
			return nil, err
		}
		policyFingerprint := fmt.Sprintf("%d:%d:%d:%d", targetUnits, hardUnits, debtUnits, maxCreditUnits)
		prepared = append(prepared, openAIRouteBudgetPreparedWindow{
			config:            config,
			originalIndex:     i,
			domainFingerprint: domainFingerprint,
			scopeFingerprint:  scopeFingerprint,
			policyFingerprint: policyFingerprint,
			redisKey:          openAIRouteBudgetRedisKey(domainFingerprint, scopeFingerprint),
			targetUnits:       targetUnits,
			hardUnits:         hardUnits,
			debtUnits:         debtUnits,
			maxCreditUnits:    maxCreditUnits,
			ttlMilliseconds:   openAIRouteDurationMilliseconds(config.TTL),
		})
	}
	sort.Slice(prepared, func(i, j int) bool {
		return prepared[i].scopeFingerprint < prepared[j].scopeFingerprint
	})
	return prepared, nil
}

func openAIRouteBudgetDomainFingerprint(groupID int64, model string, requestClass service.OpenAIRouteRequestClass) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s", groupID, strings.TrimSpace(model), requestClass)))
	return hex.EncodeToString(sum[:8])
}

func openAIRouteBudgetRedisKey(domainFingerprint, scopeFingerprint string) string {
	return openAIRouteBudgetKeyPrefix + "{" + domainFingerprint + "}:" + scopeFingerprint
}

func openAIRouteBudgetReservationRedisKey(domainFingerprint, reservationID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(reservationID)))
	return openAIRouteBudgetReservationKeyPrefix + "{" + domainFingerprint + "}:" + hex.EncodeToString(sum[:16])
}

func openAIRouteBudgetReservationSignature(reservationID, routeFingerprint string, rateUnits, estimateUnits int64, windows []openAIRouteBudgetPreparedWindow) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%s|%d|%d", strings.TrimSpace(reservationID), routeFingerprint, rateUnits, estimateUnits)
	for _, window := range windows {
		_, _ = fmt.Fprintf(hash, "|%s|%s", window.scopeFingerprint, window.policyFingerprint)
	}
	return hex.EncodeToString(hash.Sum(nil)[:16])
}

func validateOpenAIRouteBudgetRoute(route service.OpenAIRouteKey, windows []openAIRouteBudgetPreparedWindow) (string, error) {
	if !route.Valid() || len(windows) == 0 {
		return "", service.ErrOpenAIRouteNoCandidate
	}
	if route.GroupID != windows[0].config.Scope.GroupID ||
		strings.TrimSpace(route.Model) != strings.TrimSpace(windows[0].config.Scope.Model) ||
		route.RequestClass != windows[0].config.Scope.RequestClass {
		return "", fmt.Errorf("%w: route and budget scope differ", service.ErrOpenAIRouteInvalidPolicy)
	}
	return service.OpenAIRouteHealthStoreKeyForRoute(route).Fingerprint(), nil
}

func openAIRouteDurationMilliseconds(duration time.Duration) int64 {
	milliseconds := duration.Milliseconds()
	if milliseconds < 1 {
		return 1
	}
	return milliseconds
}

func openAIRouteRedisScriptInt(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected Redis integer type %T", value)
	}
}

func parseOpenAIRouteRedisInt(values map[string]string, field string) (int64, error) {
	value, ok := values[field]
	if !ok {
		return 0, fmt.Errorf("OpenAI route budget field %q is missing", field)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse OpenAI route budget field %q: %w", field, err)
	}
	if parsed > service.OpenAIRouteMaxExactInteger || parsed < -service.OpenAIRouteMaxExactInteger {
		return 0, service.ErrOpenAIRouteInvalidCost
	}
	return parsed, nil
}
