<template>
  <div class="home-page" :class="{ 'home-page--reveal-ready': revealReady }">
    <!-- 头部导航 -->
    <HomeHeader
      :is-authenticated="isAuthenticated"
      :dashboard-path="dashboardPath"
      :nav-items="navItems"
    />

    <main>
      <!-- Hero 主视觉 -->
      <HeroSection
        :is-authenticated="isAuthenticated"
        :dashboard-path="dashboardPath"
      />

      <!-- 为什么选择老实人AI -->
      <WhyChoose :feature-cards="featureCards" />

      <!-- Claude 组合 -->
      <ClaudeCombinations :models="models" />

      <!-- 模型检测报告 -->
      <ModelReports
        v-if="showModelReports"
        :reports="modelReports"
      />

      <!-- 开发者月卡 -->
      <section
        v-if="!isEnglish"
        id="monthly-credit-cards"
        class="monthly-credit-section"
      >
        <div class="monthly-credit-section__container">
          <MonthlyCreditPlans
            variant="home"
            :title="monthlyCreditTitle"
            summary=""
            :plans="monthlyPlans"
            :show-action="false"
          />
        </div>
      </section>

      <!-- 模型定价（入口跳转到独立价格页） -->
      <ModelPricing />

      <!-- 数据指标（VIP 卡片暂时隐藏） -->
      <VIPTiers />
    </main>

    <!-- 页脚 -->
    <HomeFooter :sections="footerSections" />
  </div>
</template>

<script setup lang="ts">
/**
 * 落地页主视图 - 组装层
 * 负责数据定义和子组件编排，视觉实现下沉到各子组件
 */
