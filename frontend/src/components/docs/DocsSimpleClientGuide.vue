<template>
  <section class="simple-guide">
    <DocsContent :html="renderedHtml" :markdown="guide.content" :loading="false" :not-found="false" />
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import DocsContent from './DocsContent.vue'
import { useMarkdownRenderer } from '@/composables/useMarkdownRenderer'
import type { SimpleClientGuide } from '@/docs/guides/simpleClientGuides'

const props = defineProps<{ guide: SimpleClientGuide }>()
const source = computed(() => props.guide.content)
const { renderedHtml } = useMarkdownRenderer(source)
</script>

<style scoped>
.simple-guide{margin-top:1.5rem}
</style>
