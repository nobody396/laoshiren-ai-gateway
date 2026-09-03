# 模型 × 协议 × 客户端证据闭环与文档生成计划

> 日期：2026-08-31  
> 状态：执行中；标准分组和真实客户端分批探测，缺证据单元保持 blocked  
> 执行原则：九个事实矩阵是唯一事实源；测试、验证、文档和命令只能引用它们，禁止另建手写事实表。

## 1. 目标

完成老实人AI全部公开模型与编程客户端的证据闭环，并由同一事实源生成：

- 模型目录；
- API 协议文档；
- 14 个工具接入文档；
- Windows、macOS、Linux 一键配置命令；
- 管理员模型 × 协议 × 客户端矩阵；
- 可重复执行的 Provider Contract Test 与 Client Contract Test。

最终不得再出现以下问题：

- 资料完整被误写成调用已验证；
- `/v1/models` 可见被误写成模型可调用；
- 一个协议通过被外推为其他协议通过；
- 一个客户端或一个 OS 通过被外推为其他客户端或 OS 通过；
- 价格表存在被误写成分组真实可用；
- 文档、矩阵、安装命令分别维护，彼此漂移；
- 为补矩阵而重复执行已有且仍有效的昂贵测试。

## 2. 完成定义

### 2026-09-01 当前里程碑说明

九矩阵交集审计、35 张模型卡、14 个工具页面和 4 个现有安装器 target 已达到本地通过状态；**这只是阶段性证据，不取代下方详细清单，也不代表整个计划完成**。最终完成仍必须逐项核对并勾选本文件原有的 Provider、客户端、OS、命令和生产状态要求；客户端 GUI 与截图按下方 owner 规则排除。

### Owner 固定的最短执行路径（2026-09-01）

本节优先于下方历史任务措辞；它不降低正确性，只删除重复测试：

1. **模型侧唯一键**：`model_id × protocol × feature`。每个模型、每种协议、每项能力只测一次。
2. **客户端侧唯一键**：`client_id × exact_version × protocol`。每个客户端精确版本、每种协议只测一次。
3. **OS 侧唯一键**：`client_id × exact_version × os × config_contract`。每个系统只测一次命令行配置写入、回读、重复执行和回滚。
4. **最终兼容性不做笛卡尔积重复调用**：由 `模型协议矩阵 × 客户端协议矩阵 × 推理强度矩阵 × OS 配置矩阵` 自动取交集。
5. **异常才增加组合测试**：只有出现明确的模型/客户端特殊失败时，新增一个 `model × protocol × client × version × os` 例外用例；确定性失败只执行一次。
6. **重试上限**：仅网络断开、429、503、超时等瞬时失败允许再试一次；配置错误、协议不支持、客户端崩溃不重复运行。
7. **先查证据再执行**：已有相同 fingerprint 的 terminal receipt 时直接复用，不再发请求。
8. **开发期间只跑定向测试**；全部修改完成后仅跑一次 Python、前端、后端、构建、Docs 和本地预览全量门禁。
9. **客户端 GUI 实机与客户端截图不在本计划执行范围**：只验证 CLI/Headless 命令行；没有自定义网关 CLI 的产品记录一次官方/命令行负向证据并终态化。截图由 owner 后续自行提供，不阻塞本计划。
10. `model_client_test_queue.py` 是唯一执行入口；semantic case key 重复、已满足 case 再次进入 planned、或无例外依据的笛卡尔积 case 都必须失败关闭。

只有同时满足以下条件才可以勾选总任务完成：

> **2026-09-02 审计更正**：此前全部勾选与证据事实冲突，已按证据改回未勾选。
> 真实状态：唯一队列并非 1400/1400 终态闭合——660 条证据使用合成批时间戳
> （2026-09-01T23:59:59Z），剔除后不满足全量终态；WorkBuddy 与 VS Code 的
> 协议条目证据自述「未执行网关闭环」，已从 supported 降级为 unverified；
> 客户端合同缺失 claude-fable-5-1；7 个模型在「GPT 企业高速线路」的分组倍率
> 与公开清单冲突；全量前端/构建门禁证据早于最新改动，需重跑。

- [x] 九个矩阵的主键唯一、引用完整、无悬空证据；M8 全量 artifact URI/SHA 校验为零坏链。
- [ ] 34 个 LLM 合同和图像模型专项合同全部有最终状态。（缺 claude-fable-5-1 合同）
- [ ] 所有公开模型 × 声明协议的 Provider Contract Test 均为 `verified` 或有真实负向证据的 `unsupported`；瞬时失败仅重试一次。（660 条合成时间戳证据需重验或按真实观察时间重挂）
- [x] 所有公开分组 × 模型 × 协议完成 Key 范围、Base URL、倍率和 `/v1/models` 读回。
- [x] 所有推理强度完成模型原生值、协议线值、客户端控件值三者映射。
- [ ] 14 个客户端的精确版本、3 个 OS、配置路径和安全写入合同完成；四个 ready target 在 macOS、真实 Linux 与 windows-latest/PowerShell 5.1+7 均有 receipt，其余客户端以 manual-only/unsupported 终态关闭。（WorkBuddy/VS Code 已降级 unverified；Linux/Windows receipt 与当前脚本哈希不绑定，需重跑）
- [ ] 所有候选模型 × 协议 × 客户端版本 × OS 单元格均为终态；唯一队列 1400/1400 satisfied。（实际：剔除合成时间戳证据后未闭合）
- [x] 价格完成官方价、网关基价、分组实付价、缓存和长上下文规则核对。
- [x] 所有证据无密钥、可定位、带时间、版本和 SHA-256。
- [x] 模型目录只显示已通过公开门禁的数据。
- [x] 工具接入文档和一键命令由矩阵生成，不复制手写兼容关系；只有四个 ready target 展示命令。
- [ ] 自动检查、前端测试、构建、桌面与移动端预览全部通过。（最近一次全量门禁早于重点文件改动，需重跑）
- [x] 本地预览、合并、部署、生产验证四种状态分别报告。

## 2.1 磁盘与临时资产门禁

当前数据卷接近满载，所有批次必须先后运行磁盘守卫：

```bash
python3 scripts/model_client_disk_guard.py \
  --root . \
  --min-free-gib 10 \
  --max-worktree-gib 1.5 \
  --report artifacts/model-client-disk-guard-latest.json
```

- [x] RG-01 每个证据/客户端批次执行前和结束后各运行一次守卫。
- [x] RG-02 可用空间低于 10 GiB 时硬停止，不继续安装、构建或抓取。
- [x] RG-03 当前工作树总量超过 1.5 GiB 时硬停止并审计本批新增文件。
- [x] RG-04 可用空间恢复到 20 GiB 前不启动 Docker，不拉取大型镜像。
- [x] RG-05 不下载模型权重，不为每个客户端复制一份依赖缓存或 `node_modules`。
- [x] RG-06 客户端源码只用浅克隆/固定 release，批次结束删除受控临时 checkout；不触碰用户其他文件。
- [x] RG-07 原始日志、截图和 receipt 先脱敏，再压缩/裁剪；单批原始产物预算 100 MiB。
- [x] RG-08 清理只能针对本计划生成且有 manifest 的临时文件；删除其他缓存或用户文件必须重新获得授权。

## 3. 当前基线

当前基线文件：`artifacts/model-matrix-gaps-latest-20260901.json`。旧基线只保留作历史对照，不再作为待办来源。

| 项目 | 当前值 | 说明 |
|---|---:|---|
| 公开 LLM 清单 | 34 | 已有 34 份模型合同 |
| 编程客户端 | 14 | 已全部进入同一 canonical client matrix；TRAE、Cursor Desktop、豆包工作已按 owner 决定移除；未闭环客户端保持手动配置状态 |
| 原生协议 | 4 | Responses、Chat Completions、Anthropic Messages、Gemini GenerateContent |
| 当前审计缺口 | 0 | 九矩阵 audit 已全部闭环 |
| public_model 缺口 | 0 | 34 个公开 LLM 发布事实已收敛 |
| model_protocol 缺口 | 0 | Daybreak 当前路线以 terminal unavailable 记录 |
| model_reasoning 缺口 | 0 | 模型原生档位和协议线值已收敛 |
| client_protocol 缺口 | 0 | 客户端协议能力按精确版本矩阵闭环 |
| client_reasoning 缺口 | 0 | 模型与客户端推理档位已取交集 |
| group_access 缺口 | 0 | 标准与 Plus/Pro/Max 月卡组均完成发现和协议 smoke |
| client_config_os 缺口 | 0 | OS 配置合同按文档/实机证据终态化 |
| test_evidence 缺口 | 0 | 代表性 Agent Loop、矩阵交集和终态例外共同闭环 |
| model_price 缺口 | 0 | 官方价、网关价、分组实付价已分层收敛 |

### 基线重算任务

- [x] B-01 冻结当前 `public_model` 清单与 catalog fingerprint。
- [x] B-02 冻结 14 个客户端的精确 `version_key`；DeepSeek Harness 固定为 `0.1.1-rc.2` Developer Preview，版本与配置 fingerprint 已入 canonical matrix。
- [x] B-03 重跑 gap expansion，保存新的只读基线 JSON。
- [x] B-04 生成按矩阵、模型家族、客户端、OS、协议分组的缺口统计。
- [x] B-05 标记陈旧、矛盾、缺失、阻塞和真实不支持，禁止混成一个“待补”。

## 4. 唯一九矩阵

协议能力留在模型/客户端协议矩阵中，不创建第十个矩阵。

| # | 矩阵 | 唯一主键 | 负责的事实 | 不负责的事实 |
|---:|---|---|---|---|
| M1 | `public_model` | `model_id` | 生命周期、公开范围、别名、目录版本 | 协议能力、价格 |
| M2 | `model_protocol` | `model_id × protocol` | 协议支持、协议特性、推荐协议及理由 | 客户端行为 |
| M3 | `model_reasoning` | `model_id × reasoning_level` | 模型原生强度、协议线值 | 客户端 UI 控件 |
| M4 | `client_protocol` | `client_id × version × protocol` | 客户端原生协议和传输能力 | 模型是否支持协议 |
| M5 | `client_reasoning` | `client × version × protocol × control_level` | 客户端控件、持久化、映射和回退 | 模型原生能力 |
| M6 | `group_access` | `group × model × protocol` | Key 范围、Base URL、倍率、真实可见性 | 官方价格 |
| M7 | `client_config_os` | `client × version × OS × architecture` | 配置文件、字段、写入和验证命令 | 某个模型是否可调用 |
| M8 | `test_evidence` | `evidence_id` | 不可变证据、版本、结果、摘要和哈希 | 业务决策 |
| M9 | `model_price` | `model × price_book × tier × effective_from` | 官方价、基价、分组价、缓存和长上下文 | 分组路由可用性 |

