<template>
  <!-- 顶部固定导航栏 -->
  <header class="home-header" :class="{ 'is-scrolled': isScrolled }">
    <div class="home-header__inner">
      <!-- 品牌 Logo -->
      <router-link to="/" class="home-header__brand">
        <span class="home-header__seal">
          <img src="/laoshirenai-icon.jpg" alt="老实人AI" class="home-header__logo" />
        </span>
        <span class="home-header__brand-text">
          <span class="home-header__brand-name">老实人AI</span>
          <span class="home-header__brand-tag">A Quiet Place for Code</span>
        </span>
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
          {{ isAuthenticated ? '控制台' : 'Begin' }}
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
  background: rgba(248, 243, 231, 0.92);
  border-bottom: 1px solid rgba(63, 90, 58, 0.12);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: background 0.4s ease, border-color 0.4s ease, backdrop-filter 0.4s ease;
}

.home-header.is-scrolled {
  background: rgba(248, 243, 231, 0.96);
  border-bottom-color: rgba(63, 90, 58, 0.2);
}

/* 内部布局 - 水平三栏 */
.home-header__inner {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1.5rem;
  padding: 0;
  min-height: 72px;
}

/* 品牌标识 */
.home-header__brand {
  display: flex;
  align-items: center;
  gap: 0.875rem;
  flex: 1;
  text-decoration: none;
  color: inherit;
}

.home-header__seal {
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: #9a3b1f;
  box-shadow: inset 0 0 0 2px rgba(250, 246, 236, 0.42), 0 0 0 1px #7a2d17;
}

.home-header__logo {
  width: 100%;
  height: 100%;
  object-fit: cover;
  flex-shrink: 0;
}

.home-header__brand-text {
  display: flex;
  flex-direction: column;
  line-height: 1.1;
}

.home-header__brand-name {
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 1.0625rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: #13100b;
  white-space: nowrap;
}

.home-header__brand-tag {
  margin-top: 0.125rem;
  font-family: 'Inter', sans-serif;
  font-size: 0.625rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: #8a7d63;
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
  color: #1f1a12;
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-size: 1.0625rem;
  font-style: italic;
  font-weight: 500;
  text-decoration: none;
  transition: color 0.2s ease;
}

.home-header__link:hover {
  color: #9a3b1f;
}

.home-header__link.is-active {
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
  min-height: 2.625rem;
  padding: 0 1.25rem;
  background: transparent;
  color: #1f1a12;
  border: 1.5px solid #1f1a12;
  font-family: 'Inter', sans-serif;
  font-size: 0.8125rem;
  font-weight: 500;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  text-decoration: none;
  transition: all 0.2s ease;
}

.home-header__cta:hover {
  background: #1f1a12;
  color: #f8f3e7;
}

@media (max-width: 800px) {
  .home-header__inner {
    width: min(100% - 2rem, 1200px);
  }

  .home-header__menu {
    display: none;
  }

  .home-header__brand-tag {
    display: none;
  }
}
</style>
