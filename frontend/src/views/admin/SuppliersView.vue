<template>
  <AppLayout>
    <div class="space-y-4">
          <section class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ t('admin.suppliers.monitorTitle', '供应商探针') }}
                  </h2>
                  <span :class="probeBadgeClass(snapshotOverallStatus)">
                    {{ probeStatusLabel(snapshotOverallStatus) }}
                  </span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">
                    {{ probeSnapshot ? formatDateTime(probeSnapshot.generated_at) : t('common.noData', '暂无数据') }}
                  </span>
                </div>
              </div>

              <div class="flex flex-wrap items-center gap-2">
                <button class="btn btn-secondary" :disabled="snapshotLoading" @click="loadProbeSnapshot">
                  <Icon name="refresh" size="md" :class="snapshotLoading ? 'animate-spin' : ''" />
                </button>
                <button class="btn btn-secondary" :disabled="syncingAccounts" @click="syncAccountsToSuppliers">
                  <Icon name="download" size="md" class="mr-2" />
                  {{ t('admin.suppliers.syncAccounts', '同步账号') }}
                </button>
                <button class="btn btn-secondary" :disabled="bulkSaving" @click="bulkToggleProbes(true)">
                  {{ t('admin.suppliers.enableAllProbes', '开启全部探针') }}
                </button>
                <button class="btn btn-secondary" :disabled="bulkSaving" @click="bulkToggleProbes(false)">
                  {{ t('admin.suppliers.disableAllProbes', '关闭全部探针') }}
                </button>
                <button class="btn btn-primary" :disabled="batchProbing" @click="runAllEnabledProbes">
                  <Icon name="refresh" size="md" :class="batchProbing ? 'animate-spin mr-2' : 'mr-2'" />
                  {{ t('admin.suppliers.probeAll', '检测已启用') }}
                </button>
              </div>
            </div>

            <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
              <div
                v-for="card in probeMetricCards"
                :key="card.label"
                class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</div>
                <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ card.hint }}</div>
              </div>
            </div>

            <div class="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(320px,0.7fr)]">
              <div>
                <div class="mb-2 flex items-center justify-between">
                  <div class="text-sm font-medium text-gray-800 dark:text-gray-200">
                    {{ t('admin.suppliers.hourlyStability', '每日小时稳定性') }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.suppliers.hourlyWindow', { days: probeSnapshot?.days || 7 }, `最近 ${probeSnapshot?.days || 7} 天`) }}
                  </div>
                </div>
                <div class="grid grid-cols-6 gap-1 sm:grid-cols-12">
                  <div
                    v-for="bucket in hourlyBuckets"
                    :key="bucket.hour"
                    class="h-12 rounded border px-1 py-1 text-center text-[10px] leading-tight"
                    :class="hourBucketClass(bucket)"
                    :title="hourBucketTitle(bucket)"
                  >
                    <div>{{ `${String(bucket.hour).padStart(2, '0')}` }}</div>
                    <div class="mt-1 font-semibold">{{ bucket.total > 0 ? formatPercent(bucket.success_rate) : '-' }}</div>
                  </div>
                </div>
              </div>

              <div>
                <div class="mb-2 text-sm font-medium text-gray-800 dark:text-gray-200">
                  {{ t('admin.suppliers.riskHours', '更容易出事的时段') }}
                </div>
                <div class="space-y-2">
                  <div
                    v-for="bucket in riskyHours"
                    :key="bucket.hour"
                    class="flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900"
                  >
                    <span class="font-medium text-gray-800 dark:text-gray-200">{{ bucket.label }}</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ bucket.failed + bucket.degraded }}/{{ bucket.total }} · {{ formatPercent(bucket.success_rate) }}
                    </span>
                  </div>
                  <div v-if="riskyHours.length === 0" class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
                    {{ t('admin.suppliers.noRiskHours', '还没有足够探针数据') }}
                  </div>
                </div>
              </div>
            </div>

            <div class="mt-5">
              <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                <div class="text-sm font-medium text-gray-800 dark:text-gray-200">
                  {{ t('admin.suppliers.monitorCards', '渠道状态') }}
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.suppliers.monitorCardsHint', '近 60 次记录 · 每 2 分钟探测') }}
                </div>
              </div>

              <div class="grid gap-3 lg:grid-cols-2 2xl:grid-cols-3">
                <article
                  v-for="item in monitoredSupplierRows"
                  :key="item.id"
                  role="button"
                  tabindex="0"
                  class="group flex min-h-[246px] w-full cursor-pointer flex-col rounded-xl border bg-white p-4 text-left shadow-sm transition hover:-translate-y-0.5 hover:border-primary-200 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:bg-dark-900/70 dark:focus:ring-offset-dark-800"
                  :class="probeCardClass(item)"
                  @click="openProbeDetail(item)"
                  @keydown.enter.prevent="openProbeDetail(item)"
                  @keydown.space.prevent="openProbeDetail(item)"
                >
                  <div class="flex items-start gap-3">
                    <span
                      class="grid h-10 w-10 shrink-0 place-items-center rounded-lg text-sm font-semibold ring-1 ring-black/5 dark:ring-white/10"
                      :class="providerVisualClass(item.source_platform)"
                    >
                      {{ providerInitial(item.source_platform) }}
                    </span>
                    <div class="min-w-0 flex-1">
                      <div class="flex min-w-0 flex-wrap items-center gap-2">
                        <div class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ item.name }}</div>
                        <span :class="displayProbeBadgeClass(item)">
                          {{ displayProbeStatusLabel(item) }}
                        </span>
                      </div>
                      <div class="mt-1 flex min-w-0 flex-wrap items-center gap-1.5">
                        <span
                          class="inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium"
                          :class="providerBadgeClass(item.source_platform)"
                        >
                          {{ providerLabel(item.source_platform) }}
                        </span>
                        <span class="truncate font-mono text-xs text-gray-500 dark:text-gray-400">{{ item.probe_model || '-' }}</span>
                      </div>
                    </div>
                    <div class="shrink-0" @click.stop @keydown.stop>
                      <button
                        type="button"
                        role="switch"
                        :aria-checked="item.probe_enabled"
                        :aria-label="item.probe_enabled ? '关闭这个探针' : '开启这个探针'"
                        :disabled="isSingleProbeSaving(item.id)"
                        class="relative inline-flex h-6 w-11 flex-shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60 dark:focus:ring-offset-dark-800"
                        :class="item.probe_enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'"
                        @click="toggleSingleProbe(item, !item.probe_enabled)"
                      >
                        <span
                          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                          :class="item.probe_enabled ? 'translate-x-5' : 'translate-x-0'"
                        />
                      </button>
                    </div>
                  </div>

                  <div class="mt-3 flex min-h-[24px] flex-wrap gap-1">
                    <GroupBadge
                      v-for="group in itemTargetGroups(item)"
                      :key="group.id"
                      :name="group.name"
                      :platform="group.platform"
                      :subscription-type="group.subscription_type"
                      :rate-multiplier="group.rate_multiplier"
                      class="max-w-32"
                    />
                    <span v-if="itemTargetGroups(item).length === 0" class="text-xs text-gray-400">-</span>
                  </div>

                  <div class="mt-3 grid gap-2 sm:grid-cols-2">
                    <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                      <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.suppliers.cardLastLatency', '最近延迟') }}</div>
                      <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatLatencyText(item.last_probe_latency_ms) }}</div>
                    </div>
                    <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                      <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.suppliers.cardWindowLatency', '窗口均值') }}</div>
                      <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatLatencyText(item.window_average_latency_ms) }}</div>
                    </div>
                  </div>

                  <div class="mt-3 border-t border-gray-100 pt-3 dark:border-dark-700">
                    <div class="flex items-end justify-between gap-3">
                      <div class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t('admin.suppliers.cardAvailability', '可用性') }} · {{ monitorWindowLabel }}
                      </div>
                      <div class="text-2xl font-semibold tabular-nums" :class="cardAvailabilityClass(item)">
                        {{ item.probe_enabled && item.window_total > 0 ? formatPercent(item.window_success_rate) : '-' }}
                      </div>
                    </div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ item.probe_enabled ? `${item.window_success}/${item.window_total}` : t('admin.suppliers.probeDisabledInline', '探针已关闭') }} · {{ nextProbeText(item) }}
                    </div>
                  </div>

                  <div class="mt-auto pt-3">
                    <div class="mb-2 flex justify-between text-[10px] font-semibold uppercase tracking-widest text-gray-400">
                      <span>{{ t('admin.suppliers.cardRecentRecords', '近 60 次记录') }}</span>
                      <span>{{ formatShortDateTime(item.last_probe_at) }}</span>
                    </div>
                    <div class="flex h-6 w-full items-end gap-[2px]">
                      <span
                        v-for="slot in supplierTimelineSlots(item)"
                        :key="slot.key"
                        class="min-w-[3px] flex-1 rounded-sm"
                        :class="probeSlotClass(slot.status)"
                        :style="probeSlotStyle(slot.status)"
                        :title="probeSlotTitle(slot)"
                      />
                    </div>
                    <div class="mt-1 flex justify-between text-[9px] uppercase tracking-widest text-gray-400">
                      <span>PAST</span>
                      <span>NOW</span>
                    </div>
                  </div>
                </article>

                <div
                  v-if="monitoredSupplierRows.length === 0"
                  class="rounded-xl border border-dashed border-gray-200 px-3 py-8 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400"
                >
                  {{ t('admin.suppliers.noProbeRows', '暂无供应商探针数据') }}
                </div>
              </div>
            </div>
          </section>
        </div>

    <BaseDialog
      :show="showProbeDetailDialog"
      :title="selectedProbeItem ? `${selectedProbeItem.name} · 探针详情` : t('admin.suppliers.probeDetail', '探针详情')"
      width="extra-wide"
      @close="closeProbeDetail"
    >
      <div v-if="selectedProbeItem" class="space-y-5">
        <div class="grid gap-3 md:grid-cols-4">
          <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
            <div class="text-xs text-gray-500 dark:text-gray-400">当前状态</div>
            <div class="mt-2">
              <span :class="displayProbeBadgeClass(selectedProbeItem)">
                {{ displayProbeStatusLabel(selectedProbeItem) }}
              </span>
            </div>
          </div>
          <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
            <div class="text-xs text-gray-500 dark:text-gray-400">窗口成功率</div>
            <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
              {{ selectedProbeItem.window_total > 0 ? formatPercent(selectedProbeItem.window_success_rate) : '-' }}
            </div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ selectedProbeItem.window_success }}/{{ selectedProbeItem.window_total }}
            </div>
          </div>
          <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
            <div class="text-xs text-gray-500 dark:text-gray-400">平均延迟</div>
            <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
              {{ selectedProbeItem.window_average_latency_ms > 0 ? `${selectedProbeItem.window_average_latency_ms}ms` : '-' }}
            </div>
          </div>
          <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
            <div class="text-xs text-gray-500 dark:text-gray-400">下次探针</div>
            <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
              {{ selectedProbeItem.probe_enabled ? formatDateTime(selectedProbeItem.next_probe_at) : '探针已关闭' }}
            </div>
          </div>
        </div>

        <div>
          <div class="mb-2 text-sm font-medium text-gray-800 dark:text-gray-200">对应分组</div>
          <div class="flex flex-wrap gap-1.5">
            <GroupBadge
              v-for="group in itemTargetGroups(selectedProbeItem)"
              :key="group.id"
              :name="group.name"
              :platform="group.platform"
              :subscription-type="group.subscription_type"
              :rate-multiplier="group.rate_multiplier"
            />
            <span v-if="itemTargetGroups(selectedProbeItem).length === 0" class="text-sm text-gray-400">-</span>
          </div>
        </div>

        <div>
          <div class="mb-2 flex items-center justify-between">
            <div class="text-sm font-medium text-gray-800 dark:text-gray-200">最近状态区块</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              每 {{ selectedProbeItem.probe_interval_minutes || defaultProbeIntervalMinutes }} 分钟一个 slot
            </div>
          </div>
          <div class="grid grid-cols-12 gap-1">
            <span
              v-for="slot in supplierTimelineSlots(selectedProbeItem)"
              :key="slot.key"
              class="h-5 min-w-0 rounded"
              :class="probeSlotClass(slot.status)"
              :title="probeSlotTitle(slot)"
            />
          </div>
        </div>

        <div class="overflow-hidden rounded-lg border border-gray-100 dark:border-dark-700">
          <div
            v-if="currentProbeHistoryHiddenCount > 0"
            class="border-b border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300"
          >
            当前表格只显示连续的 {{ selectedProbeIntervalMinutes }} 分钟探测段；已折叠 {{ currentProbeHistoryHiddenCount }} 条改频前或断档历史记录。
          </div>
          <div class="grid grid-cols-[130px_90px_100px_90px_minmax(180px,1fr)] bg-gray-50 px-3 py-2 text-xs font-medium text-gray-500 dark:bg-dark-900 dark:text-gray-400">
            <span>时间</span>
            <span>状态</span>
            <span>HTTP</span>
            <span>延迟</span>
            <span>备注</span>
          </div>
          <div v-if="probeHistoryLoading" class="px-3 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
            加载中...
          </div>
          <template v-else>
            <div
              v-for="result in currentProbeHistoryResults"
              :key="result.id"
              class="grid grid-cols-[130px_90px_100px_90px_minmax(180px,1fr)] items-center border-t border-gray-100 px-3 py-2 text-sm dark:border-dark-700"
            >
              <span class="text-gray-500 dark:text-gray-400">{{ formatShortDateTime(result.checked_at) }}</span>
              <span :class="probeBadgeClass(result.status)">{{ probeStatusLabel(result.status) }}</span>
              <span class="text-gray-600 dark:text-gray-300">{{ result.http_code || '-' }}</span>
              <span class="text-gray-600 dark:text-gray-300">{{ result.latency_ms ? `${result.latency_ms}ms` : '-' }}</span>
              <span class="truncate text-gray-500 dark:text-gray-400" :title="result.error_message || result.response_text">
                {{ probeHistorySummary(result) }}
              </span>
            </div>
          </template>
          <div
            v-if="!probeHistoryLoading && currentProbeHistoryResults.length === 0"
            class="px-3 py-6 text-center text-sm text-gray-500 dark:text-gray-400"
          >
            暂无历史探针记录
          </div>
        </div>
      </div>
    </BaseDialog>

  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  SupplierHourlyStability,
  SupplierProbeHistory,
  SupplierProbeResult,
  SupplierProbeSnapshot,
  SupplierProbeSnapshotItem,
  SupplierProbeStatus,
  SupplierProbeTimelinePoint,
  SupplierTargetGroup
} from '@/api/admin/suppliers'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const defaultProbeIntervalMinutes = 2
const supplierTimelineSlotCount = 60