import { computed, onMounted, onUnmounted, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'

// 子组件导入
import HomeHeader from '@/components/home/HomeHeader.vue'
import HeroSection from '@/components/home/HeroSection.vue'
import WhyChoose from '@/components/home/WhyChoose.vue'
import ClaudeCombinations from '@/components/home/ClaudeCombinations.vue'
import ModelReports from '@/components/home/ModelReports.vue'
import ModelPricing from '@/components/home/ModelPricing.vue'
import VIPTiers from '@/components/home/VIPTiers.vue'
import HomeFooter from '@/components/home/HomeFooter.vue'
import MonthlyCreditPlans from '@/components/common/MonthlyCreditPlans.vue'
import { useMonthlyCreditCardPlans } from '@/composables/useMonthlyCreditCardPlans'

const authStore = useAuthStore()
const appStore = useAppStore()
const { locale } = useI18n()

// 认证相关状态
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const showModelReports = computed(() => appStore.cachedPublicSettings?.landing_reports_enabled !== false)
const { plans: monthlyPlans, loadMonthlyCreditCardPlans } = useMonthlyCreditCardPlans()
const isEnglish = computed(() => locale.value === 'en')
const monthlyCreditTitle = computed(() => (isEnglish.value ? 'Developer Monthly Cards' : '开发者月卡'))

// ── 导航项 ──
const navItems = computed(() => {
  const labels = isEnglish.value
    ? {
      enterprise: 'Enterprise',
      reports: 'Reports',
      pricing: 'Pricing',
      status: 'Status',
      changelog: 'Changelog',
      docs: 'Docs'
    }
    : {
      enterprise: '企业',
      reports: '报告',
      pricing: '定价',
      status: '服务状态',
      changelog: '更新日志',
      docs: '文档'
    }

  return [
    { label: labels.enterprise, href: '/enterprise', active: false, external: false, routerPush: true },
    ...(showModelReports.value
      ? [{ label: labels.reports, href: '#model-reports', active: false, external: false, routerPush: false }]
      : []),
    { label: labels.pricing, href: '/models', active: false, external: false, routerPush: true },
    { label: labels.status, href: '/status', active: false, external: false, routerPush: true },
    { label: labels.changelog, href: '/changelog', active: false, external: false, routerPush: true },
    { label: labels.docs, href: '/docs', active: false, external: false, routerPush: true }
  ]
})

// ── 特性卡片 ──
const featureCards = computed(() => (isEnglish.value
  ? [
    {
      title: 'Five-minute setup',
      description: 'One domain and one key format for mainstream IDE plugins and CLI tools, easy even for first-time users.'
    },
    {
      title: 'Official-style precise billing',
      description: 'Pricing follows the same dimensions as the official providers, so every charge can be checked.'
    },
    {
      title: 'Every call is traceable',
      description: 'Complete logs and cost breakdowns make it clear where each cent was spent.'
    },
    {
      title: 'Coding-first, no bloat',
      description: 'Only the models that matter for coding workflows, with continuous improvements to stability and latency.'
    }
  ]
  : [
    {
      title: '5 分钟完成配置',
      description: '统一的域名与密钥格式，覆盖主流 IDE 插件与 CLI 工具，新手也能快速上手。'
    },
    {
      title: '与官方一致的精确计费',
      description: '采用与官方完全一致的计价方式，每一笔费用都经得起核对。'
    },
    {
      title: '每一笔调用，费用可查',
      description: '完整的调用日志与费用明细，花了多少、用在哪里，一目了然。'
    },
    {
      title: '专注编码，拒绝臃肿',
      description: '只接入编码场景最需要的模型，持续优化链路稳定性与响应速度。'
    }
  ]))

// ── 模型卡片（三大 AI 品牌，按场景定位展示） ──
const models = computed(() => (isEnglish.value
  ? [
    {
      eyebrow: 'Architecture',
      name: 'Claude',
      subtitle: 'System design and complex reasoning',
      description: 'Best for technical design, code review, and multi-step reasoning across a whole project.'
    },
    {
      eyebrow: 'Implementation',
      name: 'ChatGPT',
      subtitle: 'Code generation and feature work',
      description: 'A daily coding engine for generation, iteration, debugging, and fixes.'
    },
    {
      eyebrow: 'Multimodal',
      name: 'Gemini',
      subtitle: 'Vision understanding and design tasks',
      description: 'Useful for design-to-code, image understanding, and multimodal frontend workflows.'
    }
  ]
  : [
    {
      eyebrow: '架构规划',
      name: 'Claude',
      subtitle: '系统设计与复杂推理',
      description: '擅长技术方案设计、代码审查与多步推理，适合把控项目全局方向。'
    },
    {
      eyebrow: '编码实现',
      name: 'ChatGPT',
      subtitle: '代码生成与功能开发',
      description: '擅长代码生成、功能迭代与调试修复，日常编码的主力引擎。'
    },
    {
      eyebrow: '多模态设计',
      name: 'Gemini',
      subtitle: '图像理解与视觉任务',
      description: '擅长设计稿转代码、图像理解与多模态任务，前端设计的得力助手。'
    }
  ]))

// ── 模型检测报告 ──
const modelReports = computed(() => {
  const verdict = isEnglish.value ? 'Perfect match' : '完美匹配'

  return [
    {
      provider: 'Claude',
      title: 'Claude Opus 4.7',
      sourceUrl: 'https://www.hvoy.ai/report/K9TIp4lRV1Q',
      endpoint: 'https://api.laoshirenai.com',
      modelId: 'claude-opus-4-7',
      score: 100,
      verdict,
      testedAt: '2026-05-20 11:07',
      passedChecks: 9,
      totalChecks: 9,
      latency: '4.5s',
      tps: '15.3',
      inputTokens: '1,855',
      outputTokens: '315'
    },
    {
      provider: 'Claude',
      title: 'Claude Opus 4.6',
      sourceUrl: 'https://www.hvoy.ai/report/q6R_YiEFJhc',
      endpoint: 'https://api.laoshirenai.com',
      modelId: 'claude-opus-4-6',
      score: 100,
      verdict,
      testedAt: '2026-05-20 11:08',
      passedChecks: 10,
      totalChecks: 10,
      latency: '4.0s',
      tps: '15.1',
      inputTokens: '1,781',
      outputTokens: '216'
    },
    {
      provider: 'GPT',
      title: 'GPT-5.5',
      sourceUrl: 'https://www.hvoy.ai/report/H186QGxR4K8',
      endpoint: 'https://api.laoshirenai.com',
      modelId: 'gpt-5.5',
      score: 100,
      verdict,
      testedAt: '2026-05-20 11:09',
      passedChecks: 4,
      totalChecks: 4,
      latency: '9.9s',
      tps: '43.3',
      inputTokens: '407',
      outputTokens: '429'
    },
    {
      provider: 'GPT',
      title: 'GPT-5.4',
      sourceUrl: 'https://www.hvoy.ai/report/Xj8IKpcfEJE',
      endpoint: 'https://api.laoshirenai.com',
      modelId: 'gpt-5.4',
      score: 100,
      verdict,
      testedAt: '2026-05-20 11:10',
      passedChecks: 4,
      totalChecks: 4,
      latency: '2.9s',
      tps: '14.7',
      inputTokens: '393',
      outputTokens: '42'
    }
  ]
})

// ── 套餐定价（保留供后续使用） ──
// @ts-expect-error 暂未使用，后续会接入套餐定价组件
const _pricingPlans = [
  {
    category: '按量付费',
    badge: '自由使用',
    name: 'PAYGO套餐',
    description: '按量付费,一次性获取额度,额度不过期。',
    price: '¥66.00',
    unit: '/ 66美元 起',
    variant: 'is-paygo',
    buttonText: '暂不开放',
    buttonVariant: 'mirror-home__button--outline',
    soldOut: true,
    ctaType: 'route',
    features: [
      '覆盖主流 Claude / Codex 编码模型调用',
      '适合少量使用,随用随停',
      '共可用 173美元 Claude逆向 或 66美元 Claude 或 132美元 Codex'
    ]
  },
  {
    category: '包月付费',
    badge: '最受欢迎',
    name: '包月付费套餐',
    description: '包月付费,每月固定可用额度,额度不结转。',
    price: '¥399.00',
    unit: '/ 月',
    variant: 'is-max',
    buttonText: '暂不开放',
    buttonVariant: 'mirror-home__button--accent',
    soldOut: true,
    ctaType: 'route',
    features: [
      '覆盖主流 Claude / Codex 编码模型调用',
      '每月固定额度,整月共享使用',
      '可与PAYGO套餐同时使用'
    ]
  },
  {
    category: '企业 / 部门',
    badge: '按需定制',
    name: '定制套餐',
    description: '适合有并发率要求、预算管控和集中管理需求的团队。',
    price: '按需定价',
    unit: '',
    variant: 'is-pro',
    buttonText: '联系客服获取定制报价',
    buttonVariant: 'mirror-home__button--primary',
    ctaType: 'anchor',
    href: '#about',
    features: [
      '支持高级审计,团队使用',
      '专享高吞吐量通道,保障业务零延迟',
      '专属技术支持与接入咨询'
    ]
  }
]

// ── VIP 分组（暂未推出） ──
// const vipTiers = [
//   { name: 'VIP1', discount: '0.98', threshold: '累计消费满 ¥500', features: ['全模型倍率优惠', '优先活动资格'] },
//   { name: 'VIP2', discount: '0.96', threshold: '累计消费满 ¥3,000', features: ['并发额度翻倍', '高峰期优先排队'] },
//   { name: 'VIP3', discount: '0.94', threshold: '累计消费满 ¥10,000', features: ['新模型优先体验', '更稳定调用体验'] },
//   { name: 'VIP4', discount: '0.92', threshold: '累计消费满 ¥30,000', features: ['专属技术支持', '最高并发额度'] },
//   { name: 'VIP5', discount: '0.90', threshold: '累计消费满 ¥80,000', features: ['专属客户经理', '定制化服务方案'] }
// ]

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

// ── 滚动进场动画（IntersectionObserver） ──
let observer: IntersectionObserver | null = null
let revealFallbackTimer: number | null = null
const revealReady = ref(false)

function revealAllSections(): void {
  document.querySelectorAll('.mirror-reveal').forEach((node) => {
    node.classList.add('is-visible')
  })
}

/**
 * 兜底只负责"首屏别空着"，不负责整页。
 *
 * 原来的兜底是 1.6 秒后无条件 revealAllSections()，包括视口外几千像素的区块 ——
 * 于是用户滚下去时每一屏都已经是终态，滚动入场动效等于不存在。这正是"每个章节
 * 都没有入场动画"的由来，不是动效没写，是被兜底提前拆了。
 *
 * 现在兜底只揭开当前视口里的元素；视口外的继续交给 observer。真正需要全量兜底的
 * 只有 IntersectionObserver 不可用的情况，那条路径仍然直接调 revealAllSections()。
 */
function revealVisibleSections(): void {
  const vh = window.innerHeight || 0
  document.querySelectorAll('.mirror-reveal').forEach((node) => {
    const r = node.getBoundingClientRect()
    if (r.top < vh && r.bottom > 0) node.classList.add('is-visible')
  })
}

onMounted(() => {
  // 认证检查
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  void loadMonthlyCreditCardPlans()

  // 滚动进场动画 - 等子组件挂载后再绑定
  nextTick(() => {
    const revealNodes = Array.from(document.querySelectorAll('.mirror-reveal'))
    if (revealNodes.length === 0) return

    if (typeof window === 'undefined' || typeof IntersectionObserver === 'undefined') {
      revealAllSections()
      return
    }

    try {
      observer = new IntersectionObserver(
        (entries) => {
          entries.forEach((entry) => {
            if (entry.isIntersecting) {
              entry.target.classList.add('is-visible')
              observer?.unobserve(entry.target)
            }
          })
        },
        { threshold: 0.12, rootMargin: '0px 0px -48px 0px' }
      )

      // 顺序很关键，反了首屏就没有入场动效。
      //
      // observe() 对已在视口内的元素会立刻回调并打上 is-visible。如果先 observe
      // 再挂门控类，两者落在同一帧，浏览器从来没画出过
      // `.home-page--reveal-ready .mirror-reveal:not(.is-visible)` 这个隐藏态 ——
      // 没有隐藏态就没有过渡，首屏元素直接以最终样式出现。
      //
      // 所以：先挂门控类 → 双 rAF 确保隐藏态真的被绘制过 → 再启动 observer。
      revealReady.value = true
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          revealNodes.forEach((node) => observer?.observe(node))
        })
      })
      revealFallbackTimer = window.setTimeout(revealVisibleSections, 1600)
    } catch {
      revealAllSections()
    }
  })
})

