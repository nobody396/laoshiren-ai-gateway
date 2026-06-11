<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-4 border-b border-gray-100 px-6 py-5 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex items-start gap-4">
            <div class="flex h-12 w-12 items-center justify-center rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
              <span class="text-xl">⌁</span>
            </div>
            <div>
              <div class="flex flex-wrap items-center gap-3">
                <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">月卡监控</h1>
                <span class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium" :class="summaryBadgeClass">
                  <span class="h-2 w-2 rounded-full" :class="summaryDotClass" />
                  {{ summaryText }}
                </span>
              </div>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-300">
                最近更新时间 {{ latestUpdateText }}，展示最近 {{ windowMinutes }} 分钟，每 {{ probeIntervalMinutes }} 分钟一个状态区块
              </p>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-3">
            <label class="flex items-center gap-3 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
              <Toggle :model-value="enabled" :disabled="saving" @update:model-value="handleToggle" />
              <span class="text-sm font-medium text-gray-700 dark:text-dark-200">常驻探针</span>
            </label>
            <label class="flex items-center gap-3 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
              <Toggle :model-value="publicStatusEnabled" :disabled="saving" @update:model-value="handlePublicStatusToggle" />
              <span class="text-sm font-medium text-gray-700 dark:text-dark-200">用户可见</span>
            </label>
            <button
              type="button"
              class="inline-flex h-10 items-center justify-center rounded-lg border border-gray-200 px-4 text-sm font-medium text-gray-700 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700"
              :disabled="loading"
              @click="loadSnapshot"
            >
              刷新
            </button>
          </div>
        </div>

        <div class="p-5">
          <div v-if="loading && !snapshot" class="space-y-4">
            <div v-for="idx in 2" :key="idx" class="h-40 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
          </div>

          <div v-else-if="accounts.length === 0" class="rounded-lg border border-dashed border-gray-300 px-6 py-12 text-center dark:border-dark-600">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">暂无探针数据</h2>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-300">
              开启常驻探针后，服务器会每 {{ probeIntervalMinutes }} 分钟检测 Codex 和 Claude 月卡通道。
            </p>
          </div>

          <div v-else class="space-y-4">
            <article
              v-for="account in accounts"
              :key="account.account_name"
              class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
            >
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <div class="flex flex-wrap items-center gap-2">
                    <h2 class="text-xl font-semibold text-gray-900 dark:text-white">{{ displayAccountName(account) }}</h2>
                    <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-200">
                      {{ platformLabel(account.platform) }}
                    </span>
                    <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-200">
                      {{ account.model }}
                    </span>
                  </div>
                  <div class="mt-3 flex flex-wrap items-center gap-3 text-sm text-gray-500 dark:text-dark-300">
                    <span class="text-xs font-medium uppercase tracking-[0.14em] text-gray-400">网关主探针</span>
                    <span class="inline-flex items-center gap-2">
                      <span class="h-2 w-2 rounded-full" :class="statusDotClass(account.latest_status)" />
                      {{ statusLabel(account.latest_status) }}
                    </span>
                    <span>HTTP {{ account.latest_http_status ?? '-' }}</span>
                    <span>{{ account.latest_latency_ms || 0 }} ms</span>
                    <span>{{ formatTime(account.latest_checked_at) }}</span>
                  </div>
                </div>
                <div class="text-left lg:text-right">
                  <div class="font-mono text-3xl font-semibold" :class="uptimeTextClass(account.uptime)">
                    {{ formatPercent(account.uptime) }}
                  </div>
                  <div class="mt-1 text-xs uppercase tracking-[0.18em] text-gray-400">UPTIME</div>
                </div>
              </div>

              <div class="mt-5">
                <div class="grid grid-cols-[180px_1fr] items-center gap-4 max-md:grid-cols-1">
                  <div class="text-sm font-medium text-gray-600 dark:text-dark-300">
                    {{ platformLabel(account.platform) }}
                  </div>
                  <div class="grid gap-1" :style="timelineGridStyle">
                    <span
                      v-for="slot in timelineSlots(account)"
                      :key="slot.key"
                      class="h-4 min-w-0 rounded-full"
                      :class="slotClass(slot.status)"
                      :title="slotTitle(slot)"
                    />
                  </div>
                </div>
                <div class="mt-3 flex justify-between text-xs text-gray-400">
                  <span>{{ windowMinutes }} 分钟前</span>
                  <span>现在</span>
                </div>
                <p v-if="account.latest_error" class="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
                  {{ account.latest_error_code || 'probe_error' }}：{{ account.latest_error }}
                </p>
                <p
                  v-if="account.latest_direct_upstream"
                  class="mt-3 rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-300"
                >
                  直连上游诊断：
                  {{ statusLabel(account.latest_direct_upstream.status) }}
                  · HTTP {{ account.latest_direct_upstream.http_status ?? '-' }}
                  · {{ account.latest_direct_upstream.latency_ms || 0 }} ms
                  · {{ formatTime(account.latest_direct_upstream.checked_at) }}
                  <span v-if="directDiagnosticError(account)" class="text-red-600 dark:text-red-300">
                    · {{ directDiagnosticError(account) }}
                  </span>
                </p>
              </div>

              <div v-if="account.cost_estimate" class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-700">
                <div class="grid grid-cols-2 gap-x-6 gap-y-3 lg:grid-cols-4">
                  <div>
                    <div class="text-xs text-gray-400">单次估算</div>
                    <div class="font-mono text-sm font-semibold text-gray-900 dark:text-white">${{ formatCost(account.cost_estimate.actual_cost_per_probe) }}</div>
                  </div>
                  <div>
                    <div class="text-xs text-gray-400">每分钟</div>
                    <div class="font-mono text-sm font-semibold text-gray-900 dark:text-white">${{ formatCost(account.cost_estimate.actual_cost_per_minute) }}</div>
                  </div>
                  <div>
                    <div class="text-xs text-gray-400">每小时</div>
                    <div class="font-mono text-sm font-semibold text-gray-900 dark:text-white">${{ formatCost(account.cost_estimate.actual_cost_per_hour) }}</div>
                  </div>
                  <div>
                    <div class="text-xs text-gray-400">每天</div>
                    <div class="font-mono text-sm font-semibold text-gray-900 dark:text-white">${{ formatCost(account.cost_estimate.actual_cost_per_day) }}</div>
                  </div>
                </div>
                <p class="mt-3 text-xs text-gray-500 dark:text-dark-300">
                  按 {{ account.cost_estimate.input_tokens }} input + {{ account.cost_estimate.output_tokens }} output tokens，
                  账号倍率 {{ formatMultiplier(account.cost_estimate.rate_multiplier) }}；
                  {{ account.cost_estimate.estimate_note }}
                </p>
              </div>
            </article>

            <div
              v-if="totalCostEstimate"
              class="border-t border-gray-100 pt-4 text-sm text-gray-600 dark:border-dark-700 dark:text-dark-300"
            >
              全量探针成本估算：
              <span class="font-mono font-semibold text-gray-900 dark:text-white">${{ formatCost(totalCostEstimate.probe) }}</span>
              / 次，
              <span class="font-mono font-semibold text-gray-900 dark:text-white">${{ formatCost(totalCostEstimate.minute) }}</span>
              / 分钟，
              <span class="font-mono font-semibold text-gray-900 dark:text-white">${{ formatCost(totalCostEstimate.hour) }}</span>
              / 小时，
              <span class="font-mono font-semibold text-gray-900 dark:text-white">${{ formatCost(totalCostEstimate.day) }}</span>
              / 天。
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { MonthlyUpstreamProbeAccount, MonthlyUpstreamProbePoint, MonthlyUpstreamProbeSnapshot, MonthlyUpstreamProbeStatus } from '@/api/admin/monthlyUpstreams'
import { useAppStore } from '@/stores/app'