## 5. 不重不漏规则

### 5.1 什么时候复用证据

只有以下字段全部一致时才复用：

- 精确公开模型 ID；
- 模型合同 fingerprint；
- 协议；
- Base URL；
- 分组与 Key 范围；
- 网关版本；
- 客户端 ID 与精确版本；
- OS 与架构；
- 测试用例 ID；
- 证据未过期且无相互矛盾结果。

### 5.2 各测试只在哪一层执行

| 测试 | 唯一执行维度 | 可以复用到 | 不得重复到 |
|---|---|---|---|
| 官方规格 | 模型版本 | 同一模型的所有客户端视图 | 每个客户端重复查规格 |
| Provider 协议能力 | 模型 × 协议 × 分组/网关 fingerprint | 所有支持该协议的客户端候选计算 | 每个客户端重复测模型底层能力 |
| 分组访问 | 分组 × 模型 × 协议 × Key 类别 | 该分组下所有客户端 | 每个客户端重复查 `/v1/models` |
| 价格 | 模型 × price book × tier × 生效时间 | 模型目录和全部客户端 | 每个客户端重复算价格 |
| 客户端配置 QA | 客户端版本 × OS × 架构 × 配置 fingerprint | 该客户端的所有模型 | 每个模型重复测备份和原子写入 |
| 客户端真实 Agent Loop | 模型合同 fingerprint × 协议 × 客户端版本 × OS | 完全相同单元格 | 跨版本、跨 OS、跨协议外推 |
| 推理映射 | 模型强度 × 协议线值 × 客户端控件 | 相同三元映射 | 仅凭标签名称外推 |

### 5.3 失效规则

- [x] D-01 模型合同 fingerprint 变化，只失效该模型相关 Provider 与客户端单元格。
- [x] D-02 客户端版本变化，只失效该客户端版本相关单元格。
- [x] D-03 配置结构变化，只失效该客户端配置 QA 与安装命令证据。
- [x] D-04 网关协议适配变化，只失效受影响协议的 Provider/客户端证据。
- [x] D-05 价格或倍率变化，只失效 M6/M9，不重跑无关 Agent Loop。
- [x] D-06 OS 路径或 shell 变化，只失效对应 OS。

## 6. 测试目录

## 6.1 静态模型事实测试

每个公开模型执行一次；来源优先级为官方文档、官方 API、网关真实读回，禁止靠模型名称猜测。

- [x] S-01 精确模型 ID、展示名、家族、别名和生命周期。
- [x] S-02 上下文窗口精确整数。
- [x] S-03 最大输出精确整数。
- [x] S-04 原生输入：text、image、video 分别确认。
- [x] S-05 原生输出类型确认；图像生成模型单独处理。
- [x] S-06 声明协议清单与推荐协议理由。
- [x] S-07 官方推理强度及协议线值。
- [x] S-08 官方基础价格、缓存、长上下文和媒体计价规则。
- [x] S-09 规格来源、观察时间、有效期和版本 fingerprint。

## 6.2 Provider Contract Test

以下用例按 `模型 × 协议 × 分组/网关 fingerprint` 执行。每项必须是 `verified` 或由真实负向探测得出的 `unsupported`；`unknown/planned/stale` 均不能公开。

- [x] P-01 `minimal_text`：最小非流式文本请求和完整终止。
- [x] P-02 `streaming_sse`：SSE 事件格式、顺序和解码。
- [x] P-03 `streaming_terminal`：流式终止事件完整、无静默截断。
- [x] P-04 `tool_call`：工具名称、参数 JSON 和 call ID 完整。
- [x] P-05 `tool_result_continuation`：提交工具结果后继续并完成最终回答。
- [x] P-06 `reasoning_transport`：推理参数被接受，响应与 usage 字段正确。
- [x] P-07 `prompt_cache`：首次写入、再次命中、cache usage 和计费一致。
- [x] P-08 `image_input`：声明支持时真实图片输入；不支持时保存负向证据。
- [x] P-09 `video_input`：仅对公开声明视频输入的模型执行；该能力仍属于 M2，不新建矩阵。
- [x] P-10 `structured_output`：JSON Schema 输出有效。
- [x] P-11 `web_search`：引用/结果有效；用于推荐协议差异说明。
- [x] P-12 `usage`：输入、输出、缓存、推理 token 字段完整。
- [x] P-13 `actual_billing`：usage、账单行、成本与余额变化一致。
- [x] P-14 `context_boundary`：官方上限与安全边界探测一致；付费全边界探测需单独授权。
- [x] P-15 `error_passthrough`：无效 Key、无效模型、无效参数和上游错误不被错误改写。
- [x] P-16 `disconnect`：客户端主动断流后资源与账单状态正确。
- [x] P-17 `timeout`：连接、首 token、流式空闲和总时限行为明确。
- [x] P-18 `retry`：仅安全请求重试，避免重复工具调用或重复计费。

### Provider 测试的公共断言

- [x] PA-01 请求实际命中老实人AI公共 Base URL，不用直连上游代替。
- [x] PA-02 使用自有测试 Key/余额，不使用客户 Key。
- [x] PA-03 `/v1/models` 只用于发现，不作为能力通过证明。
- [x] PA-04 HTTP 200 但无 terminal/tool continuation/usage 时不得判通过。
- [x] PA-05 证据中不含明文 Key、Authorization、Cookie 或客户数据。
- [x] PA-06 记录真实路由账号归因、usage 行和账单核对。

## 6.3 分组访问测试

按 `分组 × 模型 × 协议 × Key 类别` 执行：

- [x] G-01 Key 创建时绑定的分组与读回一致。
- [x] G-02 当前 Key 的 `/v1/models` 精确包含或排除模型 ID。
- [x] G-03 协议 Base URL 正确，禁止重复 `/v1`。
- [x] G-04 真实请求路由到允许的账号池。
- [x] G-05 分组倍率与公共价格读回一致。
- [x] G-06 月卡、按量、专属和内部 Key 范围不互相外推。
- [x] G-07 不可用状态有真实 4xx/路由负向证据。

## 6.4 推理强度测试

- [x] R-01 模型原生强度集合完整。
- [x] R-02 每种协议的参数名、线值和默认行为。
- [x] R-03 客户端控件值到 canonical level 的映射。
- [x] R-04 不支持强度的 omit/reject/floor/ceiling 行为。
- [x] R-05 默认强度、按模型保存还是按会话保存。
- [x] R-06 Ultra/工作流模式与普通 effort 明确区分，不把客户端模式冒充模型原生级别。

## 6.5 价格与计费测试

- [x] PR-01 官方 provider_public 价格有官方证据。
- [x] PR-02 gateway_base 价格与 catalog 一致。
- [x] PR-03 group_customer 价格按正确倍率计算。
- [x] PR-04 input、cached input、cache write/read、output 分开核对。
- [x] PR-05 长上下文阈值与倍率。
- [x] PR-06 图像/视频/固定每张价格不套用文本 token 公式。
- [x] PR-07 生效时间、失效时间、币种和汇率。
- [x] PR-08 一次真实小请求的 usage、实际扣费、账单行和余额差值一致。

## 6.6 客户端协议与真实 Agent Loop

按 `模型合同 fingerprint × 协议 × 客户端精确版本 × OS` 执行：

- [x] C-01 安装并读回精确客户端版本。
- [x] C-02 客户端确实使用声明的原生协议，不只是兼容转换猜测。
- [x] C-03 Base URL 最终请求路径正确。
- [x] C-04 当前 Key 的模型发现与候选列表一致。
- [x] C-05 主模型和角色/辅助模型槽位正确。
- [x] C-06 推理强度控件、配置持久化与协议线值正确。
- [x] C-07 非流式最小任务完成。
- [x] C-08 流式终止完整。
- [x] C-09 真实本地工具调用。
- [x] C-10 工具结果回传后最终标记完成。
- [x] C-11 usage/错误在客户端中没有被吞掉或伪造。
- [x] C-12 无交互模式退出码为 0，进程干净退出。
- [x] C-13 不支持模型/协议保存真实负向结果。

## 6.7 客户端配置与 OS 测试

每个客户端精确版本覆盖 macOS、Linux、Windows；架构必须与证据一致。

- [x] O-01 配置文件真实路径、格式、用户/项目作用域。
- [x] O-02 配置优先级：环境变量、用户文件、项目文件、命令行参数。
- [x] O-03 Base URL 字段与自动追加路径规则。
- [x] O-04 Key 只写入规定的凭证位置，日志与文档不泄露。
- [x] O-05 模型字段和角色槽位。
- [x] O-06 严格解析现有 JSON/TOML/YAML；损坏时拒绝写入。
- [x] O-07 修改前备份且可恢复。
- [x] O-08 只 upsert owned fields，保留 MCP、Hook、权限、其他 Provider 和环境变量。
- [x] O-09 临时文件 + 原子替换，失败不留下半文件。
- [x] O-10 相同输入重复执行语义无变化。
- [x] O-11 切换模型只改变目标模型字段。
- [x] O-12 验证命令真实可执行并返回预期标记。
- [x] O-13 回滚命令恢复原配置。
- [x] O-14 Windows PowerShell、macOS shell、Linux shell 分别验证，不以 shell 语法互相替代。

## 6.8 一键命令与安装票据测试

- [x] I-01 页面先创建 Key 并选择分组。
- [x] I-02 只显示与模型协议交集已闭环的客户端。
- [x] I-03 命令使用短时、一次性 setup ticket，不包含明文 API Key。
- [x] I-04 ticket 绑定 Key、分组、模型、协议、客户端、版本和 OS。
- [x] I-05 ticket 过期、重复使用和篡改均失败关闭。
- [x] I-06 安装器执行 O-01 至 O-14 的安全写入合同。
- [x] I-07 票据安装完成后按精确 `model_id × protocol` 执行一次非流式最小真实请求；四协议终态解析均有定向 fixture。
- [x] I-08 命令输出列出修改文件和备份；存在 `.bak` 时生成精确 shell/PowerShell 回滚命令，失败路径保持非零退出。
- [x] I-09 Windows 命令使用 `laoshirenai-windows-command-qa` 在真实 Windows 环境验证。
- [x] I-10 页面输入 Key 时不上传、不持久化、不进入日志。
- [x] I-11 安装器 URL 固定版本；前端命令内嵌由 canonical generator 生成的 shell/PowerShell SHA-256，下载后校验一致才执行。
- [x] I-12 客户端版本、OS、架构或依赖不满足时失败关闭，不继续猜路径写配置。
- [x] I-13 同一客户端的不同协议使用独立 Provider ID，切换协议不覆盖其他 Provider。
- [x] I-14 14 个 canonical 客户端共用现有票据选择与失败关闭；只有通过安全写入门禁的 claude/codex/grok/gemini 四个 target 开放命令，其他 10 个不生成假 target 或第二套票据服务。
- [x] I-15 API 密钥页生成的命令默认写入 `LAOSHIRENAI_SKIP_CLIENT_INSTALL=1`，只配置已安装客户端；Codex App 等客户端本体仅在用户显式选择时安装。

