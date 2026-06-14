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
      <section id="monthly-credit-cards" class="monthly-credit-section mirror-reveal">
        <div class="monthly-credit-section__container">
          <MonthlyCreditPlans
            variant="home"
            title="开发者月卡"
            summary=""
            :plans="monthlyPlans"
            :show-entitlement-details="true"
            :show-action="false"
          />
        </div>
      </section>

      <!-- 模型定价 -->
      <ModelPricing
        :claude-rows="claudePricingRows"
        :gpt-rows="gptPricingRows"
        :deepseek-reference="deepseekPricingReference"
        :max-ledger-label="pricingDisplay.maxLedgerLabel"
        :pro-ledger-label="pricingDisplay.proLedgerLabel"
        :max-discount="pricingDisplay.maxDiscount"
        :pro-discount="pricingDisplay.proDiscount"
        :exchange-rate-label="pricingDisplay.exchangeRateLabel"
        :is-authenticated="isAuthenticated"
      />

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

// 认证相关状态
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const showModelReports = computed(() => appStore.cachedPublicSettings?.landing_reports_enabled !== false)
const { plans: monthlyPlans, loadMonthlyCreditCardPlans } = useMonthlyCreditCardPlans()

// ── 导航项 ──
const navItems = computed(() => [
  { label: '首页', href: '#', active: true, external: false, routerPush: false },
  { label: '企业', href: '/enterprise', active: false, external: false, routerPush: true },
  ...(showModelReports.value
    ? [{ label: '报告', href: '#model-reports', active: false, external: false, routerPush: false }]
    : []),
  { label: '定价', href: '#model-pricing', active: false, external: false, routerPush: false },
  { label: '服务状态', href: '/status', active: false, external: false, routerPush: true },
  { label: '文档', href: '/docs', active: false, external: false, routerPush: true }
])

// ── 特性卡片 ──
const featureCards = [
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
]

// ── 模型卡片（三大 AI 品牌，按场景定位展示） ──
const models = [
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
]

