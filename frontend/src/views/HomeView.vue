<template>
  <div class="home-page">
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

      <!-- 中转体验 -->
      <TransferExperience />

      <!-- 为什么选择 DragonCode -->
      <WhyChoose :feature-cards="featureCards" />

      <!-- Claude 组合 -->
      <ClaudeCombinations :models="models" />

      <!-- IDE 协同 -->
      <IDEIntegration />

      <!-- 模型定价 -->
      <ModelPricing
        :rows="modelPricingRows"
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
import { computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useAuthStore, useAppStore } from '@/stores'

// 子组件导入
import HomeHeader from '@/components/home/HomeHeader.vue'
import HeroSection from '@/components/home/HeroSection.vue'
import TransferExperience from '@/components/home/TransferExperience.vue'
import WhyChoose from '@/components/home/WhyChoose.vue'
import ClaudeCombinations from '@/components/home/ClaudeCombinations.vue'
import IDEIntegration from '@/components/home/IDEIntegration.vue'
import ModelPricing from '@/components/home/ModelPricing.vue'
import VIPTiers from '@/components/home/VIPTiers.vue'
import HomeFooter from '@/components/home/HomeFooter.vue'

const authStore = useAuthStore()
const appStore = useAppStore()

const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')

// 认证相关状态
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

// ── 导航项 ──
const navItems = computed(() => [
  { label: '首页', href: '#', active: true, external: false, routerPush: false },
  { label: '定价', href: '#model-pricing', active: false, external: false, routerPush: false },
  { label: '服务状态', href: 'https://status.your-domain.example', active: false, external: true, routerPush: false },
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
    description: '擅长技术方案设计、代码审查与多步推理，适合把控项目全局方向。',
    subModels: 'Opus 4.6 · Sonnet 4.6 · Haiku 4.5'
  },
  {
    eyebrow: '编码实现',
    name: 'ChatGPT',
    subtitle: '代码生成与功能开发',
    description: '擅长代码生成、功能迭代与调试修复，日常编码的主力引擎。',
    subModels: 'GPT-5.4 · GPT-5.3 Codex'
  },
  {
    eyebrow: '多模态设计',
    name: 'Gemini',
    subtitle: '图像理解与视觉任务',
    description: '擅长设计稿转代码、图像理解与多模态任务，前端设计的得力助手。',
    subModels: '2.5 Pro · 2.0 Flash'
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

// ── 模型定价表格数据 ──
const modelPricingRows = [
  { provider: 'Claude', name: 'Opus 4.7', badgeClass: 'is-claude', group: 'Claude Max', groupClass: 'is-enterprise', multiplier: '2', input: '¥10', output: '¥50', official: '¥35/¥175', discount: '2.8折', openclaw: false },
  { provider: 'Claude', name: 'Sonnet 4.6', badgeClass: 'is-claude', group: 'Claude Max', groupClass: 'is-enterprise', multiplier: '2', input: '¥6', output: '¥30', official: '¥21/¥105', discount: '2.8折', openclaw: false },
  { provider: 'Claude', name: 'Opus 4.7', badgeClass: 'is-claude', group: 'Kiro逆向', groupClass: 'is-claude-discount', multiplier: '0.65', input: '¥3.25', output: '¥16.25', official: '¥35/¥175', discount: '0.9折', openclaw: true },
  { provider: 'Claude', name: 'Sonnet 4.6', badgeClass: 'is-claude', group: 'Kiro逆向', groupClass: 'is-claude-discount', multiplier: '0.65', input: '¥1.95', output: '¥9.75', official: '¥21/¥105', discount: '0.9折', openclaw: true },
  { provider: 'Claude', name: 'Opus 4.6', badgeClass: 'is-claude', group: '反重力逆向', groupClass: 'is-antigravity', multiplier: '0.85', input: '¥4.25', output: '¥21.25', official: '¥35/¥175', discount: '1.2折', openclaw: true },
  { provider: 'Claude', name: 'Sonnet 4.6', badgeClass: 'is-claude', group: '反重力逆向', groupClass: 'is-antigravity', multiplier: '0.85', input: '¥2.55', output: '¥12.75', official: '¥21/¥105', discount: '1.2折', openclaw: true },
  { provider: 'GPT', name: '5.4', badgeClass: 'is-gpt', group: 'codex', groupClass: 'is-codex', multiplier: '0.5', input: '¥2.5', output: '¥12.75', official: '¥35/¥157.5', discount: '0.7折', openclaw: true },
  { provider: 'GPT', name: '5.5', badgeClass: 'is-gpt', group: 'codex', groupClass: 'is-codex', multiplier: '0.5', input: '¥2.5', output: '¥15', official: '¥35/¥210', discount: '0.7折', openclaw: true }
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
const footerSections = computed(() => [
  {
    title: '产品',
    links: [
      { label: 'DragonCode 介绍', href: '#about', external: false },
      { label: '价格方案', href: '#model-pricing', external: false },
      { label: '登录', href: '/login', external: false }
    ]
  },
  {
    title: '资源',
    links: [
      { label: '使用教程', href: docUrl.value || '#guide', external: !!docUrl.value },
      { label: '品牌故事', href: '#brand-story', external: false }
    ]
  },
  {
    title: 'Claude 模型',
    links: [
      { label: 'Claude Opus 4.6', href: '#', external: false },
      { label: 'Claude Sonnet 4.6', href: '#', external: false },
      { label: 'Claude Haiku 4.5', href: '#', external: false }
    ]
  },
  {
    title: '服务承诺',
    links: [
      { label: '透明定价', href: '#model-pricing', external: false },
      { label: '服务状态', href: 'https://status.your-domain.example', external: true },
      { label: '隐私保护', href: '#about', external: false },
      { label: '安全合规', href: '#about', external: false }
    ]
  },
  {
    title: '解决方案',
    links: [
      { label: 'AI 编程助手', href: '#guide', external: false },
      { label: '代码生成', href: '#guide', external: false },
      { label: '技术支持', href: '#guide', external: false }
    ]
  },
  {
    title: '关于',
    links: [
      { label: '关于我们', href: '#about', external: false },
      { label: '联系我们', href: '#about', external: false }
    ]
  }
])

// ── 滚动进场动画（IntersectionObserver） ──
let observer: IntersectionObserver | null = null

onMounted(() => {
  // 认证检查
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  // 滚动进场动画 - 等子组件挂载后再绑定
  nextTick(() => {
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

    document.querySelectorAll('.mirror-reveal').forEach((node) => observer?.observe(node))
  })
})

onUnmounted(() => {
  observer?.disconnect()
})
</script>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700;800&family=Noto+Sans+SC:wght@300;400;500;700;800;900&display=swap');

/* 页面全局样式 - 蓝白色系 */
.home-page {
  min-height: 100vh;
  background: #ffffff;
  color: #1a1a2e;
  font-family: 'Noto Sans SC', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Manrope', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}
</style>

<style>
/* 全局滚动进场动画（需要非 scoped 以穿透子组件） */
.mirror-reveal {
  opacity: 0;
  transform: translateY(24px);
  filter: blur(12px);
  transition: opacity 0.8s cubic-bezier(0.16, 1, 0.3, 1),
              transform 0.8s cubic-bezier(0.16, 1, 0.3, 1),
              filter 0.8s cubic-bezier(0.16, 1, 0.3, 1);
}

.mirror-reveal.is-visible {
  opacity: 1;
  transform: translateY(0);
  filter: blur(0);
}
</style>
