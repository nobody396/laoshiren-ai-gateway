<template>
  <AppLayout>
    <div class="matrix-page mx-auto max-w-[1800px] space-y-5">
      <header class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <div class="flex flex-wrap items-center gap-2 text-xs font-semibold text-indigo-600 dark:text-indigo-300">
            <span>管理员 · 只读</span><span class="text-slate-300 dark:text-slate-600">/</span><span>九矩阵交叉视图</span>
          </div>
          <h1 class="mt-2 text-2xl font-bold tracking-tight text-slate-950 dark:text-white sm:text-3xl">模型 × 协议 × 客户端</h1>
          <p class="mt-2 max-w-3xl text-sm leading-6 text-slate-500 dark:text-slate-400">
            只有模型协议与客户端原生协议同时支持，才进入可导入候选；精确客户端版本和 OS 证据未闭环时保持待补，不把兼容猜测当成可用。
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span class="rounded-lg border border-slate-200 bg-white px-3 py-2 text-slate-500 dark:border-slate-700 dark:bg-slate-900">
            合同 {{ modelDocContracts.length }} · 客户端 {{ clients.length }}
          </span>
          <span class="rounded-lg border border-slate-200 bg-white px-3 py-2 text-slate-500 dark:border-slate-700 dark:bg-slate-900" :class="pricingError && 'text-amber-700 dark:text-amber-300'">
            {{ pricingStatus }}
          </span>
        </div>
      </header>

      <div v-if="matrixLoading" class="rounded-2xl border border-slate-200 bg-white px-5 py-10 text-center text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900">正在读取管理员矩阵…</div>
      <div v-else-if="matrixError" class="rounded-2xl border border-red-200 bg-red-50 px-5 py-6 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-300">{{ matrixError }}</div>

      <section v-else class="grid grid-cols-2 gap-3 xl:grid-cols-4">
        <button class="summary-card text-left" :class="statusFilter === 'all' && 'summary-card--active'" @click="statusFilter = 'all'">
          <span class="summary-label">候选交集</span><strong>{{ candidateRows.length }}</strong><small>模型协议与客户端协议相交</small>
        </button>
        <button class="summary-card text-left" :class="statusFilter === 'verified' && 'summary-card--active'" @click="statusFilter = 'verified'">
          <span class="summary-label text-emerald-600">已验证</span><strong>{{ counts.verified }}</strong><small>全部声明 OS 已有终态证据</small>
        </button>
        <button class="summary-card text-left" :class="statusFilter === 'blocked' && 'summary-card--active'" @click="statusFilter = 'blocked'">
          <span class="summary-label text-amber-600">待补证据</span><strong>{{ counts.blocked }}</strong><small>候选可配，但证据仍不完整</small>
        </button>
        <button class="summary-card text-left" :class="!candidateOnly && statusFilter === 'unsupported' && 'summary-card--active'" @click="showUnsupported">
          <span class="summary-label text-slate-500">不支持</span><strong>{{ unsupportedCount }}</strong><small>模型或客户端协议不相交</small>
        </button>
      </section>

      <section class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-700 dark:bg-slate-900">
        <div class="grid gap-3 border-b border-slate-200 p-4 dark:border-slate-700 lg:grid-cols-[minmax(220px,1.4fr)_repeat(3,minmax(150px,0.7fr))_auto]">
          <label class="filter-field">
            <span>模型</span>
            <input v-model.trim="modelQuery" type="search" placeholder="名称或模型 ID" aria-label="筛选模型" />
          </label>
          <label class="filter-field">
            <span>客户端</span>
            <select v-model="clientFilter" aria-label="筛选客户端"><option value="all">全部客户端</option><option v-for="client in clients" :key="client.id" :value="client.id">{{ client.name }}</option></select>
          </label>
          <label class="filter-field">
            <span>协议</span>
            <select v-model="protocolFilter" aria-label="筛选协议"><option value="all">全部协议</option><option v-for="protocol in protocols" :key="protocol" :value="protocol">{{ protocolLabel(protocol) }}</option></select>
          </label>
          <label class="filter-field">
            <span>状态</span>
            <select v-model="statusFilter" aria-label="筛选状态"><option value="all">全部状态</option><option value="verified">已验证</option><option value="blocked">待补证据</option><option value="unsupported">不支持</option></select>
          </label>
          <label class="mt-5 inline-flex h-10 cursor-pointer items-center gap-2 rounded-xl border border-slate-200 px-3 text-xs font-semibold text-slate-600 dark:border-slate-700 dark:text-slate-300">
            <input v-model="candidateOnly" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-indigo-600" />仅候选交集
          </label>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-slate-50/70 px-4 py-3 text-xs text-slate-500 dark:border-slate-700 dark:bg-slate-950/30">
          <div class="flex flex-wrap gap-2">
            <button v-for="client in clients" :key="client.id" class="client-shortcut" :class="clientFilter === client.id && 'client-shortcut--active'" @click="clientFilter = clientFilter === client.id ? 'all' : client.id">
              <img :src="client.icon" alt="" /><span>{{ client.name }}</span><b>{{ candidateCountFor(client.id) }}</b>
            </button>
          </div>
          <span>显示 {{ visibleRows.length }} / {{ filteredRows.length }} 行</span>
        </div>

        <div class="hidden grid-cols-[minmax(190px,1.25fr)_minmax(130px,0.8fr)_minmax(160px,0.95fr)_minmax(150px,0.85fr)_130px_32px] gap-4 border-b border-slate-200 px-5 py-2 text-[10px] font-bold uppercase tracking-[0.12em] text-slate-400 dark:border-slate-700 md:grid">
          <span>模型</span><span>协议</span><span>客户端</span><span>推理强度交集</span><span>证据状态</span><span />
        </div>

        <div v-if="visibleRows.length" class="divide-y divide-slate-200 dark:divide-slate-700" data-testid="matrix-rows">
          <article v-for="row in visibleRows" :key="row.key" class="matrix-row">
            <button class="matrix-row__button grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-5 py-4 text-left md:grid-cols-[minmax(190px,1.25fr)_minmax(130px,0.8fr)_minmax(160px,0.95fr)_minmax(150px,0.85fr)_130px_32px] md:gap-4" @click="toggle(row.key)">
              <span class="min-w-0"><strong class="block truncate text-sm text-slate-950 dark:text-white">{{ row.model.model.display_name }}</strong><code class="mt-1 block truncate text-[11px] text-slate-400">{{ row.model.model.id }}</code></span>
              <span><span class="protocol-badge">{{ protocolLabel(row.protocol) }}</span><small v-if="row.protocol === row.model.recommended_protocol" class="mt-1.5 block text-[10px] font-semibold text-indigo-600">推荐协议</small></span>
              <span class="flex min-w-0 items-center gap-2"><img :src="row.client.icon" alt="" class="h-6 w-6 rounded-md object-contain" /><span class="min-w-0"><strong class="block truncate text-sm text-slate-800 dark:text-slate-100">{{ row.client.name }}</strong><small class="block truncate text-[10px] text-slate-400">{{ row.client.client_config_os.release.display }}</small></span></span>
              <span><span v-if="row.reasoningLevels.length" class="flex flex-wrap gap-1"><code v-for="level in row.reasoningLevels" :key="level" class="rounded bg-indigo-50 px-1.5 py-0.5 text-[10px] font-semibold text-indigo-700 dark:bg-indigo-950/60 dark:text-indigo-300">{{ level }}</code></span><small v-else class="text-xs text-slate-400">无确认档位</small></span>
              <MatrixStatusBadge :status="row.status" />
              <span class="text-right text-lg text-slate-400 transition-transform" :class="expanded.has(row.key) && 'rotate-180'">⌄</span>
            </button>
            <ModelMatrixDetail v-if="expanded.has(row.key)" :row="row" :evidence-index="matrixPayload?.evidence_index ?? {}" />
          </article>
        </div>
        <div v-else class="px-6 py-16 text-center"><p class="text-sm font-semibold text-slate-700 dark:text-slate-200">没有符合条件的矩阵行</p><p class="mt-1 text-xs text-slate-400">放宽模型、客户端、协议或状态筛选。</p></div>

        <div v-if="filteredRows.length > pageSize" class="flex items-center justify-center border-t border-slate-200 p-4 dark:border-slate-700">
          <button class="rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-600 hover:border-indigo-300 hover:text-indigo-600 dark:border-slate-700 dark:text-slate-300" @click="pageSize += 60">再显示 60 行</button>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import MatrixStatusBadge from '@/components/admin/model-matrix/MatrixStatusBadge.vue'
