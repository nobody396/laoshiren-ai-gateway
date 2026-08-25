<template>
  <div ref="pageRef" class="home-page" :class="{ 'home-page--reveal-ready': revealReady }">
    <!-- 顶部公告条(自动赔付上线,可关闭) -->
    <ZenBanner />

    <!-- 头部导航 -->
    <ZenHeader
      :is-authenticated="isAuthenticated"
      :dashboard-path="dashboardPath"
      :nav-items="navItems"
    />

    <main>
      <!-- Hero 主视觉 -->
      <ZenHero
        :is-authenticated="isAuthenticated"
        :dashboard-path="dashboardPath"
      />

      <!-- 数据计数器(TOKEN 处理量 / 累计赔付金额) -->
      <ZenCounters
        :tokens-millions="tokensMillions"
        :compensation-cny="compensationCny"
      />

      <!-- 模型墙(国际 + 国产双 marquee) -->
      <ZenModelWall />

      <!-- 自动赔付 -->
      <ZenCompensation />

      <!-- 接入网格 -->
      <ZenAccess />

      <!-- 收尾 CTA -->
      <ZenCta
        :is-authenticated="isAuthenticated"
        :dashboard-path="dashboardPath"
      />
    </main>

    <!-- 页脚 -->
    <ZenFooter :sections="footerSections" />
  </div>
</template>

<script setup lang="ts">
/**
 * 落地页主视图 - 组装层(ZenMux 白底风格 + 希腊点缀)
 * 负责数据定义和子组件编排,视觉实现下沉到 components/home/zen/*
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import { getPublicStats } from '@/api/publicStats'

// 子组件导入
import ZenBanner from '@/components/home/zen/ZenBanner.vue'
import ZenHeader from '@/components/home/zen/ZenHeader.vue'
import ZenHero from '@/components/home/zen/ZenHero.vue'
import ZenCounters from '@/components/home/zen/ZenCounters.vue'
import ZenModelWall from '@/components/home/zen/ZenModelWall.vue'
import ZenCompensation from '@/components/home/zen/ZenCompensation.vue'
import ZenAccess from '@/components/home/zen/ZenAccess.vue'
import ZenCta from '@/components/home/zen/ZenCta.vue'
import ZenFooter from '@/components/home/zen/ZenFooter.vue'
import { useReveal } from '@/components/home/zen/useReveal'

const authStore = useAuthStore()
const appStore = useAppStore()
const { locale } = useI18n()

// 认证相关状态
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const showModelReports = computed(() => appStore.cachedPublicSettings?.landing_reports_enabled !== false)
const isEnglish = computed(() => locale.value === 'en')

// ── 导航项(# 开头为页内锚点,其余为路由路径) ──
const navItems = computed(() => (isEnglish.value
  ? [
    { label: 'Models', href: '#models' },
    { label: 'Compensation', href: '#compensation' },
    { label: 'Access', href: '#access' },
    { label: 'Pricing', href: '/models' },
    { label: 'Status', href: '/status' },
    { label: 'Changelog', href: '/changelog' },
    { label: 'Docs', href: '/docs' }
  ]
  : [
    { label: '模型', href: '#models' },
    { label: '自动赔付', href: '#compensation' },
    { label: '接入', href: '#access' },
    { label: '定价', href: '/models' },
    { label: '服务状态', href: '/status' },
    { label: '更新日志', href: '/changelog' },
    { label: '文档', href: '/docs' }
  ]))

// ── 公开统计计数器 ──
// 首次成功拉取前的兜底展示值(与线上同口径,服务端已含 ×10 显示倍率)
const FALLBACK_TOKENS_MILLIONS = 221911.14
const FALLBACK_COMPENSATION_CNY = 17838.0
const STATS_POLL_INTERVAL_MS = 12000

const tokensMillions = ref(0)
const compensationCny = ref(0)
let statsEntryTimer: number | null = null
let statsPollTimer: number | null = null

async function pullStats(): Promise<void> {
  try {
    const stats = await getPublicStats()
    if (typeof stats.tokens_total === 'number') tokensMillions.value = stats.tokens_total / 1e6
    if (typeof stats.compensation_cny === 'number') compensationCny.value = stats.compensation_cny
  } catch {
    /* 接口不可达时保持当前数值 */
  }
}

