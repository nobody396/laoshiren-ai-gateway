<template>
  <article
    class="model-contract-card overflow-hidden rounded-xl border border-[#eadfca] bg-white shadow-[0_12px_32px_-28px_rgba(78,61,33,0.3)] dark:border-dark-700 dark:bg-dark-900"
    data-model-card
    :data-model-id="modelId"
    :data-contract-state="contractState"
  >
    <header class="relative overflow-hidden border-b border-[#eee4d2] bg-gradient-to-r from-[#fbfaf6] via-white to-[#f3f6ff] px-4 py-4 sm:px-5 dark:border-dark-700 dark:from-dark-900 dark:via-dark-900 dark:to-blue-950/20">
      <span aria-hidden="true" class="absolute -right-10 -top-16 h-40 w-40 rounded-full bg-blue-300/20 blur-3xl" />
      <div class="relative flex flex-wrap items-center justify-between gap-3">
        <div class="flex min-w-0 items-center gap-3">
          <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-white shadow-sm ring-1 ring-[#e8dfcf]">
            <ModelIcon :model="modelId" size="30px" />
          </span>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="break-all text-lg font-bold tracking-tight text-[#181713] sm:text-xl dark:text-white">{{ modelId }}</h2>
              <span
                class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold ring-1"
                :class="contractState === 'verified'
                  ? 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-300 dark:ring-emerald-900/50'
                  : 'bg-amber-50 text-amber-800 ring-amber-200 dark:bg-amber-950/30 dark:text-amber-300 dark:ring-amber-900/50'"
              >
                <i class="h-1.5 w-1.5 rounded-full" :class="contractState === 'verified' ? 'bg-emerald-500' : 'bg-amber-500'" />
                {{ contractState === 'verified' ? '证据完整' : '验证中' }}
              </span>
            </div>
            <p class="mt-1 text-[11px] text-[#8d806b] dark:text-dark-400">{{ contractStatusDetail }}</p>
          </div>
        </div>
        <span class="rounded-full bg-white px-2.5 py-1 text-[10px] font-semibold text-[#756853] shadow-sm ring-1 ring-[#e8dfcf] dark:bg-dark-800 dark:text-dark-300 dark:ring-dark-700">
          {{ displayPlans.length }} 个可用分组
        </span>
      </div>
    </header>

    <div class="px-3 pb-3 pt-3 sm:px-4">
      <section class="overflow-hidden rounded-xl border border-[#e8dfcf] bg-white dark:border-dark-700 dark:bg-dark-900" aria-label="模型规格">
        <dl class="grid grid-cols-2 divide-x divide-y divide-[#eee6d8] sm:grid-cols-4 sm:divide-y-0 dark:divide-dark-700">
          <div class="px-3 py-2.5" data-field="context-window">
            <dt class="text-[9px] font-semibold tracking-[0.08em] text-[#9b8d76]">上下文</dt>
            <template v-if="contract">
              <dd class="mt-0.5 font-mono text-[12px] font-semibold text-[#25221d] dark:text-white">{{ formatExactTokens(contract.model.context_window) }}</dd>
              <small class="text-[9px] text-[#a89b87]">{{ formatCompactTokens(contract.model.context_window) }} tokens</small>
            </template>
            <MissingEvidence v-else />
          </div>
          <div class="px-3 py-2.5" data-field="max-output-tokens">
            <dt class="text-[9px] font-semibold tracking-[0.08em] text-[#9b8d76]">最大输出</dt>
            <template v-if="contract?.model.max_output_tokens != null">
              <dd class="mt-0.5 font-mono text-[12px] font-semibold text-[#25221d] dark:text-white">{{ formatExactTokens(contract.model.max_output_tokens) }}</dd>
              <small class="text-[9px] text-[#a89b87]">{{ formatCompactTokens(contract.model.max_output_tokens) }} tokens</small>
            </template>
            <dd v-else class="mt-1 text-[10px] font-semibold text-amber-700 dark:text-amber-300">未公开</dd>
          </div>
          <div class="px-3 py-2.5" data-field="groups">
            <dt class="text-[9px] font-semibold tracking-[0.08em] text-[#9b8d76]">分组 · 倍率</dt>
            <template v-if="displayPlans.length <= 3">
              <dd v-for="plan in displayPlans" :key="`${modelId}-${plan.key}`" class="mt-0.5">
                <span class="flex items-baseline justify-between gap-2">
                  <span class="truncate text-[11px] font-semibold text-[#25221d] dark:text-white">{{ plan.name }}</span>
                  <span class="shrink-0 font-mono text-[10px] font-semibold text-[#756853]">{{ plan.rateLabel }}</span>
                </span>
                <span v-if="plan.priceLabel" class="mt-0.5 block text-[8px] leading-4 text-[#9b8d76]">{{ plan.priceLabel }}</span>
              </dd>
            </template>
            <details v-else class="group mt-1">
              <summary class="cursor-pointer list-none text-[11px] font-semibold text-[#25221d] marker:content-none dark:text-white">
                {{ displayPlans.length }} 个分组 · {{ planRateRange }}
                <svg viewBox="0 0 16 16" aria-hidden="true" class="ml-1 inline h-3 w-3 fill-none stroke-current transition group-open:rotate-180" stroke-width="1.5"><path d="m4 6 4 4 4-4" /></svg>
              </summary>
              <div class="mt-1.5 space-y-1 border-t border-[#eee6d8] pt-1.5">
                <template v-for="(plan, index) in displayPlans" :key="`${modelId}-${plan.key}`">
                  <p v-if="index === 0 || displayPlans[index - 1]?.isExclusive !== plan.isExclusive" class="pt-1 text-[8px] font-semibold tracking-[0.08em] text-[#a0927a]">
                    {{ plan.isExclusive ? '专属分组' : '公开分组' }}
                  </p>
                  <div class="flex items-baseline justify-between gap-2">
                    <span class="truncate text-[10px] text-[#554b3d] dark:text-dark-200">{{ plan.name }}</span>
                    <span class="font-mono text-[9px] text-[#756853]">{{ plan.rateLabel }}</span>
                  </div>
                </template>
              </div>
            </details>
          </div>
          <div class="px-3 py-2.5" data-field="model-protocols">
            <dt class="text-[9px] font-semibold tracking-[0.08em] text-[#9b8d76]">模型协议</dt>
            <dd
              v-for="protocol in displayProtocols"
              :key="protocol.key"
              class="mt-1 flex flex-wrap items-center gap-1.5 text-[10px] font-semibold"
              :class="statusTextClass(protocol.status)"
              :data-protocol-status="protocol.status"
            >
              <i class="h-1.5 w-1.5 rounded-full" :class="statusDotClass(protocol.status)" />
              {{ protocol.label }}
              <span v-if="protocol.recommended" class="model-protocol-recommended rounded-full bg-blue-50 px-1.5 py-0.5 text-[7px] font-bold text-blue-700 ring-1 ring-blue-100">推荐</span>
              <span v-else-if="protocol.status === 'blocked'" class="rounded-full bg-amber-50 px-1.5 py-0.5 text-[7px] font-bold text-amber-700 ring-1 ring-amber-100">缺证据</span>
              <span v-else-if="protocol.status === 'unsupported'" class="rounded-full bg-gray-100 px-1.5 py-0.5 text-[7px] font-bold text-gray-500">不支持</span>
            </dd>
            <MissingEvidence v-if="displayProtocols.length === 0" />
          </div>
        </dl>

        <div class="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-[#eee6d8] px-3 py-2.5 dark:border-dark-700" data-field="native-io">
          <span class="text-[9px] font-semibold tracking-[0.08em] text-[#9b8d76]">模型原生输入</span>
          <div
            v-for="capability in inputCapabilities"
            :key="capability.key"
            class="relative inline-flex items-center gap-1.5 text-[10px] font-semibold"
            :class="statusTextClass(capability.status)"
            :data-input-modality="capability.key"
            :data-status="capability.status"
            :data-supported="capability.status === 'verified'"
            :title="capabilityHint(capability.key, capability.status)"
          >
            <span class="relative flex h-6 w-6 items-center justify-center rounded-md border bg-white" :class="capability.status === 'verified' ? 'border-blue-200' : capability.status === 'blocked' ? 'border-amber-200' : 'border-gray-200 dark:border-dark-700 dark:bg-dark-800'">
              <span v-if="capability.key === 'text'" class="font-serif text-xs font-black">T</span>
              <svg v-else-if="capability.key === 'image'" viewBox="0 0 20 20" aria-hidden="true" class="h-3.5 w-3.5 fill-none stroke-current" stroke-width="1.5"><rect x="2.5" y="3.5" width="15" height="13" rx="2"/><circle cx="7" cy="8" r="1.25"/><path d="m4.5 14 4-4 2.5 2.5 2-2 2.5 3.5"/></svg>
              <svg v-else viewBox="0 0 20 20" aria-hidden="true" class="h-3.5 w-3.5 fill-none stroke-current" stroke-width="1.5"><rect x="2.5" y="4" width="11" height="12" rx="2"/><path d="m13.5 8 4-2v8l-4-2z"/></svg>
              <i v-if="capability.status === 'unsupported'" aria-hidden="true" class="absolute inset-x-0 top-1/2 h-px -rotate-45 bg-gray-300" />
              <span v-else-if="capability.status === 'blocked'" aria-hidden="true" class="absolute -right-1 -top-1 flex h-3 w-3 items-center justify-center rounded-full bg-amber-100 text-[8px] font-black text-amber-700">?</span>
            </span>
            <span>{{ capability.label }}</span>
          </div>
          <span class="ml-auto text-[9px] text-[#9b8d76]">输出 · <strong class="font-semibold" :class="contract ? 'text-[#756853]' : 'text-amber-700'">{{ outputLabel }}</strong></span>
        </div>
      </section>

      <section class="flex items-center gap-2 border-b border-[#eee6d8] px-1 py-2.5 dark:border-dark-700" :aria-label="`${modelId} Base URL`" data-field="base-url">
        <span class="shrink-0 text-[9px] font-semibold tracking-[0.08em] text-[#9b8d76]">Base URL</span>
        <code class="min-w-0 flex-1 truncate font-mono text-[10px] text-[#554b3d]">{{ baseURL }}</code>
        <span v-if="!contract" class="hidden shrink-0 text-[8px] font-semibold text-amber-700 sm:inline">路由待验证</span>
        <button type="button" class="inline-flex shrink-0 items-center gap-1 rounded-md px-1.5 py-1 text-[9px] font-semibold text-[#756853] transition hover:bg-[#f7f2e9] hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-200" :aria-label="`复制 ${modelId} Base URL`" @click="copyBaseURL">
          <svg viewBox="0 0 16 16" aria-hidden="true" class="h-3 w-3 fill-none stroke-current" stroke-width="1.5"><rect x="5.25" y="5.25" width="7.5" height="7.5" rx="1.5" /><path d="M3.5 10.75H3A1.75 1.75 0 0 1 1.25 9V3A1.75 1.75 0 0 1 3 1.25h6A1.75 1.75 0 0 1 10.75 3v.5" /></svg>
          {{ copied ? '已复制' : '复制' }}
        </button>
      </section>

      <section class="py-3" :aria-labelledby="`${modelId}-tools`" data-field="tools">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h3 :id="`${modelId}-tools`" class="text-[12px] font-semibold text-[#25221d] dark:text-white">可用工具</h3>
          <span class="text-[9px] text-[#9b8d76]">{{ verifiedClientCount }} 个已通过<span v-if="blockedClientCount"> · {{ blockedClientCount }} 个缺证据</span></span>
        </div>
        <div v-if="displayClients.length" class="mt-2 grid grid-cols-2 divide-x divide-y divide-[#eee6d8] overflow-hidden rounded-lg border border-[#eee6d8] sm:grid-cols-4 dark:divide-dark-700 dark:border-dark-700">
          <div v-for="client in displayClients" :key="client.name" class="relative flex min-h-12 items-center gap-2 px-2.5 py-2 text-[10px]" :class="client.recommended ? 'bg-[#f7f9ff]' : 'bg-white dark:bg-dark-900'" :data-client-status="client.status">
            <span v-if="client.recommended" class="model-tool-recommended absolute right-1.5 top-1.5 rounded-full bg-blue-600 px-1.5 py-0.5 text-[7px] font-bold tracking-wide text-white">推荐</span>
            <span class="flex h-7 w-7 shrink-0 items-center justify-center overflow-hidden rounded-md bg-white ring-1 ring-black/5">
              <img v-if="clientIcon(client.name)" :src="clientIcon(client.name) || undefined" :alt="`${client.name} 图标`" class="h-full w-full object-contain" />
              <span v-else class="text-[10px] font-bold text-gray-500">{{ client.name.slice(0, 1) }}</span>
            </span>
            <span class="min-w-0">
              <span class="block truncate font-semibold text-[#25221d] dark:text-white">{{ client.name }}</span>
              <span class="mt-0.5 inline-flex items-center gap-1 text-[8px] font-medium" :class="statusTextClass(client.status)"><i class="h-1 w-1 rounded-full" :class="statusDotClass(client.status)" />{{ statusLabel(client.status) }}</span>
            </span>
          </div>
        </div>
        <div v-else class="mt-2 rounded-lg border border-dashed border-amber-200 bg-amber-50/60 px-3 py-2 text-[10px] font-semibold text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/20 dark:text-amber-300" :data-client-status="blockedClientCount ? 'blocked' : undefined">缺证据：尚未确认可用工具</div>
      </section>

      <details class="group border-t border-[#eee6d8] pt-2 dark:border-dark-700" aria-label="推理强度兼容" data-field="reasoning">
        <summary class="flex cursor-pointer list-none items-center justify-between gap-3 text-[10px] font-semibold text-[#756853] marker:content-none">
          <span>推理强度</span>
          <span class="text-[9px] font-normal text-[#9b8d76]">{{ reasoningSummary }} <svg viewBox="0 0 16 16" aria-hidden="true" class="ml-1 inline h-3 w-3 fill-none stroke-current transition group-open:rotate-180" stroke-width="1.5"><path d="m4 6 4 4 4-4" /></svg></span>
        </summary>
        <div v-if="reasoningLevels.length || verifiedReasoningClients.length || verifiedReasoningMappings.length" class="mt-2 flex flex-wrap items-start gap-x-5 gap-y-2 pb-1">
          <div v-if="reasoningLevels.length">
            <span class="text-[8px] font-semibold tracking-[0.08em] text-[#9b8d76]">模型档位</span>
            <div class="mt-1 flex flex-wrap gap-1">
              <code v-for="effort in reasoningLevels" :key="effort.level" class="rounded px-1.5 py-0.5 text-[8px]" :class="effort.status === 'verified' ? 'bg-[#f3eee5] text-[#6f6250] dark:bg-dark-800 dark:text-dark-300' : 'bg-amber-50 text-amber-700 dark:bg-amber-950/30 dark:text-amber-300'" :data-reasoning-status="effort.status">{{ effort.level }}<span v-if="effort.status === 'blocked'">?</span></code>
            </div>
          </div>
          <div v-for="client in verifiedReasoningClients" :key="client.client">
            <span class="text-[8px] font-semibold tracking-[0.08em] text-[#9b8d76]">{{ client.client }}</span>
            <div class="mt-1 flex flex-wrap gap-1"><code v-for="effort in client.levels" :key="effort" class="rounded bg-[#f3eee5] px-1.5 py-0.5 text-[8px] text-[#6f6250] dark:bg-dark-800 dark:text-dark-300">{{ effort }}</code></div>
          </div>
          <div v-if="verifiedReasoningMappings.length" class="min-w-0 flex-1 sm:text-right">
            <span class="text-[8px] font-semibold tracking-[0.08em] text-[#9b8d76]">已验证映射</span>
            <div class="mt-1 flex flex-wrap gap-1 sm:justify-end"><code v-for="mapping in verifiedReasoningMappings" :key="`${mapping.client}-${mapping.from}`" class="rounded bg-[#eef3ff] px-1.5 py-0.5 text-[8px] text-blue-700 dark:bg-blue-950/30 dark:text-blue-300">{{ mapping.client }} · {{ mapping.from }} → {{ mapping.to }}</code></div>
          </div>
        </div>
        <p v-else class="mt-2 rounded bg-amber-50 px-2 py-1.5 text-[9px] font-semibold text-amber-700 dark:bg-amber-950/20 dark:text-amber-300">缺证据：尚未确认推理强度</p>
      </details>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import type { ModelDocContract, ModelDocProtocolName } from '@/generated/modelDocContracts'

type EvidenceStatus = 'verified' | 'blocked' | 'unsupported' | 'not_exposed'

interface ImageGenerationPricing {
  mode: 'fixed_per_image' | 'token'
  price_per_image?: number
  text_input_price?: number
  image_input_price?: number
  image_output_price?: number
}

interface AvailabilityPlan {
  groupId: number
  name: string
  multiplier: number
  isExclusive: boolean
  subscriptionType: string
  imageGeneration?: ImageGenerationPricing
}

interface DisplayPlan {
  key: string
  name: string
  rateLabel: string
  priceLabel?: string
  minimumRate: number
  maximumRate: number
  isExclusive: boolean
}

interface DisplayProtocol {
  key: string
  label: string
  status: EvidenceStatus
  recommended: boolean
}

interface DisplayClient {
  name: string
  status: Exclude<EvidenceStatus, 'not_exposed'>
  recommended: boolean
}

const MissingEvidence = defineComponent({
  name: 'MissingEvidence',
  setup: () => () => h('dd', { class: 'mt-1 text-[10px] font-semibold text-amber-700 dark:text-amber-300' }, '缺证据'),
})

const props = withDefaults(defineProps<{
  modelId: string
  contract?: ModelDocContract
  plans: AvailabilityPlan[]
  candidateProtocols?: string[]
}>(), {
  contract: undefined,
  candidateProtocols: () => [],
})

const copied = ref(false)
const contract = computed(() => props.contract)
const testMatrix = computed<Record<string, unknown> | undefined>(() => props.contract?.test_matrix)
const blockedEvidenceCount = computed(() => props.contract?.publication.missing_evidence.blocked_cells
  ?? collectStatuses(testMatrix.value).filter(status => status === 'blocked').length)
const contractState = computed<'verified' | 'pending'>(() => props.contract?.publication.publishable ? 'verified' : 'pending')
const contractStatusDetail = computed(() => {
  if (!props.contract) return '合同缺失 · 规格与兼容性待补证据'
  if (blockedEvidenceCount.value) return `${blockedEvidenceCount.value} 项证据待补，不作为已支持展示`
  return `合同证据更新于 ${props.contract.verification.verified_at}`
})
const baseURL = computed(() => props.contract?.access.base_url || 'https://api.laoshirenai.com/v1')

const displayPlans = computed<DisplayPlan[]>(() => {
  const monthly = props.plans.filter(isMonthlyPlan)
  const normal = props.plans.filter(plan => !isMonthlyPlan(plan)).map(plan => ({
    key: String(plan.groupId), name: plan.name, rateLabel: formatMultiplier(plan.multiplier),
    priceLabel: plan.imageGeneration ? formatImagePricing(plan.imageGeneration) : undefined,
    minimumRate: plan.multiplier, maximumRate: plan.multiplier, isExclusive: plan.isExclusive,
  }))
  if (monthly.length) {
    const rates = monthly.map(plan => plan.multiplier)
    const minimumRate = Math.min(...rates)
    const maximumRate = Math.max(...rates)
    normal.push({
      key: 'monthly', name: '月卡组',
      rateLabel: minimumRate === maximumRate ? formatMultiplier(minimumRate) : `${formatMultiplier(minimumRate)}–${formatMultiplier(maximumRate)}`,
      priceLabel: undefined, minimumRate, maximumRate, isExclusive: true,
    })
  }
  return normal.sort((left, right) => {
    if (left.isExclusive !== right.isExclusive) return left.isExclusive ? 1 : -1
    if (left.minimumRate !== right.minimumRate) return left.minimumRate - right.minimumRate
    return left.name.localeCompare(right.name, 'zh-CN')
  })
})

const displayProtocols = computed<DisplayProtocol[]>(() => {
  if (!props.contract) return props.candidateProtocols.map(label => ({ key: label, label: candidateProtocolLabel(label), status: 'blocked', recommended: false }))
  const protocols = [...props.contract.protocols, ...(props.contract.protocol_candidates ?? [])]
  const unique = new Map(protocols.map(protocol => [protocol.name, protocol]))
  return [...unique.values()].map(protocol => {
    const status = resolvedProtocolStatus(protocol.name, protocol.status)
    return { key: protocol.name, label: protocolLabel(protocol.name), status, recommended: status === 'verified' && protocol.name === props.contract?.recommended_protocol }
  })
})

const outputLabel = computed(() => {
  const outputs = props.contract?.model.output_modalities
  if (!outputs?.length) return '缺证据'
  if (outputs.length === 1 && outputs[0] === 'text') return 'Text · 仅文本输出'
  return outputs.map(labelModality).join(' / ')
})

const inputCapabilities = computed(() => (['text', 'image', 'video'] as const).map(key => ({
  key, label: labelModality(key), status: normalizeStatus(props.contract?.verification.modalities[key]) ?? 'blocked',
})))

const displayClients = computed<DisplayClient[]>(() => {
  if (!props.contract) return []
  return props.contract.clients.flatMap(client => {
    const coverage = props.contract?.client_coverage.find(item => item.name === client.name)
    const exactVerified = props.contract?.compatibility.exact_clients.some(item => (
      item.client === client.name
      && item.model_id === props.contract?.model.id
      && item.protocol === client.protocol
      && item.status === 'verified'
      && Boolean(item.client_version)
      && Boolean(item.verified_at)
    ))
    const declaredProtocol = props.contract?.protocols.find(protocol => protocol.name === client.protocol)
    const protocolVerified = declaredProtocol
      ? resolvedProtocolStatus(client.protocol, declaredProtocol.status) === 'verified'
      : false
    if (coverage?.status !== 'verified' || !coverage.protocols.includes(client.protocol) || !exactVerified || !protocolVerified) return []
    return [{ name: client.name, status: 'verified', recommended: client.recommended } satisfies DisplayClient]
  }).sort((left, right) => Number(right.recommended) - Number(left.recommended) || left.name.localeCompare(right.name))
})

const verifiedClientCount = computed(() => displayClients.value.filter(client => client.status === 'verified').length)
const blockedClientCount = computed(() => props.contract?.client_coverage.filter(client => client.status === 'blocked').length ?? 0)
const reasoningLevels = computed(() => (props.contract?.reasoning?.model_levels ?? []).map(level => ({ level, status: reasoningLevelStatus(level) })))
const verifiedReasoningClients = computed(() => (props.contract?.reasoning?.client_levels ?? []).filter(client => reasoningClientStatus(client.client) === 'verified'))
const verifiedReasoningMappings = computed(() => (props.contract?.reasoning?.client_mappings ?? []).filter(mapping => reasoningClientStatus(mapping.client, mapping.protocol) === 'verified'))
const reasoningSummary = computed(() => {
  if (!reasoningLevels.value.length) return '缺证据'
  const verified = reasoningLevels.value.filter(level => level.status === 'verified').length
  const blocked = reasoningLevels.value.length - verified
  return blocked ? `${verified} 个已验证 · ${blocked} 个缺证据` : `${verified} 个档位已验证`
})
const planRateRange = computed(() => {
  if (!displayPlans.value.length) return '—'
  const minimum = Math.min(...displayPlans.value.map(plan => plan.minimumRate))
  const maximum = Math.max(...displayPlans.value.map(plan => plan.maximumRate))
  return minimum === maximum ? formatMultiplier(minimum) : `${formatMultiplier(minimum)}–${formatMultiplier(maximum)}`
})

function resolvedProtocolStatus(name: ModelDocProtocolName, declared: string): EvidenceStatus {
  void name
  // Protocol support and protocol features are separate matrix dimensions.
  // A protocol can be verified for text while a tool/stream capability is
  // explicitly unsupported; the tools/evidence sections communicate those
  // limits without relabeling the whole protocol as unsupported.
  return normalizeStatus(declared) ?? 'blocked'
}

function reasoningLevelStatus(level: string): 'verified' | 'blocked' {
  const projected = props.contract?.compatibility.reasoning.model_levels.find(item => item.level === level)
  if (projected) return projected.status === 'verified' ? 'verified' : 'blocked'
  const levels = getRecord(getRecord(testMatrix.value, 'reasoning'), 'model_levels')
  if (!levels) return testMatrix.value ? 'blocked' : 'verified'
  return normalizeStatus(getRecord(levels, level)?.status) === 'verified' ? 'verified' : 'blocked'
}

function reasoningClientStatus(name: string, protocol?: ModelDocProtocolName): EvidenceStatus | undefined {
  const projected = props.contract?.compatibility.reasoning.clients.filter(item => item.client === name && (!protocol || item.protocol === protocol)) ?? []
  if (projected.length) {
    const statuses = projected.map(item => normalizeStatus(item.status) ?? 'blocked')
    if (statuses.includes('verified')) return 'verified'
    if (statuses.includes('blocked')) return 'blocked'
    if (statuses.includes('unsupported')) return 'unsupported'
    if (statuses.includes('not_exposed')) return 'not_exposed'
  }
  const clients = getRecord(getRecord(testMatrix.value, 'reasoning'), 'clients')
  const client = getRecord(clients, name)
  if (!client) return undefined
  const target = protocol ? getRecord(client, protocol) : client
  const statuses = collectStatuses(target)
  if (statuses.includes('verified')) return 'verified'
  if (statuses.includes('blocked')) return 'blocked'
  if (statuses.includes('unsupported')) return 'unsupported'
  if (statuses.includes('not_exposed')) return 'not_exposed'
  return undefined
}

function getRecord(value: unknown, key: string): Record<string, unknown> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined
  const child = (value as Record<string, unknown>)[key]
  if (!child || typeof child !== 'object' || Array.isArray(child)) return undefined
  return child as Record<string, unknown>
}

