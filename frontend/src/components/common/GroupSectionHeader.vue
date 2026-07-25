<template>
  <div class="flex w-full items-center justify-between gap-3">
    <div class="flex min-w-0 items-center gap-2.5">
      <span
        :class="[
          'h-4 w-1 shrink-0 rounded-full',
          section === 'monthly'
            ? 'bg-blue-500 dark:bg-blue-400'
            : 'bg-amber-500 dark:bg-amber-400'
        ]"
      />
      <span class="truncate text-xs font-semibold tracking-wide text-gray-700 dark:text-gray-200">
        {{ label }}
      </span>
    </div>
    <span
      class="shrink-0 rounded-full bg-white px-2 py-0.5 text-[11px] font-medium text-gray-500 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-gray-400 dark:ring-dark-600"
    >
      {{ count }}
    </span>
  </div>
</template>

<script setup lang="ts">
import type { GroupOptionSectionId } from '@/utils/groupOptionSections'

defineProps<{
  section: GroupOptionSectionId
  label: string
  count: number
}>()
</script>