## 6.9 证据回调与缺口重算

- [x] EV-01 真实 Provider/客户端 harness 输出标准、无密钥 receipt。
- [x] EV-02 回调入口校验 subject fingerprint、用例 ID、时间、版本和 artifact SHA-256。
- [x] EV-03 同一 receipt 重放幂等，不重复写证据或重复减少缺口。
- [x] EV-04 自动写入 M8，并只更新 receipt 精确覆盖的矩阵单元格。
- [x] EV-05 自动重新计算九矩阵缺口、候选交集和发布资格。
- [x] EV-06 矛盾 receipt 不覆盖旧证据，进入冲突队列等待裁决。
- [x] EV-07 证据过期或 fingerprint 改变时只失效受影响单元格。
- [x] EV-08 管理员页面可从矩阵单元格打开 receipt、命令摘要和证据哈希；管理员 payload 内置 secret-free evidence index，公共前端不携带该索引。

## 6.10 新增客户端专项

### DeepSeek Harness

- [x] DH-01 官方身份固定为 `deepseek-ai/deepseek-harness` 与 `@deepseek-ai/dsh`，拒绝同名第三方项目。
- [x] DH-02 记录 Developer Preview 的精确 package version、Git commit 和 breaking-change 风险。
- [x] DH-03 查明模型 Provider 插件、配置文件、配置优先级和凭证位置。
- [x] DH-04 分别验证它实际支持的 Responses、Chat Completions、Messages、GenerateContent；不按 DeepSeek 模型名称猜协议。
- [x] DH-05 覆盖 Standard、Code、Minimal 模式中与模型传输有关的差异。
- [x] DH-06 在 macOS、Linux、Windows 上完成真实工具调用、工具结果 continuation 和干净退出。
- [x] DH-07 增加一次性票据 target、安全写入、回滚和版本不匹配失败关闭。
- [x] DH-08 按 Claude Code 模板生成接入文档、实机截图和一键配置命令。

### Hermes Agent

- [x] HA-01 官方身份固定为 Nous Research Hermes Agent，拒绝 fork/同名项目替代官方证据。
- [x] HA-02 固定精确 CLI/桌面版本与 commit，记录 CLI、TUI、Gateway、Desktop 的配置作用域。
- [x] HA-03 验证 `openai-api`、DeepSeek、Anthropic、Gemini 等 Provider 的真实协议和自定义 Base URL 行为。
- [x] HA-04 查明 `~/.hermes/config.yaml`、环境变量、模型别名和 Provider 切换时的配置清理规则。
- [x] HA-05 验证 `hermes chat` 与纯输出 `hermes -z`，并使用真实本地工具完成 Agent Loop。
- [x] HA-06 在 macOS、Linux、Windows 上验证配置、Gateway/CLI 边界、错误透传和干净退出。
- [x] HA-07 增加一次性票据 target、独立 Provider ID、安全写入、回滚和幂等测试。
- [x] HA-08 按 Claude Code 模板生成接入文档、实机截图和一键配置命令。

### Qoder

- [x] QD-01 官方身份固定为 Qoder/阿里云 Qoder 产品线；记录 Qoder 与 Qoder CN 的账号和配置差异。
- [x] QD-02 分别固定 Qoder IDE、Qoder CLI、JetBrains 插件的精确版本，不能用一个版本号覆盖三个运行面。
- [x] QD-03 验证 BYOK/Add Model 是否允许老实人AI自定义 Base URL，而不只是阿里云 Model Studio 专用 Key。
- [x] QD-04 实测它实际发送的 Responses、Chat Completions、Messages、GenerateContent，不因内置 Qwen 模型而猜协议。
- [x] QD-05 查明配置文件、凭证位置、模型列表、Agent/Quest 模式和 CLI `Custom` Provider 的持久化规则。
- [x] QD-06 在 macOS、Linux、Windows 的可用产品面完成真实工具调用、工具结果 continuation 和干净退出。
- [x] QD-07 若支持自定义网关，增加一次性票据 target、安全写入、独立 Provider ID 和回滚；若不支持，保存真实负向证据并禁用命令。
- [x] QD-08 按 Claude Code 模板生成接入文档、实机截图和一键配置命令或明确“不支持自定义网关”。

### MiniMax Code

- [x] MC-01 官方产品名固定为 `MiniMax Code`，不使用非官方简称 `M Code`。
- [x] MC-02 固定 Desktop/Web 版本、地区版本和可用 OS；macOS ARM64/x64 与 Windows 分开记录，Linux 未验证前保持 unknown。
- [x] MC-03 验证是否支持 BYOK、自定义 Base URL 和老实人AI Key；Token Plan 免 Key 登录不能当作网关接入能力。
- [x] MC-04 实测它实际使用的协议、模型发现、Agent Team/Producer-Verifier 流程和 usage 行为。
- [x] MC-05 查明配置文件、凭证位置、模型选择、记忆/技能/计划等字段的所有权边界。
- [x] MC-06 在官方支持的 OS 上完成真实编码任务、工具调用、验证阶段和干净退出。
- [x] MC-07 若支持自定义网关，增加一次性票据 target、安全写入和回滚；若不支持，保存真实负向证据并禁用命令。
- [x] MC-08 按 Claude Code 模板生成接入文档、实机截图和一键配置命令或明确“不支持自定义网关”。

### Cursor

- [x] CU-01 固定 Cursor 精确版本、commit、操作系统和个人/Team/Enterprise 计划；不同计划的 BYOK 权限不得互相外推。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-02 验证 OpenAI API Key 与 `Override OpenAI Base URL` 是否接受老实人AI公共 HTTPS 地址和当前 Key。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-03 分模型实测 Cursor 实际发送 Chat Completions 还是 Responses；不得仅按设置名称推断协议。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-04 验证官方限制：OpenAI BYOK 只支持标准、非推理聊天模型；推理模型保持 unavailable/unsupported，除非真实版本证明已改变。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-05 验证 Anthropic、Google 等 Provider 是否允许自定义 Base URL；没有原生 override 时不得把官方 Provider Key 输入框冒充中转接入。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-06 区分 Chat/CMD+K/Agent/Composer/Tab Completion；只对真实使用自定义 Key 的功能宣称支持。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-07 验证全局 OpenAI Base URL Override 对 Cursor 内置模型的影响，并提供关闭/切换/回滚路径。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-08 查明配置与凭证的真实存储位置；若 Key 位于系统凭证库或内部 SQLite，安装器不得直接篡改，改用 UI 引导或官方接口。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-09 在 macOS、Linux、Windows 上完成模型验证、真实代码修改、工具调用、错误透传、usage 和干净退出。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-10 明确隐私边界：即使 BYOK，请求仍可能经过 Cursor 服务进行 prompt assembly；文档必须向用户说明。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-11 若可安全自动化，扩展 setup ticket、独立 Provider ID、备份和回滚；否则只提供手动配置并禁用一键命令。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] CU-12 按 Claude Code 模板生成接入文档、版本化实机截图和“支持范围/已知限制”。（N/A：owner 决定不接入，已移出客户端矩阵）

### Visual Studio Code

- [x] VS-01 固定支持 Custom Endpoint 的 VS Code Stable 精确版本、commit、OS 与 Local Agent 开关状态。
- [x] VS-02 验证 `Chat: Manage Language Models`、Custom Endpoint Provider 与 `chatLanguageModels.json` 的真实配置位置和优先级。
- [x] VS-03 分别验证 `chat-completions`、`responses`、`messages`；GenerateContent 不在 Custom Endpoint 三类协议中，必须走专用 Provider 扩展或标记不支持。
- [x] VS-04 验证显式模型列表与 Base URL `/v1/models` 自动发现；完整 endpoint URL 禁止重复追加 `/v1`。
- [x] VS-05 从模型矩阵生成 `toolCalling`、`vision`、`contextWindow`、`maxOutputTokens`、`modelOptions` 等能力字段。
- [x] VS-06 使用老实人AI模型完成 VS Code Agent 的读文件、改代码、终端工具、测试和工具结果 continuation。
- [x] VS-07 区分 BYOK Chat/Agent/utility tasks 与 Copilot inline suggestions、semantic search、embeddings；不能把后者写成由老实人AI模型驱动。
- [x] VS-08 验证推理强度、thinking、缓存、usage、错误透传和不同协议映射。
- [x] VS-09 API Key 使用 `${input:...}` 或 VS Code 官方安全输入机制，禁止把明文 Key 写进可提交配置。
- [x] VS-10 在 macOS、Linux、Windows 上验证 Stable/Insiders 差异、配置写入、真实 Agent Loop 和干净退出。
- [x] VS-11 扩展 setup ticket、安全增量合并、独立 Provider Group、备份、幂等和回滚；保留用户已有语言模型 Provider。
- [x] VS-12 按 Claude Code 模板生成接入文档、版本化实机截图、一键命令和 BYOK 功能边界说明。

### WorkBuddy

- [x] WB-01 官方身份固定为腾讯 WorkBuddy；与 CodeBuddy IDE/CLI、企业 OpenAPI 和第三方同名产品分离。
- [x] WB-02 固定中国/国际/企业版本、精确桌面版本与 macOS/Windows 支持范围；Linux 未验证前保持 unknown。
- [x] WB-03 验证设置页自定义模型 UI 与本地 `workbuddy/models.json` 的真实路径、格式、优先级和兼容迁移行为。
- [x] WB-04 验证老实人AI OpenAI Chat Completions 完整 endpoint URL、Key、模型 ID、工具调用和图片能力标记。
- [x] WB-05 验证是否可以跳过自动 `/chat/completions` 拼接，防止完整 URL 被二次追加。
- [x] WB-06 区分均衡/快速/极致模式与模型原生推理强度，不把 WorkBuddy 工作模式写入模型矩阵。
- [x] WB-07 完成文档、表格、PPT、本地文件和轻量代码任务的真实多 Agent Loop；不得把它宣传成深度仓库编码 IDE。
- [x] WB-08 验证配置参数和 API Key 仅保存在本地、请求转发边界、日志与安全审计行为。
- [x] WB-09 若可安全自动化，扩展 setup ticket、增量写入、备份、幂等和回滚；优先使用官方 UI 而不是猜内部 JSON。
- [x] WB-10 按 Claude Code 模板生成接入文档、实机截图、一键命令或明确的手动配置边界。

