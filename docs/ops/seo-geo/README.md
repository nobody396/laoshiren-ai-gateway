# GEO/SEO 复查说明

这个目录用于沉淀老实人AI的搜索获客与 AI 引用复查结果。

## 第一性原理

网站不是展示页，而是搜索获客和信任承接中枢。复查不只看技术标签，而要看：用户问题是否被回答、爬虫是否能直接抓到正文、页面是否能把高意图用户导向注册/创建 Key/咨询。

## 固定检查

本仓库提供无密钥体检脚本：

```bash
python3 tools/seo_geo_audit.py --base-url https://laoshirenai.com --timeout 8 --workers 8 --out docs/ops/seo-geo/daily/$(date +%F).md
```

检查项：

- sitemap 页面 200 覆盖率
- 原始 HTML 正文长度
- title / description / canonical
- JSON-LD / FAQPage
- llms.txt
- soft 404 / noindex
- P0 页面状态

## P0 页面

- `/docs/claude-code-china-guide`
- `/docs/codex-china-guide`
- `/docs/codex-no-api-key-guide`
- `/docs/codex-custom-api-guide`

## 凭证边界

Search Console、GA4、OpenAI/Anthropic/Firecrawl 等凭证只能放 Agent Switch secrets，不能写入本仓库、日报、周报或聊天记录。
