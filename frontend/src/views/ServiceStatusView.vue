<template>
  <div class="service-status-page">
    <header class="status-header">
      <router-link to="/" class="status-brand">
        <img src="/laoshirenai-icon.jpg" alt="" aria-hidden="true" />
        <span>老实人AI</span>
      </router-link>
      <nav aria-label="公开页面导航">
        <router-link to="/enterprise">企业</router-link>
        <router-link to="/security">安全</router-link>
        <router-link to="/status">状态</router-link>
        <router-link to="/changelog">更新日志</router-link>
        <router-link to="/legal/terms">条款</router-link>
        <router-link to="/legal/affiliate-program">联盟规则</router-link>
        <router-link v-if="PUBLIC_DOCS_ENABLED" to="/docs">文档</router-link>
        <router-link to="/login">登录</router-link>
      </nav>
    </header>

    <main class="status-main">
      <div v-if="loading" class="status-loading" aria-live="polite">
        <span class="sr-only">正在加载服务状态</span>
        <div class="loading-line loading-title" />
        <div class="loading-line" />
        <div class="loading-panel" />
      </div>

      <section v-else-if="liveSnapshot" data-test="service-status-live" aria-labelledby="status-title">
        <div class="status-intro">
          <div>
            <h1 id="status-title">服务状态</h1>
            <p>查看当前公开服务的可用性。出现调用异常时，可先在这里确认影响范围。</p>
          </div>
          <p class="status-updated" :class="{ stale: snapshotIsStale }">
            {{ snapshotIsStale ? '状态数据更新延迟' : '最近更新' }}
            <time :datetime="liveSnapshot.generated_at">{{ formatServiceStatusTime(liveSnapshot.generated_at) }}</time>
          </p>
        </div>

        <section v-if="allProducts.length" class="status-overview" :class="`service-status-tone-${overallStatus}`" aria-label="整体可用性">
          <span class="overview-dot" aria-hidden="true" />
          <div>
            <strong>{{ overallCopy.title }}</strong>
            <p>{{ overallCopy.description }}</p>
          </div>
        </section>

        <div v-if="allProducts.length" class="availability-rail" aria-hidden="true">
          <span
            v-for="product in allProducts"
            :key="product.code"
            :class="`service-status-tone-${serviceStatusForPresentation(product, nowMs)}`"
          />
        </div>

        <div v-if="!allProducts.length" class="empty-catalog" role="status">
          <strong>暂未发布可展示的服务</strong>
          <p>服务目录正在整理中。你仍可以通过文档和反馈入口排查问题。</p>
        </div>

        <div v-else class="status-sections">
          <section v-if="commonFamilies.length" aria-labelledby="common-services-title">
            <div class="section-heading">
              <h2 id="common-services-title">常用服务</h2>
              <span>{{ commonProductCount }} 项</span>
            </div>
            <div class="family-list">
              <StatusFamilyBlock
                v-for="family in commonFamilies"
                :key="family.code"
                :family="family"
                :now-ms="nowMs"
              />
            </div>
          </section>

          <section v-if="otherFamilies.length" aria-labelledby="other-services-title">
            <div class="section-heading">
              <h2 id="other-services-title">其他服务</h2>
              <span>{{ otherProductCount }} 项</span>
            </div>
            <div class="family-list">
              <StatusFamilyBlock
                v-for="family in otherFamilies"
                :key="family.code"
                :family="family"
                :now-ms="nowMs"
              />
            </div>
          </section>
        </div>

        <section v-if="publicIncidents.length" class="incident-timeline" aria-labelledby="incident-timeline-title">
          <div class="section-heading"><h2 id="incident-timeline-title">事件时间线</h2><span>{{ publicIncidents.length }} 项</span></div>
          <article v-for="incident in publicIncidents" :key="incident.id" class="incident-card">
            <header><div><strong>{{ incident.resolved_at ? '已恢复' : phaseLabel(incident.phase) }}</strong><p>{{ incident.affected_products.join('、') }}</p></div><time :datetime="incident.started_at">{{ formatServiceStatusTime(incident.started_at) }}</time></header>
            <ol><li v-for="update in incident.timeline" :key="`${update.published_at}:${update.message}`"><span class="incident-dot" aria-hidden="true" /><div><p>{{ update.message }}</p><time :datetime="update.published_at">{{ formatServiceStatusTime(update.published_at) }}</time></div></li></ol>
          </article>
        </section>

        <aside class="status-help">
          <div>
            <strong>仍然无法判断问题？</strong>
            <p>请保留请求时间、模型名称与报错信息，再通过反馈入口联系我们。</p>
          </div>
          <router-link to="/feedbacks">提交反馈</router-link>
        </aside>
      </section>

      <section v-else class="status-fallback" data-test="service-status-fallback">
        <div v-if="fallbackDueToError" class="fallback-notice" role="status">
          <strong>实时状态暂不可用</strong>
          <p>下面保留服务排查与支持说明；线上服务本身不一定受到影响。</p>
        </div>
        <DocsContent
          :html="renderedFallbackHtml"
          :markdown="fallbackMarkdown"
          :loading="fallbackLoading"
          :not-found="fallbackNotFound"
        />
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import DocsContent from '@/components/docs/DocsContent.vue'
import StatusFamilyBlock from '@/components/status/StatusFamilyBlock.vue'
import { useMarkdownRenderer } from '@/composables/useMarkdownRenderer'
import { loadMarkdown } from '@/docs/config'
import { PUBLIC_DOCS_ENABLED } from '@/config/publicFeatures'
import {
  getPublicIncidents,
  getServiceStatus,
  type PublicIncident,
  type PublicIncidentPhase,
  type ServiceStatus,
  type ServiceStatusSnapshot
} from '@/api/serviceStatus'
import {
  SERVICE_STATUS_STALE_AFTER_MS,
  formatServiceStatusTime,
  isServiceStatusProductStale,
  maxServiceStatus,
  serviceStatusDateMs,
  serviceStatusForPresentation,
  worstServiceStatus
} from '@/features/service-status/presentation'

