# Reliability Control Implementation Plan

This plan turns the accepted Reliability Control glossary and ADRs into small,
reversible pull requests. It is an implementation map, not authorization to
deploy or execute compensation.

## Current seams to preserve

- Public `/status` currently renders the `sla-support` public document through
  `frontend/src/views/PublicInfoView.vue`; replace it only behind a read flag.
- The user dashboard consumes `/api/v1/monthly-card/status`; keep this contract
  until the new Service Status page has production evidence.
- Operator monitoring already lives under `/api/v1/admin/ops`,
  `frontend/src/views/admin/ops/OpsDashboard.vue`, and
  `frontend/src/views/admin/MonthlyUpstreamsView.vue`.
- OpenAI adaptive routing already has append-only Shadow decisions, Redis recent
  observations, PostgreSQL hourly observations, evidence epochs, and promotion
  assessment. Reliability Control must adapt those facts rather than create a
  second router or enable Enforce.
- `account_change_records`, paid balance lots, monthly entitlement cycles, and
  `user_notifications` are the existing ledger and delivery primitives.

## Module design

Keep four deep modules with small, consumer-specific interfaces. HTTP handlers,
scheduled jobs, probes, gateway handlers, and admin pages are adapters at these
seams.

### Reliability Evidence

```go
type ReliabilityRecorder interface {
    RecordFinalOutcome(context.Context, FinalOutcome) error
    RecordAttempt(context.Context, AttemptOutcome) error
    RecordProbe(context.Context, ProbeOutcome) error
}

type ReliabilityReader interface {
    Snapshot(context.Context, EvidenceQuery) (EvidenceSnapshot, error)
}
```

The implementation normalizes facts, enforces idempotency, excludes client and
business-limit outcomes from reliability, and stores no prompt, response,
credential, raw Base URL, or API key. One final customer request is recorded
once after recovery attempts finish; Upstream Attempts and Active Probes remain
separate fact types.

### Status Control

```go
type StatusReader interface {
    PublicSnapshot(context.Context) (PublicStatusSnapshot, error)
    OperatorSnapshot(context.Context, MonitoringQuery) (MonitoringSnapshot, error)
}

type StatusController interface {
    Evaluate(context.Context, EvaluationWindow) (StatusEvaluation, error)
    Override(context.Context, StatusOverrideCommand) (StatusOverride, error)
}
```

The implementation owns the explicit Status Catalog, evidence precedence,
staleness, hysteresis, Monitoring, and sanitized public projection. It never
changes routing.

### Incident Control

```go
type IncidentController interface {
    Reconcile(context.Context, StatusEvaluation) (IncidentChangeSet, error)
    Transition(context.Context, IncidentTransitionCommand) (Incident, error)
}

type IncidentReader interface {
    PublicTimeline(context.Context, IncidentQuery) ([]PublicIncident, error)
}
```

The implementation owns Incident Candidates, phases, affected products,
Customer Impact Segments, Monitoring relapses, public updates, and evidence
links. Alerts are inputs; they are not Incidents.

### Compensation Control

```go
type CompensationDrafting interface {
    Draft(context.Context, IncidentID) (CompensationDraft, error)
    Revise(context.Context, RevisionCommand) (CompensationDraft, error)
}

type CompensationExecution interface {
    Execute(context.Context, ApprovedDraftID) (ExecutionSummary, error)
}
```

The module hides tier snapshots, policy versions, paid-value evidence, group
weights, caps, rounding, channel allocation, evidence snapshots, idempotency,
readback, and notices. Shadow exposes `CompensationDrafting` only;
`CompensationExecution` is introduced with its dedicated PR.

## Pull-request sequence

### PR 0 — Track the accepted domain contract

**Title:** `docs(reliability): record status incident and compensation contract`

Scope:

- Track `CONTEXT.md`, `docs/adr/0001` through `0013`, and the agent-domain
  pointers.
- Unignore only `docs/agents/**` and `docs/adr/**`.
- Include this implementation map.

Checks:

- `git diff --check`
- link/path review; no runtime files or production configuration

Rollback: revert the documentation commit. No database or runtime effect.

### PR 1 — Add the append-only Reliability Observation spine

**Title:** `feat(reliability): persist normalized request attempt and probe facts`

Scope:

- Add an additive `reliability_observations` migration with an idempotency key,
  fact type, final outcome, ownership/exclusion classification, user/group/
  account internal identifiers where required, Service Component lookup fields,
  route fingerprint, latency, and timestamps.
- Implement `ReliabilityEvidence` and a PostgreSQL adapter.
- Add adapters at final gateway outcomes, existing upstream-attempt reporting,
  and `RunMonthlyUpstreamProbeOnce`.
- Deduplicate Active Probes by Upstream Route fingerprint so each concrete route
  is probed once per interval; projection to dependent Status Products and
  Service Components belongs to Status Control rather than duplicate requests.
- Add completeness counters and bounded asynchronous ingestion. Observation
  failure must never fail a customer request.
