-- 111_add_commission_consumption_idempotency.sql
-- 为代理商消耗分佣记录增加幂等唯一约束。
-- 约束键：(type, source_id)，限定 consumption 类型且 source_id 为真实 usage_logs.id。
-- 作用：同一 usage_log 在运行态或 backfill 里被重复触发时，由数据库兜底忽略第二次写入。
CREATE UNIQUE INDEX IF NOT EXISTS commission_consumption_source_unique
    ON commission_records (type, source_id)
    WHERE type IN ('consumption', 'consumption_commission')
      AND source_id IS NOT NULL
      AND source_id > 0;