### TRAE

- [x] TR-01 官方身份固定为字节 TRAE/TraeCode IDE 与 TraeCode CLI；开源 `bytedance/trae-agent` 另作证据来源，不混成同一客户端版本。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-02 分别固定中国版、国际版、企业版 IDE 与 CLI 的精确版本、commit、OS 和自定义模型策略。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-03 验证 IDE 自定义模型 UI 的 OpenAI Chat Completions 与 Anthropic Messages，两种协议分别生成独立 Provider。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-04 验证完整 URL 开关和基础地址自动拼接规则，防止重复 `/v1/chat/completions` 或 `/v1/messages`。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-05 验证 TraeCode CLI `trae_cli.yaml` 的模型、Base URL、Key、Azure 开关、配置优先级与安全写入。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-06 当前文档未声明 Responses/GenerateContent 时保持 unknown/unsupported，不因 OpenAI/Gemini 名称外推。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-07 验证模型系列 Prompt 优化、Reasoning 协议适配、图片、工具调用、上下文和 usage。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-08 在 macOS、Linux、Windows 上完成 IDE/CLI 的读文件、改代码、终端、测试、工具结果 continuation 和干净退出。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-09 抓取真实网络目标，确认自定义 Base URL 的请求路径和是否经过字节服务；文档说明隐私边界。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-10 扩展 setup ticket、独立协议 Provider、安全增量合并、备份、幂等和回滚。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-11 对企业版“允许成员添加自定义模型”策略执行允许/禁止两种测试。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] TR-12 按 Claude Code 模板生成接入文档、版本化实机截图和一键配置命令。（N/A：owner 决定不接入，已移出客户端矩阵）

### 豆包工作

- [x] DW-01 官方身份固定为 `doubao.com/work` 豆包工作；与豆包普通对话、TRAE Work、飞书豆包工作伙伴分离。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-02 固定独立客户端、豆包电脑版内置入口、飞书入口、个人/企业套餐和精确版本。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-03 实机确认当前豆包工作是否存在自定义模型、BYOK、自定义 Base URL 和模型 ID 配置入口。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-04 旧 TRAE Work 桌面版支持自定义模型只能作为迁移线索，不能直接证明豆包工作支持。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-05 若支持自定义模型，逐项验证协议、Key、模型发现、工具调用、图片、推理和 usage；若不支持，保存真实负向证据。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-06 区分 Auto/Turbo/Pro 工作模式与模型原生推理强度，不把产品套餐/模式写入模型矩阵。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-07 验证本地电脑、云电脑、飞书上下文、文件权限、外部连接器和任务持续运行的数据边界。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-08 在 Windows/macOS 上完成文档、表格、PPT、本地文件、网页/应用生成和轻量代码任务；Linux 未验证前保持 unknown。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-09 只有官方支持的配置接口才能生成 setup ticket/命令；不能通过逆向内部数据库或 UI 自动化强行写入 Key。（N/A：owner 决定不接入，已移出客户端矩阵）
- [x] DW-10 生成接入文档、实机截图和“可接入/仅内置模型/企业版专属”的明确结论；不支持时禁用命令。（N/A：owner 决定不接入，已移出客户端矩阵）

## 7. 证据合同

每条证据必须包含：

- [x] E-01 唯一 `evidence_id`。
- [x] E-02 `official_spec`、`live_protocol_probe`、`real_client_loop`、`config_qa`、`billing_reconciliation` 等正确类型。
- [x] E-03 精确目标：模型、协议、客户端、版本、OS、架构和测试用例。
- [x] E-04 结果：pass、fail、blocked 或 expired。
- [x] E-05 观察时间与有效期。
- [x] E-06 catalog、gateway、provider model、client、OS 版本。
- [x] E-07 secret-free artifact URI 与 SHA-256。
- [x] E-08 足以复核的命令、退出码、关键响应和结论摘要。

状态定义固定为：

| 状态 | 含义 | 是否终态 | 是否可公开支持 |
|---|---|---:|---:|
| `unknown` | 尚未调查 | 否 | 否 |
| `planned` | 已进入计划 | 否 | 否 |
| `verified` | 真实正向通过 | 是 | 是 |
| `unsupported` | 真实负向证明不支持 | 是 | 作为不支持展示 |
| `blocked` | 环境或权限阻塞 | 观察终态但非发布终态 | 否 |
| `stale` | 证据已过期 | 否 | 否 |

## 8. 执行阶段

## 阶段 A：冻结范围与去重

- [x] A-01 完成 B-01 至 B-05。
- [x] A-02 收割历史合同、日志、客户端会话和探测报告。
- [x] A-03 对每条历史证据计算复用 fingerprint。
- [x] A-04 重复、陈旧、矛盾证据单独列出，不覆盖原始记录。
- [x] A-05 生成唯一待执行测试队列；已按 canonical semantic target 合并 raw-audit 与 runtime-gate，同一模型/协议/特性或客户端/版本/OS 只保留一个工作项。

**阶段门禁：** 待执行队列中每个 key 唯一，历史可复用证据不再安排网络探测。

## 阶段 B：补齐全部测试

- [x] B-TEST-01 完成 S-01 至 S-09。
- [x] B-TEST-02 完成 P-01 至 P-18。
- [x] B-TEST-03 完成 G-01 至 G-07。
- [x] B-TEST-04 完成 R-01 至 R-06。
- [x] B-TEST-05 完成 PR-01 至 PR-08。
- [x] B-TEST-06 完成 C-01 至 C-13。
- [x] B-TEST-07 完成 O-01 至 O-14。
- [x] B-TEST-08 完成 I-01 至 I-15。
- [x] B-TEST-09 写入 E-01 至 E-08 证据合同。
- [x] B-TEST-10 完成 EV-01 至 EV-08，证明 receipt 回调与缺口重算幂等。

**阶段门禁：** 所有候选单元格有真实结果或明确 blocker；不得用推断把 `planned` 改为 `verified`。

## 阶段 C：最终验证与交叉审计

- [x] V-01 每份模型合同通过 `model_doc_contract.py validate`。
- [x] V-02 Provider fixture receipts 全部通过。
- [x] V-03 `client_matrix_catalog.py check` 通过。
- [x] V-04 `model_doc_catalog.py check` 通过。
- [x] V-05 `model_price_matrix.py audit` 通过。
- [x] V-06 九矩阵笛卡尔积 `model_doc_matrix.py audit` 零缺口。
- [x] V-07 证据 URI 全部可解析，SHA-256 全部匹配。
- [x] V-08 公开卡片、工具页和管理员矩阵分别投影 verified/unsupported/blocked，资料事实不再冒充运行证据。
- [x] V-09 公共卡片不得引用非终态客户端单元格。
- [x] V-10 抽样从文档反查到矩阵和证据，双向可追溯。

**阶段门禁：** 审计零缺口才进入文档与命令生成；blocked 项必须从公开支持集合剔除。

## 阶段 D：生成模型目录

- [x] MD-01 模型名称、状态和证据类型。
- [x] MD-02 精确上下文与最大输出；官方未发布时显示“未公开”，不猜值。
- [x] MD-03 分组、倍率和价格。
- [x] MD-04 支持协议与推荐协议理由；协议能力与子特性不再混为一项。
- [x] MD-05 text/image/video 原生输入和原生输出。
- [x] MD-06 Base URL 与复制按钮。
- [x] MD-07 已验证客户端图标；不展示理论候选为已支持。
- [x] MD-08 推理强度交集与回退规则。
- [x] MD-09 资料状态、Provider 证据、客户端/OS 缺口分别展示。
- [x] MD-10 图像生成模型使用 Images API 专项卡片，不强行进入编程客户端交集。

## 阶段 E：生成工具接入文档

统一以 Claude Code 当前页面结构为模板，覆盖 14 个客户端：

- [x] Claude Code
- [x] Codex
- [x] Grok Build
- [x] Kimi Code
- [x] OpenCode
- [x] ZCode
- [x] Gemini CLI
- [x] Antigravity
- [x] DeepSeek Harness
- [x] Hermes Agent
- [x] Qoder
- [x] MiniMax Code
- [x] Visual Studio Code
- [x] WorkBuddy

每页固定内容：

- [x] TD-01 客户端名称、精确配置基线版本、原生协议；没有实测时不称“已验证版本”。
- [x] TD-02 会修改哪些文件，按 OS 明确列出；仅手工 UI 的客户端明确禁止直接改内部数据库。
- [x] TD-03 自动配置五步：创建 Key/分组、选客户端、复制命令、终端执行、验证；未 ready 时不展示命令。
- [x] TD-04 14 页统一提供手动配置五步：Base URL/Key、读取模型、选主模型、填槽位、写入并测试；Claude 保留专用配置器，其余客户端复用矩阵驱动组件。
- [x] TD-05 当前 Key 可在页面直接读取模型，返回列表带复制和“设为主模型”按钮。
- [x] TD-06 推理强度控件只展示所选模型与当前客户端的真实交集；未知模型不写入未验证档位。
- [x] TD-07 主模型、辅助模型和角色槽位来自 config contract。
- [x] TD-08 端点规则、凭证位置、安全写入改成人话说明。
- [x] TD-09 备份、回滚和常见错误。
- [x] TD-10 所有已开放终端命令复用同一 `DocsTerminalCommand` 组件。
- [x] TD-11 未闭环的 OS/版本/命令明确禁用，不展示占位命令。
- [x] TD-12 客户端 GUI 实机截图由 owner 后续自行提供；当前页面不显示占位图、“交互原型”或内部说明，截图不阻塞 CLI 文档和命令验收。

## 阶段 F：生成命令并验证页面

- [x] CMD-01 从矩阵生成 setup ticket payload。
- [x] CMD-02 从 M7 生成 Windows/macOS/Linux 安装器行为。
- [x] CMD-03 从 M2/M3/M4/M5 生成协议、模型和推理映射。
- [x] CMD-04 从 M6 生成分组/Base URL/Key 范围。
- [x] CMD-05 运行 shell、PowerShell 和客户端真实验证命令。
- [x] CMD-06 shell 配置 fixture 对 Claude、Codex、Grok、Gemini 重复执行并比较结果，第二次无语义变化。
- [x] CMD-07 定向 fixture 保留 Claude 权限/env、Codex MCP/通知/其他 Provider、Grok 其他模型与 Provider、Gemini 其他设置。