const REFRESH_INTERVAL_MS = 30_000

const loading = ref(true)
const snapshot = ref<ServiceStatusSnapshot | null>(null)
const publicIncidents = ref<PublicIncident[]>([])
const fallbackMarkdown = ref('')
const fallbackLoading = ref(false)
const fallbackNotFound = ref(false)
const fallbackDueToError = ref(false)
const nowMs = ref(Date.now())
const { renderedHtml: renderedFallbackHtml } = useMarkdownRenderer(fallbackMarkdown)
let refreshTimer: ReturnType<typeof setInterval> | null = null
let refreshInFlight = false
let stopped = false

const liveSnapshot = computed(() => snapshot.value?.enabled ? snapshot.value : null)
const allProducts = computed(() => liveSnapshot.value?.families.flatMap((family) => family.products) ?? [])
const commonFamilies = computed(() => liveSnapshot.value?.families.filter((family) => family.code !== 'other' && family.products.length) ?? [])
const otherFamilies = computed(() => liveSnapshot.value?.families.filter((family) => family.code === 'other' && family.products.length) ?? [])
const commonProductCount = computed(() => commonFamilies.value.reduce((count, family) => count + family.products.length, 0))
const otherProductCount = computed(() => otherFamilies.value.reduce((count, family) => count + family.products.length, 0))
const snapshotIsStale = computed(() => {
  if (!liveSnapshot.value) return false
  const generatedAt = serviceStatusDateMs(liveSnapshot.value.generated_at)
  return !generatedAt || nowMs.value - generatedAt > SERVICE_STATUS_STALE_AFTER_MS || allProducts.value.some((product) => isServiceStatusProductStale(product, nowMs.value))
})
const overallStatus = computed<ServiceStatus>(() => {
  const observed = worstServiceStatus(allProducts.value, nowMs.value)
  return snapshotIsStale.value ? maxServiceStatus(observed, 'monitoring') : observed
})
const overallCopy = computed(() => {
  switch (overallStatus.value) {
    case 'major_outage': return { title: '部分常用服务当前不可用', description: '我们已经记录到较大范围的调用影响。' }
    case 'partial_outage': return { title: '部分服务受到影响', description: '只有页面列出的服务或范围受到影响。' }
    case 'degraded_performance': return { title: '服务性能有所下降', description: '请求通常仍可完成，但体验可能不稳定。' }
    case 'maintenance': return { title: '部分服务正在维护', description: '维护期间请关注下方服务状态。' }
    case 'monitoring': return { title: '部分服务正在观察', description: '我们正在确认最新恢复情况与证据。' }
    default: return { title: '当前公开服务运行正常', description: '未发现影响正常调用的公开故障。' }
  }
})