type SupplierTimelineSlot = {
  key: string
  at: Date
  status: SupplierProbeStatus | 'missing'
  point?: SupplierProbeTimelinePoint
}

const probeStatusLabels: Record<SupplierProbeStatus, string> = {
  unknown: '暂无数据',
  success: '正常',
  degraded: '需观察',
  failed: '异常'
}

const probeSubStatusLabels: Record<string, string> = {
  slow_latency: '慢响应',
  rate_limit: '限流',
  server_error: '服务器错误',
  client_error: '客户端错误',
  auth_error: '认证失败',
  invalid_request: '请求无效',
  network_error: '网络错误',
  response_timeout: '响应超时',
  content_mismatch: '内容不匹配'
}

const allGroups = ref<AdminGroup[]>([])
const probeSnapshot = ref<SupplierProbeSnapshot | null>(null)
const snapshotLoading = ref(false)
const syncingAccounts = ref(false)
const bulkSaving = ref(false)
const batchProbing = ref(false)
const singleProbeSavingIds = ref<Set<number>>(new Set())

const showProbeDetailDialog = ref(false)
const selectedProbeItem = ref<SupplierProbeSnapshotItem | null>(null)
const probeHistory = ref<SupplierProbeHistory | null>(null)
const probeHistoryLoading = ref(false)