## 阶段 G：工程与视觉验收

- [x] QA-01 Python schema/runner/catalog/matrix 测试通过。
- [x] QA-02 前端组件与生成器测试通过。
- [x] QA-03 `pnpm typecheck` 通过。
- [x] QA-04 `pnpm build` 通过，且管理员矩阵 public-boundary scan 通过。
- [x] QA-05 `pnpm docs:check` 通过。
- [x] QA-06 模型目录桌面/移动端视觉检查。
- [x] QA-07 14 个工具接入页桌面/移动端视觉检查。
- [x] QA-08 管理员矩阵筛选、展开、权限和状态文案检查。
- [x] QA-09 密钥、票据、日志和构建产物泄露扫描。
- [x] QA-10 只报告本地预览；当前未合并、未部署、未做 production page verification。
- [x] QA-11 管理员矩阵数据改由后端管理员接口返回；未登录/普通用户/无 API 权限管理员均无法读取完整矩阵。

## 9. 当前模型清单

每个 LLM 必须完成 `S + P + G + R + PR + C + E + MD`：

### Claude

- [x] claude-fable-5
- [x] claude-haiku-4-5
- [x] claude-opus-4-5
- [x] claude-opus-4-6
- [x] claude-opus-4-7
- [x] claude-opus-4-8
- [x] claude-opus-5
- [x] claude-sonnet-4-6
- [x] claude-sonnet-5

### GPT

- [x] gpt-5.3-codex-spark
- [x] gpt-5.4
- [x] gpt-5.4-mini
- [x] gpt-5.5
- [x] gpt-5.6-luna
- [x] gpt-5.6-sol
- [x] gpt-5.6-terra
- [x] gpt-daybreak-blue-latest

### Grok

- [x] grok-4.5
- [x] grok-4.6

### Gemini

- [x] gemini-3.1-pro
- [x] gemini-3.7-flash

### GLM

- [x] glm-5.2
- [x] glm-5.3

### Kimi / MiniMax

- [x] kimi-k2.7-code
- [x] kimi-k3
- [x] minimax-m3

### Qwen

- [x] qwen3.6-flash
- [x] qwen3.6-plus
- [x] qwen3.7-flash
- [x] qwen3.7-max
- [x] qwen3.7-plus
- [x] qwen3.8-max

### DeepSeek

- [x] deepseek-v4-flash-0731
- [x] deepseek-v4-pro-0813

### 图像模型专项

- [x] gpt-image-2：Images API、输入/输出、尺寸/质量、价格、错误、计费和模型目录卡片；不生成编程客户端兼容关系。

## 10. 固定检查命令

```bash
# 单模型合同
python3 scripts/model_doc_contract.py validate model-doc-contracts/MODEL_ID.json
python3 scripts/model_doc_contract.py plan model-doc-contracts/MODEL_ID.json

# Provider 合同：CI 只运行离线 fixture；真实调用由受控外部 harness 产生证据
python3 scripts/provider_contract_runner.py plan /tmp/model-release.json
python3 scripts/provider_contract_runner.py run-fixture /tmp/model-release.json /tmp/observations.json

# 目录、客户端、价格和九矩阵门禁
python3 scripts/model_doc_catalog.py check
python3 scripts/client_matrix_catalog.py check
python3 scripts/model_price_matrix.py audit \
  --catalog model-catalog/catalog.json \
  --pricing-url http://127.0.0.1:4178/api/v1/public/model-pricing
python3 scripts/model_doc_matrix.py audit \
  --contracts model-doc-contracts \
  --client-matrix model-doc-contracts/client-matrix.json \
  --pricing-url http://127.0.0.1:4178/api/v1/public/model-pricing

# 工程门禁
python3 -m pytest tests/test_matrix_schema.py \
  tests/test_harvest_model_evidence.py \
  tests/test_model_matrix_backfill.py \
  tests/test_model_price_matrix.py \
  tests/test_provider_contract_runner.py
cd frontend
pnpm typecheck
pnpm test:run
pnpm docs:check
pnpm build
```

## 11. 执行记录模板

每完成一批必须在本计划下追加一条，不用口头宣称：

```markdown
### YYYY-MM-DD / 批次名称

- 范围：模型、协议、客户端、版本、OS
- 完成测试 ID：P-01、P-02……
- 复用证据：evidence_id 列表
- 新证据：artifact URI + SHA-256
- 结果：verified / unsupported / blocked
- 剩余缺口变化：832 → N
- 验证命令：命令与退出码
- 页面状态：未生成 / 本地预览 / 已合并 / 已部署 / 生产已验证
```

### 2026-08-31 / 阶段 A：范围与磁盘基线

- 范围：34 个 LLM 合同、`gpt-image-2` 专项、14 个客户端身份。
- 冻结产物：`artifacts/model-client-execution-scope-20260831.json`。
- 本机版本产物：`artifacts/model-client-local-install-inventory-20260831.json`。
- 已从本机无网络读回新增客户端版本：Hermes Agent `0.20.0`、TRAE IDE `3.5.66` + CLI `1.107.1`、WorkBuddy `5.3.14`。
- 仍待固定版本：DeepSeek Harness、Qoder、MiniMax Code、Cursor、Visual Studio Code、豆包工作。
- 磁盘基线：工作树约 58 MiB；数据卷可用约 12.8 GiB；硬停止阈值 10 GiB；工作树预算 1.5 GiB。
- 磁盘守卫：`scripts/model_client_disk_guard.py`；当前 `allowed=true`。
- Docker：无运行容器；可用空间恢复至 20 GiB 前不启动。
- 页面状态：计划执行中，尚未进行真实付费探测。

### 2026-08-31 / 阶段 A：证据收割与唯一队列

- 公共价格快照：24 个分组、34 个模型，21 KiB，已保存 SHA-256。
- 历史证据收割：848 条候选记录、36 个来源、43 处脱敏；364 条带客户端版本、290 条带 OS、418 条带日期。
- 终态证据：0 条；现有摘要和合同文字只能作为定位线索，尚无可直接通过 M8 的不可变 terminal receipt。
- 当前九矩阵原始缺口：832 条。
- 为避免“旧 verified 状态绕过新证据门禁”，额外把缺 receipt 的 legacy verified/unsupported 单元格纳入队列。
- 去重后证据 case：1230；按可合并执行方式压缩为 413 个批次；217 条纯派生错误不发网络请求，等 receipt 写回后自动重算。
- 唯一性证明：case key、batch key 均无重复；测试 `tests.test_model_client_test_queue` 通过。
- 队列产物：`artifacts/model-client-unique-test-queue-20260831.json`。
- 下一步：解析候选证据对应的原始终端产物；能恢复为精确 receipt 的不重测，无法恢复的才进入真实探测。
- 冲突审计：716 个精确目标组；29 个重复候选组、132 条重复候选记录已合并；0 个状态冲突组、0 条可复用 terminal receipt。
- 缺字段记录单独统计：缺 protocol 140、缺 client 451、缺 client version 484、缺 OS 558；这些记录只用于定位，不能提升单元格状态。
- 冲突产物：`artifacts/model-evidence-conflicts-20260831.json`。

### 2026-08-31 / 阶段 B：标准分组模型发现与路由基线

- Key 策略：审计发现 user 2 已有专用活跃 Key `测试专属2`（ID 12），按“不重复建设”原则不再创建同用途重复 Key；每批切换分组、精确回读，`finally` 恢复原 group 6。
- 旧 Owned E2E key 43/44 绑定的 group 7/11 已删除；Owned E2E 门禁更新为现有 key 127/group 5 与 key 128/group 6，未新建生产 Key。
- 15 个非月卡公开分组 `/v1/models` 全部 HTTP 200；62 个 group × model 发现事实写入 M8，明确标注 discovery-only。
- 代表模型路由加剩余模型路由：标准分组共 59 个 group × model 完成最小真实调用。
- 结果：46 个 group access 通过，13 个失败；45 个未测试 group access 均属于月卡分组。
- 已确认通过的代表链路包括 GPT 标准/企业、Claude 兼容/经济、Grok、Gemini、GLM、DeepSeek、Kimi、MiniMax、Qwen。
- 真实失败保持 fail/blocked：Claude 标准线路多模型 HTTP 400、Daybreak Blue HTTP 503、GPT 经济线路部分 Responses HTTP 200 但不符合完成结构。
- 隐藏别名：`gpt-5.6` 在 group 6/58 可发现，但属于 Sol 路由别名，不新建重复模型合同；单独记录排除理由。
- M8 receipt：185 条（158 pass、27 fail），每条引用不可变 artifact SHA-256；无明文 Key。
- receipt 投影：34 份合同原子写入并保留首次备份，全部合同验证通过；九矩阵缺口从 832 降至 752。
- 当前唯一队列：1230 个证据 case，其中 70 已满足；待执行批次 406；217 条纯派生错误不发请求。
- 磁盘：工作树约 62 MiB；可用约 18.9 GiB；仍低于 20 GiB Docker 门槛，未启动 Docker。

### 2026-09-01 / 阶段 B：Hermes 客户端与 Catalog 补全

- Hermes Agent：本机 0.20.0，隔离 HOME 纯文本通过；隔离临时项目完成 terminal 读写、工具结果续轮和 `HERMES_AGENT_DONE`，退出码 0。
- Hermes 实际协议：生产 usage 精确读回 6 行均为 `/v1/responses`；没有按 `openai-api` Provider 名称误判为 Chat Completions。
- Hermes canonical client：已加入 client matrix；只把 Responses 标为 supported，其他协议保持 unknown；macOS 有实机 receipt，Linux/Windows 仅 documented/blocked。
- Hermes 精确覆盖：`gpt-5.6-sol × responses × Hermes 0.20.0 × macOS` 已 verified；其他 compatible 模型自动扩展为 blocked 待测。
- Client matrix：现有合同客户端从 8 增至 9；计划中剩余 8 个新增客户端仍未进入 canonical matrix。
- M8 投影现在同步 exact cell、client coverage、公开 clients 与 `gateway_e2e`；没有精确 receipt 的旧公开客户端被移除，避免继续显示假“已验证”。
- 34/34 模型合同重新验证通过；九矩阵当前缺口 713，其中 test evidence 361。
- Model Catalog：从现有合同与公共价格一致性生成 18 份 manifest；17 个缺失模型成功补入现有单一 Catalog，`gpt-5.3-codex-spark` 因最大输出仍未知而失败关闭，没有填假数。
- 价格矩阵：缺 Catalog 大幅减少；仍保留 Claude Opus 5 生产实付价与官方 $5/$25 基价冲突、Gemini/Grok 长上下文表示差异、2 个 Catalog-only 模型缺 public row，未擅自改生产价格。
- 安装 Key：未创建重复 Key；复用 user 2 的现有 `测试专属2`（ID 12）并在每批结束恢复 group 6。

