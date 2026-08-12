# Intelligent Routing V2 implementation plan

## Baseline

- Development base after the coordinated releases: `origin/main@beae9a8e1d3c704b472d6c5915cdd240fc84ceb5`.
- Reuse the merged cost budgeter, route/provider health state machine, weighted
  allocator, Redis atomic budget store, versioned Shadow controller, and
  durable decision audit.
- Reuse the merged image request classifier and fixed renderer/fallback path.
- Do not change image fixed-price billing in this workstream while its supplier
  cost reconciliation is still being verified.

## Current gaps

The merged router is a safe Shadow skeleton, not an enforce-ready learner:

1. reliability and TTFT inputs are process-local and keyed only by account;
2. route health is read but production outcomes do not yet drive transitions;
3. account/provider traffic shares and exploration inputs are not populated;
4. the budget needs a conservative per-route prediction because actual cost is
   known only after a request completes;
5. `enforce` is intentionally hard-disabled;
6. text and image observations previously shared policy/health/budget identity;
7. no Beijing hour-of-week profile exists for Base URL variants.

## Route identity

All learning and health decisions use this identity:

```text
group + account + model + request_class + endpoint_hash + transport + failure_domain
```

- A PomoAI Base URL variant is represented by a separate account.
- Accounts backed by the same supplier/failure domain must explicitly share
  `routing_failure_domain_id`; they are not independent disaster recovery.
- Text keeps session affinity. Image requests are stateless single-request
  decisions and never train text routing.

## Delivery sequence

### Implementation status (2026-08-12 Beijing time)

- V2.0 is implemented on this branch: request-class isolation and full route
  identity are covered by migrations and unit tests.
- V2.1 passive collection is implemented: every real upstream attempt is sent
  through a bounded asynchronous collector into shared Redis rolling windows;
  settled text usage cost is recorded separately without double-counting the
  attempt. Image/video cost learning remains deliberately disabled.
- V2.2 Shadow scoring inputs are implemented: the controller consumes shared
  Wilson reliability, P90 TTFT, P95 completion latency, partial-stream rate,
  recent account/provider share, bounded exploration, and route-specific
  expected text cost. The cost estimate uses the exact route's seven-day
  authoritative settlements only after 20 samples, shrinks them toward the
  configured prior, and caps drift to 0.25x--4x. Image cost never learns from
  these observations. All factors and estimate provenance are persisted in the
  existing decision snapshot.
- V2.3 passive health foundations are implemented: real outcomes drive the
  narrow route, and only infrastructure-like failures seen on at least two
  distinct accounts inside the same explicit failure domain can open the
  provider circuit. Key/model/rate-limit/payment failures never fan out.
  Single-owner active probes and durable aggregate checkpoints remain deferred.
- Shadow audit writes use a bounded asynchronous queue instead of blocking
  account selection. Audit, observation, and health-application completeness
  count in-flight, dropped, rejected, and failed evidence conservatively, and
  each must remain at least 99% before manual review. Client cancellations are
  neutral evidence and never train an upstream circuit.
- Real selection remains Legacy-only. No production policy or account setting
  is changed by this development branch.

### V2.0 — identity and audit correctness

- add `request_class=text|image` to route, health, policy, budget, and audit
  identity;
- default old policies without the field to `text`, not all request classes;
- keep historical audit rows as `unknown` rather than relabeling them;
- keep real selection Legacy-only.

### V2.1 — continuous shared observations

- add a shared Redis rolling-stat store keyed by the complete route identity;
- record every upstream attempt, including covered failovers;
- track success/failure class, TTFT histogram, completion latency, partial
  stream, sample count, last observation, and actual settled cost;
- maintain global, recent-window, and Beijing hour-of-week views;
- persist non-sensitive aggregate checkpoints for restart/audit recovery.

The rolling store currently uses 5-minute buckets for a one-hour recent view,
Beijing calendar-day buckets for a seven-day global view, and the matching
Beijing hour-of-week across eight ISO weeks. Keys contain only a route
fingerprint. A collector queue overflow never delays a customer request and is
instead exposed as evidence loss in the admin health endpoint.

Settled-cost feedback is text-only. Until 20 cost samples exist for an exact
route, the policy's configured estimate remains authoritative. Afterwards the
learner uses a 20-sample prior and a bounded empirical mean; the candidate
snapshot records configured prior, learned estimate, observed mean, samples and
source. The allocator, atomic reservation and Shadow settlement all use the
same selected-route estimate, avoiding a scoring/budget mismatch.

### V2.2 — Shadow V2 scoring

- hard filter capability, model, endpoint, transport, circuit, balance/quota,
  and concurrency before scoring;
- derive conservative reliability from Wilson lower bounds;
- combine reliability, P90/P95 TTFT, tail risk, load/headroom, cost, and bounded
  exploration;
- blend hour-of-week statistics with global statistics only after a minimum
  local sample threshold;
- populate account/provider share caps and common-failure-domain penalties;
- audit every input, factor, exclusion, and Legacy/Adaptive divergence.

### V2.3 — probes and health transitions

- feed real request outcomes and active probes into the existing health state
  machine with route-scoped failure classification;
- isolate model/rate-limit/key failures to the narrowest route;
- escalate correlated failures to the shared failure domain;
- use single-owner half-open probes and staged recovery shares.

Coordination boundary: this worktree does not start an active probe, timer, or
production observation job. It implements only the passive evidence and health
primitives until the unified release finishes and a separate production change
is authorized.

### V2.4 — guarded enforcement

- use 24 hours only as an early health checkpoint; require a full 72-hour query
  window, at least 71 hours between its first and last real decision (at most
  one hour of total end-exclusive boundary gap; the same one-hour total applies
  to any wider retry window), and at least 200 valid,
  usage-linked Shadow decisions per policy slice;
- require audit/linkage completeness >= 99%, no billing inconsistency, and no
  regression in user-visible errors or P95/P99 latency;
- require process-local audit and observation completeness counters to have
  started before T0; any in-window restart invalidates that evidence slice,
  and multi-replica deployments need per-replica review until counters are
  durably aggregated;
- require zero audit and observation storage-check failures since those
  counters started; a later successful probe must not erase an evidence gap;
- require adaptive account and provider assignment totals to equal the full
  evaluated-decision count before checking concentration caps;
- enable deterministic canary assignment at `1% -> 5% -> 20% -> 50% -> 100%`;
- preserve text stickiness and one-click Legacy rollback at every stage;
- never enable `enforce` in the same deployment that introduces the code path.

## First production candidates

Begin with GPT text routes only. Add PomoAI HK/JP/US Base URL variants and
MoreCode only after each exact key/model/transport combination passes real
streaming and usage gates. Image and Claude policies remain separate slices and
are admitted after their own evidence thresholds.