function collectStatuses(value: unknown): EvidenceStatus[] {
  if (Array.isArray(value)) return value.flatMap(collectStatuses)
  if (!value || typeof value !== 'object') return []
  const record = value as Record<string, unknown>
  const own = normalizeStatus(record.status)
  return [...(own ? [own] : []), ...Object.values(record).flatMap(collectStatuses)]
}

function normalizeStatus(value: unknown): EvidenceStatus | undefined {
  if (value === 'verified' || value === 'blocked' || value === 'unsupported' || value === 'not_exposed') return value
  return undefined
}

function isMonthlyPlan(plan: AvailabilityPlan): boolean {
  return plan.subscriptionType === 'subscription' || plan.subscriptionType === 'credit' || plan.name.includes('月卡')
}

function capabilityHint(key: 'text' | 'image' | 'video', status: EvidenceStatus): string {
  const label = labelModality(key)
  if (status === 'verified') return `${label}：模型原生支持，证据已验证`
  if (status === 'blocked') return `${label}：缺少完整证据，暂不作为支持展示`
  if (key === 'video') return '视频：模型原生不支持；客户端抽帧或本地工具不算原生支持'
  return `${label}：模型原生不支持`
}

async function copyBaseURL(): Promise<void> {
  try { await navigator.clipboard.writeText(baseURL.value) } catch {
    const textarea = document.createElement('textarea')
    textarea.value = baseURL.value
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    textarea.remove()
  }
  copied.value = true
  window.setTimeout(() => { copied.value = false }, 1600)
}

