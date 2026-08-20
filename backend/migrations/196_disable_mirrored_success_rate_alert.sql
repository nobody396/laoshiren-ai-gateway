-- The default success_rate < 95% rule is mathematically identical to the
-- default error_rate > 5% rule because both use the same SLA request set.
-- Keeping both enabled produces two Feishu messages for one incident with
-- conflicting P0/P1 labels. Preserve the rule for history/UI visibility, but
-- disable only the untouched default mirror. The tiered error-rate rules remain
-- active: P1 >5% sustained and P0 >20% immediate escalation.

WITH disabled AS (
  UPDATE ops_alert_rules
  SET enabled = FALSE,
      description = '已停用：与错误率 > 5% 使用同一 SLA 样本且完全镜像，避免同一事故重复推送',
      updated_at = NOW()
  WHERE name = '成功率过低'
    AND metric_type = 'success_rate'
    AND operator = '<'
    AND threshold = 95.0
    AND window_minutes = 5
    AND sustained_minutes = 5
    AND severity = 'P0'
  RETURNING id
)
UPDATE ops_alert_events AS event
SET status = 'resolved',
    resolved_at = COALESCE(event.resolved_at, NOW())
FROM disabled
WHERE event.rule_id = disabled.id
  AND event.status = 'firing';