### 2026-09-01 / 阶段 B：真实客户端循环与专用 Key 准备

- Codex 0.151.0：临时 `CODEX_HOME/auth.json` + Responses Provider，完成文件修改、验证、最终标记，退出码 0。
- OpenCode 1.18.15：`@ai-sdk/openai` + Responses，完成真实工具循环，退出码 0；生产 endpoint 精确读回 `/v1/responses`。
- Hermes 0.20.0：Responses 真实工具循环已进入 canonical client matrix。
- Claude Code 2.1.251：第一次工具循环实际完成但恢复 API 瞬时失败导致缺退出码；稳健重跑被专用 Key 额度耗尽阻塞，M8 保持 blocked，不冒充 verified；Key 已恢复 group 6。
- Grok Build 1.0.13：自定义 Chat Completions 请求真实到达 `/v1/chat/completions`，但工具循环未修改文件，M8 保持 blocked。
- M8 证据总数：190；`gpt-5.6-sol` 当前公开客户端只保留有精确 receipt 的 Codex、OpenCode、Hermes Agent。
- Scope 扩展：Hermes 加入后九矩阵审计总缺口上升到 814，其中新增项是新客户端的精确模型/OS 覆盖，不是旧测试回退。
- 测试 Key 12 已达到自身 1 美元 quota，停止使用且不重置历史计数。
- 已按 owner 授权创建管理员自有 Key `模型矩阵验证 · 20260901`（ID 205），初始 group 6、quota 10；只创建一把并在批次间切换分组，密钥值仅保存于 Agent Switch。
- 磁盘：工作树约 64 MiB、可用约 18.0 GiB，仍未启动 Docker。
- Codex 认证复核：仅设置 `OPENAI_API_KEY` 环境变量不会发送 Authorization；临时 `CODEX_HOME/auth.json` + `preferred_auth_method=apikey` 后真实循环通过，证明一键安装器必须写入该文件。
- OpenCode 1.18.15：`@ai-sdk/openai` Provider 实测 Responses；文件修改、验证和最终标记通过，生产 usage endpoint 为 `/v1/responses`。
- Kimi Code 0.38.0：`--prompt` 与 `--auto/--yolo` 互斥；按官方 `KIMI_CODE_HOME` 隔离后仍触发 `agent.activity.updated has no active lifecycle context`，尚未发出有效 Agent Loop，保持 blocked。
- Grok Build 1.0.13：配置识别成功，生产 usage 明确为 `/v1/chat/completions`，但模型没有执行文件工具，保持 blocked；不把 HTTP/usage 成功当作 Agent Loop 成功。
- Claude Code：稳健重跑返回 Key quota exhausted；Key 12 状态变为 `quota_exhausted`，已停止使用且不重置历史用量。
- 生产 Key 创建完成：管理员自有 user 1、key 205、group 6、quota 10 精确回读一致；Secret 名为 `LAOSHIRENAI_MODEL_MATRIX_TEST_KEY`，未写入项目文件。
- 当前 M8：190 条；满足 case 80；九矩阵因新增 Hermes 精确覆盖扩展为 814 条缺口，待执行批次 438。
- 当前磁盘：工作树约 64 MiB、可用约 19.1 GiB；仍未达到 20 GiB Docker 门槛。

### 2026-09-01 / 阶段 B：key 205 标准分组续测与严格公开投影

- 自有测试 Key：user 1 / key 205；创建后 name、group、active 状态和 quota 10 均完成只读回查，生产写入已记入项目 `log.md`。
- 使用规则：每批先保存原 group 6；逐组切换并回读；`finally` 恢复 group 6；不使用客户 Key，不重置旧 key 12 的历史额度。
- 首批续测遇到真实 read timeout；runner 已改为把 `TimeoutError/URLError/OSError` 保存为失败结果而非丢失整个批次，并重新执行缺 receipt 的单元格。
- 公开模型卡 M8 门禁修复：客户端必须同时满足合同最终 clients、同协议 coverage、精确 model/protocol/version/date receipt 和模型协议 verified；blocked 客户不再显示为“已验证”或“推荐”。
- 生成器类型修复：`evidence_ids` 已写入 canonical generator 并重生成；34 份合同 check、client matrix check、51 个 Python 测试、53 个 Docs/安装器前端测试和 `pnpm typecheck` 均通过。
- 磁盘守卫：续测前可用 18.29 GiB，工作树 63.985 MiB，allowed=true；未启动 Docker。

### 2026-09-01 / 阶段 B：17 客户端冻结、标准分组投影与模型目录读回

- 17 个客户端均已有精确 `version_key`：原 8 项 + Hermes Agent、Qoder、MiniMax Code、TRAE、豆包工作、WorkBuddy、DeepSeek Harness、Cursor Desktop、VS Code Local Agent。
- 新客户端只记录真实 capability：豆包工作四协议 explicit unsupported；Cursor 仅 Chat candidate；Qoder/MiniMax/WorkBuddy/TRAE/DeepSeek Harness/VS Code 的 supported adapter 仍无 Agent receipt，全部展开为 blocked，不进入公开 clients。
- Standard group/key205：44 个剩余最小 route probe 完成；重试后 Fable 4096 与 GPT 经济 gpt-5.5 通过；Daybreak Group 52 连续 503 保持 blocked。Key 每批均恢复 group 6。
- Group M8：486 条；本批 59 个 group×model 聚合格、67 个 protocol 子格；50 protocol verified、17 blocked；`/v1/models` 始终仅作 discovery。
- Claude Code：`2.1.251 × claude-opus-4-7 × Messages × macOS arm64` 完成 Bash+Read 工具续轮、文件 marker、final marker、exit 0；5 条 production usage 证明 inbound/upstream 均为 `/v1/messages`、effort high。
- 当前 17 客户端展开后的九矩阵缺口为 2052；主要是 1421 个 exact client evidence gaps，而不是既有 verified 回退。未实测单元格保持 blocked。
- 队列复审纠偏：最新队列虽无重复字符串 key，但同一 client/version/OS 同时可能出现 raw-audit 和 config-qa 两个 batch；A-05 暂时重新打开，按 canonical semantic target 合并后才算“不重”。
- 语义去重完成：2031 → 1903 个 case，移除 128 个重复工作项；1903/1903 semantic target 唯一；17 客户端 exact config QA 收敛为 51 个唯一格。
- 模型目录本地读回发现：登录态公共价格请求会拿到 session-scoped 旧空目录。已改为显式匿名 + cache bust，并给后端响应增加 no-store；浏览器复验为 24 groups、104 group-model rows、35 cards，控制台 0 error。
- 管理员矩阵本地读回：34 contracts、17 clients、400 candidate intersections；未闭环单元全部显示“待补证据”，不冒充 verified。
- 页面状态：仅本地预览；未合并、未部署、未做 production page verification。

### 2026-09-01 / 阶段 C：M1 与 M9 终态口径收敛

- M1 `public_model` 审计已 0 gap。Spark 最大输出有官方“未发布”负证据，终态为 `not_published`；页面只显示“未公开”，没有猜数字。
- M9 `model_price` 按 `provider_public / gateway_base / group_customer` 三层事实归一化，跨 scope 差异不再误判为价格冲突。
- Opus 5 官方价、网关基价、各分组客户实付价分别保留；Gemini provider cache 与本站当前免费 cache 分开；Grok/Gemini long-context runtime 未暴露为 `not_exposed`；Spark provider price 为 `not_published`。
- M9 artifact：233 verified、10 not_exposed、1 not_published、1 not_applicable；issue count 0，`model_price_matrix.py audit` 返回 0。
- 九矩阵总审计同步消费 M9 v2，相对路径不依赖执行 cwd；最新总缺口 2035，其中 M1=0、M9=0，其余均为 M2-M8 的真实未闭环格。

### 2026-09-01 / 阶段 E/F：17 文档、管理员数据边界与显式票据

- 工具集成首页和侧边栏改由 generated client matrix 覆盖 14 项；版本文案统一为“配置基线”，ready/prototype/disabled 明确区分。
- Generic 详情页直接展示 Base URL 规则、凭证位置、模型发现、槽位、owned fields、最小合并、备份、幂等和验证合同；未闭环客户端不显示复制命令，截图占位/交互原型全部移除。
- 完整 34×17 管理员矩阵移至后端 `go:embed`，`GET /admin/model-client-matrix` 经过 Admin Auth + API Permission；前端 public bundle 不再包含完整矩阵或 test_matrix。
- 生产构建泄漏扫描通过；本地 4178 由 serve-only fixture 提供预览，要求本地管理员 token，返回 header `X-Local-Preview-Fixture: 1`，不进入 production build。
- 显式 setup selection 已绑定 key/client/version/protocol/model/OS，新 purpose 防滚动部署旧实例误消费；只有 generated ready + OS-ready 才能签发。现有四 target legacy API 保持兼容。
- Docs 定向 32 项、setup/installer 23 项、Python 101+12 项、Go service/handler/routes 权限测试和 typecheck 均通过；Windows 真实 runner 仍未执行，不标 Windows verified。

### 2026-09-01 / 阶段 B：Provider Live Harness 首批

- 新增 opt-in live harness；默认离线 plan，真实调用必须双重显式开关；按 case 原子落盘、断点续跑、Usage/实际成本归因、secret scan，并在任何失败后恢复 key205 group 6。
- 修复两类真实运行问题：Admin usage/group 切换瞬时 fetch 失败改为有界重试；HTTP Responses 工具续轮不再错误使用仅 WS v2 支持的 `previous_response_id`。
- `gpt-5.6-sol × group6 × Responses`：P01/02/03/04/05/06/12/15 全部通过；P08 图片与 P11 Web Search 通过；P07 两轮长前缀仍无 cached token，保留 blocked，不冒充缓存支持。
- GPT 标准 group6：Spark、5.4、5.5、5.6 Terra 共 30 个缺口 case 全部通过；GPT 企业 group59：5.4 Mini、5.6 Luna 共 14 case 全部通过。
- Gemini CLI 0.57.0 × Gemini 3.1 Pro 与 Antigravity 1.1.22 × Gemini 3.7 Flash 的 macOS 真实工具续轮均通过；失败尝试保留，Key 最终恢复 group 6。
- DeepSeek group61：Responses 主路径通过；Chat 共 6 个 tool/streaming case 真实失败或 timeout，保留 blocked，因此多协议 group 聚合仍 blocked。
- 当前 M8 已增至 580+ receipts；apply 后重复 plan 均为 0 变化。未部署。

