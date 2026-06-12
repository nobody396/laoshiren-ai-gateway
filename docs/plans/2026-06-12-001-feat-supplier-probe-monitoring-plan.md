# Phase 1 - Local Context And Boundaries

Objective: turn supplier evaluation into an operational probe surface without mixing it with monthly-card quota accounting.

Inputs: existing supplier tables, supplier probe service, account API key records, monthly upstream monitor UI, admin RBAC route registry, deployment workflow.

Actions: inspect current supplier CRUD/probe implementation, account credential shape, monthly monitor aggregation patterns, and production deployment rules.

Outputs: implementation scope for supplier probe snapshot, account sync, bulk enable/disable, immediate probe run, and hourly stability buckets.

Validation: confirm the feature can be added without committing provider secrets or changing production configuration silently.

Risks: API keys must stay out of migrations, docs, logs, and replies; account billing multiplier must remain an account field, not supplier procurement cost.

Upgrade path: add provider-specific probe strategies for platforms that are not OpenAI-compatible or Anthropic-compatible.

# Phase 2 - Backend Monitoring Loop

Objective: make supplier probes a durable monitoring loop instead of only a manual single-supplier action.

Inputs: `suppliers`, `supplier_probe_results`, active account list, existing probe runner conventions.

Actions: add source-account linkage, due-probe listing, probe-result aggregation, batch probe toggles, account-to-supplier sync, batch immediate probe, and a background due-probe runner.

Outputs: admin APIs for probe snapshot, bulk probe enabled state, account sync, and batch probe execution.

Validation: run focused service, handler, repository, server route tests, plus backend build.

Risks: large provider lists can make batch immediate probes slow; background runner should bound each cycle.

Upgrade path: add a setting-backed global supplier probe switch if operators want a hard kill switch beyond per-supplier toggles.

# Phase 3 - Admin UI

Objective: make the supplier evaluation page read like the monthly monitor: status cards, timeline-like hourly signal, and operational controls.

Inputs: existing `SuppliersView.vue`, supplier admin API module, monthly monitor display conventions.

Actions: add top monitoring panel with coverage, window success rate, problem counts, latency, hourly buckets, risk hours, per-supplier rows, sync button, bulk enable/disable, and batch probe button.

Outputs: updated supplier admin page while preserving the existing CRUD table and edit dialog.

Validation: run frontend typecheck and production build; open the local route in the browser to confirm routing behavior.

Risks: local browser preview may be blocked by missing admin login state; build and typecheck remain the primary local frontend checks.

Upgrade path: split monitor controls into a reusable probe dashboard component if channel/account monitoring expands.

# Phase 4 - Data Import And Release

Objective: add the POMOAI CodexPlus account with a billing multiplier of `0.12`, sync accounts to suppliers, run probes, and deploy.

Inputs: user-provided provider key, production admin/API or database access, deployment workflow.

Actions: import the account through a protected runtime path, set `rate_multiplier` to `0.12`, sync active accounts into supplier probes, enable probes, run immediate checks, then deploy after local validation.

Outputs: production supplier monitor covering active supported API-key accounts and the new POMOAI CodexPlus account.

Validation: production health check, supplier snapshot API, account sync result, and probe result summary.

Risks: unsupported platforms are skipped until a compatible probe strategy exists; production auth/deploy credentials may be required at release time.

Upgrade path: add platform-specific cheapest-model mapping and per-provider probe adapters for Gemini/Antigravity if needed.