function formatMultiplier(value: number): string { return `${Number(value.toFixed(4))}×` }
function formatExactTokens(value: number): string { return new Intl.NumberFormat('en-US').format(value) }
function formatCompactTokens(value: number): string {
  if (value === 1_048_576 || value === 1_000_000) return '1M'
  if (value >= 1_000_000) return `${Number((value / 1_000_000).toFixed(2))}M`
  if (value % 1_000 === 0) return `${Number((value / 1_000).toFixed(1))}K`
  if (value % 1_024 === 0) return `${Number((value / 1_024).toFixed(1))}K`
  if (value >= 1_000) return `${Number((value / 1_000).toFixed(1))}K`
  return String(value)
}

function formatImagePricing(pricing: ImageGenerationPricing): string {
  if (pricing.mode === 'fixed_per_image' && typeof pricing.price_per_image === 'number') return `¥${Number(pricing.price_per_image.toFixed(4))}/张`
  const parts: string[] = []
  if (typeof pricing.text_input_price === 'number') parts.push(`文本输入 ¥${Number(pricing.text_input_price.toFixed(4))}/M`)
  if (typeof pricing.image_input_price === 'number') parts.push(`图片输入 ¥${Number(pricing.image_input_price.toFixed(4))}/M`)
  if (typeof pricing.image_output_price === 'number') parts.push(`图片输出 ¥${Number(pricing.image_output_price.toFixed(4))}/M`)
  return parts.join(' · ') || '价格以实时目录为准'
}

