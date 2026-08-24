<template>
  <AppLayout>
    <div class="mx-auto max-w-[1500px] space-y-6 p-4 sm:p-6 lg:p-8">
      <header class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p class="text-sm font-semibold text-primary-600 dark:text-primary-300">Reliability Control</p>
          <h1 class="mt-1 text-3xl font-bold text-gray-950 dark:text-white">渠道监控</h1>
          <p class="mt-2 max-w-3xl text-sm text-gray-500 dark:text-gray-400">从公开服务下钻到组件、用户分组、账号与 Route 证据。此页面只读，不会修改调度。</p>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <select v-model.number="windowMinutes" aria-label="证据时间窗口" class="rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-800" @change="load">
            <option :value="15">最近 15 分钟</option><option :value="60">最近 60 分钟</option><option :value="120">最近 2 小时</option>
          </select>
          <button class="rounded-xl bg-primary-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60" :disabled="loading" @click="load">{{ loading ? '刷新中…' : '刷新证据' }}</button>
        </div>
      </header>

      <nav class="flex flex-wrap gap-3 text-sm" aria-label="监控明细入口">
        <router-link class="text-primary-600 hover:underline" to="/admin/ops?open_error_details=1&error_type=request">请求与错误明细</router-link>
        <a class="text-primary-600 hover:underline" href="#shadow-audit">OpenAI Shadow 审计</a>
      </nav>

      <div v-if="loading && !snapshot" class="rounded-2xl border border-dashed border-gray-300 p-10 text-center text-sm text-gray-500 dark:border-dark-600" aria-live="polite">正在加载渠道监控证据…</div>
      <div v-if="error" class="rounded-2xl border border-danger-200 bg-danger-50 p-4 text-sm text-danger-700 dark:border-danger-900/50 dark:bg-danger-950/30 dark:text-danger-200" role="alert">渠道监控数据暂不可用。{{ error }}</div>

      <template v-if="snapshot">
        <div v-if="snapshot.evidence_truncated" class="rounded-2xl border border-warning-200 bg-warning-50 p-4 text-sm text-warning-800 dark:border-warning-900/50 dark:bg-warning-950/20">
          当前窗口共有 {{ snapshot.evidence_bucket_total }} 个证据桶，详情仅展示最新 {{ snapshot.evidence.length }} 个；顶部汇总仍为完整窗口精确值。
        </div>

        <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <article v-for="card in summaryCards" :key="card.label" class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
            <p class="text-xs font-semibold uppercase tracking-wider text-gray-500">{{ card.label }}</p><strong class="mt-2 block text-2xl text-gray-950 dark:text-white">{{ card.value }}</strong><span class="mt-1 block text-xs text-gray-500">{{ card.note }}</span>
          </article>
        </section>

        <section class="rounded-2xl border p-4" :class="snapshot.status.evaluation_ready ? 'border-success-200 bg-success-50/60 dark:border-success-900/40 dark:bg-success-950/20' : 'border-warning-200 bg-warning-50/60 dark:border-warning-900/40 dark:bg-warning-950/20'">
          <div class="flex flex-wrap items-center justify-between gap-3"><div><strong>{{ snapshot.status.evaluation_ready ? '状态证据可用于评估' : '状态证据尚不完整' }}</strong><p class="mt-1 text-xs text-gray-600 dark:text-gray-400">队列 {{ snapshot.completeness.queue_depth }} · 写入中 {{ snapshot.completeness.in_flight }} · 丢失 {{ snapshot.completeness.dropped }} · 失败 {{ snapshot.completeness.failed }}</p></div><time class="text-xs text-gray-500" :datetime="snapshot.generated_at">{{ formatTime(snapshot.generated_at) }}</time></div>
        </section>

        <section class="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="grid gap-3 md:grid-cols-[1fr_220px]">
            <input v-model.trim="search" aria-label="搜索渠道监控证据" class="rounded-xl border border-gray-200 px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900" placeholder="搜索服务、模型、账号、分组或 Route" />
            <select v-model="factType" aria-label="证据类型" class="rounded-xl border border-gray-200 px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900"><option value="">全部证据</option><option value="customer_request">最终客户请求</option><option value="upstream_attempt">上游尝试</option><option value="active_probe">主动探针</option></select>
          </div>
        </section>

        <section v-if="!products.length" class="rounded-2xl border border-dashed border-gray-300 p-10 text-center text-sm text-gray-500 dark:border-dark-600">当前没有已发布的状态产品。</section>

        <section v-for="product in products" :key="product.code" class="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <header class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-5 dark:border-dark-700"><div><span class="text-xs font-medium text-gray-500">{{ product.familyName }}</span><h2 class="mt-1 text-lg font-bold text-gray-950 dark:text-white">{{ product.display_name }}</h2></div><span class="rounded-full border px-3 py-1 text-xs font-semibold" :class="statusClass(product.effective_status)">{{ statusLabel(product.effective_status) }}</span></header>
          <div class="grid gap-5 p-5 xl:grid-cols-[280px_1fr]">
            <div class="space-y-3 text-sm"><MetricRow label="客户可用性" :value="formatRate(product.customer_availability.rate)" /><MetricRow label="探针可用性" :value="formatRate(product.probe_availability.rate)" /><MetricRow label="Computed" :value="statusLabel(product.computed_status)" /><MetricRow label="Effective" :value="statusLabel(product.effective_status)" /></div>
            <div class="space-y-4">
              <article v-for="component in product.components" :key="component.code" class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
                <div class="flex flex-wrap items-center justify-between gap-2"><div><strong>{{ component.display_name }}</strong><span class="ml-2 text-xs text-gray-500">{{ component.access_mode.toUpperCase() }}</span></div><span class="text-xs font-semibold">{{ statusLabel(component.computed_status) }}</span></div>
                <div class="mt-3 flex flex-wrap gap-2 text-xs text-gray-500"><span v-for="binding in component.bindings" :key="binding.binding_key" class="rounded-lg bg-gray-50 px-2 py-1 dark:bg-dark-900">{{ binding.group_name || binding.platform || binding.model_pattern || shortRoute(binding.route_fingerprint) }}</span></div>
                <div class="mt-4 overflow-x-auto"><table class="min-w-[980px] text-left text-xs">
                  <thead class="text-gray-500"><tr><th scope="col" class="pb-2 pr-4">证据</th><th scope="col" class="pb-2 pr-4">账号 / 分组</th><th scope="col" class="pb-2 pr-4">样本</th><th scope="col" class="pb-2 pr-4">失败 / 影响</th><th scope="col" class="pb-2 pr-4">负载</th><th scope="col" class="pb-2 pr-4">延迟</th><th scope="col" class="pb-2 pr-4">成功 / 恢复</th><th scope="col" class="pb-2">Route</th></tr></thead>
                  <tbody>
                    <tr v-for="item in evidenceFor(component.code)" :key="evidenceKey(item)" class="border-t border-gray-100 dark:border-dark-700"><td class="py-2 pr-4">{{ factLabel(item) }}</td><td class="py-2 pr-4">{{ item.account_name || '—' }}<span v-if="item.group_name" class="block text-gray-500">{{ item.group_name }}</span></td><td class="py-2 pr-4">{{ item.sample_count }}</td><td class="py-2 pr-4" :class="item.failure_count ? 'text-danger-600' : ''">{{ item.failure_count }}<span class="block text-gray-500">影响 {{ item.customer_impact_count }}</span></td><td class="py-2 pr-4">{{ item.samples_per_minute.toFixed(2) }}/min</td><td class="py-2 pr-4">平均 {{ Math.round(item.average_latency_ms) }}ms<span class="block text-gray-500">P95 {{ Math.round(item.p95_latency_ms) }}ms</span></td><td class="py-2 pr-4">成功 {{ item.success_count }} · 恢复 {{ item.recovered_count }}<span class="block text-gray-500">{{ recoveryCopy(item) }}</span></td><td class="py-2 font-mono text-[11px]">{{ shortRoute(item.route_fingerprint) }}</td></tr>
                    <tr v-if="!evidenceFor(component.code).length"><td colspan="8" class="py-4 text-center text-gray-500">当前筛选范围内没有证据</td></tr>
                  </tbody>
                </table></div>
              </article>
            </div>
          </div>
        </section>

        <section v-if="directDiagnostics.length" class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700"><h2 class="text-lg font-bold">不参与服务状态的上游直连诊断</h2><p class="mt-1 text-xs text-gray-500">这是绕过客户网关的内部线路检查，仅用于定位上游或网络问题；不会计入客户可用率或公开服务状态，也不代表客户请求发生故障。</p><div class="mt-4 overflow-x-auto"><table class="min-w-full text-left text-xs"><thead class="text-gray-500"><tr><th scope="col" class="pb-2 pr-4">证据</th><th scope="col" class="pb-2 pr-4">平台 / 模型</th><th scope="col" class="pb-2 pr-4">账号</th><th scope="col" class="pb-2 pr-4">样本</th><th scope="col" class="pb-2">Route</th></tr></thead><tbody><tr v-for="item in directDiagnostics" :key="evidenceKey(item)" class="border-t border-gray-100 dark:border-dark-700"><td class="py-2 pr-4">{{ factLabel(item) }}</td><td class="py-2 pr-4">{{ item.platform }} / {{ item.model }}</td><td class="py-2 pr-4">{{ item.account_name || '—' }}</td><td class="py-2 pr-4">{{ item.sample_count }}</td><td class="py-2 font-mono text-[11px]">{{ shortRoute(item.route_fingerprint) }}</td></tr></tbody></table></div></section>
        <section v-if="unclassifiedEvidence.length" class="rounded-2xl border border-warning-200 bg-warning-50/60 p-5 dark:border-warning-900/40 dark:bg-warning-950/20"><h2 class="text-lg font-bold">待归类的内部监控数据</h2><p class="mt-1 text-xs text-gray-600 dark:text-gray-400">这些数据尚未关联 Status Catalog，可能包含真实客户失败或缺失的产品绑定，需要运营排查；完成归类前不会展示到公开状态页。</p><div class="mt-4 overflow-x-auto"><table class="min-w-full text-left text-xs"><thead class="text-gray-500"><tr><th scope="col" class="pb-2 pr-4">证据</th><th scope="col" class="pb-2 pr-4">平台 / 模型</th><th scope="col" class="pb-2 pr-4">账号</th><th scope="col" class="pb-2 pr-4">样本</th><th scope="col" class="pb-2">Route</th></tr></thead><tbody><tr v-for="item in unclassifiedEvidence" :key="evidenceKey(item)" class="border-t border-warning-200/70"><td class="py-2 pr-4">{{ factLabel(item) }}</td><td class="py-2 pr-4">{{ item.platform }} / {{ item.model }}</td><td class="py-2 pr-4">{{ item.account_name || '—' }}</td><td class="py-2 pr-4">{{ item.sample_count }}<span v-if="item.failure_count" class="block text-danger-600">失败 {{ item.failure_count }} · 影响 {{ item.customer_impact_count }}</span></td><td class="py-2 font-mono text-[11px]">{{ shortRoute(item.route_fingerprint) }}</td></tr></tbody></table></div></section>

        <section id="shadow-audit" class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <h2 class="text-lg font-bold">OpenAI 智能路由 Shadow 审计</h2>
          <p class="mt-1 text-xs text-gray-500">读取现有 Shadow 统计、Reliability Evidence 对比和持久化健康度；不会启用 Enforce。</p>
          <template v-if="shadowAudit">
            <div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"><MetricRow label="24h 决策" :value="String(shadowAudit.stats.total)" /><MetricRow label="已评估" :value="String(shadowAudit.stats.evaluated)" /><MetricRow label="分歧" :value="String(shadowAudit.stats.diverged)" /><MetricRow label="证据健康" :value="shadowAudit.health.ready ? 'Ready' : 'Not ready'" /></div>
            <div class="mt-5 overflow-x-auto">
              <table class="min-w-[760px] text-left text-xs">
                <thead class="text-gray-500"><tr><th scope="col" class="pb-2 pr-4">Decision</th><th scope="col" class="pb-2 pr-4">模型</th><th scope="col" class="pb-2 pr-4">Adapter</th><th scope="col" class="pb-2">Legacy → Reliability Evidence</th></tr></thead>
                <tbody><tr v-for="decision in shadowAudit.decisions" :key="decision.decision_id" class="border-t border-gray-100 dark:border-dark-700"><td class="py-2 pr-4 font-mono">{{ decision.decision_id.slice(0, 12) }}</td><td class="py-2 pr-4">{{ decision.model }}</td><td class="py-2 pr-4">{{ decision.snapshot?.reliability_evidence_adapter_applied ? 'Applied' : decision.snapshot?.reliability_evidence_adapter_reason || 'Disabled' }}</td><td class="py-2">{{ candidateComparison(decision.snapshot?.candidates?.[0]) }}</td></tr><tr v-if="!shadowAudit.decisions.length"><td colspan="4" class="py-4 text-center text-gray-500">最近 24 小时暂无 Shadow decision</td></tr></tbody>
              </table>
            </div>
          </template>
          <p v-else class="mt-4 text-sm text-gray-500">{{ shadowError || 'Shadow 审计加载中…' }}</p>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getChannelMonitoring, getOpenAIShadowAudit, type ChannelMonitoringEvidence, type ChannelMonitoringSnapshot, type OpenAIShadowAuditSummary, type OpenAIShadowReliabilityCandidate } from '@/api/admin/channelMonitoring'
