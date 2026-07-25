<template>
  <span
    class="inline-flex shrink-0 items-center justify-center overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-sm dark:border-white/10 dark:bg-dark-700"
    :data-client-icon="client"
    aria-hidden="true"
  >
    <template v-if="client === 'codex'">
      <img
        src="/brand/client-tools/codex-light.png"
        alt=""
        class="h-full w-full object-contain dark:hidden"
      />
      <img
        src="/brand/client-tools/codex-dark.png"
        alt=""
        class="hidden h-full w-full object-contain dark:block"
      />
    </template>
    <img
      v-else
      :src="iconSource"
      alt=""
      :class="['object-contain', iconSizeClass]"
    />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import type { CcsImportTarget } from '@/utils/ccSwitchImport'

const props = defineProps<{
  client: CcsImportTarget
}>()

const iconSources: Record<CcsImportTarget, string> = {
  claude: '/brand/client-tools/claude.svg',
  codex: '/brand/client-tools/codex-light.png',
  opencode: '/brand/client-tools/opencode.svg',
  openclaw: '/brand/client-tools/openclaw.svg',
  hermes: '/brand/client-tools/hermes.png',
  gemini: '/brand/client-tools/gemini.svg'
}

const iconSource = computed(() => iconSources[props.client])

const iconSizeClass = computed(() => {
  switch (props.client) {
    case 'claude':
    case 'gemini':
      return 'h-[62%] w-[62%]'
    case 'openclaw':
      return 'h-[80%] w-[80%]'
    default:
      return 'h-full w-full'
  }
})
</script>
