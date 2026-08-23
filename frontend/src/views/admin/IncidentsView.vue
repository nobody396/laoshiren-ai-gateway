<template>
  <AppLayout>
    <div class="mx-auto max-w-[1400px] space-y-6 p-4 sm:p-6 lg:p-8">
      <header class="flex flex-wrap items-end justify-between gap-4">
        <div><p class="text-sm font-semibold text-primary-600">Reliability Control</p><h1 class="mt-1 text-3xl font-bold">Incident 管理</h1><p class="mt-2 text-sm text-gray-500">确认候选、维护阶段、记录内部过程，并单独发布脱敏后的公开时间线。</p></div>
        <button class="rounded-xl bg-primary-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60" :disabled="loading || acting" @click="load">{{ loading ? '刷新中…' : '刷新' }}</button>
      </header>

      <div v-if="error" role="alert" class="rounded-2xl border border-danger-200 bg-danger-50 p-4 text-sm text-danger-700">{{ error }}</div>
      <section v-if="snapshot" class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-4"><div><h2 class="font-bold">功能开关</h2><p class="text-xs text-gray-500">关闭后保留全部 Incident 证据；不会改变 Service Status 或路由。</p></div><div class="flex gap-4 text-sm"><label><input v-model="settings.enabled" type="checkbox" /> 自动归集</label><label><input v-model="settings.public_enabled" type="checkbox" :disabled="!settings.enabled" /> 公开时间线</label><button class="text-primary-600" @click="saveSettings">保存</button></div></div>
      </section>

      <section v-if="snapshot" class="space-y-3" aria-labelledby="candidate-title">
        <div class="flex items-center justify-between"><h2 id="candidate-title" class="text-xl font-bold">Incident Candidates</h2><span class="text-xs text-gray-500">{{ openCandidates.length }} 个待确认</span></div>
        <p v-if="!openCandidates.length" class="rounded-2xl border border-dashed p-8 text-center text-sm text-gray-500">当前没有待确认候选。</p>
        <article v-for="candidate in openCandidates" :key="candidate.id" class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="flex flex-wrap justify-between gap-3"><div><strong>Candidate #{{ candidate.id }}</strong><p class="text-xs text-gray-500">{{ formatTime(candidate.first_observed_at) }} — {{ formatTime(candidate.last_observed_at) }}</p></div><div class="flex flex-wrap gap-2"><span v-for="product in candidate.products" :key="product.product_code" class="rounded-full bg-warning-50 px-3 py-1 text-xs text-warning-800">{{ product.product_name }} · {{ statusLabel(product.latest_status) }}</span></div></div>
          <div class="mt-4 grid gap-3 lg:grid-cols-[1fr_2fr_auto]"><input v-model="candidateDrafts[candidate.id].title" aria-label="Incident 标题" class="rounded-xl border px-3 py-2 text-sm dark:bg-dark-900" placeholder="内部 Incident 标题" /><input v-model="candidateDrafts[candidate.id].summary" aria-label="内部摘要" class="rounded-xl border px-3 py-2 text-sm dark:bg-dark-900" placeholder="内部摘要（不会公开）" /><button data-test="confirm-candidate" :disabled="acting" class="rounded-xl bg-primary-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60" @click="confirm(candidate.id)">确认 Incident</button></div>
          <div class="mt-3 flex gap-3"><input v-model="candidateDrafts[candidate.id].dismiss" aria-label="忽略原因" class="min-w-0 flex-1 rounded-xl border px-3 py-2 text-sm dark:bg-dark-900" placeholder="若忽略，请填写原因" /><button class="text-sm text-danger-600" @click="dismiss(candidate.id)">忽略候选</button></div>
        </article>
      </section>

      <section v-if="snapshot" class="space-y-4" aria-labelledby="incident-title">
        <h2 id="incident-title" class="text-xl font-bold">Incident Timeline</h2>
        <p v-if="!snapshot.incidents.length" class="rounded-2xl border border-dashed p-8 text-center text-sm text-gray-500">尚无 Incident。</p>
        <article v-for="incident in snapshot.incidents" :key="incident.id" class="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <header class="flex flex-wrap items-start justify-between gap-3 border-b p-5 dark:border-dark-700"><div><span class="text-xs font-mono text-gray-500">{{ incident.public_id }}</span><h3 class="mt-1 text-lg font-bold">{{ incident.title }}</h3><p class="mt-1 text-sm text-gray-500">{{ incident.internal_summary || '暂无内部摘要' }}</p><div v-if="incident.evidence_gap" data-test="evidence-gap" class="mt-2 space-y-2 text-sm font-semibold text-danger-600"><p>证据存在超过七天的恢复缺口：禁止 Resolved 与恢复公告，需人工复核。</p><div class="flex gap-2"><input v-model="incidentDrafts[incident.id].gapReason" aria-label="证据缺口复核原因" class="min-w-0 flex-1 rounded-xl border px-3 py-2 text-sm font-normal text-gray-900 dark:bg-dark-900 dark:text-white" placeholder="填写复核结论后解除阻断" /><button data-test="acknowledge-gap" :disabled="acting" class="text-sm" @click="acknowledgeGap(incident.id)">确认已复核</button></div></div></div><span class="rounded-full border px-3 py-1 text-xs font-semibold">{{ phaseLabel(incident.phase) }}</span></header>
          <div class="grid gap-6 p-5 xl:grid-cols-[1fr_1.2fr]">
            <div class="space-y-4"><article v-for="product in incident.products" :key="product.id" class="rounded-xl border p-4 dark:border-dark-700"><div class="flex justify-between"><strong>{{ product.product_name }}</strong><span class="text-xs">{{ statusLabel(product.current_status) }}</span></div><p class="mt-2 text-xs text-gray-500">客户影响 {{ formatDuration(product.compensable_seconds) }} · {{ product.segments.length }} 个连续区间</p><ol class="mt-3 space-y-1 text-xs text-gray-500"><li v-for="segment in product.segments" :key="segment.id">{{ formatTime(segment.started_at) }} → {{ segment.ended_at ? formatTime(segment.ended_at) : '持续中' }}</li></ol></article></div>
            <div class="space-y-4"><ol class="space-y-3"><li v-for="update in incident.updates" :key="update.id" class="border-l-2 border-primary-200 pl-3 text-sm"><strong>{{ phaseLabel(update.phase) }}</strong><p>{{ update.internal_message }}</p><time class="text-xs text-gray-500">{{ formatTime(update.created_at) }}</time></li></ol>
              <div class="grid gap-2 sm:grid-cols-[180px_1fr_auto]"><select v-model="incidentDrafts[incident.id].phase" aria-label="目标阶段" class="rounded-xl border px-3 py-2 text-sm dark:bg-dark-900"><option v-for="phase in phases" :key="phase" :value="phase" :disabled="phase === 'resolved' && incident.evidence_gap">{{ phaseLabel(phase) }}</option></select><input v-model="incidentDrafts[incident.id].transition" aria-label="阶段说明" class="rounded-xl border px-3 py-2 text-sm dark:bg-dark-900" placeholder="阶段变更说明" /><button :disabled="acting" class="text-sm text-primary-600 disabled:opacity-50" @click="transition(incident.id)">变更阶段</button></div>
              <div class="flex gap-2"><input v-model="incidentDrafts[incident.id].internal" aria-label="内部更新" class="min-w-0 flex-1 rounded-xl border px-3 py-2 text-sm dark:bg-dark-900" placeholder="内部更新" /><button class="text-sm text-primary-600" @click="addInternal(incident.id)">记录</button></div>
              <div class="flex gap-2"><select v-model="incidentDrafts[incident.id].public" aria-label="公开更新" class="min-w-0 flex-1 rounded-xl border px-3 py-2 text-sm dark:bg-dark-900"><option value="">选择与当前阶段匹配的公开模板</option><option v-for="message in publicTemplatesFor(incident.phase)" :key="message" :value="message" :disabled="incident.evidence_gap && incident.phase === 'resolved'">{{ message }}</option></select><button :disabled="acting || (incident.evidence_gap && incident.phase === 'resolved')" class="text-sm text-success-600 disabled:opacity-50" @click="publish(incident.id)">发布公开时间线</button></div>
            </div>
          </div>
        </article>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { acknowledgeIncidentEvidenceGap, addIncidentUpdate, confirmIncidentCandidate, dismissIncidentCandidate, getIncidentAdminSnapshot, publishIncidentUpdate, transitionIncident, updateIncidentSettings, type IncidentAdminSnapshot, type IncidentPhase } from '@/api/admin/incidents'
