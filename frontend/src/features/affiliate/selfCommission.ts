import type { AffiliateRiskPrincipal } from '@/api/admin/agents'
import type { CommissionRecord } from '@/api/agent'
import { extractApiErrorCode } from '@/utils/apiError'

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

export function getSelfCommissionOperatorLabel(item: AffiliateRiskPrincipal): string {
  const username = item.self_commission_updated_by_username?.trim()
  const email = item.self_commission_updated_by_email?.trim()
  if (username && email) return `${username}（${email}）`
  if (username || email) return username || email || '—'
  return item.self_commission_updated_by ? `管理员 #${item.self_commission_updated_by}` : '—'
}

export function getSelfCommissionPolicyErrorMessage(error: unknown): string {
  const code = extractApiErrorCode(error)
  if (code === 'AFFILIATE_SELF_COMMISSION_POLICY_REVISION_CONFLICT') {
    return '设置已被其他管理员修改，页面已刷新，请重新确认后再操作。'
  }
  if (code === 'AFFILIATE_SELF_COMMISSION_NOT_ELIGIBLE') {
    return '该合伙人当前不符合开通条件，请检查上级关系及合伙人状态。'
  }
  if (code === 'AFFILIATE_SELF_COMMISSION_AGENT_NOT_FOUND') {
    return '未找到该合伙人，可能已被移除或状态已经变化。'
  }
  if (code === 'FORBIDDEN') {
    return '您没有修改本人消费返佣设置的权限。'
  }

  const candidate = (error && typeof error === 'object' ? error : {}) as {
    status?: number
    response?: { status?: number }
  }
  const status = candidate.status ?? candidate.response?.status
  if (status === 409) return '设置保存冲突，页面已刷新，请重新确认后再操作。'
  if (status === 404) return '未找到该合伙人，可能已被移除或状态已经变化。'
  if (status === 403) return '您没有修改本人消费返佣设置的权限。'
  return '本人消费返佣设置保存失败，请稍后重试。'
}
