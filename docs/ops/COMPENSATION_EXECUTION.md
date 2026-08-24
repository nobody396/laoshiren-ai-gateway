# Approved Compensation Execution

PR9 adds the execution adapter, but `compensation_execution_enabled` remains
`false` by default. Deployment does not enable it. A production enable request
must carry the exact owner confirmation phrase and is rejected unless the
current Shadow assessment has passed 30 days, three real Incidents, three latest
aligned reviews, and complete evidence.

## Approval boundary

Only the latest resolved, aligned, non-high-value immutable draft revision can
be approved. Approval and every money mutation are restricted to an authenticated
super-admin owner, in addition to the exact confirmation phrase. Approval
requires a unique idempotency key, a reason, and the exact hash of the preview
the owner viewed. It freezes the draft evidence plus every per-user amount and
notification body. Once approved, the whole revision series is frozen;
high-value drafts must therefore be redesigned before approval.

Execution requires both that approval and a separately enabled execution
permission whose audit snapshot proves the Shadow exit gate. Turning the switch
off immediately blocks new and resumed execution without deleting evidence.
Before approval or execution, the dry-run endpoint shows the exact per-user
channel amounts and the exact in-app notification body that will be delivered.
Preview facts are aggregated in one bounded query and frozen with one batch
write, so a large Incident does not hold the revision lock for per-user N+1
round trips. Preview and approval also have explicit service deadlines.
Creation of execution rows, every incomplete channel transaction, and notice
delivery acquire the same permission lock as the kill switch and revalidate the
approved draft under the revision lock. An optimistic preview is never authority
for a money write.

## Resumable benefit delivery

The durable idempotency identity is `(Incident, user, benefit channel)`.
Balance and Builder Pass channels execute in independent transactions, so a
partial batch resumes only the failed channel.

- Balance writes one compensation balance lot, one account-change ledger row,
  and the exact user balance delta.
- Builder Pass writes group-specific compensation credit cycles linked to the
  existing active subscription. Every cycle ends at the existing subscription
  expiry; no subscription or entitlement validity is extended.
- Each verified channel has an immutable receipt containing exact asset and
  ledger readback. A verified execution cannot be changed or deleted.
- One in-app Compensation Notice is created only after every positive channel
  for that user is verified. It contains public service names, impact interval,
  delivered values/channels, and observed failed-request charge status. It
  uses Beijing time, equals the approved preview, contains no supplier, account,
  route, group, or raw-log details, and sends no email.
  Its charge-status statement considers only frozen qualified final customer
  failures; retry attempts, probes, recovered outcomes, client errors, business
  limits, and other excluded facts cannot trigger a refund warning.
- A partial HTTP conflict carries the durable batch state so the admin UI shows
  which channels already verified and which channel must be resumed.

## Erroneous Charge Refund

Erroneous charge reversal is a separate exact command. It does not use failure
counts, product rates, tier multipliers, relationship caps, rolling goodwill
caps, or draft approval.

- Balance billing restores the exact `usage_logs.actual_cost`, writes a dedicated
  refund lot, and records an immutable refund/ledger relation.
- Builder Pass billing restores the exact attributed cycle credit without
  changing expiry and appends a consumption reversal so tier supporting
  evidence no longer counts the reversed consumption.
- The usage log and idempotency key can each be reversed only once.

## Rollback

Set `compensation_execution_enabled=false`. Preserve approvals, execution rows,
assets, events, receipts, notices, and refunds permanently. Recovery is
resume/readback, never delete, rerun a verified channel, or automatically
reverse a goodwill benefit. No production deployment or execution enablement is
authorized by this document.