- Default the collector off. Backfill is a separate read-only command and must
  not run from a migration.

Checks:

- unit tests for final-request versus attempt/probe separation
- idempotency, exclusion, cancellation, and recovered-request tests
- unique-route probe deduplication tests
- Testcontainers migration/schema/index tests
- secret/PII snapshot tests
- existing gateway and OpenAI route-observation tests

Rollback: disable the collector. Keep the additive table for evidence and later
cleanup; do not down-migrate production.

### PR 2 — Add Status Catalog and Computed Status backend

**Title:** `feat(status): compute public service health from approved catalog`

Scope:

- Add explicit Service Family, Status Product, Service Component, binding,
  current-state, and Manual Status Override storage.
- Seed common services and Other; seed Builder Pass GPT, Claude, and Grok as
  separate products. Exclude legacy monthly groups and unpublished access modes.
- Implement `StatusControl` with customer evidence precedence, sparse-traffic
  probes, stale evidence => Monitoring, accepted hysteresis, and override expiry.
- Project one route-level probe observation to every explicitly bound Status
  Product and Service Component without counting it as customer traffic.
- Add sanitized `GET /api/v1/service-status` plus admin catalog/evaluation/
  override endpoints.
- Add feature flags with public visibility off by default.

Checks:

- table-driven state-transition and staleness tests
- replay tests for Customer Availability versus Probe Availability
- multi-product/component projection from one unique route probe
- catalog seed/mapping integration tests
- response tests proving no supplier, account, Base URL, route, or internal
  group disclosure

Rollback: disable evaluation/public flags; existing `/status` and monthly-card
status remain unchanged.

### PR 3 — Ship the standalone public Service Status page

**Title:** `feat(status): add customer service availability page`

Scope:

- Add a dedicated Service Status view at `/status` backed by the sanitized API.
- Render common services before Other, Builder Pass GPT/Claude/Grok separately,
  HTTP only, and model details only for a partial model impact.
- Render Operational, Degraded Performance, Partial Outage, Major Outage,
  Maintenance, and Monitoring with last-updated/stale semantics.
- Keep the current public support document as the disabled/fallback state.
- Link the existing dashboard status surface to the new page; do not remove
  `/monthly-card/status`.

Checks:

- frontend unit tests for every state, stale data, empty catalog, partial model
  impact, and sanitization
