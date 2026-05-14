<template>
  <nav class="docs-sidebar-nav">
    <div v-for="(category, idx) in docsConfig" :key="idx" class="mb-4">
      <button
        @click="toggleCategory(idx)"
        class="flex w-full items-center justify-between px-3 py-2 text-sm font-semibold text-gray-900 dark:text-white"
      >
        <span>{{ category.title }}</span>
        <svg
          class="h-4 w-4 text-gray-400 transition-transform duration-200"
          :class="{ 'rotate-90': !collapsed[idx] }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
        </svg>
      </button>

      <transition name="collapse">
        <ul v-show="!collapsed[idx]" class="mt-1 space-y-0.5">
          <li v-for="item in category.items" :key="item.slug">
            <router-link
              :to="`/docs/${item.slug}`"
              class="block rounded-md px-3 py-1.5 text-sm transition-colors"
              :class="
                currentSlug === item.slug
                  ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-400'
                  : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white'
              "
              @click="$emit('navigate')"
            >
              {{ item.title }}
            </router-link>
          </li>
        </ul>
      </transition>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { reactive } from 'vue'
import { docsConfig } from '@/docs/config'

defineProps<{
  currentSlug: string
}>()

defineEmits<{
  navigate: []
}>()

const collapsed = reactive<Record<number, boolean>>(
  Object.fromEntries(docsConfig.map((cat, idx) => [idx, cat.collapsed ?? false])),
)

function toggleCategory(idx: number) {
  collapsed[idx] = !collapsed[idx]
}
</script>

<style scoped>
.collapse-enter-active,
.collapse-leave-active {
  transition: all 0.2s ease;
  overflow: hidden;
}

.collapse-enter-from,
.collapse-leave-to {
  opacity: 0;
  max-height: 0;
}

.collapse-enter-to,
.collapse-leave-from {
  opacity: 1;
  max-height: 500px;
}
</style>