onUnmounted(() => {
  if (revealFallbackTimer != null) {
    window.clearTimeout(revealFallbackTimer)
  }
  observer?.disconnect()
})
</script>

<style scoped>
.home-page {
  --papyrus: rgb(var(--color-papyrus));
  --papyrus-100: rgb(var(--color-stone));
  --marble: rgb(var(--color-marble));
  --parchment: rgb(var(--color-parchment));
  --terracotta: rgb(var(--color-terracotta));
  --terracotta-dark: rgb(var(--color-terracotta-dark));
  --laurel: rgb(var(--color-laurel));
  --laurel-dark: rgb(var(--color-laurel-dark));
  --ink: rgb(var(--color-ink));
  --ink-deep: rgb(var(--color-ink-deep));
  --ink-fade: rgb(var(--color-muted));
  min-height: 100vh;
  background: var(--papyrus);
  color: var(--ink);
  font-family: 'EB Garamond', 'Noto Serif SC', Georgia, serif;
  position: relative;
  overflow-x: hidden;
}

.home-page::before {
  content: '';
  position: fixed;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  opacity: 0.06;
  mix-blend-mode: multiply;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='220' height='220'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/><feColorMatrix values='0 0 0 0 0.12 0 0 0 0 0.1 0 0 0 0 0.07 0 0 0 0.55 0'/></filter><rect width='100%25' height='100%25' filter='url(%23n)'/></svg>");
}