const selectedProbeIntervalMinutes = computed(() =>
  Math.max(1, Number(selectedProbeItem.value?.probe_interval_minutes || defaultProbeIntervalMinutes))
)
const currentProbeHistoryResults = computed<SupplierProbeResult[]>(() => {
  const results = probeHistory.value?.results || []
  if (results.length <= 1) return results

  const intervalMs = selectedProbeIntervalMinutes.value * 60_000
  const maxContinuousGapMs = Math.max(intervalMs * 3, intervalMs + 2 * 60_000)
  let cutoff = results.length
  for (let idx = 1; idx < results.length; idx++) {
    const newer = new Date(results[idx - 1].checked_at).getTime()
    const older = new Date(results[idx].checked_at).getTime()
    if (!Number.isFinite(newer) || !Number.isFinite(older)) continue
    if (newer - older > maxContinuousGapMs) {
      cutoff = idx
      break
    }
  }
  return results.slice(0, cutoff)
})
const currentProbeHistoryHiddenCount = computed(() =>
  Math.max(0, (probeHistory.value?.results || []).length - currentProbeHistoryResults.value.length)
)

const snapshotOverallStatus = computed<SupplierProbeStatus>(() => {
  const snapshot = probeSnapshot.value
  if (!snapshot || snapshot.window_total === 0) return 'unknown'
  if (snapshot.window_success_rate >= 95) return 'success'
  if (snapshot.window_success_rate >= 80) return 'degraded'
  return 'failed'
})

