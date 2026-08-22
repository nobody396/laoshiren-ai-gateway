# Universal routing Shadow evidence (PR3)

## Runtime boundary

- The universal route selected by PR2 remains the real route.
- The concrete target group's existing Legacy scheduler remains authoritative.
- Intelligent Routing V2 only evaluates the already hard-filtered account
  candidates in Shadow and cannot override the selected account.
- `Enforce` remains disabled in code. This PR creates no policy, universal
  group, route, or production configuration.

## Evidence added

Every enabled target-group Shadow evaluation now retains:

- universal access-group ID while `group_id` remains the concrete billing group;
- public model and inbound protocol;
- normalized **requested** service tier (`fast` is recorded as `priority`);
- the existing request IDs, candidate snapshot, exclusions, Legacy selection,
  adaptive selection and experiment lineage.

Universal Responses WebSocket routing remains unsupported. This PR does not
claim a connection-level model candidate set.

The requested tier is deliberately not called effective tier: PR4's safety
policy may filter it before upstream forwarding and billing.

Operators can isolate evidence with `access_group_id`, `inbound_protocol`, and
`requested_service_tier` on the existing decisions/stats endpoints. Ordinary
API keys keep empty provenance and unchanged behavior.

## Promotion rule

Universal samples do not authorize traffic control. Any future canary still
requires 24-72 hours, at least 200 valid decisions, at least 99% outcome
linkage, explicit owner approval, and staged rollout. Cross-target-group
selection is not claimed by this PR; each universal route still names one
concrete target group whose account pool is evaluated.
