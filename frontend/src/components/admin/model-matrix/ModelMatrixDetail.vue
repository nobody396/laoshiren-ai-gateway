<template>
  <div class="grid gap-5 border-t border-slate-200 bg-slate-50/70 px-5 py-5 dark:border-slate-700 dark:bg-slate-950/30 xl:grid-cols-[1.05fr_1fr_1.2fr]">
    <section class="space-y-4">
      <div>
        <h3 class="detail-title">模型合同</h3>
        <dl class="mt-3 grid grid-cols-2 gap-2">
          <div class="metric"><dt>上下文</dt><dd>{{ compact(row.model.model.context_window) }}</dd></div>
          <div class="metric"><dt>最大输出</dt><dd>{{ compact(row.model.model.max_output_tokens) }}</dd></div>
          <div class="metric"><dt>原生输入</dt><dd>{{ row.model.model.input_modalities.join(' · ') }}</dd></div>
          <div class="metric"><dt>原生输出</dt><dd>{{ row.model.model.output_modalities.join(' · ') }}</dd></div>
        </dl>
      </div>

      <div>
        <h3 class="detail-title">分组与 Base URL</h3>
        <code class="mt-2 block break-all rounded-lg bg-slate-900 px-3 py-2 text-xs text-slate-100">{{ row.model.access.base_url }}</code>
        <div class="mt-2 space-y-2">
          <div v-for="group in row.model.access.groups" :key="group.name" class="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900">
            <div class="flex items-center justify-between gap-2 text-sm"><strong>{{ group.name }}</strong><span class="font-mono text-xs text-slate-500">{{ group.multiplier }}×</span></div>
            <div v-if="priceFor(group.name)" class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs text-slate-500">
              <span>输入 {{ price(priceFor(group.name)?.inputPrice) }}</span><span>输出 {{ price(priceFor(group.name)?.outputPrice) }}</span>
              <span>缓存写 {{ price(priceFor(group.name)?.cacheWritePrice) }}</span><span>缓存读 {{ price(priceFor(group.name)?.cacheReadPrice) }}</span>
            </div>
            <p v-else class="mt-1 text-xs text-amber-700 dark:text-amber-300">价格目录暂无此分组读回</p>
          </div>
        </div>
      </div>
    </section>

    <section class="space-y-4">
      <div>
        <h3 class="detail-title">协议能力与工具</h3>
        <div class="mt-3 flex flex-wrap gap-2">
          <span v-for="feature in features" :key="feature.name" class="feature-chip">
            <span>{{ feature.name }}</span><MatrixStatusBadge :status="feature.status" compact />
          </span>
        </div>
      </div>

      <div>
        <h3 class="detail-title">推理强度交集</h3>
        <p class="mt-2 text-xs leading-5 text-slate-500">模型原生：{{ row.model.reasoning?.model_levels?.join(' / ') || '未声明' }}</p>
        <p class="text-xs leading-5 text-slate-500">客户端可请求：{{ row.clientRequestedLevels.join(' / ') || '由模型目录决定' }}</p>
        <div class="mt-2 flex flex-wrap gap-1.5">
          <span v-for="level in row.reasoningLevels" :key="level" class="reasoning-chip">{{ level }}</span>
          <span v-if="!row.reasoningLevels.length" class="text-xs text-slate-400">无可确认交集</span>
        </div>
        <p v-if="row.client.client_reasoning.fallback?.strategy" class="mt-2 text-xs text-slate-500">
          回退：{{ row.client.client_reasoning.fallback.strategy }} · {{ row.client.client_reasoning.fallback.notes }}
        </p>
      </div>
    </section>

    <section class="space-y-4">
      <div>
        <div class="flex items-center justify-between gap-2"><h3 class="detail-title">配置文件与 OS 证据</h3><span class="text-xs text-slate-400">{{ row.client.client_config_os.release.version_key }}</span></div>
        <div class="mt-3 space-y-2">
          <div v-for="result in row.osResults" :key="result.os" class="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900">
            <div class="flex items-center justify-between"><strong class="text-sm uppercase">{{ result.os }}</strong><MatrixStatusBadge :status="result.status" compact /></div>
            <code v-for="file in result.configFiles" :key="file.path" class="mt-2 block break-all text-[11px] text-slate-600 dark:text-slate-300">{{ file.path }} · {{ file.format }}</code>
            <p class="mt-2 text-xs leading-5 text-slate-500">{{ result.evidence }}</p>
          </div>
        </div>
      </div>

      <div class="rounded-lg bg-slate-900 p-3 text-xs text-slate-200">
        <p class="font-semibold text-white">端点规则</p><p class="mt-1 leading-5">{{ row.client.client_config_os.endpoint.base_url_rule }}</p>
        <p class="mt-3 font-semibold text-white">凭证位置</p><p class="mt-1 break-all font-mono text-[11px]">{{ row.client.client_config_os.endpoint.credential_location }}</p>
        <p class="mt-3 font-semibold text-white">安全写入</p><p class="mt-1 leading-5">{{ row.client.client_config_os.mutation.merge_strategy }}</p>
      </div>

      <div class="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900">
        <h3 class="detail-title">命令与证据</h3>
        <div v-if="verificationCommands.length" class="mt-2 space-y-1.5">
          <code v-for="command in verificationCommands" :key="command" class="block break-all rounded bg-slate-950 px-2 py-1.5 text-[10px] text-slate-100">{{ command }}</code>
        </div>
        <p v-else class="mt-2 text-xs text-slate-400">当前配置合同没有可公开命令。</p>
        <div v-if="receiptIds.length" class="mt-3 flex flex-wrap gap-1.5">
          <button v-for="evidenceId in receiptIds" :key="evidenceId" type="button" class="receipt-button" @click="selectedReceiptId = evidenceId">{{ evidenceId }}</button>
        </div>
        <p v-else class="mt-3 text-xs text-slate-400">当前单元格没有可解析的 receipt ID。</p>
        <div v-if="selectedReceipt" class="receipt-detail" data-testid="receipt-detail">
          <div class="flex items-center justify-between gap-2"><strong>{{ selectedReceiptId }}</strong><MatrixStatusBadge :status="receiptStatus" compact /></div>
          <p class="mt-2 leading-5">{{ selectedReceipt.summary || '无摘要' }}</p>
          <dl class="mt-2 space-y-1 font-mono text-[10px]">
            <div><dt>artifact</dt><dd>{{ selectedReceipt.artifact_uri || '—' }}</dd></div>
            <div><dt>sha256</dt><dd>{{ selectedReceipt.artifact_sha256 || '—' }}</dd></div>
            <div><dt>observed</dt><dd>{{ selectedReceipt.observed_at || '—' }}</dd></div>
          </dl>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { AdminEvidenceReceipt } from '@/api/admin/modelClientMatrix'