import ModelMatrixDetail from '@/components/admin/model-matrix/ModelMatrixDetail.vue'
import { buildModelClientMatrix, protocolLabel, type ClientMatrixV2Data, type MatrixProtocol, type MatrixStatus } from '@/components/admin/model-matrix/matrix'
import { getPublicModelPricing, type PublicModelPricingCatalog } from '@/api/publicPricing'
import { getAdminModelClientMatrix, type AdminModelClientMatrixPayload } from '@/api/admin/modelClientMatrix'

const matrixPayload = ref<AdminModelClientMatrixPayload | null>(null)
const matrix = computed<ClientMatrixV2Data>(() =>
  (matrixPayload.value?.client_matrix as ClientMatrixV2Data | undefined) ?? { clients: [] },
)
const modelDocContracts = computed(() => matrixPayload.value?.contracts ?? [])
const clients = computed(() => matrix.value.clients)
const protocols: readonly MatrixProtocol[] = ['responses', 'chat_completions', 'messages', 'generate_content']
const matrixLoading = ref(true)
const matrixError = ref('')
const pricing = ref<PublicModelPricingCatalog | null>(null)
const pricingLoading = ref(true)
const pricingError = ref('')
const modelQuery = ref('')
const clientFilter = ref('all')
const protocolFilter = ref<'all' | MatrixProtocol>('all')
const statusFilter = ref<'all' | MatrixStatus>('all')
const candidateOnly = ref(true)
const pageSize = ref(60)
const expanded = ref(new Set<string>())

