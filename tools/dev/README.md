# 本地开发辅助脚本

仅用于本地 docker 开发栈，**不要在生产库上执行**。

## seed-demo-usage.sql

往 `usage_logs` 灌 30 天演示调用数据，让看板图表有东西可看。空库下所有
图表都是 "No data"，前端改动无法评估。

```bash
docker cp tools/dev/seed-demo-usage.sql sub2api-postgres-dev:/tmp/seed.sql
docker exec sub2api-postgres-dev psql -U sub2api -d sub2api -f /tmp/seed.sql
```

写完等约一分钟（`dashboard_aggregation.interval_seconds` 默认 60），聚合器
会自己把 `usage_dashboard_*` 重建出来。

可重复执行：脚本先按 `request_id LIKE 'demo-%'` 删掉自己上次写的行。

几个当时踩过、写进脚本注释的坑：

- **只写来源表，不手写聚合表。** 直接写 `usage_dashboard_hourly` 能立刻
  看到图，但明细与汇总对不上，一钻取就露馅。
- **时间戳必须从整点锚定。** 用 `now()` 当锚点时它带着分秒偏移，再叠加
  小时内随机偏移会把一部分行推进上一个桶，画出来是「这小时 49 次、下
  小时 0 次」的锯齿——看着像负载曲线，其实是取整错误。
- **重灌前要 TRUNCATE 聚合表。** 聚合器是增量 UPSERT，只更新「有明细的
  小时」，不会把已清空的桶归零；旧值会留在那儿（实测 147 个桶卡在上一
  版的数值）。
- **调用条数本身要随负载变化**，不能只让 token 量变化。否则「请求数」
  是条直线，任何按请求数画的图都没有形状。
