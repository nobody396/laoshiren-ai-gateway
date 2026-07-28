export type PartnerAccessState = 'not_partner' | 'available' | 'under_review' | 'suspended'

export interface PartnerAccessCopy {
  badge: string
  banner: string
  inviteLabel: string
  inviteHint: string
}

const COPY: Record<PartnerAccessState, PartnerAccessCopy> = {
  not_partner: {
    badge: '',
    banner: '',
    inviteLabel: '我的普通邀请链接',
    inviteHint: '首笔真实付费后，邀请人与被邀请人各获得实付金额 5% 的 ⚡，均为 T+0；不设最低金额，也没有固定奖励。'
  },
  available: {
    badge: '合伙人已开通',
    banner: '',
    inviteLabel: '我的默认合伙人链接',
    inviteHint: '默认链接使用固定 10% 奖励池；可在下方动态调整客户返利与现金佣金的分配。'
  },
  under_review: {
    badge: '待审核',
    banner: '你的合伙人权限正在审核中，审核完成后会自动恢复。有疑问请联系客服。',
    inviteLabel: '邀请链接暂不可用',
    inviteHint: '审核期间暂时不能邀请、提现或转换佣金。有疑问请联系客服。'
  },
  suspended: {
    badge: '已暂停',
    banner: '你的合伙人权限已暂停。有疑问请联系客服。',
    inviteLabel: '邀请链接暂不可用',
    inviteHint: '暂停期间不能邀请、提现或转换佣金。有疑问请联系客服。'
  }
}

export function resolvePartnerAccessState(agentStatus?: string, riskStatus?: string): PartnerAccessState {
  if (agentStatus !== 'active') return 'not_partner'
  if (riskStatus === 'clear') return 'available'
  if (riskStatus === 'review') return 'under_review'
  return 'suspended'
}

export function getPartnerAccessCopy(state: PartnerAccessState): PartnerAccessCopy {
  return COPY[state]
}
