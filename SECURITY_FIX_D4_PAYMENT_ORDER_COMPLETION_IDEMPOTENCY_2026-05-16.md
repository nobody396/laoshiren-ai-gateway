# SECURITY FIX D4: Payment Order Completion Idempotency

Date: 2026-05-16

## What Was Fixed

Payment order completion is now DB-level idempotent.

Previously, `PaymentService.completeOrder` read the order status and then called `UpdateStatus` without a database condition requiring `status = pending`. Concurrent Alipay notify/query completion paths could both observe a pending order and both grant or extend the user's subscription.

The completion path now performs an atomic `pending -> completed` transition through `PaymentOrderRepository.CompleteIfPending`. Only the caller that successfully updates one row is allowed to call `AssignOrExtendSubscription`. Duplicate or already-processed completion attempts return as idempotent no-ops.

## Changed Files

- `backend/internal/service/payment_order_repository.go`
  - Added `CompleteIfPending(ctx, id, alipayTradeNo) (bool, error)` to the payment order repository interface.
- `backend/internal/repository/payment_order_repo.go`
  - Implemented conditional completion with `WHERE id = ? AND status = pending`.
  - Sets `status = completed`, `completed_at`, `updated_at`, and optional `alipay_trade_no` only for the winning transition.
- `backend/internal/service/payment_service.go`
  - Changed `completeOrder` to call `CompleteIfPending` inside the transaction.
  - Subscription assignment now runs only when `CompleteIfPending` returns `true`.
  - Non-pending orders are treated as idempotent no-ops.
- `backend/internal/repository/payment_order_repo_unit_test.go`
  - Added repository coverage for pending completion, duplicate completion, non-pending no-op behavior, and concurrent completion.
- `backend/internal/service/payment_service_idempotency_test.go`
  - Added service coverage proving two completion paths for the same pending order trigger only one subscription side effect.

## Tests Added Or Updated

- `TestPaymentOrderRepositoryCompleteIfPendingCompletesOnce`
- `TestPaymentOrderRepositoryCompleteIfPendingNonPendingReturnsFalse`
- `TestPaymentOrderRepositoryCompleteIfPendingConcurrentOnlyOneWinner`
- `TestPaymentServiceCompleteOrderConcurrentOnlyOneSubscriptionSideEffect`
- `TestPaymentServiceCompleteOrderCompleteIfPendingFalseSkipsSubscription`
- `TestPaymentServiceCompleteOrderPendingCompletionAssignsSubscription`

## Checks Run

- `go test -tags=unit ./internal/service -run 'TestPaymentService|Test.*Payment.*Idempot|Test.*CompleteIfPending' -count=1`
  - Result: passed
- `go test -tags=unit ./internal/repository -run 'TestPaymentOrder.*CompleteIfPending|TestPaymentOrder' -count=1`
  - Result: passed
- `go build -o /tmp/sub2api-server-check ./cmd/server`
  - Result: passed

## Database Migration

No database migration is required. The fix uses existing columns on `payment_orders`.

## Before Going Online

- Review the D4 diff together with the existing A1/B1/B2/D1/A2/B3/B4/C1/C2/D2/D3 fixes.
- Run the broader backend test set chosen by the release owner if this branch is being batched with the other security fixes.
- Deploy only after explicit owner approval, per `AGENTS.md`.

## Remaining Risk

This fix prevents duplicate payment-order completion side effects by making the order state transition atomic.

It does not change subscription expiry concurrency behavior inside `AssignOrExtendSubscription`. That is intentionally left for D5.

## Next Issue

D5: subscription extension has lost update and nested transaction risk.