async function ensureFallbackDocument() {
  if (fallbackMarkdown.value || fallbackLoading.value) return
  fallbackLoading.value = true
  fallbackNotFound.value = false
  try {
    const markdown = await loadMarkdown('sla-support')
    if (stopped) return
    if (markdown === null) fallbackNotFound.value = true
    else fallbackMarkdown.value = markdown
  } catch {
    if (!stopped) {
      fallbackMarkdown.value = '# 服务支持说明\n\n实时状态和完整排查文档暂不可用。请保留请求时间、模型名称与报错信息，稍后重试或提交反馈。'
    }
  } finally {
    if (!stopped) fallbackLoading.value = false
  }
}

async function refreshStatus(initial = false) {
  if (stopped || refreshInFlight) return
  refreshInFlight = true
  if (initial) loading.value = true
  nowMs.value = Date.now()
  try {
    const next = await getServiceStatus()
    if (stopped) return
    snapshot.value = next
    void refreshIncidentTimeline()
    fallbackDueToError.value = false
    if (!next.enabled) await ensureFallbackDocument()
  } catch {
    if (stopped) return
    snapshot.value = null
    fallbackDueToError.value = true
    await ensureFallbackDocument()
  } finally {
    refreshInFlight = false
    if (initial && !stopped) loading.value = false
  }
}

async function refreshIncidentTimeline() {
  try {
    const next = await getPublicIncidents()
    if (!stopped) publicIncidents.value = next.enabled ? next.incidents : []
  } catch {
    if (!stopped) publicIncidents.value = []
  }
}

function phaseLabel(phase: PublicIncidentPhase) {
  return ({ investigating: '调查中', identified: '已定位', mitigating: '缓解中', monitoring: '观察中', resolved: '已恢复' } as const)[phase]
}

onMounted(() => {
  stopped = false
  void refreshStatus(true)
  refreshTimer = setInterval(() => void refreshStatus(false), REFRESH_INTERVAL_MS)
})

