<template>
  <div v-if="items.length > 0" class="docs-toc">
    <h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
      本页目录
    </h4>
    <ul class="space-y-1">
      <li v-for="item in items" :key="item.id">
        <a
          :href="`#${item.id}`"
          class="block truncate rounded py-1 text-sm transition-colors"
          :class="[
            activeId === item.id
              ? 'font-medium text-primary-600 dark:text-primary-400'
              : 'text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white',
          ]"
          :style="{ paddingLeft: `${(item.level - 2) * 12 + 8}px` }"
          @click.prevent="scrollTo(item.id)"
        >
          {{ item.text }}
        </a>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import type { TocItem } from '@/composables/useMarkdownRenderer'

const props = defineProps<{
  items: TocItem[]
}>()

const activeId = ref('')
let observer: IntersectionObserver | null = null

function scrollTo(id: string) {
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

function setupObserver() {
  if (observer) observer.disconnect()
  if (props.items.length === 0) return

  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          activeId.value = entry.target.id
          break
        }
      }
    },
    { rootMargin: '-80px 0px -70% 0px', threshold: 0 },
  )

  for (const item of props.items) {
    const el = document.getElementById(item.id)
    if (el) observer.observe(el)
  }
}

watch(
  () => props.items,
  () => {
    // Wait a tick for the DOM to update with new heading IDs
    setTimeout(setupObserver, 100)
  },
)

onMounted(setupObserver)

onBeforeUnmount(() => {
  if (observer) observer.disconnect()
})
</script>
