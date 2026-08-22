# Reliability Control

The reliability-control domain explains service availability to users, gives operators detailed upstream evidence, and supplies health facts to routing without conflating those responsibilities.

## Language

**Service Status**:
The public view of whether a published status product is currently usable, based on final customer outcomes when available and fresh active probes when customer evidence is sparse, presented without internal supplier details.
_Avoid_: Channel Status, Provider Status

**Channel Monitoring**:
The private operator view that explains service status using upstream accounts, routes, failures, latency, load, and recovery evidence.
_Avoid_: Service Status

**Pricing Channel**:
The internal pricing and model-mapping configuration associated with user-facing groups.
_Avoid_: Upstream Route, Provider Line

**User-facing Group**:
The product and access unit presented to users, such as a monthly-card group, enterprise group, or pay-as-you-go group.
_Avoid_: Upstream Account, Pricing Channel

**Service Family**:
A public navigation category that groups related status products, such as OpenAI/CodeX, Claude, Grok, Gemini, or Other.
_Avoid_: User-facing Group, Provider

**Status Product**:
A product published in the service-status catalogue. It may map to one user-facing group or to several groups that together deliver one customer product.
_Avoid_: User-facing Group, Service Component

**Status Catalog**:
The operator-approved set of service families, status products, and service components published on the service-status page. Membership is explicit rather than automatically derived from traffic.
_Avoid_: Group List, Model Catalog

**Monthly Plan**:
One of the current Plus, Pro, or Max monthly-card products. A monthly plan combines its paired GPT and Claude user-facing groups into one customer product with one logical quota.
_Avoid_: Legacy Monthly Group, Monthly-card Group

**Builder Pass**:
The public service family for the shared GPT, Claude, and Grok subscription service pools. Commercial plan tiers are not reliability boundaries within Builder Pass.
_Avoid_: Monthly Service, Monthly-card Status, Cloud

**Builder Pass Service**:
One status product within Builder Pass: GPT, Claude, or Grok.
_Avoid_: Monthly Plan, Legacy Monthly Group

**Legacy Monthly Group**:
A previous fractional-multiplier monthly-card group retained for historical entitlements but excluded from the current monthly-plan lineup and Status Catalog.
_Avoid_: Monthly Plan

**Upstream Account**:
A credential-bearing supplier account that can serve one or more models.
_Avoid_: User-facing Group, Upstream Route

**Upstream Route**:
A concrete path from one upstream account through a Base URL, endpoint, transport, and protocol.
_Avoid_: Pricing Channel, Account

**Reliability Observation**:
A non-sensitive fact about a customer request, upstream attempt, probe, latency, failure, load, or recovery that may be consumed by status, monitoring, or routing.
_Avoid_: Service Status, Routing Decision

**Customer Request**:
One user invocation counted once after all upstream attempts, retries, and route switches have finished.
_Avoid_: Upstream Attempt, Probe

**Upstream Attempt**:
One invocation of a concrete upstream route within a customer request or active probe.
_Avoid_: Customer Request

**Customer Availability**:
The proportion of final customer requests that succeed; active probes and excluded client or business-limit errors are not customer requests.
_Avoid_: Probe Availability, Route Health

**Probe Availability**:
The proportion of valid active probes that succeed. It may support a current status when customer evidence is sparse but never contributes to Customer Availability.
_Avoid_: Customer Availability

**Customer-impacting Failure**:
A final customer request that remains unsuccessful after permitted recovery is exhausted because of provider, route, or platform failure.
_Avoid_: Client Error, Business-limited Request, Probe Failure

**Service Component**:
The smallest published status unit: one status product, model or model family, and explicitly offered access mode. An access mode that is disabled or not offered to users is not a service component.
_Avoid_: Upstream Route, Pricing Channel

**Published Access Mode**:
A transport class explicitly offered to users for a service component. The initial catalogue contains HTTP only; WebSocket becomes a separate access mode only if it is explicitly released later.
_Avoid_: Internal Transport, Protocol Variant

**Operational**:
A published status indicating that the service is currently usable without material customer impact.
_Avoid_: Healthy, Green

**Degraded Performance**:
A published status indicating that requests generally succeed but latency, capacity, or recovery behaviour is materially worse than normal.
_Avoid_: Slow, Unstable

**Partial Outage**:
A published status indicating that some offered functionality or a meaningful subset of requests is unavailable while the rest remains usable.
_Avoid_: Degraded Performance, Major Outage

