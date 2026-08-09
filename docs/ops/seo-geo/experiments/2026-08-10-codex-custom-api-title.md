# Codex 自定义 API 页面标题实验

- 页面：`/docs/codex-custom-api-guide`
- 证据级别：observed（结果置信度低，样本仍小）
- 假设：在标题中直接展示 `Base URL` 与 `config.toml`，能让搜索“Codex 自定义 API”的用户更快判断页面可解决配置问题，从而改善点击。
- 单一变量：页面 SEO title；正文、H1、description 与 URL 不变。

## Baseline

- GSC Property：`https://laoshirenai.com/`
- Search type：web
- Data state：final
- 日期：2026-07-12 至 2026-08-08（Search Console 日期时区 America/Los_Angeles）
- 过滤器：无
- Page 维度：22 曝光、0 点击、平均排名 8.9
- Query+Page 可见下界：3 曝光、0 点击；出现查询为“codex自定义api”“codex 自定义api”
- 原 title：`Codex 自定义 API 配置教程`

## Treatment

- 新 title：`Codex 自定义 API 配置：Base URL 与 config.toml`
- Guardrail：不加入“免费”“官方”等无法由页面证明的承诺；不改变合规口径。

## Observation

- Primary outcome：同一页面在 GSC page 维度的点击与 CTR。
- Supporting evidence：上述查询簇的可见 query+page 明细，仅作下界。
- 最短观察窗：Google 重新处理页面后 28 天 finalized 数据。
- 停止条件：完成上线可见、GSC URL 检查确认重新处理，并走满观察窗；否则不判定输赢。

## Outcome stages

- [x] Implemented
- [ ] Deployed and observable
- [ ] Processed by search platform
- [ ] Outcome observed
