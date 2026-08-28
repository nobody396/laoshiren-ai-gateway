# OpenAI Responses / Chat Completions / Anthropic Messages 开源桥接审计

> 审计时间：2026-08-28（北京时间）
> 目标：为 PR #262 的协议桥接选择可信基线，明确哪些代码直接吸收、哪些设计只作参考、哪些能力必须拒绝降级。
> 范围：只查官方 GitHub 源码、官方发布说明、官方 SDK/协议；没有使用二手教程。

## 结论先行

1. **最适合直接吸收的基线是 Wei-Shaw/Sub2API。** 我们本身是其派生代码，许可证为 LGPL-3.0；正式版 `v0.1.179` 已把 Responses→Chat fallback 用于国产 API Key 账号，`main` 又补齐严格客户端、工具参数、命名空间工具和流式终态等修复。
2. **当前分支应采用“Sub2API main 成熟转换核心 + 我们自己的按模型路由和更严格 fail-closed”。** 当前 worktree 的 `chatcompletions_responses_bridge.go` 已与 Sub2API main 高度同源；不应保留 PR 初版的简化转换器。
3. **QuantumNous/New API 证明了正确的长期架构是转换注册表 + 独立流状态机 + golden matrix。** 但 New API 是 AGPL-3.0，不能直接复制进当前 LGPL 项目；只吸收设计和测试思想。
4. **LiteLLM 证明 Responses→Chat 不应靠“猜”，而应有显式模型/部署协议选择。** 它用 `use_chat_completions_api` 或模型前缀显式切换，并把 `previous_response_id` 做成有状态会话层。我们当前按模型配置协议的方向正确；在没有会话存储前必须拒绝 stateful 字段。
5. **vLLM 不是通用上游代理桥，不能整段搬。** 它原生提供 Responses 和 Anthropic endpoint，再统一进内部 Chat/生成结构；最值得参考的是事件状态机、复合 delta 拆分、并行工具索引和 Anthropic usage/tool_result 测试。
6. **OpenRouter 的核心代理没有可审计开源实现。** 可采用其官方 Open Responses 规范作为 wire-level 验收合同：事件名和 JSON `type` 一致、单调 `sequence_number`、完整 item 生命周期、usage 只在权威终态落账。

## 一级来源快照

