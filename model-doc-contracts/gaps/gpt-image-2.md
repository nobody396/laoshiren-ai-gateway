# `gpt-image-2` model-doc contract schema gap

Status: `schema_gap`
Checked at: `2026-08-31 10:55 CST`
Publishable with schema v1: **no**

No `gpt-image-2.json` contract was created. Filling the current required fields with placeholder values would make the validator pass while producing a false public card.

## Facts we can support now

| Area | Evidence-backed result | Evidence |
| --- | --- | --- |
| Identity | Public model ID is `gpt-image-2`; it is an image-generation and image-editing model. | [OpenAI model page](https://developers.openai.com/api/docs/models/gpt-image-2) |
| Modalities | Text is input-only; image is input and output; audio and video are unsupported. | [OpenAI model page](https://developers.openai.com/api/docs/models/gpt-image-2) |
| Official image endpoints | OpenAI documents `POST /v1/images/generations` and `POST /v1/images/edits` for direct model use. | [OpenAI image-generation guide](https://developers.openai.com/api/docs/guides/image-generation) |
| Official output controls | GPT Image 2 supports flexible image dimensions, `low` / `medium` / `high` quality, output format/compression, and transparent backgrounds subject to format constraints. | [OpenAI image-generation guide](https://developers.openai.com/api/docs/guides/image-generation#customize-image-output) |
| Official edit behavior | Reference-image and mask editing are documented; GPT Image 2 always processes image inputs at high fidelity and does not accept a selectable `input_fidelity` value. | [OpenAI image-generation guide](https://developers.openai.com/api/docs/guides/image-generation#image-input-fidelity) |
| Public group readback | The latest live unauthenticated pricing readback still returns `GPT 图片生成线路` (group 51), multiplier `4`, but its public `models` array is now empty. Therefore the current public price inventory does **not** prove a `gpt-image-2` model row or current customer price. | `GET https://api.laoshirenai.com/api/v1/public/model-pricing`, payload updated at `2026-08-31T11:36:37.816099323Z` |
| Standard gateway surface | Current user documentation exposes `https://api.laoshirenai.com/v1` with `/images/generations` and `/images/edits`; generation returns `data[].b64_json`, and edits use multipart uploads. | `frontend/src/docs/content/api-images.md` |
| Gateway generation mechanics | Unit tests cover request validation, native Images forwarding, model mapping, Base64 output, usage, image count, billing attribution, and failover behavior. | `backend/internal/service/openai_images_test.go` |
| Gateway edit mechanics | Unit tests cover multipart single/multi-image upload, masks, endpoint isolation, parameter forwarding, response usage, and upstream field normalization. | `backend/internal/handler/openai_images_edits_test.go` |
| Current stream behavior | The current gateway rejects `stream=true`; this is a platform behavior that must not be confused with the broader official API guide. | `backend/internal/service/openai_images.go`; `frontend/src/docs/content/api-images.md` |

The existing unit tests prove local gateway behavior against controlled upstream doubles. They do **not** by themselves prove a current paid production generation/edit round. No new paid image request was made for this task.

The historical evidence inventory does contain an earlier owned production claim: public group `6`/key `9` and monthly group `7`/key `44` each returned HTTP 200 with a decodable `1536x1024` PNG through a Codex Desktop-style request, with routing recorded as `gpt-image-2 -> gpt-5.6-sol` and `/v1/images/generations -> /v1/responses`. The harvester classifies that entry as a candidate claim rather than reusable exact terminal proof, and it does not match today's group-51 public-price state. It is preserved as history, not promoted to a current verified contract cell.

Focused read-only validation passed in the current worktree:

```text
go test ./internal/service -run 'Test(ParseOpenAIImagesRequest|ForwardOpenAIImagesJSONEditUsesEditsEndpointAndMappedModel|ForwardCodexNativeImageGenerationUsesNativeImagesFallback)$' -count=1
ok github.com/bozhouDev/DragonCode-sub2api/internal/service

go test ./internal/handler -run 'TestOpenAIImagesEdits_(MultipartForwardsFilesMaskAndParameters|SDKMultiImageCanNormalizeArrayFieldsForCompatibleAccount)$' -count=1
ok github.com/bozhouDev/DragonCode-sub2api/internal/handler
```

## Why schema v1 cannot represent this product honestly

1. **Token-window fields are mandatory but not applicable.**
   `model.context_window` and `model.max_output_tokens` must be positive integers. OpenAI does not publish a chat-style context window or one fixed maximum output-token value for GPT Image 2. Image output token usage varies with dimensions and quality. Using `1`, a rate-limit TPM value, or an estimated image-token count would be fabricated.

2. **The protocol proof matrix is coding-model-specific.**
   Schema v1 requires text streaming, tool calls, tool-result continuation, and a final natural-language answer. The direct Images product instead needs separate contracts for generation, edit, mask edit, multi-image edit, output decoding, usage, moderation, error shape, timeout/idempotency, and optional partial-image streaming.

3. **A coding client is mandatory.**
   `clients` must contain a versioned client that completes an Agent tool loop. Direct consumers here are the OpenAI Images SDK/cURL and the `laoshirenai-imagegen` integration. Making up an SDK version or treating an image generation call as a shell-tool continuation would misstate the evidence.

4. **Modality verification is input-only.**
   `verification.modalities` validates only input `text/image/video`. It cannot record that image is both an input and the primary output, nor separately prove text input, reference-image input, image output, and video rejection.

5. **One Base URL cannot describe the two current product surfaces.**
   `frontend/src/docs/content/api-images.md` documents the standard OpenAI-compatible `/v1/images/*` surface. `frontend/src/docs/content/gpt-image-quickstart.md` still documents the older asynchronous `/gpt-image/v1` submit/task-poll surface. A contract must identify the canonical public surface and model each endpoint explicitly; silently combining them under one `base_url` would be ambiguous.

6. **Image-specific limits and pricing are missing.**
   The public card needs dimensions/aspect-ratio rules, quality values, formats, reference-image count, generation/edit result count, streaming status, and token-based image pricing. `groups[].multiplier` alone cannot communicate this product contract.

7. **Official capability and our gateway capability need separate status.**
   For example, the official guide describes image streaming and multiple outputs, while the current gateway documentation says generation is non-streaming and fixed to one returned image. Schema v1 has only one protocol status and cannot show `official`, `gateway_verified`, and `gateway_unsupported` separately.

8. **One current parameter claim conflicts with the official GPT Image 2 contract.**
   `frontend/src/docs/content/api-images.md` currently presents `input_fidelity=high` as supported. OpenAI's GPT Image 2 guide says callers must omit `input_fidelity` because the model always uses high fidelity and does not allow that setting to be changed. This must be resolved against a current production response before it appears on the public card.

## Smallest safe schema extension

The contract should branch on `product_kind` rather than forcing image products into a text-model shape:

```json
{
  "schema_version": 2,
  "product_kind": "image_generation",
  "model": {
    "id": "gpt-image-2",
    "display_name": "GPT Image 2",
    "family": "gpt-image",
    "context_window": null,
    "max_output_tokens": null,
    "input_modalities": ["text", "image"],
    "output_modalities": ["image"]
  },
  "access": {
    "base_url": "https://api.laoshirenai.com/v1",
    "endpoints": [
      {"name": "image_generations", "method": "POST", "path": "/images/generations"},
      {"name": "image_edits", "method": "POST", "path": "/images/edits"}
    ]
  },
  "image_capabilities": {
    "generation": {},
    "editing": {},
    "output": {},
    "streaming": {}
  },
  "integrations": [],
  "verification": {
    "input_modalities": {},
    "output_modalities": {},
    "endpoint_checks": []
  }
}
```

Required validator changes:

- allow `context_window` and `max_output_tokens` to be `null` only for non-text products;
- allow `clients` to be empty and validate `integrations` instead;
- validate generation and edit endpoints independently;
- validate input and output modalities independently;
- require image-specific capability evidence for every public claim;
- distinguish official support from current gateway verification;
- accept image-token pricing instead of requiring only a multiplier presentation.

## Remaining release evidence after the schema is fixed

Before rendering a full public card from a contract, preserve a secret-free production receipt for:

1. `/v1/models` visibility with an owned image-group key;
2. one low-cost generation returning a decodable image plus request ID, usage, and billing readback;
3. one single-image edit returning a decodable image plus request ID, usage, and billing readback;
4. one negative video-input check;
5. current non-streaming behavior;
6. `laoshirenai-imagegen` integration version and one successful end-to-end result if it will be shown as a verified integration.

These are paid or credentialed probes and were intentionally not executed without explicit authorization.
