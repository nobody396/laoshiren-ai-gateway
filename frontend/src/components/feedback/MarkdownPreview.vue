<template>
  <MdPreview
    v-if="!loadFailed"
    :id="previewId"
    :modelValue="content"
    :theme="theme"
    :sanitize="sanitizeHtml"
    class="feedback-md-preview"
  />
  <div
    v-else
    class="prose max-w-none rounded-2xl border border-gray-200 bg-white p-5 dark:prose-invert dark:border-dark-700 dark:bg-dark-900"
    v-html="sanitizedHtml"
  />
</template>

<script setup lang="ts">
import { computed, ref, onErrorCaptured } from 'vue'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'

const props = defineProps<{
  content: string
  previewId?: string
}>()

const loadFailed = ref(false)
const theme = computed(() => document.documentElement.classList.contains('dark') ? 'dark' : 'light')

const sanitizeHtml = (html: string) => DOMPurify.sanitize(html)

onErrorCaptured(() => {
  loadFailed.value = true
  return false
})

const sanitizedHtml = computed(() => {
  const html = marked.parse(props.content || '') as string
  return DOMPurify.sanitize(html)
})
</script>

<style scoped>
.feedback-md-preview :deep(.md-editor-preview-wrapper) {
  border-radius: 1rem;
}
</style>