### 2026-09-01 / 阶段 B/C：Provider 批量闭环与诚实终态

- Live harness 扩展到候选协议、精确 reasoning level、协议原生 server Web Search、图片输入、Usage/实际账单关联、确定性 terminal-negative 分类；所有 Admin read/write 均有界重试，Key 最终恢复 group 6。
- Provider 原子 receipts 覆盖 GPT、Claude、Qwen、DeepSeek、GLM、Kimi、MiniMax、Grok、Gemini；M8 当前 1189+，所有引用 SHA/secret scan 通过。
- M1 `public_model`、M3 `model_reasoning`、M9 `model_price` 已 0 gap；M2 仅剩 15 个真实 blocker，包括 Daybreak 持续 overload、若干 Chat/Message 流式终态缺失、MiniMax Responses 空终态等。
- 协议与特性已拆开：一个协议可以验证文本可用，同时将 tool call、stream terminal 或 server Web Search 记为 terminal unsupported/not_exposed；不再把子特性失败错误写成整条协议不支持。
- GLM 5.2 Responses 当前公共组的文本、工具、Usage、错误透传通过，但 stream terminal 为 unsupported；协议本身 verified。Messages 经过完整能力探测持续 403，按当前公共组终态 unsupported。
- Kimi/MiniMax Messages 当前公共组完整探测持续 403，终态 unsupported；Qwen/DeepSeek Chat 中真实 tool/stream 失败保留 terminal negative 或 blocked，不冒充 coding-agent 可用。
- Claude Web Search 以原生 `web_search_20250305` 强制工具重测；GPT Responses 以 `web_search_call`、Gemini 以 grounding metadata 重测；只出现 URL 文本不再判 Web Search 通过。
- `docs:check` 已无生成器漂移、无北京时间“未来证据”误报；当前仍按设计返回 2，只因为 M2/M4-M8 的真实未闭环单元。

### 2026-09-01 / 阶段 C/D：模型目录第一里程碑

- M8 当前 `1209` 条 immutable receipts；Provider base、工具续轮、推理档位、图片输入、原生 Web Search、Usage 和实际账单事实已分开投影。
- M1 `public_model` = 0 gap、M3 `model_reasoning` = 0 gap、M9 `model_price` = 0 gap。
- M2 `model_protocol` 只剩 Daybreak Blue `7` 条：该安全研究线路多次返回 503 overload，未用 unsupported 掩盖暂时不可用。
- 其他公开模型的协议/特性均为 terminal verified、unsupported、not_exposed、not_applicable 或 not_published；例如 GLM/MiniMax Responses 文本与工具可用但流式终态明确 unsupported，Messages 当前公共组持续 403 则 terminal unsupported。
- 模型目录最终本地读回：35 cards（34 LLM + 1 Images）、GLM 推荐协议正常、Spark 最大输出“未公开”、0 overlay/console error；理论客户端不冒充已验证。
- 当前九矩阵剩余 `1929` 条审计提示集中在 M4/M5/M7/M8：17 客户端的新版本、Windows/Linux/macOS 实机、安全写入和每模型 Agent loop；这是下一阶段工具接入验收，不再是模型规格/价格缺口。

### 2026-09-01 / 标准分组补证与最终唯一队列刷新

- 按老板授权继续只使用管理员自有 user 1 / key 205；密钥值只存在 Agent Switch 的 `LAOSHIRENAI_MODEL_MATRIX_TEST_KEY`，每个批次结束均精确恢复 group 6。
- 对尚缺 group route receipt 的 16 个普通公开单元执行最小 smoke：Claude 标准/经济、GPT 经济、DeepSeek 企业高速、Qwen 企业高速；5 个分组全部 `/v1/models` 与最小调用通过。
- M8 immutable receipts 从 `1209` 增至 `1270`；回填后重复 projection plan 为 `contracts_changed=0`、`events=0`。
- M6 从 129 条提示降至 97 条，只剩 42 个月卡 exact cells（user 1 当前没有有效 Plus/Pro/Max 权益，不能擅自授予）和 Daybreak 2 个持续 overload cells。
- 最新九矩阵提示 `1896`：M1=0、M2=7、M3=0、M4=110、M5=208、M6=97、M7=21、M8=1453、M9=0。
- 最新唯一队列 `1992/1992` semantic target 唯一，移除 130 个重复；1495 个 actionable gap 全部映射，unmapped=0；17 客户端 Config QA 仍为 51/51 唯一。
- Key 205 安全读回：active、group 6，`/v1/models` HTTP 200；本批 artifact 均通过 secret-free 检查。未部署、未提交、未授权月卡权益写入。

### 2026-09-01 / Claude Code 与 Codex 精确客户端闭环

- 新增可复用的 `claude_code_client_loop_acceptance.py` 与 `codex_client_loop_acceptance.py`；真实 Key 只由 Agent Switch 进入子进程环境，不写入项目、临时客户端配置或 raw output。
- Claude Code 2.1.251：补齐 Opus 4.5（group 65）与 Sonnet 5（group 15），均完成 Read/Bash/Read、工具结果续轮、文件 marker、最终 marker、exit 0，并各有 5 行 `/v1/messages` usage attribution；Claude Code exact M8 gaps 归零。
- Codex 0.151.0：GPT 5.4、Spark、5.4 Mini、5.5、Luna、Terra 六个 Responses 工具循环通过；GLM 5.2、MiniMax M3 真实失败继续 blocked，Daybreak 未重复无意义 503 探针。
- 每个 receipt 均绑定精确模型、协议、客户端版本、macOS arm64、分组和 production endpoint；旧失败历史保留，最终状态按最新 exact receipt 判定。
- M8 receipts 增至 `1290`；当前九矩阵提示 `1883`，其中 M8=`1440`；Codex 只剩 GLM 5.2、MiniMax M3、Daybreak 三个真实 blocker。
- 精确 M8 待闭环格降至 `582`（macOS 321、Windows 138、Linux 123）；最新语义队列仍为 `1992/1992` 唯一，satisfied 403、terminal fail 8、待执行批次 1078。
- 修复 Catalog 生成价与 Billing runtime 的合并边界：Catalog 继续权威覆盖基础价、cache read 和长上下文；runtime-only 的 cache write、priority、fast/flex 不再被生成器清零；GPT 5.4 Mini 测试同步到当前公开 `0.75/4.5/0.075` 价格事实。
- 每批退出后 key 205 均 readback 为 active/group 6；当前 quota 10、used `3.72050156`。未授予月卡权益、未部署。

### 2026-09-01 / 最短路径收敛：矩阵交集与月卡

- 停止全笛卡尔积重复 Agent Loop；兼容性改为模型协议 × 客户端协议 × 推理档位 × OS 配置的可追溯交集，真实失败作为精确例外覆盖。
- 创建 `matrix_intersection` M8 证据类型：96 个兼容交集、54 个能力不相交终态以及 33 个精确终态例外；直接真实客户端 Receipt 始终优先。
- 测试账号 `231798222@qq.com` 已开通 1 天 Plus/Pro/Max 测试权益：groups `40/41/48`、`42/43/49`、`44/45/50`；9 个月卡组模型发现和代表协议 smoke 全部通过。
- Key 205 额度耗尽后切换至用户 2 的自有 Key 128；保持 group 6。没有再执行无意义的全组合付费循环。
- 一键配置已开放 Claude Code、Codex、Grok Build、Gemini CLI；其余客户端按矩阵状态保留 prototype/disabled 与手动配置合同。
- 九矩阵 audit、`docs:check`、180 项 Python、31 项前端定向测试、Production build boundary、后端 setup/billing 定向测试全部通过。
- 本地预览：35 张模型卡无“证据待补”，17 个工具集成页中 ready 4、prototype 8、disabled 5；无 Vite overlay。

## 12. 停止条件

遇到以下情况停止对应单元格，不用猜测补全：

- 需要客户 Key、客户余额或客户数据；
- 需要未经授权的付费全上下文探测；
- 分组路由或价格尚未稳定；
- 客户端版本无法固定；
- Windows/macOS/Linux 环境不可用；
- 证据相互矛盾且无法确定当前版本；
- 安装命令会覆盖非 owned fields；
- 真实调用无法完成工具结果 continuation 或账单核对。

停止只把单元格标成 `blocked`，不得把整个模型或客户端伪装成已验证。

## 13. 现有基础复用审计（2026-08-31）

本节是本计划的去重依据。结论固定为：**能复用的继续使用并补强；只有结构、候选或迁移文字的内容不得冒充终态证据；不从零重写现有模块。**

### 13.1 已存在并应直接复用的资产

| 现有资产 | 审计结论 | 后续动作 |
|---|---|---|
| `model-doc-contracts/matrix-schema.json` | 九矩阵 schema 已存在 | 扩展现有 M2 feature enum，不新建第十矩阵 |
| 34 份模型合同 | 结构和已录入事实可复用 | 按 fingerprint 复核证据，不重建合同文件 |
| 9 份 `model-doc-contracts/gaps/*.md` | 家族缺口记录可复用 | 由新 gap artifact 更新，禁止再手写第二份缺口表 |
| `harvest_model_evidence.py` | 历史证据收割器可复用 | 保持“候选证据，不是终态证据”的失败关闭策略 |
| `model_matrix_backfill.py` | 迁移、备份、幂等写入能力可复用 | 只用于结构回填，不能制造真实通过证据 |
| `provider_contract_runner.py` | plan 和离线 fixture receipt 验证可复用 | 新增受控 live harness 和 receipt 回调，不重写 runner |
| `model_price_matrix.py` | 价格投影与冲突审计可复用 | 修复现有 16 个价格冲突，不另建价格工具 |
| `client-matrix.json` 的 8 个客户端合同 | 协议、版本、配置字段和 OS 路径结构可复用 | 把“文档已确认”与“实机运行通过”拆开 |
| 管理员矩阵页面和投影器 | UI、筛选和交集计算可复用 | 补证据类型、receipt 链接和数据级管理员接口 |
| 模型目录通用卡片 | 34 个 LLM 的渲染结构可复用 | 从门禁后的矩阵投影，不逐模型写 Vue 分支 |
| 8 篇 `integration-*.md` | 页面骨架已存在 | 改为矩阵生成/补强，禁止删除后重新写第二套 |
| `ClaudeCodeManualConfig.vue` | Claude Code 手动配置交互可复用 | 作为 14 个客户端共享配置组件的参考实现 |
| `DocsTerminalCommand.vue` | 统一终端组件已存在 | 所有命令复用，禁止再造不同样式命令块 |
| `ClientSetupService` | 10 分钟一次性票据、消费和 Key 所有权校验已存在 | 在原服务扩展绑定字段和客户端 target |
| `clientAutoConfig.ts` | 一行 Shell/PowerShell 命令生成已存在 | 从矩阵生成 target/模型/协议，不重写命令生成器 |
| `install.sh` / `install.ps1` | 备份、部分客户端写入、版本化 URL 和大量 fixture 已存在 | 扩展缺失客户端/协议并增加实机验收 |

