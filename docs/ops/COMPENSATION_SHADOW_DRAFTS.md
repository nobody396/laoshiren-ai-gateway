# Compensation Shadow Drafts

PR8 adds a draft-only Compensation Control module. It cannot approve or execute
benefits and contains no adapter for balance, Builder Pass, notifications, or
email. `compensation_shadow_draft_enabled` defaults to `false`.

## Frozen calculation

A draft is created only for an Incident with a complete Customer Impact Window.
It uses immutable policy, Product Compensation Rate, Group Compensation Weight,
Customer Tier Snapshot, final Customer Request failures, product impact
segments, and exact paid-value evidence.

A user is eligible after four distinct final Customer Request failures linked to
the Incident. Attempts, probes, recovered requests, excluded requests, client
errors, and business-limit errors do not count. For each product and
user-facing group, the duration is the intersection of every impact segment
with the interval starting at that user's first final failure for that
product/group. Monitoring gaps and pre-failure segment time are excluded.

Raw value is:

```
impact duration * product CNY-fen/hour rate * group weight * tier multiplier
```

The initial product rate is CNY 30/hour. Group weights snapshot the group's
commercial rate multiplier when a group is created or its multiplier changes;
drafting never reads a later live multiplier back into an older Incident. A
missing historical version is retained as an evidence gap with zero proposal.
The result is capped by the frozen tier cap, ten
percent of rolling 90-day Verified Paid Value, and the applicable remaining
30-day rolling goodwill cap. Existing `compensation` balance lots and monthly
entitlement cycles are the read-only rolling exposure source; PR8 never creates
them. A capped mixed balance/Builder Pass result is allocated proportionally by
uncapped raw value and rounded to exact CNY cents with a largest-remainder rule.

For the high-value redesign gate, each affected product contributes the exact
30-day Verified Paid Value of each distinct affected user of that product. A
proposal above ten percent of that frozen total is `redesign_required`; it
cannot be reviewed as aligned until a reasoned immutable revision falls below
the gate. This rule is evidence-backed and intentionally does not infer revenue
from account balances or unlinked finance notes.

## Shadow review and exit gate

Every revision preserves its predecessor, reason, actor, before/after values,
full qualified request facts, clipped segment boundaries, calculation inputs,
policy versions, and evidence hashes. Snapshot-only reproduction does not need
the original observation or segment rows.
Evidence is retained for at least three years. Operators can add one immutable
comparison between the latest revision and owner judgement.

The read-only assessment can become eligible for owner review only after all of
these are true:

- at least 30 elapsed Shadow days;
- at least three real Incidents drafted in the current Shadow period;
- at least three latest draft revisions reviewed in that period;
- no Incident evidence gap or missing tier/rate/group/segment evidence.

Even then `execution_available` remains `false`. PR9 must independently enforce
explicit owner approval and every execution/readback prerequisite.

## Rollback and enablement

Disable `compensation_shadow_draft_enabled` to stop new drafts. Keep immutable
policies, drafts, revisions, reviews, and evidence. Before any production Shadow
enablement, audit the prior 90 days of structured refund relations and confirm
Customer Tier/Incident evidence completeness. No production deployment or
Shadow activation is authorized by this document.