import type { ServiceStatus } from '@/api/serviceStatus'

const MetricRow = defineComponent({ props: { label: String, value: String }, setup: (props) => () => h('div', { class: 'flex items-center justify-between gap-3 border-b border-gray-100 pb-2 dark:border-dark-700' }, [h('span', { class: 'text-gray-500' }, props.label), h('strong', props.value)]) })
const snapshot = ref<ChannelMonitoringSnapshot | null>(null)
const shadowAudit = ref<OpenAIShadowAuditSummary | null>(null)
const shadowError = ref('')
const loading = ref(false)
const error = ref('')
const windowMinutes = ref(15)
const search = ref('')
const factType = ref('')
let controller: AbortController | null = null

const products = computed(() => snapshot.value?.status.families.flatMap((family) => family.products.map((product) => ({ ...product, familyName: family.display_name }))) ?? [])
const filteredEvidence = computed(() => { const term = search.value.toLowerCase(); return (snapshot.value?.evidence ?? []).filter((item) => { if (factType.value && item.fact_type !== factType.value) return false; if (!term) return true; return [item.platform, item.model, item.account_name, item.group_name, item.route_fingerprint, ...item.product_codes, ...item.component_codes].some((value) => String(value || '').toLowerCase().includes(term)) }) })
const unmappedEvidence = computed(() => filteredEvidence.value.filter((item) => !item.component_codes.length))
const directDiagnostics = computed(() => unmappedEvidence.value.filter((item) => item.protocol === 'http_direct'))
const unclassifiedEvidence = computed(() => unmappedEvidence.value.filter((item) => item.protocol !== 'http_direct'))
const customerSamples = computed(() => snapshot.value?.totals.customer_request_count ?? 0)
const customerSuccesses = computed(() => snapshot.value?.totals.customer_success_count ?? 0)
const totalFailures = computed(() => snapshot.value?.totals.failure_count ?? 0)
const maxP95 = computed(() => snapshot.value?.evidence.reduce((max, item) => Math.max(max, item.p95_latency_ms), 0) ?? 0)
const summaryCards = computed(() => [{ label: '状态产品', value: String(products.value.length), note: '显式 Status Catalog' }, { label: '客户可用性', value: customerSamples.value ? `${(customerSuccesses.value / customerSamples.value * 100).toFixed(2)}%` : '—', note: `${customerSamples.value} 个最终请求` }, { label: '失败证据', value: String(totalFailures.value), note: `最近 ${snapshot.value?.window_minutes ?? windowMinutes.value} 分钟` }, { label: '最高 P95', value: `${Math.round(maxP95.value)}ms`, note: '当前详情证据桶最大值' }])

