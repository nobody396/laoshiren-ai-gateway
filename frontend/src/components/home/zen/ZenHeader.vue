<template>
  <!-- 顶部固定导航栏:白底毛玻璃,滚动时吸附 -->
  <header class="zen-header">
    <router-link to="/" class="zen-header__brand">
      <img src="/laoshirenai-icon.jpg" :alt="brandName" class="zen-header__logo" />
      <strong class="zen-header__brand-name">{{ brandName }}</strong>
    </router-link>

    <div class="zen-header__right">
      <LocaleSwitcher class="zen-header__locale" />
      <a :href="ctaTarget" class="zen-header__console" @click="onCtaClick">{{ ctaLabel }}</a>
    </div>
  </header>
</template>

<script setup lang="ts">
/**
 * 落地页头部导航(ZenMux 白底风格)
 * - 白底 + backdrop blur,sticky 吸附
 * - 右侧:语言切换 + 黑色药丸 CTA(登录/控制台,按登录态切换)
 * - 不放任何导航菜单(owner 决定:页内锚点与站点链接均不展示)
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'

const props = defineProps<{
  /** 是否已认证 */
  isAuthenticated: boolean
  /** 控制台路径 */
  dashboardPath: string
}>()

const $router = useRouter()
const { locale } = useI18n()

const isEnglish = computed(() => locale.value === 'en')
const brandName = computed(() => (isEnglish.value ? 'LaoshirenAI' : '老实人AI'))
const ui = computed(() => (isEnglish.value
  ? { dashboard: 'Dashboard', begin: 'Get started' }
  : { dashboard: '控制台', begin: '开始使用' }))

const ctaTarget = computed(() => (props.isAuthenticated ? props.dashboardPath : '/login'))
const ctaLabel = computed(() => (props.isAuthenticated ? ui.value.dashboard : ui.value.begin))

function onCtaClick(event: MouseEvent): void {
  event.preventDefault()
  void $router.push(ctaTarget.value)
}
</script>

<style scoped>
.zen-header {
  position: sticky;
  top: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  gap: 36px;
  height: 64px;
  padding: 0 max(32px, calc((100vw - 1200px) / 2));
  background: rgb(var(--zen-bg) / 0.82);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  border-bottom: 1px solid rgb(var(--zen-line));
}

.zen-header__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
  color: inherit;
}

.zen-header__logo {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  object-fit: cover;
}

.zen-header__brand-name {
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: rgb(var(--zen-ink));
  white-space: nowrap;
}

.zen-header__right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
}

.zen-header__console {
  display: inline-flex;
  align-items: center;
  height: 36px;
  padding: 0 16px;
  border-radius: 9px;
  background: rgb(var(--zen-ink));
  color: rgb(var(--zen-bg));
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
  transition: background 0.2s ease;
}

.zen-header__console:hover {
  background: rgb(var(--zen-text-strong));
}

@media (max-width: 900px) {
  .zen-header {
    height: 60px;
    padding: 0 20px;
    gap: 20px;
  }
}
</style>
