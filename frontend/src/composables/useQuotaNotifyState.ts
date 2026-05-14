import { reactive, ref } from 'vue'
import { adminAPI } from '@/api/admin'

const QUOTA_NOTIFY_DIMS = ['daily', 'weekly', 'total'] as const
type QuotaNotifyDim = (typeof QUOTA_NOTIFY_DIMS)[number]
type QuotaThresholdType = 'fixed' | 'percent'

interface DimState {
  enabled: boolean | null
  threshold: number | null
  thresholdType: QuotaThresholdType | null
}

export function useQuotaNotifyState() {
  const globalEnabled = ref(false)
  const state = reactive<Record<QuotaNotifyDim, DimState>>({
    daily: { enabled: null, threshold: null, thresholdType: null },
    weekly: { enabled: null, threshold: null, thresholdType: null },
    total: { enabled: null, threshold: null, thresholdType: null }
  })

  function loadGlobalState() {
    adminAPI.settings
      .getSettings()
      .then((settings) => {
        globalEnabled.value = settings.account_quota_notify_enabled === true
      })
      .catch(() => {
        globalEnabled.value = false
      })
  }

  function loadFromExtra(extra: Record<string, unknown> | null | undefined) {
    for (const dim of QUOTA_NOTIFY_DIMS) {
      state[dim].enabled = (extra?.[`quota_notify_${dim}_enabled`] as boolean) ?? null
      state[dim].threshold = (extra?.[`quota_notify_${dim}_threshold`] as number) ?? null
      state[dim].thresholdType =
        (extra?.[`quota_notify_${dim}_threshold_type`] as QuotaThresholdType) ?? null
    }
  }

  function writeToExtra(extra: Record<string, unknown>, mode: 'create' | 'update') {
    for (const dim of QUOTA_NOTIFY_DIMS) {
      const dimState = state[dim]
      if (dimState.enabled) {
        extra[`quota_notify_${dim}_enabled`] = true
        if (dimState.threshold != null) {
          extra[`quota_notify_${dim}_threshold`] = dimState.threshold
        } else if (mode === 'update') {
          delete extra[`quota_notify_${dim}_threshold`]
        }
        extra[`quota_notify_${dim}_threshold_type`] = dimState.thresholdType || 'fixed'
      } else if (mode === 'update') {
        delete extra[`quota_notify_${dim}_enabled`]
        delete extra[`quota_notify_${dim}_threshold`]
        delete extra[`quota_notify_${dim}_threshold_type`]
      }
    }
  }

  function reset() {
    for (const dim of QUOTA_NOTIFY_DIMS) {
      state[dim].enabled = null
      state[dim].threshold = null
      state[dim].thresholdType = null
    }
  }

  return { globalEnabled, state, loadGlobalState, loadFromExtra, writeToExtra, reset }
}
