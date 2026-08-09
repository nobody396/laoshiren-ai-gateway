<template>
  <div class="space-y-2" @paste.capture="handlePaste">
    <MdEditor
      v-if="!loadFailed"
      :modelValue="modelValue"
      @update:modelValue="updateValue"
      :language="language"
      :theme="theme"
      :toolbarsExclude="toolbarsExclude"
      :onUploadImg="handleUploadImg"
      :placeholder="placeholder"
      class="feedback-md-editor"
    />
    <textarea
      v-else
      :value="modelValue"
      class="input min-h-[240px] resize-y font-mono"
      :placeholder="placeholder"
      @input="updateValue(($event.target as HTMLTextAreaElement).value)"
    />
    <p v-if="loadFailed" class="text-xs text-amber-600 dark:text-amber-400">
      {{ fallbackMessage }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, ref, onErrorCaptured } from 'vue'
import type { ToolbarNames } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  uploadHandler?: (files: File[]) => Promise<string[]>
  fallbackMessage?: string
}>(), {
  placeholder: '',
  fallbackMessage: '',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'paste-image-blocked': []
}>()

const loadFailed = ref(false)
// Keep the editor behind a real dynamic import. A static import made Rollup's
// optional editor chunk and this wrapper import each other in production.
const MdEditor = defineAsyncComponent({
  loader: () => import('md-editor-v3').then((module) => module.MdEditor),
  onError(_error, _retry, fail) {
    loadFailed.value = true
    fail()
  },
})
const theme = computed(() => document.documentElement.classList.contains('dark') ? 'dark' : 'light')
const language = 'en-US'
const toolbarsExclude: ToolbarNames[] = ['save', 'github', 'catalog', 'mermaid', 'katex', 'htmlPreview']

onErrorCaptured(() => {
  loadFailed.value = true
  return false
})

function updateValue(value: string) {
  emit('update:modelValue', value)
}

async function handleUploadImg(files: File[], callback: (urls: string[]) => void) {
  if (!props.uploadHandler) {
    callback([])
    return
  }
  const urls = await props.uploadHandler(files)
  callback(urls)
}

function handlePaste(event: ClipboardEvent) {
  const items = Array.from(event.clipboardData?.items ?? [])
  if (items.some((item) => item.type.startsWith('image/'))) {
    event.preventDefault()
    emit('paste-image-blocked')
  }
}
</script>

<style scoped>
.feedback-md-editor :deep(.md-editor) {
  border-radius: 1rem;
  overflow: hidden;
}
</style>