onBeforeUnmount(() => {
  stopped = true
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.service-status-page {
  min-height: 100vh;
  background: rgb(var(--color-vellum));
  color: rgb(var(--color-ink));
}
.status-header {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  min-height: 4rem;
  align-items: center;
  justify-content: space-between;
  gap: 1.5rem;
  border-bottom: 1px solid rgb(var(--color-gray-200));
  background: rgb(var(--color-vellum) / 0.92);
  padding: 0 2rem;
  backdrop-filter: blur(12px);
}
.status-brand { display: inline-flex; align-items: center; gap: .75rem; color: rgb(var(--color-ink-deep)); font-weight: 750; text-decoration: none; }
.status-brand img { width: 2rem; height: 2rem; border-radius: 999px; object-fit: cover; }
.status-header nav { display: flex; flex-wrap: wrap; align-items: center; gap: 1rem; }
.status-header nav a { color: rgb(var(--color-muted)); font-size: .95rem; font-weight: 600; text-decoration: none; }
.status-header nav a:hover, .status-header nav a.router-link-active { color: rgb(var(--color-terracotta)); }
.status-main { width: min(100% - 2rem, 1080px); margin: 0 auto; padding: 4rem 0 5rem; }
.status-intro { display: flex; align-items: end; justify-content: space-between; gap: 2rem; }
.status-intro h1 { margin: 0; color: rgb(var(--color-ink-deep)); font-size: clamp(2.4rem, 7vw, 4.8rem); font-weight: 780; letter-spacing: -.055em; line-height: .95; }
.status-intro p { max-width: 39rem; margin: 1.35rem 0 0; color: rgb(var(--color-muted)); font-size: 1.05rem; line-height: 1.75; }
.status-updated { flex: 0 0 auto; margin: 0 !important; font-size: .82rem !important; text-align: right; }
.status-updated time { display: block; color: rgb(var(--color-ink)); font-variant-numeric: tabular-nums; }
.status-updated.stale { color: rgb(var(--color-warning)) !important; }
.status-overview { display: flex; align-items: center; gap: 1.1rem; margin-top: 3rem; border-block: 1px solid rgb(var(--color-gray-200)); padding: 1.7rem .2rem; }
.overview-dot, .product-dot { flex: 0 0 auto; width: .8rem; height: .8rem; border-radius: 999px; background: currentColor; box-shadow: 0 0 0 .3rem color-mix(in srgb, currentColor 13%, transparent); }
.status-overview strong { color: rgb(var(--color-ink-deep)); font-size: 1.2rem; }
.status-overview p { margin: .3rem 0 0; color: rgb(var(--color-muted)); }
.availability-rail { display: flex; gap: .2rem; height: .3rem; margin: 1.5rem 0 4rem; overflow: hidden; border-radius: 99px; background: rgb(var(--color-gray-200)); }
.availability-rail span { min-width: .4rem; flex: 1; background: currentColor; }
.status-sections { display: grid; gap: 4rem; }
.section-heading { display: flex; align-items: baseline; justify-content: space-between; border-bottom: 1px solid rgb(var(--color-gray-200)); padding-bottom: .8rem; }
.section-heading h2 { margin: 0; color: rgb(var(--color-ink-deep)); font-size: 1.45rem; letter-spacing: -.02em; }
.section-heading span { color: rgb(var(--color-muted)); font-size: .85rem; }
.family-list { display: grid; gap: 2.25rem; margin-top: 2rem; }
.incident-timeline { margin-top: 4rem; }
.incident-card { margin-top: 1.5rem; border: 1px solid rgb(var(--color-gray-200)); border-radius: 1rem; padding: 1.25rem; background: rgb(var(--color-paper)); }
.incident-card header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.incident-card header p { margin: .3rem 0 0; color: rgb(var(--color-muted)); font-size: .9rem; }
.incident-card time { color: rgb(var(--color-muted)); font-size: .78rem; }
.incident-card ol { display: grid; gap: 1rem; margin: 1.25rem 0 0; padding: 0; list-style: none; }
.incident-card li { display: flex; gap: .75rem; }
.incident-card li p { margin: 0 0 .25rem; line-height: 1.55; }
.incident-dot { flex: 0 0 auto; width: .55rem; height: .55rem; margin-top: .4rem; border-radius: 999px; background: rgb(var(--color-terracotta)); }
.status-help { display: flex; align-items: center; justify-content: space-between; gap: 2rem; margin-top: 4rem; border-top: 1px solid rgb(var(--color-gray-200)); padding-top: 1.5rem; }
.status-help strong { color: rgb(var(--color-ink-deep)); }
.status-help p { margin: .3rem 0 0; color: rgb(var(--color-muted)); font-size: .9rem; }
.status-help a { flex: 0 0 auto; border-bottom: 1px solid currentColor; color: rgb(var(--color-terracotta)); font-weight: 700; text-decoration: none; }
.empty-catalog, .fallback-notice { margin: 3rem 0; border-left: .25rem solid rgb(var(--color-muted)); padding: .5rem 0 .5rem 1.25rem; }
.empty-catalog strong, .fallback-notice strong { color: rgb(var(--color-ink-deep)); }
.empty-catalog p, .fallback-notice p { margin: .4rem 0 0; color: rgb(var(--color-muted)); }
.status-fallback { width: min(100%, 860px); margin: 0 auto; }
.status-loading { display: grid; gap: 1rem; }
.loading-line, .loading-panel { border-radius: .5rem; background: linear-gradient(90deg, rgb(var(--color-gray-200)), rgb(var(--color-gray-100)), rgb(var(--color-gray-200))); background-size: 200% 100%; animation: loading var(--duration-ambient) var(--ease-standard) infinite; }
.loading-line { width: 55%; height: 1rem; }
.loading-title { width: 32%; height: 4.5rem; }
.loading-panel { height: 20rem; margin-top: 2rem; }
@keyframes loading { to { background-position: -200% 0; } }
@media (prefers-reduced-motion: reduce) { .loading-line, .loading-panel { animation: none; } }
@media (max-width: 720px) {
  .status-header { align-items: flex-start; flex-direction: column; padding: .9rem 1rem; }
  .status-main { padding-top: 2.5rem; }
  .status-intro { align-items: flex-start; flex-direction: column; }
  .status-updated { text-align: left; }
  .availability-rail { margin-bottom: 3rem; }
  .status-help { align-items: flex-start; flex-direction: column; }
}
</style>