| 项目 | 审计版本 | 许可证 | 结论 |
|---|---:|---|---|
| [Wei-Shaw/Sub2API](https://github.com/Wei-Shaw/sub2api) | [`v0.1.179` / `75f88be`](https://github.com/Wei-Shaw/sub2api/tree/75f88be5f75c27771836b586f7de1503afa0e3bc)、[`main` / `e866ff6`](https://github.com/Wei-Shaw/sub2api/tree/e866ff6ec431816e8b9d4b81dc7b00122ca3f7f8) | LGPL-3.0 | 直接吸收基线 |
| [QuantumNous/New API](https://github.com/QuantumNous/new-api) | [`e468b73`](https://github.com/QuantumNous/new-api/tree/e468b73915e5028e9849de62c5018a0faa203012) | AGPL-3.0 | 只参考设计/测试，禁止直接复制 |
| [BerriAI/LiteLLM](https://github.com/BerriAI/litellm) | [`02c1c45`](https://github.com/BerriAI/litellm/tree/02c1c45b53428b3fe55bce42a7b51060a448caa2) | 非 enterprise 目录 MIT | 参考显式路由、有状态会话和测试 |
| [vLLM](https://github.com/vllm-project/vllm) | [`c01b50e`](https://github.com/vllm-project/vllm/tree/c01b50e390e6d3d0019aa53f41ff1198c8105e5a) | Apache-2.0 | 参考事件状态机和 Anthropic 适配 |
| [OpenRouter Open Responses](https://github.com/OpenRouterTeam/skills/tree/012c823da319ef10ee899a64aacd10e83ce39c74/open-responses) | [`012c823`](https://github.com/OpenRouterTeam/skills/tree/012c823da319ef10ee899a64aacd10e83ce39c74/open-responses) | 仓库未声明通用许可证 | 只作为官方协议/验收参考 |
| [OpenAI Go SDK](https://github.com/openai/openai-go) | [`4d06294`](https://github.com/openai/openai-go/tree/4d062949c62507e56514af8c7beb186dc09ac075) | Apache-2.0 | 严格 wire schema 参照 |

## 1. Sub2API：直接吸收基线

### 1.1 正式版 v0.1.179 已有的能力

[v0.1.179 官方发布说明](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.179)明确说明：Kimi、智谱 GLM、DeepSeek API Key 账号可选 adaptive，同一账号承接 Chat Completions、Anthropic Messages、OpenAI Responses，并优先走供应商原生端点。

核心文件：

- [Responses→Chat 请求、Chat→Responses 响应/流转换](https://github.com/Wei-Shaw/sub2api/blob/75f88be5f75c27771836b586f7de1503afa0e3bc/backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go)
- [Responses→Chat fallback 服务路径](https://github.com/Wei-Shaw/sub2api/blob/75f88be5f75c27771836b586f7de1503afa0e3bc/backend/internal/service/openai_gateway_responses_chat_fallback.go)
- [fallback 服务行为测试](https://github.com/Wei-Shaw/sub2api/blob/75f88be5f75c27771836b586f7de1503afa0e3bc/backend/internal/service/openai_gateway_responses_chat_fallback_test.go)
- [custom/tool_search/namespace 测试](https://github.com/Wei-Shaw/sub2api/blob/75f88be5f75c27771836b586f7de1503afa0e3bc/backend/internal/pkg/apicompat/chatcompletions_responses_bridge_custom_tools_test.go)

正式版已经覆盖：

- `instructions`、string/array `input`、多模态 content part；
- function/custom/tool_search/namespace 工具降级与回程恢复；
- tool call/output 对齐和并行工具调用；
- structured output、`tool_choice`、reasoning、service tier、usage；
- 非流式与流式 Chat→Responses；
- reasoning item ID→明文缓存回注，解决 DeepSeek thinking 历史必须带 `reasoning_content` 的问题（[commit `612436a`](https://github.com/Wei-Shaw/sub2api/commit/612436a5a7cb) 已包含在该 tag）。

### 1.2 v0.1.179 之后 main 的关键修复

以下不能只停留在正式 tag，必须跟进 `main`：

| 修复 | main 一级来源 | 为什么必须吸收 |
|---|---|---|
| 合成 Responses 补 `created_at` | [commit `3c5553e`](https://github.com/Wei-Shaw/sub2api/commit/3c5553e253ac) | OpenAI/Rust 严格客户端会把 `created_at` 当必填；缺失会直接反序列化失败。OpenAI 官方 Go SDK也把它标为 required（[schema](https://github.com/openai/openai-go/blob/4d062949c62507e56514af8c7beb186dc09ac075/responses/response.go)）。 |
| malformed tool arguments fail，不再下发坏 JSON | [commit `e2d9ce0`](https://github.com/Wei-Shaw/sub2api/commit/e2d9ce0cadad)、[收窄修复 `fbc9ee6`](https://github.com/Wei-Shaw/sub2api/commit/fbc9ee626d72) | 工具参数若在 `.done`/终态仍不是合法 JSON，Codex 会在执行端爆炸；必须在网关边界拒绝。 |
| 恢复 namespaced custom tool aliases | [commit `31d5b67`](https://github.com/Wei-Shaw/sub2api/commit/31d5b67baa4f) | namespace 子工具摊平后，回程必须恢复 namespace+原名，否则 Codex 认为 unsupported call。 |
| 流式终态由已上报 item 重建 | [commit `243921d`](https://github.com/Wei-Shaw/sub2api/commit/243921dc0a95) | 终态 `response.completed.response.output` 必须和之前 emitted item 一致，不能靠脆弱的最后一个 chunk 猜。 |
| 流式空 ID/name 和首 chunk 参数重复修复 | [commit `cc894ef`](https://github.com/Wei-Shaw/sub2api/commit/cc894ef57871)、[既有 regression `29122e3`](https://github.com/Wei-Shaw/sub2api/commit/29122e30514f) | GLM/部分兼容上游会在一个 chunk 同时给 ID、name、arguments，或后续 chunk 省略字段；处理错误会重复 JSON 或生成空工具名。 |
| cross-provider reasoning replay 归一化 | [commit `32064d3`](https://github.com/Wei-Shaw/sub2api/commit/32064d39e70b) | reasoning 在不同供应商使用 `reasoning`/`reasoning_content` 等别名，重放前需归一。 |

`main` 当前成熟文件：

- [converter](https://github.com/Wei-Shaw/sub2api/blob/e866ff6ec431816e8b9d4b81dc7b00122ca3f7f8/backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go)
- [fallback service](https://github.com/Wei-Shaw/sub2api/blob/e866ff6ec431816e8b9d4b81dc7b00122ca3f7f8/backend/internal/service/openai_gateway_responses_chat_fallback.go)
- [stream lifecycle tests](https://github.com/Wei-Shaw/sub2api/blob/e866ff6ec431816e8b9d4b81dc7b00122ca3f7f8/backend/internal/pkg/apicompat/chatcompletions_responses_stream_lifecycle_test.go)
- [request invariant tests](https://github.com/Wei-Shaw/sub2api/blob/e866ff6ec431816e8b9d4b81dc7b00122ca3f7f8/backend/internal/pkg/apicompat/chatcompletions_responses_request_invariants_test.go)

### 1.3 Sub2API 中特别重要的实现约束

- 每个 assistant `tool_calls` 消息必须紧跟一条对应每个 `tool_call_id` 的 tool 消息；未回答的并行 sibling、dangling call、orphan output 要整理或丢弃，不能把无效历史发给 DeepSeek/Anthropic。
- 流式工具必须按 index 累积 ID/name/arguments，直到 `function_call_arguments.done` 和 `output_item.done` 才算关闭。
- usage 从上游 Chat 的权威 usage 映射：`prompt_tokens→input_tokens`、`completion_tokens→output_tokens`，保留 cached/cache-write/reasoning token details；缺失只能估算并标明来源，不能假装是上游账单。
- 客户端断连后仍需 drain 上游以获得 usage、正确释放并发并完成记账；不能把“客户端断开”当上游请求已取消。

## 2. New API：设计参考，不直接复制

New API 把转换从 handler 中抽成独立 `relaykit`：

- [转换注册表](https://github.com/QuantumNous/new-api/blob/e468b73915e5028e9849de62c5018a0faa203012/relaykit/relayconvert/text_converter_registry.go)为每条 from→to 路线声明 `good/fair/discouraged`，请求和响应必须成对注册；Responses↔Chat 是 `good`，Claude↔Responses 是 `fair`，需经两跳的 Claude↔Gemini 是 `discouraged`。
- [Responses→Chat 请求转换](https://github.com/QuantumNous/new-api/blob/e468b73915e5028e9849de62c5018a0faa203012/relaykit/relayconvert/internal/oai_responses/to_oai_chat_req.go)显式拒绝 `conversation`、`previous_response_id`、`prompt`、`context_management`，并转换 messages/tools/tool_choice/text.format/penalties/reasoning/service tier。
- [Chat→Responses 流状态机](https://github.com/QuantumNous/new-api/blob/e468b73915e5028e9849de62c5018a0faa203012/relaykit/relayconvert/internal/oai_chat/to_oai_responses_stream_resp.go)分别维护 text、reasoning、多个 tool index，先 added、再 delta、最后 done 和 completed/incomplete。
- [真实 handler](https://github.com/QuantumNous/new-api/blob/e468b73915e5028e9849de62c5018a0faa203012/relay/channel/openai/responses_via_chat.go)验证上游 error、流式解析失败、usage 缺失估算和终态 finalize。
- [golden matrix](https://github.com/QuantumNous/new-api/tree/e468b73915e5028e9849de62c5018a0faa203012/relaykit/relayconvert/testdata/golden)对 request/response/stream 的 Claude、Gemini、Chat、Responses 组合做固定样本。

**吸收方式：** 采用 registry/quality/golden matrix 思想；不要复制 AGPL 源码。当前 PR 无需为一次阿里接入重构整个转换 registry，但后续多协议扩展应把转换器从 gateway service 解耦。

## 3. LiteLLM：显式路由和有状态桥

LiteLLM 的核心判断非常清晰：

- [dispatch](https://github.com/BerriAI/litellm/blob/02c1c45b53428b3fe55bce42a7b51060a448caa2/litellm/responses/main.py)在 provider 没有 native Responses config，或明确 `use_chat_completions_api=true` 时，才进入 Chat bridge；还提供 `openai/chat_completions/<model>` 前缀作为显式选择。
- [Responses→Chat request](https://github.com/BerriAI/litellm/blob/02c1c45b53428b3fe55bce42a7b51060a448caa2/litellm/responses/litellm_completion_transformation/transformation.py)转换 tools/tool_choice/parallel calls，并强制 streaming Chat 请求 `include_usage=true`。
- [Chat→Responses streaming iterator](https://github.com/BerriAI/litellm/blob/02c1c45b53428b3fe55bce42a7b51060a448caa2/litellm/responses/litellm_completion_transformation/streaming_iterator.py)给每个 tool call 分配稳定 output index，处理无 ID chunk、并行 call、完整事件和 terminal usage。
- [previous_response_id session handler](https://github.com/BerriAI/litellm/blob/02c1c45b53428b3fe55bce42a7b51060a448caa2/litellm/responses/litellm_completion_transformation/session_handler.py)通过持久 spend/session 记录重建历史；这说明 stateful bridge 是一项单独的存储能力，不是简单字段改名。
- [bridge flag tests](https://github.com/BerriAI/litellm/blob/02c1c45b53428b3fe55bce42a7b51060a448caa2/tests/test_litellm/responses/test_responses_api_bridge_flag.py)锁定显式开关不会泄漏给上游，并覆盖 native/bridge 分界。

**对我们的启示：** `openai_upstream_protocol_by_model` 比全账号布尔值更准确；先按配置选 native/bridge，不要在客户请求热路径上反复探测。`previous_response_id` 在我们没有 durable session store 前必须 fail-closed。

## 4. vLLM：事件和 Anthropic 参考

vLLM 的 Responses 与 Anthropic 是原生服务入口，不是“把请求转给另一个兼容上游”的代理，因此不应整段移植：

- [Responses serving](https://github.com/vllm-project/vllm/blob/c01b50e390e6d3d0019aa53f41ff1198c8105e5a/vllm/entrypoints/openai/responses/serving.py)自行解析 reasoning/content/tool calls、执行内部工具并生成 usage。
- [Responses streaming state machine](https://github.com/vllm-project/vllm/blob/c01b50e390e6d3d0019aa53f41ff1198c8105e5a/vllm/entrypoints/openai/responses/streaming_events.py)先把一个同时含 reasoning/content/tools 的 compound delta 拆分，再按状态 transition；并按 tool index 区分连续和并行调用。
- [function stream tests](https://github.com/vllm-project/vllm/blob/c01b50e390e6d3d0019aa53f41ff1198c8105e5a/tests/entrypoints/openai/responses/test_function_call.py)检查 added→arguments delta/done→item done→completed 的成对生命周期。
- [Anthropic Messages serving](https://github.com/vllm-project/vllm/blob/c01b50e390e6d3d0019aa53f41ff1198c8105e5a/vllm/entrypoints/anthropic/serving.py)把 system/text/image/thinking/tool_use/tool_result 转成内部 Chat 请求，再把 Chat stream 转回 `message_start/content_block_*/message_delta/message_stop`。
- [Anthropic conversion tests](https://github.com/vllm-project/vllm/blob/c01b50e390e6d3d0019aa53f41ff1198c8105e5a/tests/entrypoints/anthropic/test_anthropic_messages_conversion.py)覆盖 tool_result 图片提升、reasoning、tool args 尾片、cache-read/cache-write usage。

**吸收方式：** 只吸收复合 delta 拆分、工具 index 状态机、Anthropic tool_result/usage 用例；不替换 Sub2API gateway 的实际路由和计费逻辑。

## 5. OpenRouter/OpenAI：wire-level 验收合同

OpenRouter 官方 [协议和 items](https://github.com/OpenRouterTeam/skills/blob/012c823da319ef10ee899a64aacd10e83ce39c74/open-responses/references/protocol-and-items.md)与[状态机/流式事件](https://github.com/OpenRouterTeam/skills/blob/012c823da319ef10ee899a64aacd10e83ce39c74/open-responses/references/state-machines-and-streaming.md)要求：

- SSE `event:` 必须等于 JSON `type`；
- `sequence_number` 单调递增；OpenAI 官方 Go SDK对大量 Responses event 都标记为 required（[源码](https://github.com/openai/openai-go/blob/4d062949c62507e56514af8c7beb186dc09ac075/responses/response.go)）；
- message 应有 output_item added、content part added、text delta/done、content part done、output_item done；
- function call 应有 output_item added、arguments delta/done、output_item done；
- item incomplete 时 response 也必须 incomplete；失败要发 `response.failed`，不能发伪造的 completed；
- terminal response 才携带权威完整 output 和 usage。

OpenRouter 的生产代理实现未开源，本次没有可“搞过来”的代理代码；协议文档只能作为合同和测试 oracle。

## 6. 吸收 / 参考 / 拒绝矩阵

| 能力 | 决策 | 来源 | PR #262 要求 |
|---|---|---|---|
| Responses↔Chat request/response/stream 核心 | **直接吸收** | Sub2API main | 使用成熟 converter，不维护初版简化分叉 |
| v0.1.179 adaptive API Key fallback | **直接吸收并按现架构改造** | Sub2API v0.1.179 | 复用 credential/proxy/header/usage/failover/concurrency，而不是另造旁路 |
| `created_at`、`sequence_number`、完整 lifecycle | **直接吸收** | Sub2API main + OpenAI SDK/Open Responses | 严格客户端必须通过 |
| tool arguments 终态 JSON 校验 | **直接吸收，fail-closed** | Sub2API main | 坏参数不得下发或标 completed |
| custom/function/tool_search/namespace 映射 | **直接吸收** | Sub2API main | 请求降级和回程类型/别名必须对称 |
| reasoning item cache | **直接吸收并加强隔离** | Sub2API v0.1.179 | 建议按 user/API key/model/item ID scope，而非只按 item ID |
| 模型级协议选择 | **直接保留我们的实现** | LiteLLM 显式路由思想 | 同一 Key 的不同模型可分别 native Responses 或 Chat |
| converter registry + quality | **后续参考** | New API relaykit | 此 PR 不做大重构；后续多协议平台化 |
| golden cross-protocol matrix | **吸收测试思想** | New API | 增加 Responses/Chat/Messages 的 request/response/stream fixtures |
| compound delta 拆分/工具 index state | **参考并补测试** | vLLM | reasoning+text+tools 同 chunk、并行工具、无 ID delta |
| `previous_response_id`/conversation/prompt/context management | **拒绝桥接** | LiteLLM 表明需 session store；New API也拒绝 | 路由到 native Responses 或 4xx 明确报错 |
| web_search/file_search/code_interpreter/image generation 等服务端工具 | **拒绝桥接** | Chat 没有等价执行语义 | 不能静默 drop；必须 native Responses 或专门的本地执行器 |
| encrypted-only reasoning 且缓存 miss | **拒绝桥接** | Sub2API reasoning cache问题 | 不能丢 reasoning 后继续；明确报错/native route |
| 未知 input/content/output item | **拒绝或原生路由** | 所有实现的边界共同结论 | 不得静默字符串化后声称等价 |
| 直接复制 New API 代码 | **拒绝** | AGPL-3.0 | 许可证不兼容风险，只参考设计 |

## 7. PR #262 当前差距与剩余风险

基于当前 worktree 的未提交吸收状态：converter 已几乎等同 Sub2API main，并额外有 `ResponsesEventToSSE`；服务层加入了按模型协议选择、stateful/server-tool/encrypted-reasoning fail-closed 和更细的 reasoning cache scope。方向正确。合并前仍需锁死以下风险：

### P0：必须通过

1. **流被截断不能伪造 `response.completed`。** 当前 SSE adapter 若 EOF、没有 `[DONE]` 或没有 terminal finish/usage，需要判定失败；至少在首输出前触发 failover，已输出后应结束为 failed/incomplete，不能 finalize 成 completed。Sub2API main 的共享 `scanCCStream` 会收集 usage、SawDone、首 token 和读取错误；我们的独立 pipe adapter需达到同等行为。
2. **所有合成事件必须有 required wire 字段。** `created_at`、`sequence_number`、`item_id`、`output_index`、`content_index`、tool done `arguments`、text done `text`，以及 SSE `event==type`。
3. **错误路径不得把上游 Chat 错误 JSON 当成功 SSE。** 覆盖 HTTP 4xx/5xx、200 内 error、SSE error frame、malformed JSON、超长行、unexpected EOF。
4. **usage/计费只能取权威终态。** 覆盖 cache read/write、reasoning tokens、空 choices 的 usage chunk；客户端断开后继续 drain 并释放账号并发。
5. **真实三协议 E2E。** 对同一个 Chat-only 阿里模型分别从 `/v1/responses`、`/v1/chat/completions`、`/v1/messages` 发文本和函数工具请求；不仅检查 HTTP 200，还要让 OpenAI/Anthropic 官方 SDK真正反序列化并完成下一轮 tool result。

### P1：本 PR 或紧随修复

- refusal、annotations、audio 等 Chat/Responses 非对称字段尚未证明可保真；遇到即 fail-closed 或列为不支持。
- 文本+reasoning+多个并行工具出现在同一 chunk 的顺序需按 vLLM state-machine 用例锁定。
- 命名空间摊平存在长度、hash 和撞名边界，必须保留 Sub2API 的 collision rejection。
- reasoning cache 需验证 Redis 多实例、TTL、用户/API key/model 隔离和断连后写回。
- 原生 Responses 模型与 Chat-only 模型混在同一账号时，要证明 per-model override 优先于旧的 account-level `openai_responses_supported`，未配置模型保持向后兼容。

## 8. 最终建议

本次不要重新发明协议，也不要把 New API/LiteLLM 整包引入。最稳妥路径是：

1. 以 **Sub2API `v0.1.179` 正式 fallback** 为服务集成基线；
2. 把 **Sub2API main 的 post-tag converter 修复**全部吸收；
3. 保留我们更严格的 **per-model route + fail-closed + cache scope**；
4. 用 **OpenAI SDK/Open Responses** 做 wire 合规 oracle；
5. 用 **New API golden matrix、LiteLLM route/session、vLLM stream/tool 测试**扩充回归集；
6. 上述 P0 全绿后再处理账号、模型分组、倍率与生产部署。

这能最大化复用已被大量兼容上游和 Codex/Claude 客户端踩过坑的成熟代码，同时避免静默语义损失和许可证风险。

## 9. 本地吸收状态与所有者真实链路证据

当前分支已完成以下本地重写；这些变更只在 PR/worktree，未合并、未部署：

- converter 替换为 Sub2API `main@e866ff6` 成熟核心，并带入其 custom/tool_search/namespace、reasoning、多工具、usage/cache details、严格事件生命周期测试；
- 保留本地按模型协议矩阵，且把相关 extra 字段加入 scheduler Redis snapshot 白名单，避免生产调度快照静默丢配置；
- reasoning 缓存改为 `user_id + API key id + model + reasoning item id` scope；encrypted-only cache miss 明确拒绝；
- `previous_response_id`、`conversation`、`prompt`、`context_management` 和无等价语义的 server tools 明确拒绝，不静默 drop；
- Chat SSE 只有同时拿到 `finish_reason` 与权威 usage 才允许合成终态；scanner error、clean truncation、malformed tool arguments 不得伪造 `response.completed`；
- 客户端断连后继续 drain 上游，保留 usage/cache-read/cache-write 后再结束本次计费。

### 阿里云北京 Workspace 所有者测试

测试不是从本机旁路直接请求：Linux/amd64 test binary 通过
`/root/upstream-bench/scripts/run_in_production_netns.sh` 进入当前生产
`laoshirenai-app` 容器的 network namespace，并绑定容器 resolver；因此 DNS、
公网出口和服务器→阿里云链路与生产网关一致。上游 Key 仅由 Agent Switch 私有 FD
经 SSH stdin 注入进程环境，未写入服务器、仓库或日志；测试二进制已从本机和服务器清理。

四个 Chat-only 模型均完成并通过：

| 模型 | Responses 非流文本 | Responses 流文本 | Responses 流式强制函数工具 | Messages 非流文本 |
|---|---:|---:|---:|---:|
| `ZHIPU/GLM-5.3` | PASS | PASS | PASS | PASS |
| `MiniMax/MiniMax-M3` | PASS | PASS | PASS | PASS |
| `kimi-k3` | PASS | PASS | PASS | PASS |
| `kimi-k2.7-code` | PASS | PASS | PASS | PASS |

同一测试还对十个 native Responses 模型逐个检查：请求实际落到
`/compatible-mode/v1/responses`，状态 completed，输出包含预期文本且 input/output
usage 非零。一次全矩阵中 `glm-5.2` 出现单次 usage=0；立即从同一生产 network
namespace 重跑 1 次和连续 3 次均通过。该现象没有发生在 bridge 模型，但属于上游
原生 Responses usage 的瞬时风险，最终 CI/验收不能把它隐去。