const allRows = computed(() => buildModelClientMatrix(modelDocContracts.value, matrix.value, pricing.value))
const candidateRows = computed(() => allRows.value.filter(row => row.candidate))
const counts = computed(() => ({
  verified: candidateRows.value.filter(row => row.status === 'verified').length,
  blocked: candidateRows.value.filter(row => row.status === 'blocked').length,
}))
const unsupportedCount = computed(() => allRows.value.filter(row => !row.candidate).length)
const filteredRows = computed(() => allRows.value.filter(row => {
  if (candidateOnly.value && !row.candidate) return false
  if (clientFilter.value !== 'all' && row.client.id !== clientFilter.value) return false
  if (protocolFilter.value !== 'all' && row.protocol !== protocolFilter.value) return false
  if (statusFilter.value !== 'all' && row.status !== statusFilter.value) return false
  if (modelQuery.value) {
    const query = modelQuery.value.toLocaleLowerCase()
    if (!`${row.model.model.id} ${row.model.model.display_name}`.toLocaleLowerCase().includes(query)) return false
  }
  return true
}))
const visibleRows = computed(() => filteredRows.value.slice(0, pageSize.value))
const pricingStatus = computed(() => {
  if (pricingLoading.value) return '公共价格：读取中'
  if (pricingError.value) return '公共价格失败 · 展示合同快照'
  return `公共价格：${pricing.value?.updated_at || '已读回'}`
})

watch([modelQuery, clientFilter, protocolFilter, statusFilter, candidateOnly], () => { pageSize.value = 60 })

function candidateCountFor(clientId: string): number { return candidateRows.value.filter(row => row.client.id === clientId).length }
function toggle(key: string) {
  const next = new Set(expanded.value)
  if (next.has(key)) next.delete(key); else next.add(key)
  expanded.value = next
}
function showUnsupported() { candidateOnly.value = false; statusFilter.value = 'unsupported' }

onMounted(async () => {
  try { matrixPayload.value = await getAdminModelClientMatrix() }
  catch (cause) { matrixError.value = cause instanceof Error ? cause.message : '管理员矩阵读取失败' }
  finally { matrixLoading.value = false }
  try { pricing.value = await getPublicModelPricing() }
  catch (cause) { pricingError.value = cause instanceof Error ? cause.message : '公共价格读取失败' }
  finally { pricingLoading.value = false }
})
</script>

<style scoped>
.summary-card { @apply rounded-2xl border border-slate-200 bg-white p-4 shadow-sm transition hover:border-indigo-300 dark:border-slate-700 dark:bg-slate-900 dark:hover:border-indigo-700; }
.summary-card--active { @apply border-indigo-400 ring-2 ring-indigo-100 dark:border-indigo-500 dark:ring-indigo-950; }
.summary-card strong { @apply mt-1 block text-2xl font-bold text-slate-950 dark:text-white; }
.summary-card small { @apply mt-1 block text-xs text-slate-400; }
.summary-label { @apply text-xs font-bold uppercase tracking-[0.12em] text-slate-500; }
.filter-field { @apply block; }
.filter-field > span { @apply mb-1.5 block text-[11px] font-bold uppercase tracking-[0.1em] text-slate-400; }
.filter-field input, .filter-field select { @apply h-10 w-full rounded-xl border border-slate-200 bg-white px-3 text-sm text-slate-800 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:focus:ring-indigo-950; }
.client-shortcut { @apply inline-flex items-center gap-1.5 rounded-lg border border-transparent px-2 py-1.5 text-[11px] font-semibold text-slate-500 hover:bg-white dark:hover:bg-slate-900; }
.client-shortcut img { @apply h-4 w-4 rounded object-contain; }
.client-shortcut b { @apply rounded bg-slate-200 px-1.5 py-0.5 font-mono text-[9px] text-slate-600 dark:bg-slate-700 dark:text-slate-300; }
.client-shortcut--active { @apply border-indigo-200 bg-white text-indigo-700 shadow-sm dark:border-indigo-800 dark:bg-slate-900 dark:text-indigo-300; }
.matrix-row > button:hover { @apply bg-slate-50/70 dark:bg-slate-800/50; }
.protocol-badge { @apply inline-flex rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 font-mono text-[11px] font-semibold text-slate-700 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-300; }
</style>
