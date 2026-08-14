# Claude Code 与 Codex 协同开发指南

## 为什么要这样搭配？

一句话：**让贵的模型动脑，让便宜的模型动手，账单不爆炸。**

Claude 擅长理解需求、架构规划、复杂推理，但价格也摆在那——用它来写大量代码，Token 烧得飞快。Codex 的代码能力强、速度快，关键是价格便宜很多。

所以最合理的搭配是：

- **Claude 负责规划**：理解你的需求，拆解任务，做架构决策，想清楚怎么做
- **Codex 负责执行**：通过 `/codex:rescue` 接手具体的编码任务，写代码、修 bug、跑测试
- **Codex 负责审查**：通过 `/codex:review` 给 Claude 的代码做 code review，充当第二双眼睛

这个搭配的核心逻辑是：Claude 的 Token 只花在「想清楚」上，具体的编码和审查交给更便宜的 Codex。对于每天高强度使用 Claude Code 的人来说，能显著降低整体开销。

> 本指南假设你已完成 Claude Code 和 Codex 的安装配置。如未完成，请先参考：
> - Claude Code 配置：[老实人 AI 快速开始指南](/docs/claude-code-quickstart)
> - Codex 配置：[老实人 AI × Codex 快速开始指南](/docs/codex-quickstart)

---

## 1. 安装 Codex 插件

两边都配好之后，在 Claude Code 里装一个插件就能串起来：

```bash
/plugin marketplace add openai/codex-plugin-cc
/plugin install codex@openai-codex
/reload-plugins
/codex:setup
```

`/codex:setup` 会自动检测 Codex 是否安装、是否已认证。如果提示未登录，运行 `!codex login` 完成认证。

安装完成后，输入 `/codex` 即可看到新增的斜杠命令。

---

## 2. 核心命令

这个插件一共就三类能力：**审查**、**委派任务**、**任务管理**。

### 审查类（只读，不改代码）

| 命令 | 作用 |
|---|---|
| `/codex:review` | 让 Codex 审查当前未提交的改动，或对比分支 |
| `/codex:adversarial-review` | 对抗性审查——不只是检查代码，而是主动质疑你的设计决策 |

```bash
# 审查未提交的改动
/codex:review

# 审查当前分支和 main 的差异
/codex:review --base main

# 后台运行，不阻塞当前对话
/codex:review --background

# 对抗性审查，指定关注方向
/codex:adversarial-review --background 检查是否有竞态条件，质疑缓存策略的选择
```

### 委派类（把任务交给 Codex 执行）

| 命令 | 作用 |
|---|---|
| `/codex:rescue` | 把一个具体任务交给 Codex 去做，Codex 会实际修改代码 |

这是实现「Claude 规划，Codex 执行」的核心命令。你可以把 Claude 规划好的任务，逐个交给 Codex 去实现：

```bash
# 让 Codex 去查 bug
/codex:rescue investigate why the tests started failing

# 让 Codex 去修 bug
/codex:rescue fix the failing test with the smallest safe patch

# 让 Codex 去实现一个功能
/codex:rescue 实现用户注册的表单验证逻辑

# 后台执行，适合耗时任务
/codex:rescue --background 重构数据库连接池

# 指定更便宜的模型，进一步省钱
/codex:rescue --model gpt-5.4 写一组单元测试覆盖 utils.ts

# 继续上次的任务
/codex:rescue --resume 把上次的修复方案应用上去
```

> 💡 **模型选择**：简单任务可使用 `--model gpt-5.4`；复杂任务建议使用 GPT-5.5 或 GPT-5.6 系列。

### 任务管理类

| 命令 | 作用 |
|---|---|
| `/codex:status` | 查看正在运行和最近完成的 Codex 任务 |
| `/codex:result` | 获取已完成任务的结果（含 session ID，可在 Codex 中继续） |
| `/codex:cancel` | 取消正在运行的后台任务 |

---

## 3. 推荐工作流：Claude 规划，Codex 执行

### 日常开发流程

```
你提需求 → Claude 分析拆解 → /codex:rescue 逐个执行 → /codex:review 审查 → 提交
```

1. **用自然语言描述需求**，让 Claude 理解你要什么
2. **Claude 做规划**：拆解任务、确定实现方案、理清先后顺序——这是 Claude 最擅长的
3. **用 `/codex:rescue` 把具体编码任务交给 Codex**：Claude 规划好了"要做 A、B、C 三件事"，你就用 rescue 一个个交给 Codex 去写
4. **代码写完跑审查**：`/codex:review --background`，让 Codex 做独立 code review
5. 根据审查意见修复后提交

