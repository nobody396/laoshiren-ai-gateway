# SEO/GEO 全链路闭环 2026-06-30

## 结论

- 数据：技术=有，GSC=有，GA4页面=缺，GA4事件=缺
- 自动修复：4 个改动
- 产品改动：3 个文件
- 自动部署：未触发
- 失败步骤：0

## 动作队列

- P3 / observe: 继续观察，不做大改（当前无明显异常）
- P1 /docs/claude-code-china-guide distribution: 增加站内入口和外部引用，检查 sitemap lastmod（P0 页面曝光不足）
- P1 /docs/codex-china-guide distribution: 增加站内入口和外部引用，检查 sitemap lastmod（P0 页面曝光不足）
- P1 /docs/codex-custom-api-guide distribution: 增加站内入口和外部引用，检查 sitemap lastmod（P0 页面曝光不足）
- P1 /docs/codex-no-api-key-guide distribution: 增加站内入口和外部引用，检查 sitemap lastmod（P0 页面曝光不足）

## 已变更文件

- M docs/ops/seo-geo/backlog/page-opportunities.md
- M docs/ops/seo-geo/backlog/radar-page-opportunities.md
- M docs/ops/seo-geo/data/2026-06-30/README.md
- M docs/ops/seo-geo/data/2026-06-30/manifest.json
- M docs/ops/seo-geo/data/2026-06-30/technical-audit.md
- M frontend/public/seo-manifest.json
- M frontend/src/docs/content/claude-code-china-guide.md
- M frontend/src/docs/content/codex-china-guide.md
- M frontend/src/docs/content/codex-custom-api-guide.md
- M frontend/src/docs/content/codex-no-api-key-guide.md
- M frontend/src/main.ts
- M frontend/src/router/index.ts
- M frontend/src/views/auth/EmailVerifyView.vue
- M frontend/src/views/auth/LinuxDoCallbackView.vue
- M frontend/src/views/auth/LoginView.vue
- M frontend/src/views/auth/RegisterView.vue
- M frontend/src/vite-env.d.ts
- M tools/seo_geo_intent_radar.mjs
- docs/ops/seo-geo/data/2026-06-30-ga4-test/
- docs/ops/seo-geo/radar/2026-06-30/
- docs/ops/seo-geo/repairs/
- docs/ops/seo-geo/runs/
- frontend/src/utils/analytics.ts
- tools/seo_geo_auto_repair.mjs
- tools/seo_geo_closed_loop.mjs

## 数据失败

- ga4: HTTP 403 https://analyticsdata.googleapis.com/v1beta/properties/543684074:runReport: {"error":{"code":403,"message":"Google Analytics Data API has not been used in project 112032574633 before or it is disabled. Enable it by visiting https://console.developers.google.com/apis/api/analyticsdata.googleapis.com/overview?project=112032574633 then retry. If you enabled this API recently, wait a few minutes for the action to propagate to our systems and retry.","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED","domain":"googleapis.com","metadata":{"consumer":"projects/112032574633","containerInfo":"112032574633","service":"analyticsdata.googleapis.com","activationUrl":"https://console.developers.google.com/apis/api/analyticsdata.googleapis.com/overview?project=112032574633","serviceTitle":"Google Analytics Data

## 执行步骤

- OK collect
- OK decision
- OK intent-radar
- OK auto-repair
- OK frontend-build
