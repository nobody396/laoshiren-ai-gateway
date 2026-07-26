# Production monitoring boundaries

Production monitoring has two different jobs. They must not share the same
failure domain.

## 1. Monthly upstream probe

The application-owned monthly upstream probe runs every two minutes and checks
the real Codex, Claude, and Grok monthly-card gateway paths. Probe targets are
selected by monthly-card role rather than by transport platform, because Claude
and Grok both use the Anthropic-compatible gateway transport. When a path fails,
the probe also performs a direct-upstream diagnostic so operations can
distinguish gateway/routing failures from upstream failures.

This probe answers:

- can the monthly-card gateway select an account and complete a model request;
- is the selected upstream healthy, slow, rate-limited, or failing;
- is the failure inside the gateway or at the upstream.

Because this runner lives inside the production application, it cannot prove
that the application, VPS, DNS, CDN, TLS, or public route is available when the
whole platform is down.

## 2. External public readiness monitor

An external monitor checks:

```text
https://api.laoshirenai.com/readyz
```

`/readyz` returns `200` only when the public API route reaches an application
that is ready and its required PostgreSQL and Redis dependencies are healthy.
The external monitor therefore covers the failure modes that an in-process
probe cannot observe.

The external monitor must:

- run outside the production VPS;
- not depend on a developer computer being powered on;
- not consume private-repository GitHub-hosted runner minutes;
- check every five minutes and alert on down and recovery events.

UptimeRobot's external free monitor is the selected control plane. The owner
confirmed activation on 2026-07-26. Activation and notification contacts live
in UptimeRobot; no UptimeRobot credentials are stored in this repository.

## Retired implementation

The former `.github/workflows/uptime-monitor.yml` implementation is retired.
GitHub-hosted execution was coupled to Actions billing, while self-hosted
execution was coupled to a developer computer. Both failure domains can create
monitoring failures that say nothing about production availability.