const probeMetricCards = computed(() => {
  const snapshot = probeSnapshot.value
  return [
    {
      label: t('admin.suppliers.metricCoverage', '覆盖账号'),
      value: snapshot ? `${snapshot.enabled_suppliers}/${snapshot.total_suppliers}` : '-',
      hint: t('admin.suppliers.metricCoverageHint', '启用探针 / 启用账号')
    },
    {
      label: t('admin.suppliers.metricWindowRate', '窗口成功率'),
      value: snapshot && snapshot.window_total > 0 ? formatPercent(snapshot.window_success_rate) : '-',
      hint: snapshot ? `${snapshot.window_success}/${snapshot.window_total}` : '-'
    },
    {
      label: t('admin.suppliers.metricHealthy', '当前正常'),
      value: snapshot ? String(snapshot.healthy_suppliers) : '-',
      hint: t('admin.suppliers.metricHealthyHint', '最近一次探针正常')
    },
    {
      label: t('admin.suppliers.metricProblem', '异常/观察'),
      value: snapshot ? String(snapshot.failed_suppliers + snapshot.degraded_suppliers) : '-',
      hint: t('admin.suppliers.metricProblemHint', '失败或慢响应')
    },
    {
      label: t('admin.suppliers.metricLatency', '平均延迟'),
      value: snapshot && snapshot.average_latency_ms > 0 ? `${snapshot.average_latency_ms}ms` : '-',
      hint: t('admin.suppliers.metricLatencyHint', '当前窗口')
    }
  ]
})