type TimelineSlot = {
  key: string
  at: Date
  status: MonthlyUpstreamProbeStatus | 'missing'
  point?: MonthlyUpstreamProbePoint
}

const appStore = useAppStore()
const windowMinutes = 60
const fallbackProbeIntervalMinutes = 2
const snapshot = ref<MonthlyUpstreamProbeSnapshot | null>(null)
const loading = ref(false)
const saving = ref(false)
let refreshTimer: number | undefined

const accounts = computed(() => snapshot.value?.accounts ?? [])
const enabled = computed(() => Boolean(snapshot.value?.enabled))
const publicStatusEnabled = computed(() => Boolean(snapshot.value?.public_status_enabled))
const probeIntervalMinutes = computed(() => {
  const seconds = accounts.value.find((account) => account.cost_estimate)?.cost_estimate?.probe_interval_seconds
  if (!seconds || seconds <= 0) return fallbackProbeIntervalMinutes
  return Math.max(1, Math.round(seconds / 60))
})
const timelineSlotCount = computed(() => Math.max(1, Math.ceil(windowMinutes / probeIntervalMinutes.value)))
const timelineGridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${timelineSlotCount.value}, minmax(0, 1fr))`
}))
const totalCostEstimate = computed(() => {
  const totals = accounts.value.reduce(
    (acc, account) => {
      if (!account.cost_estimate) return acc
      acc.probe += account.cost_estimate.actual_cost_per_probe
      acc.minute += account.cost_estimate.actual_cost_per_minute
      acc.hour += account.cost_estimate.actual_cost_per_hour
      acc.day += account.cost_estimate.actual_cost_per_day
      return acc
    },
    { probe: 0, minute: 0, hour: 0, day: 0 }
  )
  return totals.probe > 0 ? totals : null
})

const latestUpdateText = computed(() => {
  const latest = accounts.value
    .map((account) => account.latest_checked_at)
    .filter((value): value is string => Boolean(value))
    .sort()
    .at(-1)
  return formatTime(latest)
})

const hasFailures = computed(() => accounts.value.some((account) => account.latest_status === 'failed' || account.latest_status === 'not_schedulable'))
const hasWarnings = computed(() => accounts.value.some((account) => account.latest_status === 'slow' || account.latest_status === 'rate_limited'))

const summaryText = computed(() => {
  if (!enabled.value) return '监控已关闭'
  if (accounts.value.length === 0) return '等待数据'
  if (hasFailures.value) return '部分月卡异常'
  if (hasWarnings.value) return '部分月卡降级'
  return '全部正常'
})

const summaryBadgeClass = computed(() => {
  if (!enabled.value) return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'
  if (hasFailures.value) return 'bg-red-50 text-red-700 dark:bg-red-950/30 dark:text-red-300'
  if (hasWarnings.value) return 'bg-amber-50 text-amber-700 dark:bg-amber-950/30 dark:text-amber-300'
  return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300'
})

const summaryDotClass = computed(() => {
  if (!enabled.value) return 'bg-gray-400'
  if (hasFailures.value) return 'bg-red-500'
  if (hasWarnings.value) return 'bg-amber-500'
  return 'bg-emerald-500'
})

async function loadSnapshot() {
  loading.value = true
  try {
    snapshot.value = await adminAPI.monthlyUpstreams.getSnapshot(windowMinutes)
  } catch (error) {
    appStore.showError('加载月卡监控失败')
  } finally {
    loading.value = false
  }
}

async function handleToggle(value: boolean) {
  saving.value = true
  try {
    await adminAPI.monthlyUpstreams.updateSettings({ enabled: value })
    await loadSnapshot()
    appStore.showSuccess(value ? '月卡探针已开启' : '月卡探针已关闭')
  } catch (error) {
    appStore.showError('更新月卡探针开关失败')
  } finally {
    saving.value = false
  }
}

async function handlePublicStatusToggle(value: boolean) {
  saving.value = true
  try {
    await adminAPI.monthlyUpstreams.updateSettings({ public_status_enabled: value })
    await loadSnapshot()
    appStore.showSuccess(value ? '用户端运行状态已显示' : '用户端运行状态已隐藏')
  } catch (error) {
    appStore.showError('更新用户端运行状态开关失败')
  } finally {
    saving.value = false
  }
}

function timelineSlots(account: MonthlyUpstreamProbeAccount): TimelineSlot[] {
  const now = new Date(snapshot.value?.generated_at || Date.now())
  now.setSeconds(0, 0)
  const intervalMs = probeIntervalMinutes.value * 60_000
  const pointsBySlotDistance = new Map<number, MonthlyUpstreamProbePoint>()
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

function displayAccountName(account: MonthlyUpstreamProbeAccount): string {
  if (account.platform === 'openai') return 'Codex 月卡'
  if (account.platform === 'anthropic') return 'Claude 月卡'
  return account.account_name
}

function platformLabel(platform: string): string {
  if (platform === 'openai') return 'Codex'
  if (platform === 'anthropic') return 'Claude'
  return platform || '-'
}

function statusLabel(status: string): string {
  return ({
    ok: '正常',
    slow: '响应较慢',
    rate_limited: '限流',
    failed: '异常',
    not_schedulable: '不可调度',
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
    missing: 'bg-gray-200 dark:bg-dark-600'
  } as Record<string, string>)[status] || 'bg-gray-200 dark:bg-dark-600'
}

function uptimeTextClass(uptime: number): string {
  if (uptime >= 0.98) return 'text-emerald-600 dark:text-emerald-300'
  if (uptime >= 0.8) return 'text-amber-600 dark:text-amber-300'
  return 'text-red-600 dark:text-red-300'
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function formatCost(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0.00000000'
  if (value >= 1) return value.toFixed(4)
  if (value >= 0.0001) return value.toFixed(6)
  return value.toFixed(8)
}

function formatMultiplier(value: number): string {
  if (!Number.isFinite(value)) return 'x1'
  return `x${value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}`
}

function directDiagnosticError(account: MonthlyUpstreamProbeAccount): string {
  const diagnostic = account.latest_direct_upstream
  if (!diagnostic) return ''
  return diagnostic.error_code || diagnostic.error_message || ''
}

function slotTitle(slot: TimelineSlot): string {
  const time = slot.at.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  if (!slot.point) return `${time} 无数据`
  const http = slot.point.http_status == null ? '-' : slot.point.http_status
  const error = slot.point.error_code || slot.point.error_message
  return `${time} ${statusLabel(slot.status)} HTTP ${http} ${slot.point.latency_ms}ms${error ? ` ${error}` : ''}`
}

onMounted(() => {
  void loadSnapshot()
  refreshTimer = window.setInterval(loadSnapshot, probeIntervalMinutes.value * 60_000)
})

onUnmounted(() => {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
</script>