// ── 模型检测报告 ──
const modelReports = [
  {
    provider: 'Claude',
    title: 'Claude Opus 4.7',
    sourceUrl: 'https://www.hvoy.ai/report/K9TIp4lRV1Q',
    endpoint: 'https://api.laoshirenai.com',
    modelId: 'claude-opus-4-7',
    score: 100,
    verdict: '完美匹配',
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
    verdict: '完美匹配',
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
    verdict: '完美匹配',
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
    verdict: '完美匹配',
    testedAt: '2026-05-20 11:10',
    passedChecks: 4,
    totalChecks: 4,
    latency: '2.9s',
    tps: '14.7',
    inputTokens: '393',
    outputTokens: '42'
  }
]

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
    description: '包月付费,每周固定可用额度,额度不结转。',
    price: '¥399.00',
    unit: '/ 月',
    variant: 'is-max',
    buttonText: '暂不开放',
    buttonVariant: 'mirror-home__button--accent',
    soldOut: true,
    ctaType: 'route',
    features: [
      '覆盖主流 Claude / Codex 编码模型调用',
      '每周可用 $110 额度',
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

// ── 模型定价表格数据（单位：每 100 万 tokens） ──
type ClaudeBasePricingRow = {
  model: string
  modelId: string
  official: {
    input: number
    cacheWrite5m: number
    cacheRead: number
    output: number
  }
}

type GptBasePricingRow = {
  model: string
  modelId: string
  official: {
    input: number
    cachedInput: number | null
    output: number
  }
}

type MarketPricingReference = {
  title: string
  sourceLabel: string
  sourceUrl: string
  description: string
  rows: Array<{
    label: string
    value: string
    hint: string
  }>
}

const defaultLandingPricing = {
  proMultiplier: 1.2,
  maxMultiplier: 4,
  exchangeRate: 7
}

function positiveNumberOrDefault(value: number | null | undefined, fallback: number): number {
  return Number.isFinite(value) && Number(value) > 0 ? Number(value) : fallback
}

function formatCompactNumber(
  value: number,
  maximumFractionDigits = 2,
  minimumFractionDigits = 0
): string {
  return new Intl.NumberFormat('zh-CN', {
    maximumFractionDigits,
    minimumFractionDigits
  }).format(value)
}

function formatUSD(value: number): string {
  const minimumFractionDigits = Number.isInteger(value) ? 0 : 2
  return `$${formatCompactNumber(value, 2, minimumFractionDigits)}`
}

function formatOptionalUSD(value: number | null): string {
  return value == null ? '—' : formatUSD(value)
}

function formatCNY(
  value: number,
  maximumFractionDigits = 2,
  minimumFractionDigits = 0
): string {
  return `¥${formatCompactNumber(value, maximumFractionDigits, minimumFractionDigits)}`
}

function formatOptionalCNY(value: number | null): string {
  return value == null ? '—' : formatCNY(value)
}

function formatDiscount(multiplier: number, exchangeRate: number): string {
  return `${formatCompactNumber((multiplier / exchangeRate) * 10, 1)}折`
}

const landingPricingConfig = computed(() => {
  const settings = appStore.cachedPublicSettings
  return {
    proMultiplier: positiveNumberOrDefault(
      settings?.landing_pricing_pro_multiplier,
      defaultLandingPricing.proMultiplier
    ),
    maxMultiplier: positiveNumberOrDefault(
      settings?.landing_pricing_max_multiplier,
      defaultLandingPricing.maxMultiplier
    ),
    exchangeRate: positiveNumberOrDefault(
      settings?.landing_pricing_exchange_rate,
      defaultLandingPricing.exchangeRate
    )
  }
})

const pricingDisplay = computed(() => {
  const config = landingPricingConfig.value
  return {
    maxLedgerLabel: `¥${formatCompactNumber(config.maxMultiplier)} = $1`,
    proLedgerLabel: `¥${formatCompactNumber(config.proMultiplier)} = $1`,
    maxDiscount: formatDiscount(config.maxMultiplier, config.exchangeRate),
    proDiscount: formatDiscount(config.proMultiplier, config.exchangeRate),
    exchangeRateLabel: formatCompactNumber(config.exchangeRate)
  }
})

const claudeBasePricingRows: ClaudeBasePricingRow[] = [
  {
    model: 'Claude Opus 4.8',
    modelId: 'claude-opus-4-8',
    official: {
      input: 5,
      cacheWrite5m: 6.25,
      cacheRead: 0.5,
      output: 25
    }
  },
  {
    model: 'Claude Opus 4.7',
    modelId: 'claude-opus-4-7',
    official: {
      input: 5,
      cacheWrite5m: 6.25,
      cacheRead: 0.5,
      output: 25
    }
  },
  {
    model: 'Claude Opus 4.6',
    modelId: 'claude-opus-4-6',
    official: {
      input: 5,
      cacheWrite5m: 6.25,
      cacheRead: 0.5,
      output: 25
    }
  },
  {
    model: 'Claude Opus 4.5',
    modelId: 'claude-opus-4-5-20251101',
    official: {
      input: 5,
      cacheWrite5m: 6.25,
      cacheRead: 0.5,
      output: 25
    }
  },
  {
    model: 'Claude Sonnet 4.6',
    modelId: 'claude-sonnet-4-6',
    official: {
      input: 3,
      cacheWrite5m: 3.75,
      cacheRead: 0.3,
      output: 15
    }
  },
  {
    model: 'Claude Sonnet 4.5',
    modelId: 'claude-sonnet-4-5-20250929',
    official: {
      input: 3,
      cacheWrite5m: 3.75,
      cacheRead: 0.3,
      output: 15
    }
  },
  {
    model: 'Claude Haiku 4.5',
    modelId: 'claude-haiku-4-5-20251001',
    official: {
      input: 1,
      cacheWrite5m: 1.25,
      cacheRead: 0.1,
      output: 5
    }
  }
]

const gptBasePricingRows: GptBasePricingRow[] = [
  {
    model: 'GPT-5.5',
    modelId: 'gpt-5.5',
    official: {
      input: 5,
      cachedInput: 0.5,
      output: 30
    }
  },
  {
    model: 'GPT-5.4',
    modelId: 'gpt-5.4',
    official: {
      input: 2.5,
      cachedInput: 0.25,
      output: 15
    }
  },
  {
    model: 'GPT-5.4 Mini',
    modelId: 'gpt-5.4-mini',
    official: {
      input: 0.75,
      cachedInput: 0.075,
      output: 4.5
    }
  },
  {
    model: 'GPT-5.4 Nano',
    modelId: 'gpt-5.4-nano',
    official: {
      input: 0.2,
      cachedInput: 0.02,
      output: 1.25
    }
  },
  {
    model: 'GPT-5.3 Codex',
    modelId: 'gpt-5.3-codex',
    official: {
      input: 1.75,
      cachedInput: 0.175,
      output: 14
    }
  },
  {
    model: 'GPT-5.3 Codex Spark',
    modelId: 'gpt-5.3-codex-spark',
    official: {
      input: 1.25,
      cachedInput: 0.125,
      output: 10
    }
  },
  {
    model: 'GPT-5.2',
    modelId: 'gpt-5.2',
    official: {
      input: 1.75,
      cachedInput: 0.175,
      output: 14
    }
  },
  {
    model: 'GPT-5.2 Codex',
    modelId: 'gpt-5.2-codex',
    official: {
      input: 1.75,
      cachedInput: 0.175,
      output: 14
    }
  },
  {
    model: 'GPT-5.2 Pro',
    modelId: 'gpt-5.2-pro',
    official: {
      input: 21,
      cachedInput: null,
      output: 168
    }
  },
  {
    model: 'GPT-5.1',
    modelId: 'gpt-5.1',
    official: {
      input: 1.25,
      cachedInput: 0.125,
      output: 10
    }
  },
  {
    model: 'GPT-5.1 Codex',
    modelId: 'gpt-5.1-codex',
    official: {
      input: 1.25,
      cachedInput: 0.125,
      output: 10
    }
  },
  {
    model: 'GPT-5.1 Codex Max',
    modelId: 'gpt-5.1-codex-max',
    official: {
      input: 1.25,
      cachedInput: 0.125,
      output: 10
    }
  },
  {
    model: 'GPT-5.1 Codex Mini',
    modelId: 'gpt-5.1-codex-mini',
    official: {
      input: 0.25,
      cachedInput: 0.025,
      output: 2
    }
  },
  {
    model: 'GPT-5',
    modelId: 'gpt-5',
    official: {
      input: 1.25,
      cachedInput: 0.125,
      output: 10
    }
  },
  {
    model: 'GPT-5 Pro',
    modelId: 'gpt-5-pro',
    official: {
      input: 15,
      cachedInput: null,
      output: 120
    }
  },
  {
    model: 'GPT-5 Mini',
    modelId: 'gpt-5-mini',
    official: {
      input: 0.25,
      cachedInput: 0.025,
      output: 2
    }
  },
  {
    model: 'GPT-5 Nano',
    modelId: 'gpt-5-nano',
    official: {
      input: 0.05,
      cachedInput: 0.005,
      output: 0.4
    }
  }
]

const deepseekPricingReference: MarketPricingReference = {
  title: '市场参照：DeepSeek-V4-Pro 官方人民币价',
  sourceLabel: 'DeepSeek API Docs',
  sourceUrl: 'https://api-docs.deepseek.com/zh-cn/quick_start/pricing',
  description: '同样按每 100 万 tokens 对齐，可直接与下方 Pro / Max 分组价格对照。',
  rows: [
    {
      label: '输入',
      value: formatCNY(3),
      hint: '缓存未命中'
    },
    {
      label: '输出',
      value: formatCNY(6),
      hint: '模型生成'
    },
    {
      label: '缓存命中',
      value: formatCNY(0.025, 3),
      hint: '输入缓存'
    }
  ]
}

const claudePricingRows = computed(() => {
  const config = landingPricingConfig.value
  const discount = formatDiscount(config.maxMultiplier, config.exchangeRate)

  return claudeBasePricingRows.map((row) => ({
    model: row.model,
    modelId: row.modelId,
    official: {
      input: formatUSD(row.official.input),
      cacheWrite5m: formatUSD(row.official.cacheWrite5m),
      cacheRead: formatUSD(row.official.cacheRead),
      output: formatUSD(row.official.output)
    },
    max: {
      input: formatCNY(row.official.input * config.maxMultiplier),
      cacheWrite5m: formatCNY(row.official.cacheWrite5m * config.maxMultiplier),
      cacheRead: formatCNY(row.official.cacheRead * config.maxMultiplier),
      output: formatCNY(row.official.output * config.maxMultiplier)
    },
    discount
  }))
})

const gptPricingRows = computed(() => {
  const config = landingPricingConfig.value
  const discount = formatDiscount(config.proMultiplier, config.exchangeRate)

  return gptBasePricingRows.map((row) => ({
    model: row.model,
    modelId: row.modelId,
    official: {
      input: formatUSD(row.official.input),
      cachedInput: formatOptionalUSD(row.official.cachedInput),
      output: formatUSD(row.official.output)
    },
    pro: {
      input: formatCNY(row.official.input * config.proMultiplier),
      cachedInput: formatOptionalCNY(
        row.official.cachedInput == null ? null : row.official.cachedInput * config.proMultiplier
      ),
      output: formatCNY(row.official.output * config.proMultiplier)
    },
    discount
  }))
})

// ── VIP 分组（暂未推出） ──
// const vipTiers = [
//   { name: 'VIP1', discount: '0.98', threshold: '累计消费满 ¥500', features: ['全模型倍率优惠', '优先活动资格'] },
//   { name: 'VIP2', discount: '0.96', threshold: '累计消费满 ¥3,000', features: ['并发额度翻倍', '高峰期优先排队'] },
//   { name: 'VIP3', discount: '0.94', threshold: '累计消费满 ¥10,000', features: ['新模型优先体验', '更稳定调用体验'] },
//   { name: 'VIP4', discount: '0.92', threshold: '累计消费满 ¥30,000', features: ['专属技术支持', '最高并发额度'] },
//   { name: 'VIP5', discount: '0.90', threshold: '累计消费满 ¥80,000', features: ['专属客户经理', '定制化服务方案'] }
// ]

// ── 页脚链接 ──
const footerSections = computed(() => [
  {
    title: '产品',
    links: [
      { label: '老实人AI 介绍', href: '#about', external: false },
      ...(showModelReports.value
        ? [{ label: '模型检测报告', href: '#model-reports', external: false }]
        : []),
      { label: '价格方案', href: '#model-pricing', external: false },
      { label: '企业方案', href: '/enterprise', external: false },
      { label: '登录', href: '/login', external: false }
    ]
  },
  {
    title: '服务承诺',
    links: [
      { label: '服务状态', href: '/status', external: false },
      { label: '安全与隐私', href: '/security', external: false }
    ]
  },
  {
    title: '合规条款',
    links: [
      { label: '服务条款', href: '/legal/terms', external: false },
      { label: '使用政策', href: '/legal/usage-policy', external: false },
      { label: '支持的国家和地区', href: '/legal/supported-regions', external: false },
      { label: '服务特定条款', href: '/legal/service-specific-terms', external: false }
    ]
  }
])

// ── 滚动进场动画（IntersectionObserver） ──
let observer: IntersectionObserver | null = null
let revealFallbackTimer: number | null = null
const revealReady = ref(false)

function revealAllSections(): void {
  document.querySelectorAll('.mirror-reveal').forEach((node) => {
    node.classList.add('is-visible')
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

      revealNodes.forEach((node) => observer?.observe(node))
      revealReady.value = true
      revealFallbackTimer = window.setTimeout(revealAllSections, 1200)
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
@import url('https://fonts.googleapis.com/css2?family=Cinzel:wght@400;500;600;700&family=EB+Garamond:ital,wght@0,400;0,500;0,600;1,400;1,500&family=Inter:wght@400;500;600;700&family=Noto+Serif+SC:wght@400;500;600;700&display=swap');

.home-page {
  --papyrus: #f8f3e7;
  --papyrus-100: #efe6cf;
  --marble: #faf6ec;
  --parchment: #f2e9d2;
  --terracotta: #9a3b1f;
  --terracotta-dark: #7a2d17;
  --laurel: #3f5a3a;
  --laurel-dark: #26361f;
  --ink: #1f1a12;
  --ink-deep: #13100b;
  --ink-fade: #8a7d63;
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
    linear-gradient(180deg, rgba(239, 230, 207, 0.18), rgba(250, 246, 236, 0.58)),
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
.mirror-reveal {
  opacity: 1;
  transform: translateY(0);
  filter: blur(0);
  transition: opacity 0.8s cubic-bezier(0.16, 1, 0.3, 1),
              transform 0.8s cubic-bezier(0.16, 1, 0.3, 1),
              filter 0.8s cubic-bezier(0.16, 1, 0.3, 1);
}

.home-page--reveal-ready .mirror-reveal:not(.is-visible) {
  opacity: 0;
  transform: translateY(24px);
  filter: blur(12px);
}

.mirror-reveal.is-visible {
  opacity: 1;
  transform: translateY(0);
  filter: blur(0);
}

@media (prefers-reduced-motion: reduce) {
  .mirror-reveal {
    transition-duration: 1ms;
  }

  .home-page--reveal-ready .mirror-reveal:not(.is-visible) {
    opacity: 1;
    transform: translateY(0);
    filter: blur(0);
  }
}
</style>
