# 老实人AI SEO/GEO 第二层与第三层闭环

## 第一性原理

SEO/GEO 不是只看标签，而是看：用户问题是否被覆盖、页面是否能被抓取、搜索是否产生曝光点击、点击是否带来注册/创建 Key/调用，以及 AI 回答是否把本站当成可信引用。

本目录把闭环拆成三层：

1. 技术层：页面是否能被抓、被理解、被收录。
2. 效果层：GSC/GA4/站内事件是否证明页面有效。
3. 市场层：外部问题、竞品页面、AI 回答是否暴露新机会。

## 脚本

### 1. 数据采集

```bash
node tools/seo_geo_collect.mjs --date $(date +%F) --days 28
```

输出：

```text
docs/ops/seo-geo/data/YYYY-MM-DD/
  manifest.json
  README.md
  technical-audit.md
  gsc-query-page.json      # 有 GSC 凭证时
  ga4-pages.json           # 有 GA4 凭证时
  ga4-events.json          # 有 GA4 凭证时
  ga4-realtime.json        # 有 GA4 凭证时，记录最近 30 分钟事件
```

缺凭证时不会失败，会在 `manifest.json` 和 `README.md` 里写明跳过原因。

### 2. 效果决策

```bash
node tools/seo_geo_decision.mjs --date $(date +%F)
```

输出：

```text
docs/ops/seo-geo/weekly/YYYY-MM-DD-decision.md
docs/ops/seo-geo/weekly/YYYY-MM-DD-decision.json
docs/ops/seo-geo/backlog/page-opportunities.md
docs/ops/seo-geo/backlog/faq-opportunities.md
```

核心判断：

- 曝光高、CTR 低：改 title / description / 首段。
- 排名 8-20：补 FAQ、内容深度和内链。
- 点击高、关键事件低：改 CTA、注册/创建 Key 路径。
- GSC 有点击但 GA4 无会话：查 GA4 埋点或跨域路径。
- 技术异常：优先修 HTTP、正文、schema、canonical、404。

### 3. 市场雷达

```bash
node tools/seo_geo_intent_radar.mjs --date $(date +%F)
```

输出：

```text
docs/ops/seo-geo/radar/YYYY-MM-DD/
  README.md
  manifest.json
  search-radar.json
docs/ops/seo-geo/backlog/radar-page-opportunities.md
```

雷达默认关注：

- Claude Code 国内使用
- Claude Code Base URL
- Claude Code login 失败
- Claude Code 走官方地址
- Codex 国内使用
- Codex 免 API Key
- Codex 自定义 API
- Codex config.toml / auth.json
- Codex WSL 配置

## 需要的 Secret 名称

所有值只能写入 Agent Switch Secrets，不要写进仓库。

### 必需，第二层效果闭环

```bash
agent-switch secret set GSC_SITE_URL 'https://laoshirenai.com/'
agent-switch secret set GSC_SERVICE_ACCOUNT_JSON_B64 '<base64 service account json>'
agent-switch secret set GA4_PROPERTY_ID '<ga4 property id>'
agent-switch secret set GA4_SERVICE_ACCOUNT_JSON_B64 '<base64 service account json>'
```

也支持共用 Google service account：

```bash
agent-switch secret set GOOGLE_SERVICE_ACCOUNT_JSON_B64 '<base64 service account json>'
```

### 可选，第三层市场雷达

当前本机 Agent Switch 已有这些 secret 名称，但脚本运行环境仍需把它们注入为环境变量：

```bash
FIRECRAWL_API_KEY
TAVILY_API_KEY
XCRAWL_MCP_URL
```

### 可选，AI 回答雷达

```bash
SEO_GEO_AI_ANSWER_ENDPOINT
SEO_GEO_AI_ANSWER_API_KEY
```

当前脚本先固定 prompt 和报告框架；接入专用 LLM 评测服务后，可以自动记录“AI 是否提到老实人AI、是否提到竞品、是否回答错误”。

## 安全边界

- 自动任务可以采集、分析、写报告、做低风险本地改动。
- 不自动生产部署。
- 不把 secret 值写进仓库、报告或聊天。
- 高风险文案、合规口径、生产发布必须人工确认。