const hourlyBuckets = computed<SupplierHourlyStability[]>(() => probeSnapshot.value?.hourly || [])

const monitorWindowLabel = computed(() => {
  const minutes = probeSnapshot.value?.window_minutes || 60
  if (minutes % 60 === 0) {
    const hours = minutes / 60
    return hours === 1 ? t('admin.suppliers.lastHour', '近 1 小时') : t('admin.suppliers.lastHours', { hours }, `近 ${hours} 小时`)
  }
  return t('admin.suppliers.lastMinutes', { minutes }, `近 ${minutes} 分钟`)
})

const riskyHours = computed<SupplierHourlyStability[]>(() =>
  hourlyBuckets.value
    .filter((bucket) => bucket.total > 0 && bucket.success_rate < 95)
    .sort((a, b) => {
      const aRisk = a.failed + a.degraded
      const bRisk = b.failed + b.degraded
      if (aRisk !== bRisk) return bRisk - aRisk
      return a.success_rate - b.success_rate
    })
    .slice(0, 3)
)

const monitoredSupplierRows = computed<SupplierProbeSnapshotItem[]>(() =>
  [...(probeSnapshot.value?.suppliers || [])]
    .sort((a, b) => {
      if (a.probe_enabled !== b.probe_enabled) {
        return a.probe_enabled ? -1 : 1
      }
      const rank: Record<SupplierProbeStatus, number> = {
        failed: 0,
        degraded: 1,
        unknown: 2,
        success: 3
      }
      const aRank = rank[a.last_probe_status] ?? 2
      const bRank = rank[b.last_probe_status] ?? 2
      if (aRank !== bRank) return aRank - bRank
      return (b.window_total || 0) - (a.window_total || 0)
    })
)

function probeStatusLabel(value: SupplierProbeStatus): string {
  return probeStatusLabels[value] || value
}

function displayProbeStatusLabel(item: SupplierProbeSnapshotItem): string {
  if (!item.probe_enabled) {
    return t('admin.suppliers.probeDisabled', '已关闭')
  }
  return probeStatusLabel(item.last_probe_status)
}

