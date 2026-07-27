-- 本地演示数据：30 天调用日志，供看板图表渲染用。
--
-- 只写 usage_logs（真实来源表），聚合表交给 dashboard_aggregation 自己算
-- ——直接写聚合表会得到一份和明细对不上的假数据，看板一钻取就露馅。
--
-- 一行 = 一次真实调用。第一版把负载曲线只加在 token 量上、每小时固定
-- 七行（每个模型一行），结果「请求数」是条直线，任何按请求数画的图都
-- 没有形状。现在调用条数本身随负载变化。
--
-- 形态刻意做得像真流量，否则图表看不出问题：
--   · 工作日高、周末低
--   · 一天之内早晚双峰
--   · 模型份额长尾（Sonnet 占大头，Opus 贵但少）
--   · 成本按各模型真实价位量级挂钩，缓存读命中便宜
--
-- 可重复执行：先删掉自己写过的行（request_id 前缀 demo-）。

BEGIN;

DELETE FROM usage_logs WHERE request_id LIKE 'demo-%';

INSERT INTO usage_logs (
  user_id, api_key_id, account_id, request_id, model,
  input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
  input_cost, output_cost, cache_creation_cost, cache_read_cost,
  total_cost, actual_cost, stream, duration_ms, created_at, group_id,
  rate_multiplier, first_token_ms
)
SELECT
  k.user_id,
  k.id,
  1 + (seq % 3),
  'demo-' || h.n || '-' || m.idx || '-' || call.i,
  m.model,
  tok.inp, tok.outp, tok.cc, tok.cr,
  round((tok.inp  / 1e6 * m.in_price)::numeric, 10),
  round((tok.outp / 1e6 * m.out_price)::numeric, 10),
  round((tok.cc   / 1e6 * m.in_price * 1.25)::numeric, 10),
  round((tok.cr   / 1e6 * m.in_price * 0.1)::numeric, 10),
  round(cost.total, 10),
  round(cost.total * 0.82, 10),
  (seq % 4) <> 0,
  900 + (seq * 37) % 11000,
  -- 在整点内随机散开，避免所有调用都压在整点上
  t.ts + make_interval(secs => (seq * 613) % 3600),
  1,
  1,
  160 + (seq * 13) % 1600
FROM generate_series(0, 30 * 24 - 1) AS h(n)
-- 必须从整点锚定。用 now() 当锚点时它带着分秒偏移，再往前减 0–3600 秒
-- 会把一部分行推进上一个小时桶，结果是「这小时 49 次、下小时 0 次」的
-- 锯齿——不是负载曲线，是取整错误。
CROSS JOIN LATERAL (SELECT date_trunc('hour', now()) - make_interval(hours => h.n) AS ts) t
CROSS JOIN LATERAL (VALUES
  -- model, idx, 每百万 token 输入价, 输出价, 份额权重
  ('claude-sonnet-4-5',   1, 3.0,  15.0, 22),
  ('claude-opus-4-5',     2, 15.0, 75.0,  6),
  ('gpt-5.2',             3, 1.25, 10.0, 12),
  ('gemini-2.5-pro',      4, 1.25, 10.0,  6),
  ('claude-haiku-4-5',    5, 0.8,   4.0,  5),
  ('gpt-5.2-mini',        6, 0.25,  2.0,  3),
  ('o4-mini',             7, 1.1,   4.4,  2)
) AS m(model, idx, in_price, out_price, share)
CROSS JOIN LATERAL (
  SELECT
    (CASE WHEN extract(dow FROM t.ts) IN (0, 6) THEN 0.55 ELSE 1.0 END)
    * (0.30
       + 0.80 * exp(-power(extract(hour FROM t.ts) - 10, 2) / 12.0)
       + 0.65 * exp(-power(extract(hour FROM t.ts) - 21, 2) / 14.0))
    AS load
) w
-- 这一小时这个模型发生了几次调用 —— 条数本身随负载走
CROSS JOIN LATERAL (
  SELECT greatest(0, round(m.share * w.load
    * (0.7 + 0.6 * ((h.n * 7919 + m.idx * 104729) % 997) / 997.0))::int) AS calls
) c
CROSS JOIN LATERAL generate_series(1, c.calls) AS call(i)
CROSS JOIN LATERAL (
  -- 显式 bigint 并对大质数取模：三项相乘会冲破 int4，
  -- 而后续只把它当作确定性伪随机源，收敛到 [0, 1e6) 完全够用。
  SELECT ((h.n::bigint * 7919 + m.idx::bigint * 104729
           + call.i::bigint * 31337) % 999983)::bigint AS seq
) s
CROSS JOIN LATERAL (
  SELECT
    900 + (seq * 31)  % 2600 AS inp,
    260 + (seq * 17)  % 1500 AS outp,
    CASE WHEN seq % 3 = 0 THEN 120 + (seq * 11) % 700 ELSE 0 END AS cc,
    CASE WHEN seq % 2 = 0 THEN 400 + (seq * 23) % 3200 ELSE 0 END AS cr
) tok
CROSS JOIN LATERAL (
  SELECT (tok.inp / 1e6 * m.in_price + tok.outp / 1e6 * m.out_price
          + tok.cc / 1e6 * m.in_price * 1.25
          + tok.cr / 1e6 * m.in_price * 0.1)::numeric AS total
) cost
CROSS JOIN LATERAL (
  SELECT id, user_id FROM api_keys ORDER BY id OFFSET (seq % 9) LIMIT 1
) k;

-- 聚合表按增量 UPSERT，只会更新"有明细的小时"，不会把已经清空的桶归零。
-- 重灌时旧值会留在那儿（实测 147 个桶卡在上一版的 7）。派生数据直接清空
-- 由聚合器重建即可，比逐桶订正可靠。
TRUNCATE usage_dashboard_hourly, usage_dashboard_daily,
         usage_dashboard_hourly_users, usage_dashboard_daily_users;

-- 把聚合水位线推回 31 天前，让 dashboard_aggregation 重算这段区间。
UPDATE usage_dashboard_aggregation_watermark
SET last_aggregated_at = now() - interval '31 days',
    updated_at         = now();

COMMIT;

SELECT count(*) AS 写入行数,
       min(created_at)::date AS 起,
       max(created_at)::date AS 止,
       round(sum(actual_cost), 2) AS 实付总额,
       round(count(*)::numeric / 720, 1) AS 每小时均调用
FROM usage_logs WHERE request_id LIKE 'demo-%';
