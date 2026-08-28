<template>
  <div class="changelog-shell">
    <HomeHeader
      :is-authenticated="authStore.isAuthenticated"
      :dashboard-path="dashboardPath"
      :nav-items="navItems"
    />
    <slot />
    <HomeFooter :sections="footerSections" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import HomeHeader from '@/components/home/HomeHeader.vue'
import HomeFooter from '@/components/home/HomeFooter.vue'
import { PUBLIC_DOCS_ENABLED } from '@/config/publicFeatures'

const authStore = useAuthStore()
const appStore = useAppStore()
const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
const showReports = computed(() => appStore.cachedPublicSettings?.landing_reports_enabled !== false)

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
    { label: labels.enterprise, href: '/enterprise', active: false, routerPush: true },
    ...(showReports.value
      ? [{ label: labels.reports, href: '/#model-reports', active: false, routerPush: true }]
      : []),
    { label: labels.pricing, href: '/#model-pricing', active: false, routerPush: true },
    { label: labels.status, href: '/status', active: false, routerPush: true },
    { label: labels.changelog, href: '/changelog', active: true, routerPush: true },
    ...(PUBLIC_DOCS_ENABLED ? [{ label: labels.docs, href: '/docs', active: false, routerPush: true }] : [])
  ]
})

const footerSections = computed(() => {
  const labels = isEnglish.value
    ? {
      product: 'Product',
      home: 'Home',
      pricing: 'Pricing',
      enterprise: 'Enterprise',
      updates: 'Build in Public',
      changelog: 'Changelog',
      status: 'Service status',
      docs: 'Docs',
      trust: 'Trust',
      security: 'Security & privacy',
      terms: 'Terms of service',
      usage: 'Usage policy',
      affiliate: 'Affiliate program rules',
      account: 'Account',
      login: 'Login',
      dashboard: 'Dashboard'
    }
    : {
      product: '产品',
      home: '首页',
      pricing: '模型定价',
      enterprise: '企业方案',
      updates: '公开构建',
      changelog: '更新日志',
      status: '服务状态',
      docs: '文档',
      trust: '信任',
      security: '安全与隐私',
      terms: '服务条款',
      usage: '使用政策',
      affiliate: '联盟计划规则',
      account: '账户',
      login: '登录',
      dashboard: '控制台'
    }
  return [
    {
      title: labels.product,
      links: [
        { label: labels.home, href: '/' },
        { label: labels.pricing, href: '/#model-pricing' },
        { label: labels.enterprise, href: '/enterprise' }
      ]
    },
    {
      title: labels.updates,
      links: [
        { label: labels.changelog, href: '/changelog' },
        { label: labels.status, href: '/status' },
        ...(PUBLIC_DOCS_ENABLED ? [{ label: labels.docs, href: '/docs' }] : [])
      ]
    },
    {
      title: labels.trust,
      links: [
        { label: labels.security, href: '/security' },
        { label: labels.terms, href: '/legal/terms' },
        { label: labels.usage, href: '/legal/usage-policy' },
        { label: labels.affiliate, href: '/legal/affiliate-program' }
      ]
    },
    {
      title: labels.account,
      links: [
        { label: labels.login, href: '/login' },
        { label: labels.dashboard, href: dashboardPath.value }
      ]
    }
  ]
})
</script>

<style scoped>
.changelog-shell {
  min-height: 100vh;
  background: rgb(var(--color-papyrus));
  color: rgb(var(--color-ink-deep));
}
</style>