function probeBadgeClass(value: SupplierProbeStatus): string {
  const base = 'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium'
  const map: Record<string, string> = {
    unknown: 'bg-gray-100 text-gray-700 dark:bg-dark-600 dark:text-gray-300',
    success: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
    degraded: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
    failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  }
  return `${base} ${map[value] || map.unknown}`
}

function displayProbeBadgeClass(item: SupplierProbeSnapshotItem): string {
  if (!item.probe_enabled) {
    return 'inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300'
  }
  return probeBadgeClass(item.last_probe_status)
}

function probeCardClass(item: SupplierProbeSnapshotItem): string {
  if (!item.probe_enabled) {
    return 'border-gray-200 opacity-75 dark:border-dark-700'
  }
  const map: Record<string, string> = {
    success: 'border-gray-200 dark:border-dark-700',
    degraded: 'border-amber-200 dark:border-amber-900/50',
    failed: 'border-red-200 dark:border-red-900/50',
    unknown: 'border-gray-200 dark:border-dark-700'
  }
  return map[item.last_probe_status] || map.unknown
}

function providerLabel(value: string): string {
  const normalized = value.toLowerCase()
  if (normalized.includes('anthropic')) return 'Anthropic'
  if (normalized.includes('openai')) return 'OpenAI'
  if (normalized.includes('gemini')) return 'Gemini'
  return value || 'Provider'
}

function providerInitial(value: string): string {
  const label = providerLabel(value)
  if (label === 'OpenAI') return 'OA'
  if (label === 'Anthropic') return 'A'
  if (label === 'Gemini') return 'G'
  return label.slice(0, 2).toUpperCase()
}

function providerVisualClass(value: string): string {
  const normalized = value.toLowerCase()
  if (normalized.includes('anthropic')) return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (normalized.includes('openai')) return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (normalized.includes('gemini')) return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}

function providerBadgeClass(value: string): string {
  const normalized = value.toLowerCase()
  if (normalized.includes('anthropic')) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (normalized.includes('openai')) return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (normalized.includes('gemini')) return 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}

function cardAvailabilityClass(item: SupplierProbeSnapshotItem): string {
  if (!item.probe_enabled) return 'text-gray-400 dark:text-gray-500'
  if (item.window_total <= 0) return 'text-gray-400 dark:text-gray-500'
  if (item.window_success_rate >= 95) return 'text-emerald-600 dark:text-emerald-300'
  if (item.window_success_rate >= 80) return 'text-amber-600 dark:text-amber-300'
  return 'text-red-600 dark:text-red-300'
}

function formatLatencyText(value: number | null | undefined): string {
  if (!value || value <= 0) return '-'
  return `${value}ms`
}

function nextProbeText(item: SupplierProbeSnapshotItem): string {
  if (!item.probe_enabled) {
    return t('admin.suppliers.probeDisabledInline', '探针已关闭')
  }
  if (!item.next_probe_at) {
    return t('admin.suppliers.nextProbeUnknown', '等待调度')
  }
  const next = new Date(item.next_probe_at).getTime()
  const base = new Date(probeSnapshot.value?.generated_at || Date.now()).getTime()
  if (Number.isNaN(next) || Number.isNaN(base)) {
    return formatShortDateTime(item.next_probe_at)
  }
  const diffSeconds = Math.max(0, Math.round((next - base) / 1000))
  if (diffSeconds < 60) {
    return t('admin.suppliers.nextProbeSeconds', { seconds: diffSeconds }, `${diffSeconds} 秒后`)
  }
  const minutes = Math.ceil(diffSeconds / 60)
  return t('admin.suppliers.nextProbeMinutes', { minutes }, `${minutes} 分钟后`)
}

function isSingleProbeSaving(id: number): boolean {
  return singleProbeSavingIds.value.has(id)
}

function setSingleProbeSaving(id: number, saving: boolean): void {
  const next = new Set(singleProbeSavingIds.value)
  if (saving) {
    next.add(id)
  } else {
    next.delete(id)
  }
  singleProbeSavingIds.value = next
}