function labelModality(value: string): string {
  if (value === 'text') return '文本'
  if (value === 'image') return '图片'
  if (value === 'video') return '视频'
  return value
}

function candidateProtocolLabel(value: string): string { return value.replace('OpenAI ', '').replace('Anthropic ', '').replace('Gemini ', '') }
function protocolLabel(value: ModelDocProtocolName): string {
  if (value === 'responses') return 'Responses'
  if (value === 'chat_completions') return 'Chat Completions'
  if (value === 'messages') return 'Messages'
  if (value === 'generate_content') return 'GenerateContent'
  return 'Images API'
}

function statusTextClass(status: EvidenceStatus): string {
  if (status === 'verified') return 'text-emerald-700 dark:text-emerald-300'
  if (status === 'blocked') return 'text-amber-700 dark:text-amber-300'
  return 'text-gray-400 dark:text-dark-500'
}
function statusDotClass(status: EvidenceStatus): string {
  if (status === 'verified') return 'bg-emerald-500'
  if (status === 'blocked') return 'bg-amber-500'
  return 'bg-gray-400'
}
function statusLabel(status: EvidenceStatus): string {
  if (status === 'verified') return '已验证'
  if (status === 'blocked') return '缺证据'
  if (status === 'unsupported') return '不支持'
  return '未开放'
}

function clientIcon(name: string): string | null {
  const normalized = name.toLocaleLowerCase()
  if (normalized === 'zcode') return '/tool-icons/zcode.png'
  if (normalized === 'opencode') return '/tool-icons/opencode.png'
  if (normalized === 'kimi code') return '/tool-icons/kimi.png'
  if (normalized === 'grok build') return '/tool-icons/grok.png'
  if (normalized === 'codex') return '/brand/client-tools/codex-light.png'
  if (normalized === 'claude code') return '/brand/client-tools/claude.svg'
  if (normalized === 'gemini cli') return '/brand/client-tools/gemini.svg'
  if (normalized === 'antigravity') return '/tool-icons/antigravity.png'
  return null
}
</script>