.home-page :deep(a),
.home-page :deep(button) {
  -webkit-tap-highlight-color: transparent;
}

.monthly-credit-section {
  padding: 6rem 0 5.25rem;
  background:
    linear-gradient(180deg, rgb(var(--color-stone) / 0.18), rgb(var(--color-marble) / 0.58)),
    var(--papyrus);
  scroll-margin-top: 88px;
}

.monthly-credit-section__container {
  width: min(100% - 4rem, 1320px);
  margin: 0 auto;
}

@media (max-width: 720px) {
  .monthly-credit-section {
    padding: 4.5rem 0 4rem;
  }

  .monthly-credit-section__container {
    width: min(100% - 2rem, 1320px);
  }
}
</style>

<style>
/* 全局滚动进场动画（需要非 scoped 以穿透子组件） */
/* 时长/缓动/位移全部取自 theme.css，调节奏改那一处即可。
 * 原来还动了 filter: blur(12px) —— 去掉了：blur 触发重绘，而且在纸质系统里
 * 那种"糊一下再清晰"的观感偏廉价。只动 transform 和 opacity。 */
/* 入场用 animation 而不是 transition —— 这是被迫的，也是对的：
 *
 * 组件自己的 `transition:` 简写会把这里的整条 transition 覆盖掉。实测
 * .virtue-card 声明了 `transition: background .25s, border-color .25s,
 * transform .25s`，于是 .mirror-reveal 的 opacity/transform 过渡连同
 * transition-delay 一起归零 —— 元素在一帧内从 0 跳到 1，既没有淡入也没有
 * 错峰。改成 animation 就与组件的 transition 各走各的，互不覆盖。
 *
 * 附带好处和首屏书写那处一样：animation 不需要"隐藏态先被绘制过一帧"，
 * 挂上就一定完整播完，不受 observer 回调时机影响。 */
