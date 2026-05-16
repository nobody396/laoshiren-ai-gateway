# SECURITY FIX D5: Subscription Extension Atomicity

Date: 2026-05-16

## What Was Fixed

`AssignOrExtendSubscription` no longer computes an existing subscription's next `expires_at` in the service from a stale read and then writes an absolute timestamp.

Existing subscription extension now uses a repository-level atomic `UPDATE` that:

- Extends from the current database `expires_at` when the subscription is still valid.
- Extends from `now` when the subscription is expired.
- Caps the result at `MaxExpiresAt`.
- Restores the subscription to `active`.
- Appends notes in the same row update.
- Reuses any ent transaction already present in context through `clientFromContext`.

The service no longer starts an inner transaction for this path. When payment or redeem code flows pass a `dbent.NewTxContext`, the subscription update participates in the caller's outer transaction and rolls back with it.

## Changed Files

- `backend/internal/service/subscription_service.go`
  - Replaced service-side expiry calculation and nested transaction handling with a call to `ExtendExpiryAtomically`.
- `backend/internal/service/user_subscription_port.go`
  - Added `ExtendExpiryAtomically`.
- `backend/internal/repository/user_subscription_repo.go`
  - Added a single-statement atomic extension update for `user_subscriptions`.
- `backend/internal/service/subscription_assign_idempotency_test.go`
  - Added/updated service tests and test stubs for assign-or-extend behavior.
- `backend/internal/repository/user_subscription_repo_integration_test.go`
  - Added D5 integration coverage for concurrent extension and outer transaction rollback.
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/middleware/api_key_auth_google_test.go`
- `backend/internal/server/middleware/api_key_auth_test.go`
  - Updated test stubs for the new repository method.

## Tests Added Or Updated

- `TestAssignOrExtendSubscriptionCreatesNewSubscription`
- `TestAssignOrExtendSubscriptionD5ExtendsExistingAtomically`
- `TestUserSubscriptionAssignOrExtendConcurrentD5AddsBothExtensions`
- `TestUserSubscriptionAssignOrExtendTxRollbackD5DoesNotLeakSubscriptionUpdate`

## Checks Run

- `go test -tags=unit ./internal/service -run 'Test.*Subscription.*(AssignOrExtend|Concurrent|Transaction|Rollback|D5)' -count=1`
  - Result: passed
- `go test -tags=unit ./internal/service -run 'TestAssignOrExtendSubscription' -count=1`
  - Result: passed
- `go test -tags=unit ./internal/repository -run 'Test.*UserSubscription.*(Extend|Concurrent|Tx|D5)' -count=1`
  - Result: passed (`[no tests to run]`)
- `go test -tags=integration ./internal/repository -run 'TestUserSubscriptionAssignOrExtend.*D5' -count=1`
  - Result: passed
- `go test -tags=unit ./internal/server ./internal/server/middleware -run '^$' -count=0`
  - Result: passed
- `go build -o /tmp/sub2api-server-check ./cmd/server`
  - Result: passed

## Database Migration

No database migration is required. The fix uses existing `user_subscriptions` columns and row update semantics.

## Before Going Online

- Review the D5 diff together with the existing A1/B1/B2/D1/A2/B3/B4/C1/C2/D2/D3/D4 fixes.
- Run the broader backend release test set selected by the release owner if these security fixes are batched together.
- Deploy only after explicit owner approval, per `AGENTS.md`.

## Remaining Risk

The fixed path is `AssignOrExtendSubscription` for payment/redeem/default subscription assignment. Other admin-only subscription adjustment paths that intentionally set an absolute expiry through `ExtendSubscription` were not changed for D5.

## Next Issue

D6: agent settlement concurrent overpayment.
