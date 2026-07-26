<template>
  <section v-if="shouldRender" class="card overflow-hidden">
    <div class="flex flex-col gap-4 border-b border-gray-100 p-5 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
      <div class="flex items-start gap-3">
        <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-950/40 dark:text-primary-300">
          <Icon name="server" size="md" :stroke-width="2" />
        </div>
        <div>
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">月卡运行状态</h2>
            <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium" :class="summaryBadgeClass">
              <span class="h-1.5 w-1.5 rounded-full" :class="summaryDotClass" />
              {{ summaryText }}
            </span>
          </div>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">
            Codex / Claude / Grok 月卡通道最近 {{ windowMinutes }} 分钟运行概览
          </p>
        </div>
      </div>

      <RouterLink
        to="/get-subscription"
        class="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-[rgb(var(--color-terracotta-dark))]/20 bg-[rgb(var(--color-terracotta))] px-4 text-sm font-semibold text-[rgb(var(--color-marble))] shadow-sm shadow-[rgb(var(--color-terracotta))]/20 transition hover:-translate-y-0.5 hover:bg-[rgb(var(--color-terracotta-dark))] hover:shadow-md hover:shadow-[rgb(var(--color-terracotta))]/25 dark:border-[rgb(var(--color-terracotta-dark))]/25 dark:bg-[rgb(var(--color-terracotta))] dark:text-[rgb(var(--color-ink-deep))] dark:shadow-[rgb(var(--color-terracotta))]/10 dark:hover:bg-[rgb(var(--color-terracotta-dark))]"
      >
        <Icon name="creditCard" size="sm" :stroke-width="2" />
        开通月卡
      </RouterLink>
    </div>

    <div class="p-5">
      <div v-if="loading && !snapshot" class="grid gap-4 lg:grid-cols-2">
        <div v-for="idx in 2" :key="idx" class="h-36 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
      </div>

      <div v-else-if="accounts.length === 0" class="rounded-lg border border-dashed border-gray-200 bg-gray-50 p-6 text-center dark:border-dark-700 dark:bg-dark-800/70">
        <p class="text-sm font-medium text-gray-800 dark:text-dark-100">状态正在采集</p>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">月卡通道状态会在这里展示。</p>
      </div>

      <div v-else class="grid gap-4 lg:grid-cols-3">
        <article
          v-for="account in accounts"
          :key="`${account.channel}-${account.display_name}`"
          class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900"
        >
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ account.display_name }}</h3>
                <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-200">
                  {{ account.channel }}
                </span>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-2 text-sm text-gray-500 dark:text-dark-300">
                <span class="inline-flex items-center gap-1.5">
                  <span class="h-2 w-2 rounded-full" :class="statusDotClass(account.status)" />
                  {{ statusLabel(account.status) }}
                </span>
                <span>{{ formatTime(account.latest_checked_at) }}</span>
              </div>
            </div>

            <div class="text-right">
              <div class="font-mono text-2xl font-semibold" :class="uptimeTextClass(account.uptime)">
                {{ formatPercent(account.uptime) }}
              </div>
              <div class="mt-0.5 text-[10px] uppercase tracking-[0.18em] text-gray-400">HEALTH</div>
            </div>
          </div>

          <div class="mt-4">
            <div class="grid grid-cols-[86px_1fr] items-center gap-3 max-sm:grid-cols-1">
              <div class="text-sm font-medium text-gray-600 dark:text-dark-300">{{ account.channel }}</div>
              <div class="grid gap-1" :style="timelineGridStyle">
                <span
                  v-for="slot in timelineSlots(account)"
                  :key="slot.key"
                  class="h-3.5 min-w-0 rounded-full"
                  :class="slotClass(slot.status)"
                  :title="slotTitle(slot)"
                />
              </div>
            </div>
            <div class="mt-2 flex justify-between text-xs text-gray-400">
              <span>{{ windowMinutes }} 分钟前</span>
              <span>现在</span>
            </div>
          </div>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { getMonthlyCardStatus, type MonthlyCardStatus, type MonthlyCardStatusAccount, type MonthlyCardStatusPoint, type MonthlyCardStatusSnapshot } from '@/api/monthlyCardStatus'

type TimelineSlot = {
  key: string
  at: Date
  status: MonthlyCardStatus
  point?: MonthlyCardStatusPoint
}

const fallbackWindowMinutes = 60
const fallbackProbeIntervalMinutes = 2
const snapshot = ref<MonthlyCardStatusSnapshot | null>(null)
const loading = ref(false)
let refreshTimer: number | undefined

