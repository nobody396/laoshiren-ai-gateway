# PomoAI multi-Base-URL Shadow bridge handoff

## Purpose

This change makes low-frequency, self-owned PomoAI Base-URL benchmarks usable
as a bounded and auditable **Shadow-only prior**. It also prepares multiple
endpoints of one schedulable account as distinct Shadow route candidates while
preserving their shared account/credential/provider failure domain.

It does not enable a policy, alter an Account, execute a suggested endpoint, or
make Enforce available.

## Safety properties

1. `benchmark_prior_enabled` defaults to false. Existing policies issue no new
   benchmark query and retain their existing scoring inputs.
2. `route_variants` is rejected unless the benchmark prior is explicitly
   enabled; only text Responses over HTTP/SSE can expand.
3. A variant is synthesized from an already hard-filtered API-key Account. The
   Account pointer, account ID, rate, priority, load and failure domain remain
   shared; only the endpoint hash differs. Legacy still uses the Account's real
   configured Base URL.
4. Raw Base URLs never enter decision audit. The normalized policy snapshot
   stores only account ID and endpoint hash.
5. Active evidence has a 72-hour global window, 6-hour recent window, four-week
   Beijing hour-of-week window, 24-hour recency half-life, zero influence below
   12 samples, and a 25% maximum confidence. It cannot train cost or actual
   traffic share.
6. A generated route remains excluded until the bounded active prior produces
   an effective sample or that exact route has at least 12 passive outcomes.
7. Same-account variants use one aggregate account share; same-failure-domain
   variants use one aggregate provider share. Selection/exclusion mapping uses
   the complete route fingerprint rather than account ID.
8. Benchmark reads use a one-minute bounded L1 and singleflight with a 20ms
   load timeout. Missing schema, timeout, overflow, malformed input or storage
   error invalidates only the Shadow evaluation; Legacy serves the request.
9. Shadow audit/stats identify the selected endpoint and route variant. The
   promotion gate requires selected account, provider and endpoint counts all
   to equal the evaluated decision count.

## Schema and compatibility

- Migration `191_index_base_url_benchmarks_for_shadow_prior.sql` creates an
  optional partial covering index only when the operational table and every
  required column exist; otherwise it is a no-op.
- The production observation store implements an optional benchmark read
  interface. Readiness does not require the operational table while the feature
  is disabled.
- New audit fields use `omitempty` where necessary; a default-disabled policy
  does not emit benchmark-policy fields.

## Verification

Run from `backend/`:

```sh
GOWORK=off go test ./internal/service -count=1 -timeout=10m
GOWORK=off go test ./internal/repository -count=1 -timeout=10m
GOWORK=off go test ./internal/handler/... ./internal/server ./cmd/server -count=1 -timeout=10m
GOWORK=off go vet ./internal/service ./internal/repository ./internal/handler/admin ./internal/server ./cmd/server
GOWORK=off go test -race ./internal/service -run 'Test(OpenAIRouteController|ExpandOpenAIRouteShadowVariants|OpenAIRouteBenchmark)' -count=1 -timeout=10m
GOWORK=off go test -race ./internal/repository -run 'TestOpenAIRoute(DecisionRepositoryStats|Benchmark)' -count=1 -timeout=10m
```

## Activation boundary

Merging or deploying this code must keep the current policy unchanged. A later
Shadow experiment needs a new policy version, unique activation ID, server-time
T0, atomic CAS receipt and verified rollback. It may add only the chosen
PomoAI endpoints to Shadow. It still cannot switch real traffic. A Canary
requires a separate owner-approved package and runtime safety controller.
