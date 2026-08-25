<template>
  <!-- 顶部固定导航栏:白底毛玻璃,滚动时吸附 -->
  <header class="zen-header">
    <router-link to="/" class="zen-header__brand">
      <img src="/laoshirenai-icon.jpg" :alt="brandName" class="zen-header__logo" />
      <strong class="zen-header__brand-name">{{ brandName }}</strong>
    </router-link>

    <nav class="zen-header__menu" :class="{ 'is-open': menuOpen }">
      <a
        v-for="item in navItems"
        :key="item.label"
        :href="item.href"
        class="zen-header__link"
        @click="onNavClick(item, $event)"
      >
        {{ item.label }}
      </a>
      <!-- 移动端抽屉里保留控制台入口(桌面端是右侧黑色药丸) -->
      <a :href="ctaTarget" class="zen-header__link zen-header__link--cta-mobile" @click="onCtaClick">
        {{ ctaLabel }}
      </a>
    </nav>

    <div class="zen-header__right">
      <LocaleSwitcher class="zen-header__locale" />
      <a :href="ctaTarget" class="zen-header__console" @click="onCtaClick">{{ ctaLabel }}</a>
      <button
        class="zen-header__burger"
        :aria-label="menuOpen ? ui.closeMenu : ui.openMenu"
        :aria-expanded="menuOpen"
        @click="menuOpen = !menuOpen"
      >
        <Icon :name="menuOpen ? 'x' : 'menu'" size="md" />
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
/**
 * 落地页头部导航(ZenMux 白底风格)
 * - 白底 + backdrop blur,sticky 吸附
 * - 右侧:语言切换 + 黑色药丸 CTA(登录/控制台,按登录态切换)
 * - 移动端:汉堡按钮展开抽屉菜单
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  /** 是否已认证 */
  isAuthenticated: boolean
  /** 控制台路径 */
  dashboardPath: string
  /** 导航项列表(href 以 # 开头为页内锚点,其余走 router push) */
  navItems: Array<{
    label: string
    href: string
  }>
}>()

const $router = useRouter()
const { locale } = useI18n()

const menuOpen = ref(false)
const isEnglish = computed(() => locale.value === 'en')
const brandName = computed(() => (isEnglish.value ? 'LaoshirenAI' : '老实人AI'))
const ui = computed(() => (isEnglish.value
  ? { dashboard: 'Dashboard', begin: 'Get started', openMenu: 'Open menu', closeMenu: 'Close menu' }
  : { dashboard: '控制台', begin: '开始使用', openMenu: '打开菜单', closeMenu: '关闭菜单' }))

const ctaTarget = computed(() => (props.isAuthenticated ? props.dashboardPath : '/login'))
const ctaLabel = computed(() => (props.isAuthenticated ? ui.value.dashboard : ui.value.begin))

function onNavClick(item: { href: string }, event: MouseEvent): void {
  menuOpen.value = false
  event.preventDefault()
  if (item.href.startsWith('#')) {
    void $router.push({ path: '/', hash: item.href })
  } else {
    void $router.push(item.href)
  }
}

function onCtaClick(event: MouseEvent): void {
  menuOpen.value = false
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

.zen-header__menu {
  display: flex;
  gap: 30px;
  font-size: 14px;
}

.zen-header__link {
  color: rgb(var(--zen-text));
  text-decoration: none;
  transition: color 0.18s ease;
}

.zen-header__link:hover {
  color: rgb(var(--zen-ink));
}

.zen-header__link--cta-mobile {
  display: none;
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

.zen-header__burger {
  display: none;
  place-items: center;
  width: 38px;
  height: 38px;
  border: 0;
  border-radius: 9px;
  background: none;
  color: rgb(var(--zen-ink));
  cursor: pointer;
}

.zen-header__burger:hover {
  background: rgb(var(--zen-digit-face));
}

@media (max-width: 900px) {
  .zen-header {
    height: 60px;
    padding: 0 20px;
    gap: 20px;
  }

  .zen-header__console {
    display: none;
  }

  .zen-header__burger {
    display: grid;
  }

  .zen-header__menu {
    display: none;
    position: absolute;
    top: 60px;
    left: 0;
    right: 0;
    padding: 12px 20px 20px;
    flex-direction: column;
    gap: 4px;
    background: rgb(var(--zen-bg));
    border-bottom: 1px solid rgb(var(--zen-line));
    box-shadow: 0 20px 30px rgb(var(--zen-ink) / 0.06);
    font-size: 16px;
  }

  .zen-header__menu.is-open {
    display: flex;
  }

  .zen-header__link {
    padding: 10px 4px;
    border-radius: 8px;
  }

  .zen-header__link--cta-mobile {
    display: block;
    font-weight: 600;
    color: rgb(var(--zen-ink));
  }
}
</style>
