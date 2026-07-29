import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  getAffiliateQualification,
  type AffiliateAgentQualification
} from '@/api/agent'

export type AffiliateProgramMode = AffiliateAgentQualification['program_mode']

/**
 * 联盟计划对用户端采用失败关闭策略：
 * 只有服务端明确返回 live，侧边栏才展示入口，页面才展示邀请与奖励信息。
 */
export const useAffiliateProgramStore = defineStore('affiliateProgram', () => {
  const qualification = ref<AffiliateAgentQualification | null>(null)
  const loading = ref(false)
  const loaded = ref(false)
  let refreshPromise: Promise<AffiliateAgentQualification> | null = null

  const mode = computed<AffiliateProgramMode>(() => qualification.value?.program_mode ?? 'off')
  const isLive = computed(() => loaded.value && mode.value === 'live')

  async function refresh(force = false): Promise<AffiliateAgentQualification> {
    if (!force && qualification.value) return qualification.value
    if (refreshPromise) return refreshPromise

    loading.value = true
    refreshPromise = getAffiliateQualification()
      .then((current) => {
        qualification.value = current
        loaded.value = true
        return current
      })
      .finally(() => {
        loading.value = false
        refreshPromise = null
      })
    return refreshPromise
  }

  function setQualification(current: AffiliateAgentQualification): void {
    qualification.value = current
    loaded.value = true
  }

  function reset(): void {
    qualification.value = null
    loaded.value = false
    loading.value = false
    refreshPromise = null
  }

  return {
    qualification,
    loading,
    loaded,
    mode,
    isLive,
    refresh,
    setQualification,
    reset
  }
})
