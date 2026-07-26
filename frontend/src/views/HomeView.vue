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
            :show-entitlement-details="true"
            :show-action="false"
          />
        </div>
      </section>

      <!-- 模型定价 -->
      <ModelPricing
        :claude-rows="claudePricingRows"
        :gpt-rows="gptPricingRows"
        :special-rows="specialPricingRows"
        :max-ledger-label="pricingDisplay.maxLedgerLabel"
        :pro-ledger-label="pricingDisplay.proLedgerLabel"
        :max-discount="pricingDisplay.maxDiscount"
        :pro-discount="pricingDisplay.proDiscount"
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
const pricingCurrency = computed<PricingCurrency>(() => (isEnglish.value ? 'USD' : 'CNY'))
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
    { label: labels.pricing, href: '#model-pricing', active: false, external: false, routerPush: false },
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

type SpecialBasePricingRow = {
  model: string
  modelId: string
  provider: string
  providerKey: 'deepseek' | 'qwen' | 'glm' | 'minimax'
  discountRate: number
  officialCny: {
    input: number
    output: number
    cacheRead: number | null
    cacheCreate: number | null
  }
  officialUsd?: {
    input: number
    output: number
    cacheRead: number | null
    cacheCreate: number | null
  }
}

type PricingCurrency = 'CNY' | 'USD'

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
  return new Intl.NumberFormat(isEnglish.value ? 'en-US' : 'zh-CN', {
    maximumFractionDigits,
    minimumFractionDigits
  }).format(value)
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

function formatUSD(
  value: number,
  maximumFractionDigits = 2,
  minimumFractionDigits = Number.isInteger(value) ? 0 : 2
): string {
  return `$${formatCompactNumber(value, maximumFractionDigits, minimumFractionDigits)}`
}

function formatOptionalUSD(value: number | null): string {
  return value == null ? '—' : formatUSD(value)
}

function formatPreciseCNY(value: number | null): string {
  if (value == null) return '—'
  if (value > 0 && value < 0.01) return formatCNY(value, 4, 4)
  if (value > 0 && value < 0.1) return formatCNY(value, 3, 3)
  return formatCNY(value, 2, Number.isInteger(value) ? 0 : 2)
}

function formatPreciseUSD(value: number | null): string {
  if (value == null) return '—'
  if (value > 0 && value < 0.01) return formatUSD(value, 4, 4)
  if (value > 0 && value < 0.1) return formatUSD(value, 3, 3)
  return formatUSD(value, 2, Number.isInteger(value) ? 0 : 2)
}

function formatPreciseMoney(value: number | null, currency: PricingCurrency): string {
  return currency === 'USD' ? formatPreciseUSD(value) : formatPreciseCNY(value)
}

function formatPriceDiscount(rate: number): string {
  if (isEnglish.value) {
    return `${formatCompactNumber(rate * 100, 1)}%`
  }

  return `${formatCompactNumber(rate * 10, 1)}折`
}

function formatDiscount(multiplier: number, exchangeRate: number): string {
  const rate = multiplier / exchangeRate

  if (isEnglish.value) {
    return `${formatCompactNumber(rate * 100, 1)}%`
  }

  return `${formatCompactNumber(rate * 10, 1)}折`
}

