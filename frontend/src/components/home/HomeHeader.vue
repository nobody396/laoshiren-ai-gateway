<template>
  <!-- 顶部固定导航栏 -->
  <header class="home-header" :class="{ 'is-scrolled': isScrolled }">
    <div class="home-header__inner">
      <!-- 品牌 Logo -->
      <router-link to="/" class="home-header__brand">
        <img src="/logo.png" alt="DragonCode Logo" class="home-header__logo" />
        <span class="home-header__brand-text">DragonCode</span>
      </router-link>

      <!-- 导航菜单 -->
      <nav class="home-header__menu">
        <a
          v-for="item in navItems"
          :key="item.label"
          :href="item.href"
          class="home-header__link"
          :class="{ 'is-active': item.active }"
          :target="item.external ? '_blank' : undefined"
          :rel="item.external ? 'noopener noreferrer' : undefined"
          @click="item.routerPush ? ($event.preventDefault(), $router.push(item.href)) : undefined"
        >
          {{ item.label }}
        </a>
      </nav>

      <!-- 右侧操作按钮 -->
      <div class="home-header__actions">
        <a
          :href="isAuthenticated ? dashboardPath : '/login'"
          class="home-header__cta"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ isAuthenticated ? '控制台' : '开始使用' }}
        </a>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
/**
 * 落地页头部导航组件
 * - 固定顶部 + 毛玻璃背景
 * - 导航链接：首页/定价/文档
 * - 右侧 CTA 按钮（登录/控制台）
 */
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'

const $router = useRouter()

const isScrolled = ref(false)

const handleScroll = () => {
  isScrolled.value = window.scrollY > 20
}

onMounted(() => {
  window.addEventListener('scroll', handleScroll, { passive: true })
  handleScroll()
})

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll)
})

defineProps<{
  /** 是否已认证 */
  isAuthenticated: boolean
  /** 控制台路径 */
  dashboardPath: string
  /** 导航项列表 */
  navItems: Array<{
    label: string
    href: string
    active: boolean
    external?: boolean
    routerPush?: boolean
  }>
}>()
</script>

<style scoped>
/* 头部容器 - 固定顶部 */
.home-header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 50;
  background: transparent;
  border-bottom: 1px solid transparent;
  transition: background 0.4s ease, border-color 0.4s ease, backdrop-filter 0.4s ease;
}

.home-header.is-scrolled {
  background: rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(0, 0, 0, 0.04);
}

/* 内部布局 - 水平三栏 */
.home-header__inner {
  width: min(100% - 4rem, 1400px);
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1.5rem;
  padding: 1.25rem 0;
}

/* 品牌标识 */
.home-header__brand {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  flex: 1;
  text-decoration: none;
  color: inherit;
}

.home-header__logo {
  width: 1.75rem;
  height: 1.75rem;
  flex-shrink: 0;
}

.home-header__brand-text {
  font-size: 1.125rem;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: #1a1a2e;
  white-space: nowrap;
}

/* 导航链接组 */
.home-header__menu {
  display: flex;
  align-items: center;
  gap: 2.5rem;
  flex-shrink: 0;
}

.home-header__link {
  display: inline-flex;
  align-items: center;
  color: #1a1a2e;
  font-size: 0.9375rem;
  font-weight: 500;
  text-decoration: none;
  transition: opacity 0.2s ease;
  opacity: 0.7;
}

.home-header__link:hover {
  opacity: 1;
}

.home-header__link.is-active {
  opacity: 1;
  font-weight: 600;
}

/* 右侧操作区 */
.home-header__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex: 1;
}

/* CTA 按钮 - 圆角药丸 */
.home-header__cta {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 2.5rem;
  padding: 0 1.75rem;
  border-radius: 999px;
  background: transparent;
  color: #1a1a2e;
  border: 1px solid #1a1a2e;
  font-size: 0.875rem;
  font-weight: 500;
  text-decoration: none;
  transition: all 0.2s ease;
}

.home-header__cta:hover {
  background: #1a1a2e;
  color: #fff;
}
</style>