import MatrixStatusBadge from './MatrixStatusBadge.vue'
import { matrixFeatureCells, type ModelClientMatrixRow } from './matrix'

const props = defineProps<{ row: ModelClientMatrixRow; evidenceIndex?: Record<string, AdminEvidenceReceipt> }>()
const features = computed(() => matrixFeatureCells(props.row.model, props.row.protocol))
const selectedReceiptId = ref('')
const verificationCommands = computed(() => (props.row.client.client_config_os.verification_commands ?? [])
  .flatMap(item => item.commands)
  .slice(0, 4))
const receiptIds = computed(() => {
  const source = JSON.stringify({
    os: props.row.osResults,
    features: features.value,
    reasoning: props.row.model.test_matrix?.reasoning,
  })
  const ids = source.match(/evidence-[0-9a-f]{24}/g) ?? []
  return [...new Set(ids)].filter(id => props.evidenceIndex?.[id]).slice(0, 24)
})
const selectedReceipt = computed(() => selectedReceiptId.value ? props.evidenceIndex?.[selectedReceiptId.value] : undefined)
const receiptStatus = computed(() => selectedReceipt.value?.result === 'pass' ? 'verified' : selectedReceipt.value?.result === 'unsupported' ? 'unsupported' : 'blocked')

function compact(value: number | null): string {
  if (value == null) return '待确认 / 未公开'
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(value % 1_000_000 ? 1 : 0)}M`
  if (value >= 1_000) return `${Math.round(value / 1_000)}K`
  return String(value)
}
function price(value: number | null | undefined): string { return value == null ? '—' : `¥${value}/1M` }
function priceFor(group: string) { return props.row.prices.find(item => item.group === group) }
</script>

<style scoped>
.detail-title { @apply text-xs font-bold uppercase tracking-[0.12em] text-slate-500 dark:text-slate-400; }
.metric { @apply rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900; }
.metric dt { @apply text-[11px] text-slate-500; }
.metric dd { @apply mt-1 break-words text-sm font-semibold text-slate-900 dark:text-white; }
.feature-chip { @apply inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200; }
.reasoning-chip { @apply rounded-md bg-indigo-50 px-2 py-1 font-mono text-xs font-semibold text-indigo-700 dark:bg-indigo-950/50 dark:text-indigo-300; }
.receipt-button { @apply rounded-md border border-slate-200 px-2 py-1 font-mono text-[9px] text-indigo-600 hover:border-indigo-300 hover:bg-indigo-50 dark:border-slate-700 dark:text-indigo-300 dark:hover:bg-indigo-950/40; }
.receipt-detail { @apply mt-3 rounded-lg border border-indigo-100 bg-indigo-50/50 p-3 text-xs text-slate-600 dark:border-indigo-900/60 dark:bg-indigo-950/20 dark:text-slate-300; }
.receipt-detail dl div { @apply grid grid-cols-[4.5rem_minmax(0,1fr)] gap-2; }
.receipt-detail dt { @apply text-slate-400; }
.receipt-detail dd { @apply break-all; }
</style>
