<template>
  <RouterLink
    v-if="inbox.pendingCount > 0"
    to="/admin/feedbacks"
    data-testid="admin-feedback-alert"
    class="relative flex items-center gap-1 rounded-lg px-2 py-1.5 text-xs font-medium text-amber-800 transition-colors hover:bg-amber-50 dark:text-amber-300 dark:hover:bg-amber-950/40"
    :aria-label="t('feedback.admin.pendingInbox', { count: inbox.pendingCount })"
    :title="t('feedback.admin.pendingInbox', { count: inbox.pendingCount })"
  >
    <Icon name="inbox" size="sm" />
    <span class="hidden xl:inline">{{ t('feedback.admin.pendingReview') }}</span>
    <span class="absolute -right-1 -top-1 min-w-4 rounded-full bg-red-500 px-1 text-center text-[10px] font-bold leading-4 text-white">
      {{ inbox.pendingCount > 99 ? '99+' : inbox.pendingCount }}
    </span>
  </RouterLink>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAdminFeedbackInboxStore } from '@/stores/adminFeedbackInbox'

const { t } = useI18n()
const inbox = useAdminFeedbackInboxStore()
let timer: ReturnType<typeof setInterval> | undefined

function refreshWhenVisible() {
  if (document.visibilityState === 'visible') void inbox.refresh()
}

onMounted(() => {
  refreshWhenVisible()
  timer = setInterval(refreshWhenVisible, 60_000)
  document.addEventListener('visibilitychange', refreshWhenVisible)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', refreshWhenVisible)
})
</script>