这个流程的好处是：Claude 的 Token 只花在规划和理解上，大量的编码工作走 Codex 的便宜额度。

### 高风险改动流程

涉及数据库迁移、认证授权、基础设施变更时，多加一层对抗性审查：

```
Claude 规划 → /codex:rescue 执行 → /codex:review → 修复 → /codex:adversarial-review → 再修复 → 提交
```

对抗性审查会主动质疑你的设计——比如"为什么选这个缓存策略""回滚方案考虑了吗""这里有没有竞态条件"。高风险改动需要这种压力测试。

### Claude 卡住时

如果 Claude 在某个任务上反复尝试都不理想，直接换 Codex 来试：

```bash
/codex:rescue 用最小改动修复这个问题
```

换一个模型的思路，经常能突破僵局。

---

## 4. 后台运行（推荐）

审查和 rescue 任务都建议加 `--background`，不阻塞当前对话：

```bash
/codex:rescue --background 实现分页功能
# 继续和 Claude 聊别的事...
/codex:status          # 随时看进度
/codex:result          # 完成后取结果
```

> `/codex:result` 会返回一个 session ID，你可以用 `codex resume <session-id>` 在 Codex 里继续这个任务。

---

## 5. 审查门禁（可选）

开启后，Claude Code 在完成任务前会自动触发一次 Codex 审查，发现问题则打断流程先修复：

```bash
/codex:setup --enable-review-gate    # 开启
/codex:setup --disable-review-gate   # 关闭
```

> ⚠️ **注意**：审查门禁会显著增加 Token 消耗，可能造成 Claude 和 Codex 之间的长循环。建议仅在关键项目中使用，日常开发手动跑 `/codex:review` 即可。

---

## 6. Codex 模型配置

想改变 Codex 默认使用的模型或推理强度，可以在配置文件中设置。

用户级配置：`~/.codex/config.toml`
项目级配置：项目根目录下 `.codex/config.toml`

```toml
model = "gpt-5.4"
model_reasoning_effort = "high"
```

也可以在每次调用时通过 `--model` 和 `--effort` 临时指定：

```bash
/codex:rescue --model spark --effort medium 快速修复 lint 错误
```

---

## 7. 什么时候不该用 Codex

以下情况建议全程用 Claude：

| 场景 | 原因 |
|---|---|
| 深度业务逻辑重构 | Codex 拿不到 Claude 对话中积累的完整上下文，生成的代码可能和项目风格不一致 |
| 需要跨文件深度理解的任务 | Codex rescue 是独立运行的，不共享 Claude 的对话历史 |
| 代码量很少的任务 | 只有几行代码，没必要委派，直接让 Claude 写更快 |

---

## 8. 实用技巧

- **简单任务用便宜模型**：`--model spark` 适合修 lint、写样板代码这种不需要深度推理的活
- **先 review 再 adversarial-review**：不要上来就用对抗性审查，先用普通审查过一遍基础问题
- **善用 --resume**：`/codex:rescue --resume` 可以继续上一次的 Codex 任务，不用从头开始
- **关注两边用量**：虽然整体更省，但记得同时关注 Claude 和 OpenAI 两边的用量，避免某一边意外超支
- **Skill 冲突处理**：如果你的自定义 Skill 和 Codex 插件有触发词冲突，在冲突 Skill 里加 `priority: low`

---

## 常见问题

**Q：安装插件后没看到 `/codex` 命令？**
A：执行 `/reload-plugins` 重新加载插件。

**Q：`/codex:review` 报认证错误？**
A：说明 Codex 的认证没配好。请参考 [老实人 AI × Codex 快速开始指南](/docs/codex-quickstart) 重新配置，或运行 `!codex login` 重新登录。

**Q：`/codex:rescue` 和直接让 Claude 写代码有什么区别？**
A：rescue 是把任务交给 Codex 独立执行，消耗的是 OpenAI 额度而不是 Claude Token。适合标准化编码任务。但 Codex 拿不到你和 Claude 的对话上下文，所以任务描述要写清楚。

**Q：审查结果和 Claude 的判断矛盾怎么办？**
A：这恰恰是双模型协作的价值。两边意见不一致时，你作为开发者来做最终决策。

**Q：到底能省多少？**
A：取决于你把多少编码工作交给 Codex。如果你的工作流中代码生成占比高（比如大量写测试、写 CRUD），把这些都 rescue 出去，Claude 的消耗可以大幅降低。