**Major Outage**:
A published status indicating that the service is broadly unusable for its intended customer purpose.
_Avoid_: Partial Outage

**Maintenance**:
A published status indicating a planned or explicitly managed service interruption.
_Avoid_: Major Outage

**Monitoring**:
A published status indicating that evidence is insufficient, stale, newly recovered, or still being observed before another status can be asserted.
_Avoid_: Operational, Unknown

**Computed Status**:
The current status derived automatically from Reliability Observations, confidence, and status-transition rules before any manual override is applied.
_Avoid_: Manual Status Override, Incident Phase

**Incident Candidate**:
A detected reliability condition that has crossed the threshold for investigation but has not yet acquired a confirmed cause or complete public narrative.
_Avoid_: Incident, Alert

**Incident**:
The single lifecycle record that groups a reliability event's affected products, components, routes, customer impact, operator updates, routing actions, and compensation evidence.
_Avoid_: Status Change, Alert

**Incident Phase**:
The operational stage of an incident: Investigating, Identified, Mitigating, Monitoring, or Resolved.
_Avoid_: Service Status

**Manual Status Override**:
A reasoned, time-bounded operator instruction that changes published status presentation without changing routing.
_Avoid_: Routing Action, Computed Status

**Observation Window**:
The interval from the first qualified abnormal reliability observation through the final recovery evidence for an incident.
_Avoid_: Customer Impact Window, Alert Window

**Customer Impact Window**:
The interval from the first Customer-impacting Failure through confirmed customer recovery. It is the time boundary used for affected-user and compensation analysis.
_Avoid_: Observation Window, Alert Window

**Compensable Impact Duration**:
For one affected user and Status Product, the sum of Customer Impact Segments that begin after that user's first Customer-impacting Failure, bounded by the Incident's Customer Impact Window. A segment ends when reliable recovery evidence moves the product into Monitoring; the subsequent healthy observation period is excluded.
_Avoid_: Customer Impact Window, Observation Window

**Customer Impact Segment**:
One continuous interval of qualified customer impact for a Status Product within an Incident. A relapse during Monitoring creates another segment in the same Incident, while the healthy gap between segments is excluded from compensation duration.
_Avoid_: Customer Impact Window, Observation Window, Incident

**Affected User**:
A real customer with at least one Customer-impacting Failure linked to an incident. Inclusion in the affected set does not by itself make the user eligible for compensation.
_Avoid_: Compensation Recipient, Failed Attempt

**Compensation-eligible User**:
An Affected User with at least four distinct final failed Customer Requests across the same Incident. Internal retries, Upstream Attempts, Active Probes, client errors, business-limit errors, and requests recovered before their final outcome do not count. Once eligible, compensation is still calculated separately for each affected Status Product and User-facing Group.
_Avoid_: Affected User, Failed Attempt

**Compensation Draft**:
An automatically calculated, non-executing proposal containing candidate users, eligibility evidence, exclusions, raw values, applied caps, rounded proposed benefits, and policy results for one incident.
_Avoid_: Compensation Batch, Executed Compensation

**Compensation Draft Revision**:
An immutable replacement for a Compensation Draft after an operator changes an inclusion, exclusion, amount, cap application, or policy judgement. It preserves the prior revision and records the reason, actor, and before-and-after result.
_Avoid_: Edited Draft, Compensation Execution

**High-value Compensation Draft**:
A Compensation Draft whose proposed total exceeds ten percent of the affected Status Products' rolling 30-day Verified Paid Value. It cannot be approved unchanged: operators must redesign the proposed allocation or policy application, generate a replacement draft, and obtain explicit owner confirmation.
_Avoid_: Compensation Draft, Automatic Scaling, Approval Warning

**Compensation Policy**:
The approved rules that turn incident evidence and affected-user facts into eligibility and proposed benefits.
_Avoid_: Compensation Draft, Manual Adjustment

**Customer Tier**:
A pre-existing service-recovery classification attached to one user account. All API keys, orders, balances, and Builder Pass entitlements under that account share the tier. It is derived from verified paid value, verified paid consumption, and loyalty evidence; it changes compensation size and cap but never compensation eligibility.
_Avoid_: User Role, Subscription Plan, API Key Tier

**Verified Paid Consumption**:
Customer consumption traced to paid balance lots or paid monthly entitlement cycles, excluding gifts, compensation, tests, migration value, and unresolved legacy usage.
_Avoid_: Usage Speed, Total Usage, Account Balance

