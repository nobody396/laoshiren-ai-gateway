# Chat Completions to Responses reasoning replay

## Scope and evidence

The gateway can bridge a client `/v1/chat/completions` request to an upstream
`/v1/responses` request, and translate the result back to Chat Completions.
Previously this path returned readable reasoning summaries but discarded opaque
`reasoning.encrypted_content`. Ordinary assistant text and tool calls cannot
reconstruct that state. Offline regression tests reproduce this information loss.

The reported GPT-6 Luna requests and upstream records both reported 18,944 cached
tokens. This excludes a local display truncation, but does **not** establish that
missing reasoning was the specific cause of that conversation's cache boundary.
Successful request bodies were not retained, so that causal conclusion remains
unverified. The cache-hit count is provider-reported, never manufactured here.

## Minimal repair contract

When native output contains encrypted reasoning, the bridged Chat response now
includes a gateway extension `choices[0].message.responses_output`: the complete
ordered native output array, not only the reasoning items. Ordering matters when
reasoning and tool calls interleave. Assistant `phase`, item IDs, encrypted
content, and reasoning summaries are preserved.

For SSE the corresponding field is sent in
`choices[0].delta.responses_output` with the terminal response. A compatible
client must **replace** its accumulated sidecar with this array, retain it with
the assistant message, and replay it in the next request's assistant message.
Do not turn encrypted state into text or append reasoning summaries as model
history. Clients may omit the human-readable `reasoning_content` on replay.

Replay is opt-in and must match the visible assistant text/refusal and tool calls.
Stale sidecars, non-assistant placement, missing encrypted reasoning, and unsupported
item types are rejected as invalid requests before calling the upstream. A client
that edits/compacts a message must remove the sidecar rather than replay stale state.
Legacy clients without the extension keep their existing request/response contract.
No new Redis cache, conversation storage, implicit session reconstruction, migration,
account change, or billing adjustment is introduced.

## Customer guidance and verification limits

- Using the advertised Chat Completions interface is not a customer error.
- This code change cannot make an arbitrary legacy client preserve unknown fields.
- Prefer a client with native Responses multi-turn support: use `/v1/responses`
  and replay the complete returned `output`, including encrypted reasoning items.
  Changing only the URL is insufficient; the request and history formats differ.
- A Chat Completions client may continue using the bridge only if it supports the
  extension above. A generic streaming aggregator must explicitly retain it.
- Native Responses forwarding is unchanged. Native Chat Completions routes strip
  the gateway sidecar rather than forwarding a Responses-only field to a provider.
- Local tests prove forwarding/state fidelity, not real provider cache utilization.
  Before claiming the reported cache issue resolved, perform an authorized, owned
  same-model/same-upstream multi-turn comparison and inspect provider-reported usage.
- A local commit/PR is not production deployment or customer-visible resolution.

Official protocol references:

- https://developers.openai.com/api/docs/guides/reasoning
- https://developers.openai.com/api/docs/guides/prompt-caching

## Regression commands

```sh
cd backend
go test ./internal/pkg/apicompat
go test -tags unit ./internal/service -run 'TestChatReasoningReplay|TestChatStreamCompletion' -count=1
```
