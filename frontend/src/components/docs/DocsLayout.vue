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
      <span class="mx-2 text-gray-300 dark:text-dark-600">/</span>
      <span class="text-sm font-medium text-gray-500 dark:text-dark-400">文档</span>

      <!-- Mobile sidebar toggle -->
      <button
        @click="mobileOpen = !mobileOpen"
        class="ml-auto rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800 md:hidden"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>
    </header>

    <div class="flex flex-1">
      <!-- Mobile sidebar overlay -->
      <transition name="fade">
        <div
          v-if="mobileOpen"
          class="fixed inset-0 z-40 bg-black/40 md:hidden"
          @click="mobileOpen = false"
        ></div>
      </transition>

      <!-- Left sidebar -->
      <aside
        class="fixed left-0 top-14 z-50 h-[calc(100vh-3.5rem)] w-64 -translate-x-full overflow-y-auto border-r border-gray-200 bg-white p-4 transition-transform dark:border-dark-700 dark:bg-dark-950 md:sticky md:z-10 md:translate-x-0"
        :class="{ 'translate-x-0': mobileOpen }"
      >
        <DocsSidebar :current-slug="currentSlug" @navigate="mobileOpen = false" />
      </aside>

      <!-- Main content -->
      <main class="min-w-0 flex-1 px-6 py-8 md:px-10 lg:px-16">
        <div class="mx-auto max-w-3xl">
          <slot />
        </div>
      </main>

      <!-- Right TOC -->
      <aside
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
import type { TocItem } from '@/composables/useMarkdownRenderer'

defineProps<{
  currentSlug: string
  tocItems: TocItem[]
}>()

const mobileOpen = ref(false)
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