function formatDateTime(value: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function formatShortDateTime(value: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${Number(value).toFixed(0)}%`
}

function hourBucketClass(bucket: SupplierHourlyStability): string {
  if (bucket.total <= 0) {
    return 'border-gray-100 bg-gray-50 text-gray-400 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-500'
  }
  if (bucket.status === 'success') {
    return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-900/20 dark:text-emerald-300'
  }
  if (bucket.status === 'degraded') {
    return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-300'
  }
  return 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300'
}

function hourBucketTitle(bucket: SupplierHourlyStability): string {
  if (bucket.total <= 0) return `${bucket.label} · 暂无数据`
  return `${bucket.label} · ${formatPercent(bucket.success_rate)} · ${bucket.success}/${bucket.total} · ${bucket.average_latency_ms || '-'}ms`
}

function itemTargetGroups(item: SupplierProbeSnapshotItem): SupplierTargetGroup[] {
  if (item.target_groups?.length) {
    return item.target_groups
  }
  const ids = new Set(item.target_group_ids || [])
  return allGroups.value
    .filter((group) => ids.has(group.id))
    .map(groupToSupplierTargetGroup)
}

function groupToSupplierTargetGroup(group: AdminGroup): SupplierTargetGroup {
  return {
    id: group.id,
    name: group.name,
    platform: group.platform,
    status: group.status,
    subscription_type: group.subscription_type,
    rate_multiplier: group.rate_multiplier
  }
}

function supplierTimelineSlots(item: SupplierProbeSnapshotItem): SupplierTimelineSlot[] {
  const now = new Date(probeSnapshot.value?.generated_at || Date.now())
  now.setSeconds(0, 0)
  const intervalMinutes = Math.max(1, Number(item.probe_interval_minutes || defaultProbeIntervalMinutes))
  const intervalMs = intervalMinutes * 60_000
  const pointsBySlotDistance = new Map<number, SupplierProbeTimelinePoint>()

  for (const point of item.points || []) {
    const date = new Date(point.checked_at)
    if (Number.isNaN(date.getTime())) continue
    const distanceFromNow = Math.round((now.getTime() - date.getTime()) / intervalMs)
    if (distanceFromNow < 0 || distanceFromNow >= supplierTimelineSlotCount) continue
    const existing = pointsBySlotDistance.get(distanceFromNow)
    if (!existing || new Date(point.checked_at).getTime() > new Date(existing.checked_at).getTime()) {
      pointsBySlotDistance.set(distanceFromNow, point)
    }
  }

  const slots: SupplierTimelineSlot[] = []
  for (let idx = supplierTimelineSlotCount - 1; idx >= 0; idx--) {
    const at = new Date(now.getTime() - idx * intervalMs)
    const point = pointsBySlotDistance.get(idx)
    slots.push({
      key: at.toISOString(),
      at,
      status: point?.status || 'missing',
      point
    })
  }
  return slots
}

function probeSlotClass(status: SupplierProbeStatus | 'missing'): string {
  const map: Record<string, string> = {
    success: 'bg-emerald-500',
    degraded: 'bg-amber-500',
    failed: 'bg-red-500',
    unknown: 'bg-gray-300 dark:bg-dark-600',
    missing: 'bg-gray-200 dark:bg-dark-600'
  }
  return map[status] || map.missing
}

function probeSlotStyle(status: SupplierProbeStatus | 'missing'): Record<string, string> {
  const heightPct: Record<string, string> = {
    success: '100%',
    degraded: '65%',
    failed: '35%',
    unknown: '20%',
    missing: '14%'
  }
  return { height: heightPct[status] || heightPct.missing }
}

function probeSlotTitle(slot: SupplierTimelineSlot): string {
  const time = slot.at.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  if (!slot.point) return `${time} 无数据`
  const subStatus = slot.point.sub_status ? ` ${probeSubStatusLabels[slot.point.sub_status] || slot.point.sub_status}` : ''
  const http = slot.point.http_code > 0 ? ` HTTP ${slot.point.http_code}` : ''
  const latency = slot.point.latency_ms > 0 ? ` ${slot.point.latency_ms}ms` : ''
  return `${time} ${probeStatusLabel(slot.point.status)}${subStatus}${http}${latency}`
}

function probeHistorySummary(result: SupplierProbeResult): string {
  const parts: string[] = []
  if (result.sub_status) {
    parts.push(probeSubStatusLabels[result.sub_status] || result.sub_status)
  }
  if (result.error_message) {
    parts.push(result.error_message)
  } else if (result.response_text) {
    parts.push(result.response_text)
  }
  return parts.join(' · ') || '-'
}

async function loadProbeSnapshot(): Promise<void> {
  snapshotLoading.value = true
  try {
    probeSnapshot.value = await adminAPI.suppliers.getProbeSnapshot({
      window_minutes: 60,
      days: 7
    })
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.snapshotError', '加载探针快照失败')))
  } finally {
    snapshotLoading.value = false
  }
}

async function loadFormOptions(): Promise<void> {
  try {
    allGroups.value = await adminAPI.groups.getAll()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.optionsError', '加载分组失败')))
  }
}

async function refreshProbePage(): Promise<void> {
  await Promise.all([loadProbeSnapshot(), loadFormOptions()])
}

async function openProbeDetail(item: SupplierProbeSnapshotItem): Promise<void> {
  selectedProbeItem.value = item
  showProbeDetailDialog.value = true
  probeHistory.value = null
  probeHistoryLoading.value = true
  try {
    probeHistory.value = await adminAPI.suppliers.getProbeHistory(item.id, {
      days: 7,
      limit: 200
    })
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.probeHistoryError', '加载探针历史失败')))
  } finally {
    probeHistoryLoading.value = false
  }
}

function closeProbeDetail(): void {
  showProbeDetailDialog.value = false
  selectedProbeItem.value = null
  probeHistory.value = null
}

async function syncAccountsToSuppliers(): Promise<void> {
  syncingAccounts.value = true
  try {
    const result = await adminAPI.suppliers.syncAccounts()
    appStore.showSuccess(
      t(
        'admin.suppliers.syncAccountsSuccess',
        { created: result.created, updated: result.updated, removed: result.removed, skipped: result.skipped },
        `账号已同步：新增 ${result.created}，更新 ${result.updated}，移除 ${result.removed}，跳过 ${result.skipped}`
      )
    )
    await refreshProbePage()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.syncAccountsError', '同步账号失败')))
  } finally {
    syncingAccounts.value = false
  }
}

async function bulkToggleProbes(enabled: boolean): Promise<void> {
  bulkSaving.value = true
  try {
    const result = await adminAPI.suppliers.bulkSetProbeEnabled([], enabled)
    appStore.showSuccess(
      enabled
        ? t('admin.suppliers.enableAllSuccess', { count: result.affected }, `已开启 ${result.affected} 个探针`)
        : t('admin.suppliers.disableAllSuccess', { count: result.affected }, `已关闭 ${result.affected} 个探针`)
    )
    await refreshProbePage()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.bulkToggleError', '批量更新探针失败')))
  } finally {
    bulkSaving.value = false
  }
}

async function toggleSingleProbe(item: SupplierProbeSnapshotItem, enabled: boolean): Promise<void> {
  if (isSingleProbeSaving(item.id)) {
    return
  }
  setSingleProbeSaving(item.id, true)
  try {
    const result = await adminAPI.suppliers.bulkSetProbeEnabled([item.id], enabled)
    if (result.affected === 0) {
      appStore.showError(t('admin.suppliers.singleToggleNoop', '没有找到可更新的探针'))
      return
    }
    appStore.showSuccess(
      enabled
        ? t('admin.suppliers.singleEnableSuccess', { name: item.name }, `已开启 ${item.name} 的探针`)
        : t('admin.suppliers.singleDisableSuccess', { name: item.name }, `已关闭 ${item.name} 的探针`)
    )
    await loadProbeSnapshot()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.singleToggleError', '更新探针开关失败')))
  } finally {
    setSingleProbeSaving(item.id, false)
  }
}

async function runAllEnabledProbes(): Promise<void> {
  batchProbing.value = true
  try {
    const result = await adminAPI.suppliers.probeAll([])
    appStore.showSuccess(
      t(
        'admin.suppliers.probeAllDone',
        { success: result.success, degraded: result.degraded, failed: result.failed, skipped: result.skipped },
        `检测完成：正常 ${result.success}，观察 ${result.degraded}，失败 ${result.failed}，跳过 ${result.skipped}`
      )
    )
    await refreshProbePage()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.probeAllError', '批量探针失败')))
  } finally {
    batchProbing.value = false
  }
}

onMounted(() => {
  refreshProbePage()
})
</script>