// ── 页脚链接 ──
const footerSections = computed(() => {
  const labels = isEnglish.value
    ? {
      product: 'Product',
      intro: 'LaoshirenAI Overview',
      reports: 'Model verification reports',
      pricing: 'Pricing',
      enterprise: 'Enterprise',
      changelog: 'Build in Public',
      login: 'Login',
      guides: 'Guides',
      claudeCodeChina: 'Claude Code in China',
      codexChina: 'Codex in China',
      codexNoApiKey: 'Codex without API key',
      codexCustomApi: 'Codex custom API',
      commitment: 'Commitment',
      status: 'Service status',
      security: 'Security & privacy',
      legal: 'Legal',
      terms: 'Terms of service',
      usage: 'Usage policy',
      regions: 'Supported countries and regions',
      specific: 'Service-specific terms',
      affiliate: 'Affiliate program rules'
    }
    : {
      product: '产品',
      intro: '老实人AI 介绍',
      reports: '模型检测报告',
      pricing: '价格方案',
      enterprise: '企业方案',
      changelog: '公开构建',
      login: '登录',
      guides: '高意图指南',
      claudeCodeChina: 'Claude Code 国内使用',
      codexChina: 'Codex 国内使用',
      codexNoApiKey: 'Codex 免 API Key 使用',
      codexCustomApi: 'Codex 自定义 API 配置',
      commitment: '服务承诺',
      status: '服务状态',
      security: '安全与隐私',
      legal: '合规条款',
      terms: '服务条款',
      usage: '使用政策',
      regions: '支持的国家和地区',
      specific: '服务特定条款',
      affiliate: '联盟计划规则'
    }

  return [
    {
      title: labels.product,
      links: [
        { label: labels.intro, href: '#about', external: false },
        ...(showModelReports.value
          ? [{ label: labels.reports, href: '#model-reports', external: false }]
          : []),
        { label: labels.pricing, href: '/models', external: false },
        { label: labels.enterprise, href: '/enterprise', external: false },
        { label: labels.changelog, href: '/changelog', external: false },
        { label: labels.login, href: '/login', external: false }
      ]
    },
    {
      title: labels.guides,
      links: [
        { label: labels.claudeCodeChina, href: '/docs/claude-code-china-guide', external: false },
        { label: labels.codexChina, href: '/docs/codex-china-guide', external: false },
        { label: labels.codexNoApiKey, href: '/docs/codex-no-api-key-guide', external: false },
        { label: labels.codexCustomApi, href: '/docs/codex-custom-api-guide', external: false }
      ]
    },
    {
      title: labels.commitment,
      links: [
        { label: labels.status, href: '/status', external: false },
        { label: labels.security, href: '/security', external: false }
      ]
    },
    {
      title: labels.legal,
      links: [
        { label: labels.terms, href: '/legal/terms', external: false },
        { label: labels.usage, href: '/legal/usage-policy', external: false },
        { label: labels.regions, href: '/legal/supported-regions', external: false },
        { label: labels.specific, href: '/legal/service-specific-terms', external: false },
        { label: labels.affiliate, href: '/legal/affiliate-program', external: false }
      ]
    }
  ]
})

// ── 滚动进场动画(IntersectionObserver) ──
const pageRef = ref<HTMLElement | null>(null)
const { revealReady } = useReveal(pageRef)

onMounted(() => {
  // 认证检查
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  // 计数器:先停留在 0,短暂延迟后滚动到兜底值形成入场动画;
  // 首次接口成功后以真实数据为准,之后每 12s 轮询,失败保持当前值。
  statsEntryTimer = window.setTimeout(() => {
    if (tokensMillions.value === 0) tokensMillions.value = FALLBACK_TOKENS_MILLIONS
    if (compensationCny.value === 0) compensationCny.value = FALLBACK_COMPENSATION_CNY
  }, 350)
  void pullStats()
  statsPollTimer = window.setInterval(() => {
    void pullStats()
  }, STATS_POLL_INTERVAL_MS)
})

onUnmounted(() => {
  if (statsEntryTimer != null) {
    window.clearTimeout(statsEntryTimer)
  }
  if (statsPollTimer != null) {
    window.clearInterval(statsPollTimer)
  }
})
</script>

<style scoped>
.home-page {
  min-height: 100vh;
  background: rgb(var(--zen-bg));
  color: rgb(var(--zen-ink));
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Noto Sans SC', 'Microsoft YaHei', sans-serif;
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
  position: relative;
  overflow-x: hidden;
}

.home-page :deep(a),
.home-page :deep(button) {
  -webkit-tap-highlight-color: transparent;
}

.home-page :deep(#models),
.home-page :deep(#compensation),
.home-page :deep(#access) {
  scroll-margin-top: 88px;
}
</style>

<style>
/* 全局滚动进场动画(需要非 scoped 以穿透子组件)
 * 时长/缓动/错峰全部取自 theme.css 的 Motion 令牌,调节奏改那一处。
 * 入场用 animation 而不是 transition —— 组件自己的 `transition:` 简写会把
 * 共享 reveal 类的整条 transition(连同 delay)覆盖掉(见 AGENTS.md)。 */
@keyframes zenRise {
  from {
    opacity: 0;
    transform: translateY(18px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.zen-reveal {
  opacity: 1;
  transform: none;
}

.home-page--reveal-ready .zen-reveal:not(.is-visible) {
  opacity: 0;
}

.home-page--reveal-ready .zen-reveal.is-visible {
  /* 错峰:--zen-reveal-i 由模板内联给出,没给就是 0 */
  animation: zenRise var(--duration-reveal) var(--ease-expo) both;
  animation-delay: calc(var(--reveal-stagger) * var(--zen-reveal-i, 0));
}

/* 初始态一律挂在 --reveal-ready 门控下:JS 没跑起来时直接是最终样式 */
@media (prefers-reduced-motion: reduce) {
  .home-page--reveal-ready .zen-reveal:not(.is-visible) {
    opacity: 1;
    transform: none;
  }

  .home-page--reveal-ready .zen-reveal.is-visible {
    animation: none;
  }
}
</style>
