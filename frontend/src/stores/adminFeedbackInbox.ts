import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listAgentQueue } from '@/api/admin/feedbacks'
import type { FeedbackItem } from '@/types'

export const useAdminFeedbackInboxStore = defineStore('adminFeedbackInbox', () => {
  const items = ref<FeedbackItem[]>([])
  const pendingCount = ref(0)
  const loading = ref(false)
  const loaded = ref(false)
  let refreshPromise: Promise<void> | null = null

  async function refresh(): Promise<void> {
    if (refreshPromise) return refreshPromise

    loading.value = true
    refreshPromise = listAgentQueue({ page: 1, pageSize: 5 })
      .then((result) => {
        items.value = result.items
        pendingCount.value = result.total
        loaded.value = true
      })
      .catch(() => {
        // 顶部提醒属于辅助信息。瞬时网络错误时保留最近一次成功结果，避免角标闪烁归零。
      })
      .finally(() => {
        loading.value = false
        refreshPromise = null
      })

    return refreshPromise
  }

  function reset(): void {
    items.value = []
    pendingCount.value = 0
    loading.value = false
    loaded.value = false
    refreshPromise = null
  }

  return { items, pendingCount, loading, loaded, refresh, reset }
})