**Customer Tier Snapshot**:
The customer tier frozen at the start of an incident's Customer Impact Window and used for that incident's compensation calculation. Later payments, upgrades, downgrades, or overrides do not change the snapshot.
_Avoid_: Current Customer Tier, Incident Override

**Customer Tier Override**:
A reasoned, audited, time-bounded operator change to a customer's future tier classification. It cannot alter an existing Customer Tier Snapshot.
_Avoid_: Manual Compensation Adjustment, Incident Tier

**Verified Paid Value**:
Cash revenue that is linked to a real customer and excludes gifts, compensation, tests, migration credits, refunded value, and unresolved revenue.
_Avoid_: Balance, Face Value, Unlinked Revenue

**Product Compensation Rate**:
The approved time-based goodwill base rate for one Status Product before Group Compensation Weight and Customer Tier Multiplier are applied.
_Avoid_: Group Rate Multiplier, Model Price, Upstream Cost

**Group Compensation Weight**:
A versioned weight that adjusts a Product Compensation Rate for the affected User-facing Group. It is normally seeded from commercial group value, frozen for an Incident, and never read retroactively from a later live group multiplier.
_Avoid_: Live Group Multiplier, Customer Tier Multiplier

**Customer Tier Multiplier**:
The approved multiplier applied to a product compensation rate for one customer tier.
_Avoid_: Group Rate Multiplier, Group Compensation Weight

**Compensation Benefit**:
The approved value delivered to an eligible affected user. CNY-equivalent goodwill is delivered as equal numeric pay-as-you-go balance or Builder Pass displayed credit according to the affected product. When one user has both benefit channels and a cap applies, the final value is allocated across channels in proportion to their uncapped raw values. The final value is rounded to two decimal places without a minimum benefit.
_Avoid_: Refund, Validity Extension

**Compensation Notice**:
The single customer-facing notification sent per user and Incident only after every approved Compensation Benefit has been executed and read back. It names affected public services, the customer's impact interval, final benefit and delivery channel, and whether failed requests were charged, while excluding supplier, account, route, internal group, and raw-log details.
_Avoid_: Public Incident Update, Compensation Draft, Execution Receipt

**Compensation Execution**:
The resumable delivery record for one approved Incident, user, and benefit channel. A completed execution is immutable and must be read back rather than repeated after a partial failure.
_Avoid_: Compensation Draft, Compensation Notice, Retry

**Compensation Evidence Snapshot**:
The durable incident and customer evidence needed to reproduce a Compensation Draft after raw operational logs expire, including source request identifiers, qualified facts, calculation inputs, exclusions, policy versions, and integrity evidence.
_Avoid_: Raw Log Archive, Compensation Draft, Financial Ledger

**Customer Relationship Cap**:
The maximum goodwill Compensation Benefit supported by a customer's verified economic relationship for one Incident, equal to ten percent of rolling 90-day Verified Paid Value. It is applied together with the Customer Tier cap and does not limit an Erroneous Charge Refund.
_Avoid_: Customer Tier Cap, Account Balance, Refund Limit

**Rolling Goodwill Cap**:
A customer-level limit on executed goodwill Compensation Benefits across a rolling 30-day window. Standard is capped at the lower of CNY 20 equivalent or twenty percent of rolling 90-day Verified Paid Value; Priority is capped at the lower of CNY 150 equivalent or thirty percent of that value; Strategic has no Rolling Goodwill Cap. Per-Incident caps still apply to every tier, and Erroneous Charge Refunds are excluded.
_Avoid_: Customer Tier Cap, Customer Relationship Cap, Refund Limit

**Shadow Compensation Evaluation**:
A non-executing evaluation period that generates Compensation Drafts from real Incident evidence and compares policy results without delivering Compensation Benefits. Initial exit requires at least 30 days, at least three real Incidents, reviewed results, and explicit owner approval.
_Avoid_: Compensation Draft, Automatic Compensation, Production Approval

**Erroneous Charge Refund**:
An exact reversal of a charge that should not have been applied. It is restitution rather than goodwill compensation and does not require a repeated-failure threshold.
_Avoid_: Compensation Benefit

**Active Probe**:
A synthetic request issued by the platform to measure an offered service component or upstream route when customer evidence is sparse.
_Avoid_: Customer Request

**Routing Action**:
An explicit, audited change to real traffic selection, such as disabling, deprioritising, switching, or enforcing an adaptive decision.
_Avoid_: Status Change, Health Observation