import type { ServiceStatus } from '@/api/serviceStatus'

const phases: IncidentPhase[] = ['investigating', 'identified', 'mitigating', 'monitoring', 'resolved']
const publicTemplates: Record<IncidentPhase, string[]> = { investigating: ['我们正在调查部分服务异常，用户暂时无需进行额外操作。'], identified: ['我们已经定位到服务异常，正在进行处理。'], mitigating: ['部分服务仍受到影响，我们正在继续处理。'], monitoring: ['相关服务正在恢复，我们将继续观察。'], resolved: ['相关服务已经恢复，用户无需进行额外操作。'] }
const snapshot = ref<IncidentAdminSnapshot | null>(null)
const loading = ref(false)
const acting = ref(false)
const error = ref('')
const settings = reactive({ enabled: false, public_enabled: false })
const candidateDrafts = reactive<Record<number, { title: string; summary: string; dismiss: string }>>({})
const incidentDrafts = reactive<Record<number, { phase: IncidentPhase; transition: string; internal: string; public: string; gapReason: string }>>({})
let controller: AbortController | null = null
let stopped = false
const openCandidates = computed(() => snapshot.value?.candidates.filter((candidate) => candidate.state === 'open') ?? [])

async function load() {
  controller?.abort(); const current = new AbortController(); controller = current; loading.value = true; error.value = ''
  try {
    const next = await getIncidentAdminSnapshot(current.signal)
    if (controller !== current || current.signal.aborted) return
    snapshot.value = next; Object.assign(settings, next.settings)
    for (const candidate of next.candidates) candidateDrafts[candidate.id] ??= { title: '', summary: '', dismiss: '' }
    for (const incident of next.incidents) incidentDrafts[incident.id] ??= { phase: incident.phase, transition: '', internal: '', public: '', gapReason: '' }
  } catch (cause) { if (!current.signal.aborted) error.value = cause instanceof Error ? cause.message : 'Incident Control 加载失败' }
  finally { if (controller === current && !current.signal.aborted) loading.value = false }
}
async function execute(action: () => Promise<unknown>) { if (acting.value || stopped) return; acting.value = true; try { error.value = ''; await action(); if (!stopped) await load() } catch (cause) { if (!stopped) error.value = cause instanceof Error ? cause.message : '操作失败' } finally { if (!stopped) acting.value = false } }
function saveSettings() { void execute(() => updateIncidentSettings({ ...settings })) }
function confirm(id: number) { const draft = candidateDrafts[id]; void execute(() => confirmIncidentCandidate(id, draft.title, draft.summary)) }
function dismiss(id: number) { void execute(() => dismissIncidentCandidate(id, candidateDrafts[id].dismiss)) }
function transition(id: number) { const draft = incidentDrafts[id]; void execute(() => transitionIncident(id, draft.phase, draft.transition)) }
function addInternal(id: number) { void execute(() => addIncidentUpdate(id, incidentDrafts[id].internal)) }
function publish(id: number) { void execute(() => publishIncidentUpdate(id, incidentDrafts[id].public)) }
function acknowledgeGap(id: number) { void execute(() => acknowledgeIncidentEvidenceGap(id, incidentDrafts[id].gapReason)) }
function formatTime(value: string) { return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(Date.parse(value)) }
function formatDuration(seconds: number) { const minutes = Math.round(seconds / 60); return minutes < 60 ? `${minutes} 分钟` : `${(minutes / 60).toFixed(1)} 小时` }
function phaseLabel(value: IncidentPhase) { return ({ investigating: '调查中', identified: '已定位', mitigating: '缓解中', monitoring: '观察中', resolved: '已解决' } as const)[value] }
function publicTemplatesFor(phase: IncidentPhase) { return publicTemplates[phase] }
function statusLabel(value: ServiceStatus) { return ({ operational: '正常', degraded_performance: '性能下降', partial_outage: '部分中断', major_outage: '大范围中断', maintenance: '维护中', monitoring: '观察中' } as const)[value] }
onMounted(() => { stopped = false; void load() })
onBeforeUnmount(() => { stopped = true; controller?.abort() })
</script>