async function load() {
  controller?.abort()
  const current = new AbortController()
  controller = current
  loading.value = true
  error.value = ''
  shadowAudit.value = null
  shadowError.value = ''
  void loadShadowAudit(current)
  try {
    const next = await getChannelMonitoring(windowMinutes.value, current.signal)
    if (controller !== current || current.signal.aborted) return
    snapshot.value = next
  } catch (cause) {
    if (controller === current && !current.signal.aborted) error.value = cause instanceof Error ? cause.message : '请求失败'
  } finally {
    if (controller === current && !current.signal.aborted) loading.value = false
  }
}

async function loadShadowAudit(current: AbortController) {
  try {
    const next = await getOpenAIShadowAudit(current.signal)
    if (controller === current && !current.signal.aborted) shadowAudit.value = next
  } catch {
    if (controller === current && !current.signal.aborted) shadowError.value = 'Shadow 审计暂不可用'
  }
}
function evidenceFor(componentCode: string) { return filteredEvidence.value.filter((item) => item.component_codes.includes(componentCode)) }
function evidenceKey(item: ChannelMonitoringEvidence) { return `${item.fact_type}:${item.group_id || 0}:${item.account_id || 0}:${item.route_fingerprint || ''}:${item.model}:${item.protocol}` }
function shortRoute(value?: string) { return value ? `${value.slice(0, 8)}…${value.slice(-6)}` : '—' }
function formatRate(value?: number) { return value == null ? '—' : `${(value * 100).toFixed(2)}%` }
function formatTime(value: string) { return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(Date.parse(value)) }
function recoveryCopy(item: ChannelMonitoringEvidence) { if (item.last_recovery_at) return `恢复 ${formatTime(item.last_recovery_at)}`; if (item.last_success_at) return `成功 ${formatTime(item.last_success_at)}`; return '暂无成功证据' }
function candidateComparison(candidate?: OpenAIShadowReliabilityCandidate) {
  if (!candidate) return '—'
  const legacy = candidate.legacy_success_lower_bound == null ? '—' : `${(candidate.legacy_success_lower_bound * 100).toFixed(1)}%`
  const reliability = candidate.reliability_evidence_success_lower_bound == null ? '—' : `${(candidate.reliability_evidence_success_lower_bound * 100).toFixed(1)}%`
  return `${legacy} → ${reliability}`
}
function factLabel(item: ChannelMonitoringEvidence) { if (item.protocol === 'http_direct') return '上游直连诊断'; return ({ customer_request: '最终请求', upstream_attempt: '上游尝试', active_probe: '主动探针' } as const)[item.fact_type] }
function statusLabel(value: ServiceStatus) { return ({ operational: '正常', degraded_performance: '性能下降', partial_outage: '部分中断', major_outage: '大范围中断', maintenance: '维护中', monitoring: '观察中' } as const)[value] }
function statusClass(value: ServiceStatus) { return ({ operational: 'border-success-200 text-success-700', degraded_performance: 'border-warning-200 text-warning-700', partial_outage: 'border-warning-300 text-warning-800', major_outage: 'border-danger-200 text-danger-700', maintenance: 'border-info-200 text-info-700', monitoring: 'border-gray-200 text-gray-600' } as const)[value] }

onMounted(load)
onBeforeUnmount(() => { controller?.abort(); controller = null })
</script>
