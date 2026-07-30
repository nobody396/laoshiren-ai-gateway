import type { AffiliateRiskPrincipal } from '@/api/admin/agents'
import type { CommissionRecord } from '@/api/agent'

export type SelfCommissionTone = 'enabled' | 'disabled' | 'blocked'

export interface SelfCommissionPresentation {
  label: string
  detail: string
  tone: SelfCommissionTone
  canEnable: boolean
}

const CONSUMPTION_COMMISSION_TYPES = new Set([
  'consumption',
  'consumption_commission',
  'self_consumption_commission'
])

export function formatSelfCommissionRate(rateBPS: number): string {
  const percent = Math.max(0, rateBPS) / 100
  return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(percent)}%`
}

function selfCommissionBlockDetail(item: AffiliateRiskPrincipal): string {
  if (item.has_upstream || item.self_commission_block_reason === 'has_upstream') {
    return '已有上级，不能开启本人消费返佣'
  }
  if (item.agent_status !== 'active' || item.self_commission_block_reason === 'agent_not_active') {
    return '合伙人当前不是正常状态，不能开启'
  }
  if (item.risk_status !== 'clear' || item.self_commission_block_reason === 'risk_not_clear') {
    return '请先恢复合伙人权限，再开启本人消费返佣'
  }
  return '当前不符合开通条件'
}

export function getSelfCommissionPresentation(item: AffiliateRiskPrincipal): SelfCommissionPresentation {
  if (item.self_commission_enabled) {
    const rate = formatSelfCommissionRate(item.self_commission_rate_bps)
    const paused = item.agent_status !== 'active' || item.risk_status !== 'clear'
    return {
      label: `已开启 · ${rate}`,
      detail: paused
        ? '当前已通过合伙人状态暂停结算；恢复正常后继续按购买时规则处理'
        : '仅计算开启后新购买并实际使用的付费权益',
      tone: 'enabled',
      canEnable: false
    }
  }

  const runtimeEligible = item.self_commission_eligible &&
    !item.has_upstream &&
    item.agent_status === 'active' &&
    item.risk_status === 'clear'

  if (!runtimeEligible) {
    return {
      label: '不可开启',
      detail: selfCommissionBlockDetail(item),
      tone: 'blocked',
      canEnable: false
    }
  }

  return {
    label: '未开启',
    detail: '可由管理员单独开启，固定返还本人确认消费的 10%',
    tone: 'disabled',
    canEnable: true
  }
}

export function isSelfConsumptionCommission(record: CommissionRecord): boolean {
  return CONSUMPTION_COMMISSION_TYPES.has(record.type) &&
    record.beneficiary_id > 0 &&
    record.beneficiary_id === record.user_id
}

export function getCommissionDisplayType(record: CommissionRecord): string {
  return isSelfConsumptionCommission(record) ? 'self_consumption_commission' : record.type
}