- router/meta/SEO tests
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run build`
- browser verification on desktop and mobile widths

Rollback: turn off public visibility and serve the existing support document.

### PR 4 — Add read-only Channel Monitoring for operators

**Title:** `feat(ops): add channel reliability monitoring workspace`

Scope:

- Add `/admin/channel-monitoring` under `admin:ops` as a separate page.
- Project Status Products down to components, user-facing groups, accounts,
  route fingerprints, latency, final failures, attempts, probes, load, recovery
  evidence, and observation completeness.
- Reuse existing Ops and monthly-upstream repositories where they already own
  the fact; do not copy SQL into Vue or handlers.
- Link to existing request-error drilldown and OpenAI Shadow audit views.
- Remain read-only: no disable, priority, Base URL, or Enforce controls.

Checks:

- repository query-mode and timeout tests
- permission and sanitized DTO tests
- frontend loading/empty/error/detail tests
- typecheck, build, and browser verification

Rollback: remove the sidebar entry or disable the admin feature flag. Existing
Ops pages remain available.

### PR 5 — Feed shared evidence to the adaptive router in Shadow only

**Title:** `feat(routing): consume reliability facts in adaptive shadow scoring`

Scope:

- Add one adapter from `ReliabilityEvidence.Snapshot` to the existing OpenAI
  route observation profile; do not introduce another policy engine.
- Preserve route fingerprint, request class, protocol, access group, experiment,
  activation, treatment fingerprint, evidence epochs, and completeness gates.
- Make missing/stale Reliability Evidence neutral and fall back to the existing
  Shadow profile.
- Expose comparison fields in the Channel Monitoring workspace.
- Do not add or enable Enforce.

Checks:

- Shadow outcome parity and fallback tests
- no customer-request latency regression beyond the existing observation budget
- evidence contamination tests across model, request class, route, protocol,
  experiment, and universal access group
- promotion assessment must remain NO-GO unless its existing gates pass

Rollback: disable the adapter flag; existing Shadow observations continue.

### PR 6 — Add Incident lifecycle and impact segments

**Title:** `feat(incidents): persist service impact and recovery lifecycle`

Scope:

- Add Incident, affected-product, Customer Impact Segment, update, observation
  link, and public-timeline storage.
- Implement `IncidentControl`: automatic Incident Candidates, operator phases,
  product recovery into Monitoring, ten-minute healthy observation, relapse as
  a new segment in the same Incident, and resolution.
- Manual Status Overrides remain separate and never mutate routing.
- Add admin incident timeline/editor and public sanitized timeline.

Checks:

- multi-product and relapse state-machine tests
- concurrent evaluator idempotency tests
- customer-impact duration excludes healthy Monitoring gaps
- public copy/sanitization and admin audit tests

Rollback: disable incident reconciliation; preserve recorded evidence and leave
Computed Status operating without incident creation.

### PR 7 — Calculate Customer Tier with history and overrides

**Title:** `feat(compensation): calculate auditable customer recovery tiers`

Scope:

- Add current tier, immutable tier history, daily evaluation evidence, and
  time-bounded override storage.
- Implement the tier evidence, thresholds, refresh, grace, incident snapshot,
  and override policy exactly as ADR-0009. Exact paid cash/order/entitlement
  relations must support every included value and exclusion.
- Add admin read-only tier explanation and audited override flow.

Checks:

- exact source/exclusion fixtures for balance and Builder Pass
- boundary, upgrade, grace, override-expiry, and incident-snapshot tests
- production-like anonymized distribution replay

Rollback: stop the evaluator; existing histories remain, and compensation stays
Shadow/unavailable.

### PR 8 — Generate immutable Compensation Drafts in Shadow

**Title:** `feat(compensation): generate incident drafts in shadow mode`

Scope:

- Add versioned policy, Product Compensation Rate, Group Compensation Weight,
  draft/revision/item, Customer Tier Snapshot, and Compensation Evidence
  Snapshot storage.
- Implement `Draft` and `Revise`; no delivery adapter exists in this PR.
- Implement eligibility, product/group calculation, policy values, benefit
  conversion, rounding, caps, redesign, and Shadow gates exactly as ADR-0008
  through ADR-0012.
- For each product, intersect every segment with the interval beginning at that
  user's first qualifying final failure; never award the pre-failure portion of
  a segment to a late-arriving user.
- Expose explanation-first admin draft/revision views.

Checks:

- golden formula fixtures including today's anonymized noon incident
- late-arriving-user fixtures that clip an in-progress segment at the user's
  first qualifying failure
- multi-product, mixed-channel, cap-order, rolling-window, rounding, and high-
  value redesign tests
- immutable revision and three-year evidence reproduction tests
- assert zero account changes, entitlement changes, notices, or emails

Rollback: disable draft generation. This PR has no financial write path.

### PR 9 — Add explicitly approved, resumable compensation execution

**Title:** `feat(compensation): execute approved benefits with exact readback`

Prerequisite: the ADR-0012 Shadow exit gates have passed and the owner has
explicitly authorized this PR to leave draft-only mode.

Scope:

- Add the execution adapter only now: pay-as-you-go balance through durable
  compensation lots/account changes; Builder Pass through shared displayed
  credit. No validity extension.
- Require an immutable approved draft revision and fail closed on High-value
  Compensation Drafts.
- Use one idempotency identity per Incident, user, and benefit channel; resume
  incomplete items only and read back every ledger/entitlement delta.
- Send one Compensation Notice only after all approved channels for that user
  pass readback.
- Keep Erroneous Charge Refund as a separate exact-reversal command.

Checks:

- Testcontainers transaction and partial-failure/resume tests
- duplicate execution refusal and exact-readback tests
- balance/Builder Pass proportional allocation tests
- notification dedupe and no-email tests
- full dry-run against sanitized production snapshots

Rollback: turn off execution permission immediately. Never delete executed
ledger rows; recovery is resume/readback, not rerun or reverse.

## Release and rollout gates

Every PR follows the normal branch/PR/CI flow and stops before production until
the owner explicitly says `上线`.

1. **Additive migrations only.** New tables and indexes; no destructive rewrite
   of `usage_logs`, `ops_error_logs`, route evidence, ledgers, or notifications.
2. **Flags default off.** Observation, evaluation, public page, incidents,
   router adapter, tier evaluation, draft generation, and execution are separate
   flags. No migration enables them.
3. **Old surfaces stay available.** Static `/status`, dashboard monthly-card
   status, `/admin/ops`, and `/admin/monthly-upstreams` remain rollback paths.
4. **Evidence before presentation.** Public status cannot enable until final-
   outcome ingestion completeness, probe freshness, sanitizer, and replay gates
   pass.
5. **Routing stays Shadow.** Reliability integration does not authorize Enforce.
6. **Compensation stays Shadow.** PR 8 cannot write financial or entitlement
   state. PR 9 cannot be merged as executable until the ADR-0012 gates and a
   fresh owner approval pass.
7. **Immutable release proof.** Required local checks, PR CI, main CI, exact
   commit image digest, replica health, `/health`, and relevant UI/API readback
   remain mandatory for each production release.

## Proposed issue and PR dependency graph

```text
PR0 docs
  -> PR1 evidence
      -> PR2 status backend
          -> PR3 public page
          -> PR4 channel monitoring -> PR5 router Shadow adapter
          -> PR6 incidents -> PR7 tiers -> PR8 compensation Shadow
                                          -> [ADR-0012 gates + owner approval]
                                          -> PR9 execution
```

Do not create all branches at once. Create one temporary registered worktree for
the current frontier PR, merge it after CI and review, then remove it before
starting the next PR. This keeps the local branch/worktree inventory clear.
