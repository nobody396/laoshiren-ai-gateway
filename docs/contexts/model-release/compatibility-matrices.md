# 模型发布九矩阵合同

本合同把“一个模型能否在某个工具里使用”拆成九个独立、可审计的事实表。机器格式由
`model-doc-contracts/matrix-schema.json` 定义。矩阵 loader 必须按每张表的
`x-primary-key` 拒绝重复行；不能以数组 `uniqueItems` 代替复合主键校验。

## 九个且仅九个源矩阵

| 矩阵 | 主键 | 回答的问题 |
| --- | --- | --- |
| `public_model` | `model_id` | 哪些模型是 draft/public/hidden/deprecated；只有 public 扩张发布测试 |
| `model_protocol` | `model_id + protocol` | 模型在哪个协议上真的可用、协议内每项 feature 是否通过、推荐哪个协议以及为什么 |
| `model_reasoning` | `model_id + reasoning_level` | 模型原生推理档位及每种协议的 wire 编码 |
| `client_protocol` | `client_id + client_version + protocol` | 某版本客户端原生支持哪些协议和协议 features |
| `client_reasoning` | `client_id + client_version + protocol + control_level` | 客户端能表达哪些控制档位，如何映射到 canonical level/wire value |
| `group_access` | `group_id + model_id + protocol` | 一个分组的 Key 是否真的能调用该模型/协议、Base URL 和倍率是什么 |
| `client_config_os` | `client_id + client_version + os + architecture` | 不同 OS 的配置文件、owned fields、合并策略和验证命令 |
| `test_evidence` | `evidence_id` | 哪个版本、OS、时间、目标得到 pass/fail/blocked/expired 的可复核证据 |
| `model_price` | `model_id + price_book_id + tier_id + effective_from` | 币种、每百万 token 输入/缓存输入/输出价格和长上下文规则 |

`protocol features` 是 `model_protocol.features`（客户端对应能力在
`client_protocol.features`），不再另建第十张表。推荐协议不能只存一个布尔值；每个协议行都必须写
`recommendation` 和 `recommendation_reason`。例如：Responses 基础调用通过但原生 Web Search
不通过，所以 GLM/Minimax/Kimi 的 Chat Completions 可以标为 preferred，并保留可审计理由。

## 状态和证据

能力单元格统一使用：

- `unknown`：尚无结论；
- `planned`：已进入测试计划；
- `verified`：有当前版本的正向证据；
- `unsupported`：有真实负向证据；
- `blocked`：受外部条件阻挡，不等于 unsupported；
- `stale`：版本、配置或有效期变化，发布前必须重测。

只有 `verified` 和有负向证据的 `unsupported` 是稳定事实。证据集中保存在
`test_evidence`，其他矩阵只引用 `evidence_ids`。证据必须包含观察时间、目标、客户端/模型/网关/OS
版本、不可变 artifact URI 与 SHA-256；不得包含 Key 或 secret。客户端版本变化只使该客户端相关格子
stale；模型版本、网关协议实现或分组绑定变化也只失效受影响格子。

## Effective import 公式

对选定 `group = g`、客户端版本 `c@v`、OS `o`，可导入模型/协议集合为：

```text
Eligible(g,c,v,o) =
  PUBLIC(public_model)
  ⋈ VERIFIED(model_protocol)
  ⋈ VERIFIED(client_protocol[c,v])
  ⋈ VERIFIED(group_access[g])
  ⋈ VERIFIED(client_config_os[c,v,o])
```

所有连接均使用精确 `model_id`/`protocol`，不能以同家族、别名或价格表存在代替。

有效协议 features 是模型和客户端在该协议上 `verified` feature 的交集：

```text
EffectiveFeatures(m,p,c,v) =
  VerifiedFeatures(model_protocol[m,p])
  ∩ VerifiedFeatures(client_protocol[c,v,p])
```

有效推理档位不是裸集合交集。先把客户端 control 经过已声明的 `mapping_strategy` 映射成
canonical level，再与模型的 verified canonical level 相交，并要求双方 protocol wire encoding 可用：

```text
EffectiveReasoning(m,p,c,v) = {
  (control -> canonical -> wire)
  | client_reasoning[c,v,p,control] is verified
  ∧ model_reasoning[m,canonical] is verified for p
}
```

`floor/ceiling/alias/omit/reject` 必须显式记录；不允许 UI 猜测降档。Ultracode 一类客户端工作流是
mode，不冒充模型原生档位。

协议推荐从 `Eligible` 中选择 `recommendation=preferred` 的行；如果不存在或其必要 feature 不满足，
就停止自动导入并产生缺失测试，不能静默选择另一个协议。

`model_price` 不参与协议兼容性真假判断，但所有 public 模型必须有当前有效价格。展示和计费采用匹配
scope/group/effective date 的价格行：

```text
EffectivePrice = exact group_customer price
              ?? gateway_base price × group_access.billing_multiplier
```

价格必须带 `currency` 和 `unit_tokens=1000000`。当请求 token 超过
`long_context.threshold_tokens` 时，按明确的 input/cached-input/output multiplier 分别计算；`null`
表示官方没有该价格项，绝不能解释成 0。

## Provider Contract Test 计划

先展开候选笛卡尔积，再减去仍在有效期内且版本完全匹配的 pass/fail 证据：

```text
RequiredTests =
  EligibleCandidate(model, protocol, group, client@version, OS)
  × RequiredProtocolFeatures
  × EffectiveReasoningControls
  - FreshExactEvidence(test_evidence)
```

`RequiredProtocolFeatures` 至少覆盖：Chat Completions/Responses 基础请求、SSE/终止帧、tool call、
tool-result 续轮、reasoning、prompt cache、图片输入、上下文边界、usage、实际计费、错误透传、断流、
超时和重试。真实客户端兼容结论还必须有一次完整 Agent 工具循环，而非仅 HTTP 200。

证据复用必须同时匹配 model ID、group、Base URL、protocol、feature、reasoning level、client
major/minor version、OS、gateway/provider model version 和预期结果。任一维度变化就只重开对应格子。

## 增量扩张规则

### 新模型

1. 先写 `public_model` draft 行。
2. 填 `model_protocol`（含所有 features 与推荐理由）、`model_reasoning`、`model_price`。
3. 填目标 `group_access`，确认 Key 实际可见与倍率，不能从价格表推断。
4. 与所有当前 `client_protocol`、`client_reasoning`、`client_config_os` 行连接，自动生成缺失的
   Provider Contract Test 格子。
5. 所有发布门槛得到新鲜终态证据后，才把 publication_status 改为 public。

新增模型不改客户端事实表；只扩张该模型与现有客户端的候选测试。

### 新客户端或客户端新版本

1. 按实测版本写 `client_protocol` 与 `client_reasoning`。
2. 每个承诺支持的 OS/architecture 写 `client_config_os`，包括配置、owned fields、保留字段和验证命令。
3. 与所有 public model 的 verified model protocol/group access 连接，自动生成缺失测试。
4. 旧版证据不会自动证明新版；仅受该客户端版本影响的格子标 stale，其他模型规格/价格证据继续复用。

新增客户端不改模型能力事实表；只扩张该客户端与现有公开模型的候选测试。

## 发布门槛

自动导入/公开卡片必须同时满足：模型公开、精确分组可访问、模型与客户端协议都 verified、必要
features verified、推理映射完整、目标 OS 配置 verified、当前价格有效、真实客户端 Agent 循环和网关
E2E 有新鲜证据。`planned`、`blocked`、`stale` 或缺失行一律输出测试计划，不能渲染成“支持”。
