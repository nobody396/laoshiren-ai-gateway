import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { getAffiliateQualification, type AffiliateAgentQualification } from '@/api/agent'
import { useAffiliateProgramStore } from '@/stores/affiliateProgram'

vi.mock('@/api/agent', () => ({
  getAffiliateQualification: vi.fn()
}))

function qualification(program_mode: AffiliateAgentQualification['program_mode']): AffiliateAgentQualification {
  return {
    user_id: 1,
    program_mode,
    agent_status: 'none',
    risk_status: 'clear',
    self_consumption_micros: 0,
    direct_team_consumption_micros: 0,
    combined_consumption_micros: 0,
    valid_direct_user_count: 0,
    required_direct_user_count: 10,
    required_per_user_micros: 20_000_000,
    required_direct_team_micros: 1_000_000_000,
    required_combined_micros: 2_000_000_000,
    direct_route_qualified: false,
    combined_route_qualified: false,
    qualified: false,
    can_activate: false,
    can_apply: false
  }
}

describe('useAffiliateProgramStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getAffiliateQualification).mockReset()
  })

  it('默认失败关闭，不展示联盟入口', () => {
    const store = useAffiliateProgramStore()

    expect(store.loaded).toBe(false)
    expect(store.mode).toBe('off')
    expect(store.isLive).toBe(false)
  })

  it.each(['off', 'shadow'] as const)('%s 模式不展示联盟入口', async (mode) => {
    vi.mocked(getAffiliateQualification).mockResolvedValue(qualification(mode))
    const store = useAffiliateProgramStore()

    await store.refresh()

    expect(store.loaded).toBe(true)
    expect(store.mode).toBe(mode)
    expect(store.isLive).toBe(false)
  })

  it('只有 live 模式展示联盟入口', async () => {
    vi.mocked(getAffiliateQualification).mockResolvedValue(qualification('live'))
    const store = useAffiliateProgramStore()

    await store.refresh()

    expect(store.isLive).toBe(true)
  })

  it('接口失败时保持失败关闭', async () => {
    vi.mocked(getAffiliateQualification).mockRejectedValue(new Error('network unavailable'))
    const store = useAffiliateProgramStore()

    await expect(store.refresh()).rejects.toThrow('network unavailable')

    expect(store.loaded).toBe(false)
    expect(store.isLive).toBe(false)
  })
})