const shouldRender = computed(() => snapshot.value?.visible_to_users === true)
const accounts = computed(() => snapshot.value?.accounts ?? [])
const windowMinutes = computed(() => snapshot.value?.window_minutes || fallbackWindowMinutes)
const probeIntervalMinutes = computed(() => {
  const seconds = snapshot.value?.probe_interval_seconds
  if (!seconds || seconds <= 0) return fallbackProbeIntervalMinutes
  return Math.max(1, Math.round(seconds / 60))
})
const timelineSlotCount = computed(() => Math.max(1, Math.ceil(windowMinutes.value / probeIntervalMinutes.value)))
const timelineGridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${timelineSlotCount.value}, minmax(0, 1fr))`
}))
const hasFailures = computed(() => accounts.value.some((account) =>
  account.status === 'failed' ||
  account.status === 'not_schedulable' ||
  account.status === 'missing' ||
  account.uptime < 0.8
))
const hasWarnings = computed(() => accounts.value.some((account) =>
  account.status === 'slow' ||
  account.status === 'rate_limited' ||
  (account.uptime < 0.98 && account.uptime >= 0.8)
))
const summaryText = computed(() => {
  if (!snapshot.value) return '状态更新中'
  if (accounts.value.length === 0) return '状态采集中'
  if (hasFailures.value) return '部分异常'
  if (hasWarnings.value) return '部分波动'
  return '全部正常'
})
const summaryBadgeClass = computed(() => {
  if (!snapshot.value || accounts.value.length === 0) return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'
  if (hasFailures.value) return 'bg-red-50 text-red-700 dark:bg-red-950/30 dark:text-red-300'
  if (hasWarnings.value) return 'bg-amber-50 text-amber-700 dark:bg-amber-950/30 dark:text-amber-300'
  return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300'
})
const summaryDotClass = computed(() => {
  if (!snapshot.value || accounts.value.length === 0) return 'bg-gray-400'
  if (hasFailures.value) return 'bg-red-500'
  if (hasWarnings.value) return 'bg-amber-500'
  return 'bg-emerald-500'
})

async function loadStatus() {
  loading.value = true
  try {
    snapshot.value = await getMonthlyCardStatus(fallbackWindowMinutes)
  } catch (error) {
    if (!snapshot.value) {
      snapshot.value = {
        enabled: false,
        visible_to_users: false,
        window_minutes: fallbackWindowMinutes,
        probe_interval_seconds: fallbackProbeIntervalMinutes * 60,
        generated_at: new Date().toISOString(),
        accounts: []
      }
    }
  } finally {
    loading.value = false
  }
}

function timelineSlots(account: MonthlyCardStatusAccount): TimelineSlot[] {
  const now = new Date(snapshot.value?.generated_at || Date.now())
  now.setSeconds(0, 0)
  const intervalMs = probeIntervalMinutes.value * 60_000
  const pointsBySlotDistance = new Map<number, MonthlyCardStatusPoint>()
  for (const point of account.points) {
    const date = new Date(point.checked_at)
    if (Number.isNaN(date.getTime())) continue
    const distanceFromNow = Math.round((now.getTime() - date.getTime()) / intervalMs)
    if (distanceFromNow < 0 || distanceFromNow >= timelineSlotCount.value) continue
    const existing = pointsBySlotDistance.get(distanceFromNow)
    if (!existing || new Date(point.checked_at).getTime() > new Date(existing.checked_at).getTime()) {
      pointsBySlotDistance.set(distanceFromNow, point)
    }
  }

  const slots: TimelineSlot[] = []
  for (let idx = timelineSlotCount.value - 1; idx >= 0; idx--) {
    const at = new Date(now.getTime() - idx * probeIntervalMinutes.value * 60_000)
    const key = at.toISOString()
    const point = pointsBySlotDistance.get(idx)
    slots.push({
      key,
      at,
      status: point?.status || 'missing',
      point
    })
  }
  return slots
}

function statusLabel(status: string): string {
  return ({
    ok: '正常',
    slow: '响应较慢',
    rate_limited: '繁忙',
    failed: '异常',
    not_schedulable: '维护中',
    missing: '无数据',
    '': '无数据'
  } as Record<string, string>)[status] || status
}

function statusDotClass(status: string): string {
  return ({
    ok: 'bg-emerald-500',
    slow: 'bg-amber-500',
    rate_limited: 'bg-amber-500',
    failed: 'bg-red-500',
    not_schedulable: 'bg-gray-400',
    missing: 'bg-gray-300',
    '': 'bg-gray-300'
  } as Record<string, string>)[status] || 'bg-gray-300'
}

function slotClass(status: string): string {
  return ({
    ok: 'bg-emerald-400',
    slow: 'bg-amber-400',
    rate_limited: 'bg-amber-400',
    failed: 'bg-red-400',
    not_schedulable: 'bg-gray-400',
    missing: 'bg-gray-200 dark:bg-dark-600',
    '': 'bg-gray-200 dark:bg-dark-600'
  } as Record<string, string>)[status] || 'bg-gray-200 dark:bg-dark-600'
}

function uptimeTextClass(uptime: number): string {
  if (uptime >= 0.98) return 'text-emerald-600 dark:text-emerald-300'
  if (uptime >= 0.8) return 'text-amber-600 dark:text-amber-300'
  return 'text-red-600 dark:text-red-300'
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${(value * 100).toFixed(1)}%`
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function slotTitle(slot: TimelineSlot): string {
  const time = slot.at.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  return `${time} ${statusLabel(slot.status)}`
}

onMounted(() => {
  void loadStatus()
  refreshTimer = window.setInterval(loadStatus, fallbackProbeIntervalMinutes * 60_000)
})

onUnmounted(() => {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
</script>