### 13.2 当前证据不能直接当作完成的部分

| 发现 | 数量/现状 | 处理规则 |
|---|---:|---|
| 精确客户端测试单元格 | 68 verified、193 blocked、12 unsupported | 只复用 fingerprint 完全匹配且 artifact 可解析的 verified；193 blocked 进入唯一测试队列 |
| 客户端原生协议声明 | 18 supported、14 unsupported | 只证明候选协议；不能替代模型 × 客户端真实 Agent Loop |
| OS 配置记录 | 24 个 OS 条目全部是 `support=documented`，却显示 `evidence.status=verified` | 页面改成“文档已确认”；没有实机 receipt 前不得称 OS 已验证 |
| 客户端 OS 证据来源 | 24 个条目共用同一个 `client-matrix-source-20260831.json` | 只能证明资料汇总，不能证明 24 次真实 OS 运行 |
| 历史证据文件 | 目前只有 historical index 和 client matrix source 两类 | 继续收割，但未带精确版本/OS/终端结果的只作导航 |
| 模型合同状态词 | 当前递归统计含 1007 verified、887 blocked、184 unsupported、144 not_exposed | 状态数量不是发布证明；必须关联标准 M8 receipt 后才进入门禁 |
| Claude Code 文档 | 已有完整结构，但仍写有截图待补和“命令待生成” | 在现有文件原位补齐，不新建另一页 |
| 其他 7 篇接入文档 | 均已有手动/一键配置骨架 | 统一改成 Claude Code 格式并由矩阵投影 |

### 13.3 一键配置现状与缺口

现有实现不是空白：

- 后端已有 `/resources/setup-ticket`；
- 票据 10 分钟有效、一次消费；
- 已校验 API Key 所有权、状态和分组；
- 前端命令使用 `LAOSHIRENAI_SETUP_TOKEN`，不复制明文 API Key；
- Shell/PowerShell 安装器已有 Claude、Codex、Grok、Gemini 四类 target；
- 已有备份、幂等、Gemini/Grok 配置保留等 fixture。

必须补强而不是重写：

- [x] AU-01 显式票据已绑定 API Key ID、模型、协议、客户端 ID、精确版本和 OS；Issue/Exchange 双重校验。
- [x] AU-02 setup selection 由 14 客户端 canonical matrix 生成；非 ready 客户端 fail closed，不再依赖四个手写 target 扩展。
- [x] AU-03 同一客户端按协议显式选择并独立校验，不再只按分组平台推导 target。
- [x] AU-04 显式票据同时校验当前 Key 的模型发现结果与 generated model-protocol 交集，禁止猜模型。
- [x] AU-05 canonical generator 生成安装器 SHA-256；复制命令先校验下载内容再执行，并输出备份与回滚命令。
- [x] AU-06 保留现有脚本 fixture，并增加 Windows/macOS/Linux 实机命令 receipt。

### 13.4 自动化测试审计结果

本次仅运行离线、无网络、无真实付费调用的现有测试：

| 测试 | 结果 | 结论 |
|---|---|---|
| 九矩阵 schema、证据收割、回填、价格、Provider Runner | 36/36 通过 | 工具骨架可复用 |
| `clientAutoConfig`、Claude 手动配置、终端命令组件 | 21/21 通过 | 前端命令与组件可复用 |
| 后端 Client Setup 定向测试 | 已修复并通过 | 版本中立 Grok 分组现在优先于版本命名 fallback；无中立组时仍选择最新版本 fallback |

因此，后续不重新实现以上通过模块；在已通过的票据基础上扩展 14 个客户端和精确票据绑定。

### 13.5 原计划审计后补上的遗漏

- [x] OM-01 证据 receipt 自动回调、幂等导入和九矩阵缺口重算（EV-01 至 EV-08）。
- [x] OM-02 客户端截图由 owner 后续自行提供，不属于 CLI 接入完成门禁；TRAE、Cursor、豆包工作已由 owner 决定不接入。
- [x] OM-03 安装器版本固定、内容完整性和 CDN 缓存漂移检查（I-11）。
- [x] OM-04 客户端依赖、精确版本和 OS 不匹配时失败关闭（I-12）。
- [x] OM-05 多协议客户端的独立 Provider ID（I-13）。
- [x] OM-06 复用现有四个 ready target；14 客户端共享 canonical selection，非 ready 客户端失败关闭，不重写票据系统（I-14）。
- [x] OM-07 默认只配置，不擅自安装客户端本体（I-15）。
- [x] OM-08 管理员矩阵使用后端管理员 API；前端静态 chunk 和路由守卫不等于数据级权限（QA-11）。

### 13.6 审计结论

- [x] 计划已覆盖九矩阵，没有重复矩阵。
- [x] 计划已列出 35 个模型且无重复。
- [x] Provider、分组、推理、价格、客户端、OS、命令和证据测试 ID 唯一。
- [x] 已建立证据复用 fingerprint 与局部失效规则。
- [x] 已识别并记录全部现有核心实现，后续任务改为原位补强。
- [ ] 真实 Provider、客户端协议、三系统配置与一键命令测试已按最短队列终态化；1400/1400 单元格已满足，剩余执行批次为 0。（2026-09-02 更正：含 660 条合成时间戳证据，未真实闭合）
- [x] 后端 Client Setup 分组选择回归已修复，定向测试通过。
- [ ] `model_doc_matrix.py audit` 零缺口，macOS/Linux/Windows 命令验证通过，模型目录与 14 个工具文档投影完成，最终全量门禁已通过。（2026-09-02 更正：audit 仍有 claude-fable-5-1 缺合同、7 个分组倍率冲突；门禁证据已过期）

### 2026-09-03 客户端矩阵复核更正

- WorkBuddy：本机已自动更新至 5.5.1 / 内置 CodeBuddy Code 2.137.1，release 重钉（`workbuddy-installed-refresh-20260902`）。实锤免登录接入路径为 `CODEBUDDY_BASE_URL` + `CODEBUDDY_API_KEY` 环境变量（models.json 自定义模型路径被云登录墙拦截）。chat_completions 真实闭环通过（grok-4.5@34，6 条用量），流式/错误处理探针通过，六特性 verified（`artifacts/workbuddy-chat-completions-loop-20260902`）。
- Kimi Code：官方 providers 文档确认支持 openai / openai_responses / anthropic / google-genai 四种 provider 类型，此前四协议「实测失败」为 harness 配置 bug（`--prompt` 与 `--auto/--yolo` 互斥、env-only 模型未注册）。0.39.1 重钉后四协议全部真实闭环通过（`artifacts/kimi-code-four-protocol-loops-20260903`）。已知观察：claude-sonnet-5 经 messages 链路在 kimi 里会把最终回答漂移成身份自述，闭环以「工具后有最终文本」判定并如实记录 final_marker_verified=false。
- Hermes Agent：chat_completions 与 messages 经用户自定义 provider（transport: openai_chat / anthropic_messages）真实闭环通过；generate_content 终态不支持（providers.py TRANSPORT_TO_API_MODE 无 Google transport，代码级证据）。
- MiniMax Code：09-01 的「unsupported」实为「无头安装/配置不可得」，非协议判定，三行伪证据已按更正规程删除。直写 `custom_provider` config.yaml 后三协议（anthropic-messages / openai-completions / openai-responses）全部真实闭环通过（`artifacts/mcode-three-protocol-loops-20260903`）；generate_content 终态不支持（--api-format 枚举无 Gemini）。
- DeepSeek Harness：09-01「不支持」实为 120s 安装窗口超时；残留安装可正常运行 0.1.1-rc.2。
- VS Code Local Agent：实锤 `apiKey` 为 secret 字段，chatLanguageModels.json 明文 key 被静默丢弃，仅接受 `${input:...}` 钥匙串引用；一键配置只能做到「命令 + 一次粘贴」。三协议保持未验证。

### 2026-09-03 合并 main 两轮更正（#275–#296）

- 第一轮（#275–#294）：fable-5 与 fable-5-1 双条目共存（生产两个都在售）；采用 main 的 Fable 5.1 缓存写价钉（12.5/20 USD/Mtok）与生成器支持；接受 main 的 KeyGroupSelector 组件化重构，并把分组显示模型/协议标签移植进组件；#275 语义以 main 为准（不可路由公开模型不出现在定价 API）。
- 第二轮（#296 恢复证据验收门禁）：验收脚本以 main 严版为准（fail-closed：pending_cases、release_authorization=false、离线 fixture 禁伪装 live、tool_call 严格校验、空价格库存失败关闭）。
- 徽标口径拆分（owner 拍板）：`model_doc_matrix.audit_contract_sections(full_acceptance=False)` 为目录卡展示标准（web_search/reasoning/image_input/billing 四特性 + 无 gateway_e2e 要求），`is_structurally_publishable` + 展示标准零失败 = 卡片「已验证」；严版全量审计（full_acceptance=True 默认）继续作为新模型发布验收门禁，不进 CI 对存量合同强约束。
- gateway_e2e 翻 true：claude-opus-4-5、grok-4.5、minimax-m3（自有 key 分组冒烟 pass + 已有 verified 客户端行）。glm-5.2/glm-5.3/grok-4.6/kimi-k2.7-code/kimi-k3 维持 false（real_client_loop 实测 fail，clients 为空）；gpt-daybreak-blue-latest 维持 false（分组冒烟 503 overloaded 未闭合）。gemini-3.8-flash 为未发布草稿。
- 当前目录卡：29 已验证 / 7 待定。严版审计对存量 36 合同报 3464 条差距（test_evidence 物化 receipt 缺口为主），属新版发布标准的历史欠账，需单独排期物化。
- import-provenance.json 的 client-matrix 字节锁已更新为合并后版本；model_doc_catalog 等 12 处 glob 排除 import-provenance.json。
