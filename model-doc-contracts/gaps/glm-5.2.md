# GLM 5.2 / 5.3 contract evidence and remaining gaps

Checked in Beijing time on 2026-08-31. This contract pass made no route, account, Key, group, price, or production deployment mutation.

## Exact native model specifications

| Public model | Context | Maximum output | Native input | Native output | Official source |
| --- | ---: | ---: | --- | --- | --- |
| `glm-5.2` | 1048576 | 131072 | text | text | <https://help.aliyun.com/zh/model-studio/glm-5-2> |
| `glm-5.3` | 1048576 | 131072 | text | text | <https://help.aliyun.com/zh/model-studio/glm-5-3-by-zhipu> |

Image and video are authoritative negatives for both exact deployments. No paid full-context or maximum-output boundary request was run.

## Protocol and tool separation

The contracts distinguish a provider's direct API from the protocol actually available through the current 老实人AI group.

| Model / protocol | Base protocol | Function tools | `web_search` | Current public-product conclusion |
| --- | --- | --- | --- | --- |
| GLM 5.2 / Chat Completions | direct Alibaba matrix passed text, terminal stream, Usage, function call, continuation, final text, and invalid request | verified upstream | unsupported on the exact Beijing Chat route | verified base protocol; public group/Key E2E still blocked |
| GLM 5.2 / Responses | Alibaba officially supports it; upstream core completed | upstream supported | upstream native search completed | **blocked** because the public mapping was removed after Codex continuation repeatedly disconnected |
| GLM 5.2 / Messages | Alibaba documents compatibility | official candidate | no current exact evidence | **blocked** pending current-group base and client matrices |
| GLM 5.3 / Chat Completions | current group and real Agent loops completed | verified | officially unsupported | verified current product protocol |
| GLM 5.3 / Responses | the exact Alibaba upstream returned HTTP 400 Unsupported model | not exposed natively | unsupported | unsupported on this route; do not relabel the lossy Chat bridge as native Responses |
| GLM 5.3 / Messages | exact public group returned HTTP 403 | not exposed | unsupported | unsupported on this route |

GenerateContent is unsupported for both GLM group entries.

Repository evidence:

- `/Users/fujunhao/laoshirenai/log.md:9959,9961,10019,10062,10063,10084`
- the exact protocol and feature cells in `glm-5.2.json` and `glm-5.3.json`

## Reasoning

- GLM 5.2 accepts the seven wire values `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, and `max`. Official source: <https://help.aliyun.com/zh/model-studio/glm>.
- GLM 5.3 exposes native `low`, `high`, and `max`.
- Kimi Code's required GLM 5.3 alias mapping (`medium -> high`, `xhigh -> max`) was verified only in a local transparent-proxy regression and was not deployed. It is therefore kept blocked instead of being rendered as a production mapping.
- Official level declarations do not substitute for an exact public-route per-level Receipt. Those route-level feature cells remain blocked where no Receipt exists.

## Public access and price readback

The live public pricing inventory at `2026-08-31T11:36:37.816099323Z` returned:

- group `GLM 企业高速线路`, group id `60`, multiplier `1x`;
- both `glm-5.2` and `glm-5.3`;
- customer prices CNY `8 / 28 / 2` per 1M input/output/cache-read tokens.

GLM 5.3 has owned current-group route and Agent evidence. GLM 5.2 has direct-provider protocol/client evidence but no matching current public group/Key Receipt, so price visibility is not treated as callable-access proof.

## Remaining release blockers

1. GLM 5.2: owned public group/Key `/v1/models`, Chat Agent loop, and billing readback.
2. GLM 5.2 Responses: only re-enable after a complete public tool-result continuation and exact compatible-client matrix passes.
3. GLM 5.3: current public cache-hit Receipt, error passthrough Receipt, and deployed reasoning alias verification.
4. Windows/Linux exact client evidence remains a client-matrix task; macOS evidence must not be copied across operating systems.
