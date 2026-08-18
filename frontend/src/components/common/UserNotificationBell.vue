<template>
  <div>
    <button ref="buttonRef" class="relative flex items-center gap-1 rounded-lg px-2 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-800" :aria-label="t('notifications.title')" @click="toggle">
      <Icon name="bell" size="sm" />
      <span class="hidden sm:inline">{{ t('notifications.title') }}</span>
      <span v-if="unreadCount" class="absolute -right-1 -top-1 min-w-4 rounded-full bg-red-500 px-1 text-center text-[10px] font-bold leading-4 text-white">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
    </button>
    <Teleport to="body">
      <!-- 指向铃铛的小箭头，与公告的居中弹窗形成视觉区分 -->
      <div
        v-if="open"
        aria-hidden="true"
        :style="caretStyle"
        class="fixed z-[121] h-2.5 w-2.5 rotate-45 border-l border-t border-primary-200/70 bg-primary-50 dark:border-dark-600 dark:bg-dark-700"
      ></div>
      <div
        v-if="open"
        ref="panelRef"
        data-testid="user-notification-panel"
        :style="panelStyle"
        class="fixed z-[120] overflow-hidden rounded-2xl border border-primary-200/70 bg-white shadow-xl dark:border-dark-600 dark:bg-dark-800"
      >
        <div class="flex items-center justify-between border-b border-primary-100/80 bg-primary-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-700">
          <div class="flex items-center gap-2">
            <span class="flex h-6 w-6 items-center justify-center rounded-md bg-primary-500 text-white">
              <Icon name="bell" size="xs" />
            </span>
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('notifications.title') }}</h3>
          </div>
          <button v-if="unreadCount" class="text-xs text-primary-600 hover:underline dark:text-primary-300" @click="markAll">{{ t('notifications.markAllRead') }}</button>
        </div>
        <div class="max-h-[min(24rem,calc(100vh-8rem))] overflow-y-auto">
          <button v-for="item in items" :key="item.id" class="block w-full border-b border-gray-100 px-4 py-3 text-left hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700" :class="{ 'bg-primary-50/50 dark:bg-primary-900/10': !item.read_at }" @click="openItem(item)">
            <div class="flex items-start justify-between gap-3"><span class="min-w-0 break-words text-sm font-medium text-gray-900 dark:text-white">{{ item.title }}</span><span v-if="!item.read_at" class="mt-1 h-2 w-2 flex-none rounded-full bg-primary-500"></span></div>
            <p class="mt-1 break-words text-xs text-gray-600 dark:text-dark-300">{{ item.body }}</p>
            <p class="mt-1 text-[11px] text-gray-400">{{ formatDateTime(item.created_at) }}</p>
          </button>
          <div v-if="!loading && items.length === 0" class="px-4 py-10 text-center text-sm text-gray-500">{{ t('notifications.empty') }}</div>
          <div v-if="loading" class="px-4 py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { listNotifications, markAllNotificationsRead, markNotificationRead } from '@/api/feedbacks'
import { formatDateTime } from '@/utils/format'
import type { UserNotification } from '@/types'

const { t } = useI18n()
const router = useRouter()
const open = ref(false)
const buttonRef = ref<HTMLButtonElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const panelStyle = ref<Record<string, string>>({})
const caretStyle = ref<Record<string, string>>({})
const loading = ref(false)
const items = ref<UserNotification[]>([])
const unreadCount = ref(0)
let timer: number | undefined

async function load() {
  try {
    const result = await listNotifications()
    items.value = result.items
    unreadCount.value = result.unread_count
  } catch { /* Header notifications are best effort. */ }
}
function updatePanelPosition() {
  if (!open.value || !buttonRef.value) return
  const viewportWidth = document.documentElement.clientWidth || window.innerWidth
  const width = Math.min(384, Math.max(0, viewportWidth - 32))
  const rect = buttonRef.value.getBoundingClientRect()
  const maxLeft = Math.max(16, viewportWidth - width - 16)
  const left = Math.min(Math.max(16, rect.right - width), maxLeft)
  const top = Math.max(16, rect.bottom + 8)
  panelStyle.value = {
    left: `${left}px`,
    top: `${top}px`,
    width: `${width}px`,
  }
  caretStyle.value = {
    left: `${rect.left + rect.width / 2 - 5}px`,
    top: `${top - 5}px`,
  }
}
async function toggle() {
  open.value = !open.value
  if (open.value) {
    updatePanelPosition()
    loading.value = true
    await load()
    loading.value = false
  }
}
async function openItem(item: UserNotification) {
  if (!item.read_at) { await markNotificationRead(item.id); item.read_at = new Date().toISOString(); unreadCount.value = Math.max(0, unreadCount.value - 1) }
  open.value = false
  if (item.action_url) await router.push(item.action_url)
}
async function markAll() { await markAllNotificationsRead(); items.value = items.value.map((item) => ({ ...item, read_at: item.read_at || new Date().toISOString() })); unreadCount.value = 0 }

// 点击面板和铃铛按钮以外的任何位置（公告、客服等其他组件）时关闭
function handleClickOutside(event: MouseEvent) {
  if (!open.value) return
  const target = event.target as Node | null
  if (!target) return
  if (buttonRef.value?.contains(target) || panelRef.value?.contains(target)) return
  open.value = false
}
function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') open.value = false
}

onMounted(() => {
  void load()
  timer = window.setInterval(load, 60_000)
  window.addEventListener('resize', updatePanelPosition)
  window.addEventListener('scroll', updatePanelPosition, true)
  document.addEventListener('mousedown', handleClickOutside)
  document.addEventListener('keydown', handleEscape)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  window.removeEventListener('resize', updatePanelPosition)
  window.removeEventListener('scroll', updatePanelPosition, true)
  document.removeEventListener('mousedown', handleClickOutside)
  document.removeEventListener('keydown', handleEscape)
})
</script>
