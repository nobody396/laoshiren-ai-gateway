# Intelligent Routing V2 implementation plan

## Baseline

- Development base: `origin/main@2daa4daea17ddebe10003f54258e4aa2621523e7`.
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
4. Shadow budget uses a configured estimate rather than settled request cost;
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

### V2.4 — guarded enforcement

- require 24–72 hours and at least 200 valid, usage-linked Shadow decisions per
  policy slice;
- require audit/linkage completeness >= 99%, no billing inconsistency, and no
  regression in user-visible errors or P95/P99 latency;
- enable deterministic canary assignment at `1% -> 5% -> 20% -> 50% -> 100%`;
- preserve text stickiness and one-click Legacy rollback at every stage;
- never enable `enforce` in the same deployment that introduces the code path.

## First production candidates

Begin with GPT text routes only. Add PomoAI HK/JP/US Base URL variants and
MoreCode only after each exact key/model/transport combination passes real
streaming and usage gates. Image and Claude policies remain separate slices and
are admitted after their own evidence thresholds.
