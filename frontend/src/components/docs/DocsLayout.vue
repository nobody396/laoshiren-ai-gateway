<template>
  <div class="flex min-h-screen flex-col bg-white dark:bg-dark-950">
    <!-- Header -->
    <header
      class="sticky top-0 z-30 flex h-14 items-center border-b border-gray-200 bg-white/80 px-4 backdrop-blur dark:border-dark-700 dark:bg-dark-950/80 md:px-6"
    >
      <router-link to="/" class="flex items-center gap-2">
        <img src="/laoshirenai-icon.jpg" alt="老实人AI" class="h-7 w-7 rounded-full object-cover" />
        <span class="text-base font-semibold text-gray-900 dark:text-white">老实人AI</span>
      </router-link>
      <span class="mx-2 text-gray-300 dark:text-dark-400">/</span>
      <span class="text-sm font-medium text-gray-500 dark:text-dark-400">文档</span>

      <nav class="ml-auto hidden items-center gap-4 md:flex" aria-label="文档主导航">
        <router-link to="/docs">首页</router-link>
        <router-link to="/docs/quickstart">快速开始</router-link>
        <router-link to="/docs/category/api">API 参考</router-link>
        <router-link to="/docs/category/integrations">工具集成</router-link>
        <router-link to="/docs/category/models">模型目录</router-link>
      </nav>

      <div class="ml-4 flex items-center gap-2">
        <CustomerServiceButton v-if="authStore.isAuthenticated" />
        <router-link to="/dashboard" class="docs-console-link">控制台 ↗</router-link>
        <!-- Mobile sidebar toggle -->
        <button
          v-if="!home"
          @click="mobileOpen = !mobileOpen"
          class="rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800 md:hidden"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
      </div>
    </header>

    <div class="flex flex-1">
      <!-- Mobile sidebar overlay -->
      <transition name="fade">
        <div
          v-if="mobileOpen && !home"
          class="fixed inset-0 z-40 bg-black/40 md:hidden"
          @click="mobileOpen = false"
        ></div>
      </transition>

      <!-- Left sidebar -->
      <aside
        v-if="!home"
        class="fixed left-0 top-14 z-50 h-[calc(100vh-3.5rem)] w-64 -translate-x-full overflow-y-auto border-r border-gray-200 bg-white p-4 transition-transform dark:border-dark-700 dark:bg-dark-950 md:sticky md:z-10 md:translate-x-0"
        :class="{ 'translate-x-0': mobileOpen }"
      >
        <DocsSidebar :current-slug="currentSlug" @navigate="mobileOpen = false" />
      </aside>

      <!-- Main content -->
      <main class="min-w-0 flex-1 px-4 py-6 sm:px-6 md:px-10 lg:px-12">
        <div :class="home ? 'mx-auto w-full max-w-7xl' : 'mx-auto max-w-3xl'">
          <slot />
        </div>
      </main>

      <!-- Right TOC -->
      <aside
        v-if="!home"
        class="sticky top-14 hidden h-[calc(100vh-3.5rem)] w-56 shrink-0 overflow-y-auto border-l border-gray-200 p-4 dark:border-dark-700 xl:block"
      >
        <DocsToc :items="tocItems" />
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import DocsSidebar from '@/components/docs/DocsSidebar.vue'
import DocsToc from '@/components/docs/DocsToc.vue'
import CustomerServiceButton from '@/components/common/CustomerServiceButton.vue'
import type { TocItem } from '@/composables/useMarkdownRenderer'
import { useAuthStore } from '@/stores/auth'

defineProps<{
  currentSlug: string
  tocItems: TocItem[]
  home?: boolean
}>()

const mobileOpen = ref(false)
const authStore = useAuthStore()
</script>

<style scoped>
.docs-console-link {
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 0.45rem;
  color: rgb(var(--color-gray-700));
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.4rem 0.65rem;
  text-decoration: none;
}

.docs-console-link:hover {
  border-color: #1267d6;
  color: #1267d6;
}

header nav a {
  color: rgb(var(--color-gray-600));
  font-size: 0.8rem;
  font-weight: 500;
  text-decoration: none;
  transition: color 150ms ease;
}

header nav a:hover,
header nav a.router-link-active {
  color: rgb(var(--color-gray-950));
}

.dark .docs-console-link {
  border-color: rgb(var(--color-slate-700));
  color: rgb(var(--color-gray-300));
}

.dark header nav a { color: rgb(var(--color-gray-400)); }
.dark header nav a:hover,
.dark header nav a.router-link-active { color: rgb(var(--color-gray-50)); }

@media (max-width: 640px) {
  .docs-console-link { display: none; }
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