@keyframes mirrorRise {
  from {
    opacity: 0;
    transform: translateY(var(--reveal-lift));
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.mirror-reveal {
  opacity: 1;
  transform: translateY(0);
}

.home-page--reveal-ready .mirror-reveal:not(.is-visible) {
  opacity: 0;
}

.home-page--reveal-ready .mirror-reveal.is-visible {
  /* 错峰：--reveal-i 由模板内联给出（v-for 的 index），没给就是 0，
   * 所以整块容器自身不延迟、块内卡片依次跟上。 */
  animation: mirrorRise var(--duration-reveal) var(--ease-expo) both;
  animation-delay: calc(var(--reveal-stagger) * var(--reveal-i, 0));
}

/* ── L2 排版层：让排版本身成为动效 ───────────────────────────────
 * 必须放在这个非 scoped 块里。写在 HeroSection 的 scoped 块里时，
 * `:global(.home-page--reveal-ready) .hero-section__title span` 这类
 * 「全局祖先 + 局部后代」的选择器会被 Vue 的 scoped 编译器整条丢掉
 * （实测编译产物里一条都不剩），规则根本不进样式表。
 *
 * 时长/缓动/间隔全部取自 theme.css，调节奏改那一处。
 * ─────────────────────────────────────────────────────────── */

/* transition 里必须把 opacity/transform 一起声明。
 * 这条选择器特异性高于 .mirror-reveal，只写 letter-spacing 会把
 * 入场的淡入上浮整个覆盖掉。 */
.hero-section__title span,
.hero-section__title em {
  transition:
    opacity var(--duration-reveal) var(--ease-expo),
    transform var(--duration-reveal) var(--ease-expo),
    letter-spacing 900ms var(--ease-expo);
}

/* 字距收敛：Cinzel 品牌名从散开收紧到定位，像铅字被压进版盘。
 * 只给罗马大写做，斜体不参与（斜体收字距会糊）。
 *
 * 这里用 keyframes 而不是 transition，是踩过坑之后的选择：
 * transition 需要从元素的"当前计算值"插值，而 letter-spacing 的零值
 * 会被 Chrome 归一化成关键字 `normal`（继承也是 `normal`），
 * CSS 无法在 `normal` 与长度之间插值 —— 过渡静默失效，一次都不触发。
 * keyframes 两端都显式写成长度，绕开这个问题。 */
@keyframes heroLetterSettle {
  from { letter-spacing: 0.16em; }
  to   { letter-spacing: 0.002em; }
}

.home-page--reveal-ready .hero-section__title span.is-visible {
  animation: heroLetterSettle var(--duration-ink-settle) var(--ease-ink) both;
  animation-delay: calc(var(--reveal-stagger) * 1);
}

/* 斜体产品名：从左往右揭开，像被一笔写出来。
 *
 * 同样用 keyframes 而不是 transition，原因和上面字距一样：
 * transition 依赖"隐藏态先被浏览器绘制过一帧"，而这个元素拿到 is-visible
 * 的时机太早，隐藏态从没上过屏 —— 实测采样第一帧时 clip-path 已经是终态，
 * 揭开过程整个没播。keyframes 不依赖起始计算值，挂上就一定完整播完。 */
@keyframes heroInkWrite {
  from { clip-path: inset(0 100% -0.25em 0); }
  to   { clip-path: inset(0 -0.12em -0.25em 0); }
}

.hero-section__title em {
  clip-path: inset(0 -0.12em -0.25em 0);
}

.home-page--reveal-ready .hero-section__title em.is-visible {
  animation: heroInkWrite var(--duration-ink-write) var(--ease-ink) both;
  animation-delay: calc(var(--reveal-stagger) * 4);
}

/* 希腊文逐字浮现 —— 全站独有、竞品抄不走的一个动作 */
.hero-section__quote-greek span {
  display: inline-block;
  white-space: pre;
  transition: opacity 420ms var(--ease-out),
              transform 420ms var(--ease-out);
}

.home-page--reveal-ready .hero-section__quote:not(.is-visible) .hero-section__quote-greek span {
  opacity: 0;
  transform: translateY(0.16em);
}

/* 初始态一律挂在 --reveal-ready 门控下：JS 没跑起来时直接是最终样式，
 * 不会出现标题一直散着或被裁着的降级事故。 */

@media (prefers-reduced-motion: reduce) {
  .home-page--reveal-ready .mirror-reveal:not(.is-visible) {
    opacity: 1;
    transform: translateY(0);
  }

  .hero-section__title span,
  .hero-section__title em,
  .hero-section__quote-greek span {
    transition-duration: 1ms;
    transition-delay: 0ms;
  }

  .home-page--reveal-ready .hero-section__title span.is-visible {
    animation: none;
    letter-spacing: 0.002em;
  }

  .home-page--reveal-ready .hero-section__title em.is-visible {
    animation: none;
    clip-path: inset(0 -0.12em -0.25em 0);
  }

  .home-page--reveal-ready .hero-section__quote:not(.is-visible) .hero-section__quote-greek span {
    opacity: 1;
    transform: none;
  }
}
</style>