function resolveSpecialOfficialPrice(
  row: SpecialBasePricingRow,
  currency: PricingCurrency,
  exchangeRate: number
) {
  if (currency === 'USD') {
    return row.officialUsd ?? {
      input: row.officialCny.input / exchangeRate,
      output: row.officialCny.output / exchangeRate,
      cacheRead: row.officialCny.cacheRead == null ? null : row.officialCny.cacheRead / exchangeRate,
      cacheCreate: row.officialCny.cacheCreate == null ? null : row.officialCny.cacheCreate / exchangeRate
    }
  }

  return row.officialCny
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
  const maxRate = config.maxMultiplier / config.exchangeRate
  const proRate = config.proMultiplier / config.exchangeRate

  return {
    maxLedgerLabel: isEnglish.value
      ? `${formatUSD(maxRate, 3, 2)} per official $1`
      : `¥${formatCompactNumber(config.maxMultiplier)} = $1`,
    proLedgerLabel: isEnglish.value
      ? `${formatUSD(proRate, 3, 2)} per official $1`
      : `¥${formatCompactNumber(config.proMultiplier)} = $1`,
    maxDiscount: formatDiscount(config.maxMultiplier, config.exchangeRate),
    proDiscount: formatDiscount(config.proMultiplier, config.exchangeRate)
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
    model: 'GPT-5.6 Sol',
    modelId: 'gpt-5.6-sol',
    official: {
      input: 5,
      cachedInput: 0.5,
      output: 30
    }
  },
  {
    model: 'GPT-5.6 Terra',
    modelId: 'gpt-5.6-terra',
    official: {
      input: 2.5,
      cachedInput: 0.25,
      output: 15
    }
  },
  {
    model: 'GPT-5.6 Luna',
    modelId: 'gpt-5.6-luna',
    official: {
      input: 1,
      cachedInput: 0.1,
      output: 6
    }
  },
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

const specialBasePricingRows: SpecialBasePricingRow[] = [
  {
    model: 'DeepSeek V4 Flash',
    modelId: 'deepseek-v4-flash',
    provider: 'DeepSeek',
    providerKey: 'deepseek',
    discountRate: 0.3,
    officialCny: {
      input: 0.95,
      output: 1.9,
      cacheRead: 0.019,
      cacheCreate: null
    },
    officialUsd: {
      input: 0.14,
      output: 0.28,
      cacheRead: 0.0028,
      cacheCreate: null
    }
  },
  {
    model: 'DeepSeek V4 Pro',
    modelId: 'deepseek-v4-pro',
    provider: 'DeepSeek',
    providerKey: 'deepseek',
    discountRate: 0.3,
    officialCny: {
      input: 2.96,
      output: 5.92,
      cacheRead: 0.025,
      cacheCreate: null
    },
    officialUsd: {
      input: 0.435,
      output: 0.87,
      cacheRead: 0.003625,
      cacheCreate: null
    }
  },
  {
    model: 'Qwen3.7 Max',
    modelId: 'qwen3.7-max',
    provider: '阿里云',
    providerKey: 'qwen',
    discountRate: 0.6,
    officialCny: {
      input: 12,
      output: 36,
      cacheRead: 1.2,
      cacheCreate: 15
    }
  },
  {
    model: 'Qwen3.7 Plus',
    modelId: 'qwen3.7-plus',
    provider: '阿里云',
    providerKey: 'qwen',
    discountRate: 0.6,
    officialCny: {
      input: 2,
      output: 8,
      cacheRead: 0.4,
      cacheCreate: 2.5
    }
  },
  {
    model: 'GLM-5.2',
    modelId: 'glm-5.2',
    provider: '智谱 AI',
    providerKey: 'glm',
    discountRate: 0.6,
    officialCny: {
      input: 8,
      output: 28,
      cacheRead: 2,
      cacheCreate: null
    }
  },
  {
    model: 'MiniMax M3',
    modelId: 'MiniMax-M3 · ≤512K',
    provider: 'MiniMax',
    providerKey: 'minimax',
    discountRate: 0.6,
    officialCny: {
      input: 2.04,
      output: 8.16,
      cacheRead: 0.41,
      cacheCreate: null
    },
    officialUsd: {
      input: 0.3,
      output: 1.2,
      cacheRead: 0.06,
      cacheCreate: null
    }
  }
]

const specialPricingRows = computed(() => specialBasePricingRows.map((row) => {
  const config = landingPricingConfig.value
  const currency = pricingCurrency.value
  const official = resolveSpecialOfficialPrice(row, currency, config.exchangeRate)
  const discounted = {
    input: official.input * row.discountRate,
    output: official.output * row.discountRate,
    cacheRead: official.cacheRead == null ? null : official.cacheRead * row.discountRate,
    cacheCreate: official.cacheCreate == null ? null : official.cacheCreate * row.discountRate
  }

  return {
    model: row.model,
    modelId: row.modelId,
    provider: row.provider,
    providerKey: row.providerKey,
    official: {
      input: formatPreciseMoney(official.input, currency),
      output: formatPreciseMoney(official.output, currency),
      cacheRead: formatPreciseMoney(official.cacheRead, currency),
      cacheCreate: formatPreciseMoney(official.cacheCreate, currency)
    },
    price: {
      input: formatPreciseMoney(discounted.input, currency),
      output: formatPreciseMoney(discounted.output, currency),
      cacheRead: formatPreciseMoney(discounted.cacheRead, currency),
      cacheCreate: formatPreciseMoney(discounted.cacheCreate, currency)
    },
    discount: formatPriceDiscount(row.discountRate)
  }
}))

const claudePricingRows = computed(() => {
  const config = landingPricingConfig.value
  const currency = pricingCurrency.value
  const discount = formatDiscount(config.maxMultiplier, config.exchangeRate)

  return claudeBasePricingRows.map((row) => ({
    model: row.model,
    modelId: row.modelId,
    official: {
      input: currency === 'USD'
        ? formatUSD(row.official.input)
        : formatCNY(row.official.input * config.exchangeRate),
      cacheWrite5m: currency === 'USD'
        ? formatUSD(row.official.cacheWrite5m)
        : formatCNY(row.official.cacheWrite5m * config.exchangeRate),
      cacheRead: currency === 'USD'
        ? formatUSD(row.official.cacheRead)
        : formatCNY(row.official.cacheRead * config.exchangeRate),
      output: currency === 'USD'
        ? formatUSD(row.official.output)
        : formatCNY(row.official.output * config.exchangeRate)
    },
    max: {
      input: currency === 'USD'
        ? formatUSD((row.official.input * config.maxMultiplier) / config.exchangeRate)
        : formatCNY(row.official.input * config.maxMultiplier),
      cacheWrite5m: currency === 'USD'
        ? formatUSD((row.official.cacheWrite5m * config.maxMultiplier) / config.exchangeRate)
        : formatCNY(row.official.cacheWrite5m * config.maxMultiplier),
      cacheRead: currency === 'USD'
        ? formatUSD((row.official.cacheRead * config.maxMultiplier) / config.exchangeRate)
        : formatCNY(row.official.cacheRead * config.maxMultiplier),
      output: currency === 'USD'
        ? formatUSD((row.official.output * config.maxMultiplier) / config.exchangeRate)
        : formatCNY(row.official.output * config.maxMultiplier)
    },
    discount
  }))
})

const gptPricingRows = computed(() => {
  const config = landingPricingConfig.value
  const currency = pricingCurrency.value
  const discount = formatDiscount(config.proMultiplier, config.exchangeRate)

  return gptBasePricingRows.map((row) => ({
    model: row.model,
    modelId: row.modelId,
    official: {
      input: currency === 'USD'
        ? formatUSD(row.official.input)
        : formatCNY(row.official.input * config.exchangeRate),
      cachedInput: currency === 'USD'
        ? formatOptionalUSD(row.official.cachedInput)
        : formatOptionalCNY(
          row.official.cachedInput == null ? null : row.official.cachedInput * config.exchangeRate
        ),
      output: currency === 'USD'
        ? formatUSD(row.official.output)
        : formatCNY(row.official.output * config.exchangeRate)
    },
    pro: {
      input: currency === 'USD'
        ? formatUSD((row.official.input * config.proMultiplier) / config.exchangeRate)
        : formatCNY(row.official.input * config.proMultiplier),
      cachedInput: currency === 'USD'
        ? formatOptionalUSD(
          row.official.cachedInput == null ? null : (row.official.cachedInput * config.proMultiplier) / config.exchangeRate
        )
        : formatOptionalCNY(
          row.official.cachedInput == null ? null : row.official.cachedInput * config.proMultiplier
        ),
      output: currency === 'USD'
        ? formatUSD((row.official.output * config.proMultiplier) / config.exchangeRate)
        : formatCNY(row.official.output * config.proMultiplier)
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
      specific: 'Service-specific terms'
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
      specific: '服务特定条款'
    }

  return [
    {
      title: labels.product,
      links: [
        { label: labels.intro, href: '#about', external: false },
        ...(showModelReports.value
          ? [{ label: labels.reports, href: '#model-reports', external: false }]
          : []),
        { label: labels.pricing, href: '#model-pricing', external: false },
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
        { label: labels.specific, href: '/legal/service-specific-terms', external: false }
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
      revealFallbackTimer = window.setTimeout(revealAllSections, 1600)
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
.mirror-reveal {
  opacity: 1;
  transform: translateY(0);
  transition: opacity var(--duration-reveal) var(--ease-expo),
              transform var(--duration-reveal) var(--ease-expo);
  will-change: opacity, transform;
}

.home-page--reveal-ready .mirror-reveal:not(.is-visible) {
  opacity: 0;
  transform: translateY(var(--reveal-lift));
}

.mirror-reveal.is-visible {
  opacity: 1;
  transform: translateY(0);
  will-change: auto;
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
  animation: heroLetterSettle 900ms var(--ease-expo) both;
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
  animation: heroInkWrite 1100ms var(--ease-expo) both;
  animation-delay: calc(var(--reveal-stagger) * 2);
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
