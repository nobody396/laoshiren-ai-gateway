# Verified Fast/Flex capability gate (PR4)

## Contract

- A configured Fast/Flex multiplier is pricing data, not proof that a provider
  accepts the service tier.
- Every channel model rule defaults to `fast_supported=false` and
  `flex_supported=false`.
- The admin confirmation controls are shown for OpenAI-compatible OpenAI,
  Grok, and Gemini channel rules; native Anthropic beta policy remains a
  separate control plane.
- An administrator may mark a tier supported only with a positive multiplier
  and a provider-verification timestamp.
- The request path applies the existing global Fast policy first. A global
  block/filter still wins. After a global pass, an unverified channel/model is
  filtered back to the standard tier before upstream forwarding and billing.
- Universal keys use their concrete target/billing group's channel capability;
  they do not bypass the gate.

## Dormant production boundary

Migration 200 backfills every existing row to unsupported. This PR does not
change the global production policy, verify any provider, enable any tier, or
deploy production. Fast therefore remains unavailable until both conditions
are independently satisfied:

1. the provider/model is verified and recorded in the channel UI; and
2. the global policy is deliberately changed from filter to pass for the
   approved scope.

Those control-plane writes and any later production release require separate
owner authorization and readback.
