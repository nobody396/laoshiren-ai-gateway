<template>
  <div class="docs-content-wrapper">
    <!-- Loading skeleton -->
    <div v-if="loading" class="space-y-4 animate-pulse">
      <div class="h-8 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
      <div class="h-px w-full bg-gray-200 dark:bg-dark-700"></div>
      <div class="space-y-3">
        <div class="h-4 w-full rounded bg-gray-200 dark:bg-dark-700"></div>
        <div class="h-4 w-5/6 rounded bg-gray-200 dark:bg-dark-700"></div>
        <div class="h-4 w-4/6 rounded bg-gray-200 dark:bg-dark-700"></div>
      </div>
    </div>

    <!-- Not found -->
    <div v-else-if="notFound" class="py-20 text-center">
      <div class="text-4xl mb-4">📄</div>
      <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-2">文档未找到</h2>
      <p class="text-gray-500 dark:text-dark-400">请从左侧导航选择一个文档页面。</p>
    </div>

    <!-- Rendered markdown -->
    <div v-else class="docs-content-main">
      <button v-if="canCopyWholeDoc" type="button" class="docs-doc-copy-btn" @click="handleCopyWholeDoc">
        {{ copied ? '已复制' : '一键复制' }}
      </button>

      <div
        ref="markdownRef"
        class="docs-markdown"
        :class="{ 'docs-markdown--with-copy': canCopyWholeDoc }"
        v-html="html"
      ></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, nextTick, onBeforeUnmount } from 'vue'
import { useClipboard } from '@/composables/useClipboard'

const props = defineProps<{
  html: string
  markdown: string
  loading: boolean
  notFound: boolean
}>()

const markdownRef = ref<HTMLElement | null>(null)
const cleanups: (() => void)[] = []
const { copied, copyToClipboard } = useClipboard()

/**
 * 只有当前文档正文存在时，才展示整篇复制入口。
 */
const canCopyWholeDoc = computed(() => !props.loading && !props.notFound && !!props.markdown.trim())

/**
 * 复制当前文档的 Markdown 原文，便于直接交给 AI 使用。
 */
async function handleCopyWholeDoc() {
  if (!canCopyWholeDoc.value) return
  await copyToClipboard(props.markdown, '整篇文档已复制')
}

/**
 * 为 Markdown 中的代码块动态注入复制按钮，保持现有代码复制体验。
 */
function injectCopyButtons() {
  // Clean up previous buttons
  cleanups.forEach((fn) => fn())
  cleanups.length = 0

  if (!markdownRef.value) return

  const preBlocks = markdownRef.value.querySelectorAll('pre')
  preBlocks.forEach((pre) => {
    // Make pre relative for absolute button positioning
    pre.style.position = 'relative'

    const btn = document.createElement('button')
    btn.className = 'code-copy-btn'
    btn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>`
    btn.title = '复制'

    const copyIcon = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>`
    const checkIcon = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`

    const showCopied = () => {
      btn.innerHTML = `${checkIcon}<span class="code-copy-label">已复制</span>`
      btn.classList.add('copied')
      setTimeout(() => {
        btn.innerHTML = copyIcon
        btn.classList.remove('copied')
      }, 2000)
    }

    const handler = async () => {
      const code = pre.querySelector('code')
      const text = (code || pre).textContent || ''
      const success = await copyToClipboard(text)
      if (success) {
        showCopied()
      }
    }

    btn.addEventListener('click', handler)
    pre.appendChild(btn)

    cleanups.push(() => {
      btn.removeEventListener('click', handler)
      btn.remove()
    })
  })
}

watch(() => props.html, () => {
  nextTick(injectCopyButtons)
})

onBeforeUnmount(() => {
  cleanups.forEach((fn) => fn())
})
</script>

<style>
.docs-content-main {
  position: relative;
}

.docs-doc-copy-btn {
  position: absolute;
  top: 0.125rem;
  right: 0;
  z-index: 1;
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 0.75rem;
  background: rgba(255, 255, 255, 0.92);
  color: rgb(var(--color-gray-700));
  font-size: 0.875rem;
  font-weight: 600;
  line-height: 1;
  padding: 0.8rem 1rem;
  cursor: pointer;
  transition: border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;
  backdrop-filter: blur(8px);
}

.docs-doc-copy-btn:hover {
  border-color: rgb(var(--color-terracotta));
  color: rgb(var(--color-terracotta));
  background: rgb(var(--color-primary-50));
}

.docs-doc-copy-btn:active {
  background: rgb(var(--color-primary-50));
}

.docs-markdown--with-copy h1:first-child {
  max-width: calc(100% - 8rem);
}

/* Markdown rendering styles for docs */
.docs-markdown {
  color: rgb(var(--color-gray-700));
  line-height: 1.75;
  font-size: 1rem;
}

.dark .docs-markdown {
  color: rgb(var(--color-gray-300));
}

.dark .docs-doc-copy-btn {
  border-color: rgb(var(--color-slate-700));
  background: rgba(15, 23, 42, 0.9);
  color: rgb(var(--color-gray-200));
}

.docs-markdown h1 {
  font-size: 2rem;
  font-weight: 700;
  margin-bottom: 1rem;
  color: rgb(var(--color-gray-900));
  line-height: 1.3;
}

.dark .docs-markdown h1 {
  color: rgb(var(--color-gray-50));
}

.docs-markdown h2 {
  font-size: 1.5rem;
  font-weight: 600;
  margin-top: 2.5rem;
  margin-bottom: 0.75rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid rgb(var(--color-gray-200));
  color: rgb(var(--color-gray-800));
}

.dark .docs-markdown h2 {
  border-bottom-color: rgb(var(--color-slate-700));
  color: rgb(var(--color-gray-100));
}

.docs-markdown h3 {
  font-size: 1.25rem;
  font-weight: 600;
  margin-top: 2rem;
  margin-bottom: 0.5rem;
  color: rgb(var(--color-gray-800));
}

.dark .docs-markdown h3 {
  color: rgb(var(--color-gray-100));
}

.docs-markdown h4 {
  font-size: 1.1rem;
  font-weight: 600;
  margin-top: 1.5rem;
  margin-bottom: 0.5rem;
  color: rgb(var(--color-gray-700));
}

.dark .docs-markdown h4 {
  color: rgb(var(--color-gray-200));
}

.docs-markdown p {
  margin-bottom: 1rem;
}

.docs-markdown a {
  color: rgb(var(--color-terracotta));
  text-decoration: underline;
  text-underline-offset: 2px;
}

.docs-markdown a:hover {
  color: rgb(var(--color-terracotta));
}

.docs-markdown strong {
  font-weight: 600;
  color: rgb(var(--color-gray-900));
}

.dark .docs-markdown strong {
  color: rgb(var(--color-gray-50));
}

.docs-markdown ul,
.docs-markdown ol {
  margin-bottom: 1rem;
  padding-left: 1.5rem;
}

.docs-markdown ul {
  list-style-type: disc;
}

.docs-markdown ol {
  list-style-type: decimal;
}

.docs-markdown li {
  margin-bottom: 0.375rem;
}

.docs-markdown li > ul,
.docs-markdown li > ol {
  margin-top: 0.375rem;
  margin-bottom: 0;
}

.docs-markdown blockquote {
  border-left: 4px solid rgb(var(--color-terracotta));
  padding: 0.5rem 1rem;
  margin: 1rem 0;
  background-color: rgb(var(--color-primary-50));
  border-radius: 0 0.375rem 0.375rem 0;
  color: rgb(var(--color-primary-800));
}

.dark .docs-markdown blockquote {
  background-color: rgba(216, 119, 87, 0.1);
  color: rgb(var(--color-primary-300));
}

.docs-markdown code {
  background-color: rgb(var(--color-gray-100));
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  font-size: 0.875em;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: rgb(var(--color-terracotta));
}

.dark .docs-markdown code {
  background-color: rgb(var(--color-slate-800));
  color: rgb(var(--color-primary-300));
}

.docs-markdown pre {
  background-color: rgb(var(--color-slate-800));
  color: rgb(var(--color-slate-200));
  padding: 1rem;
  border-radius: 0.5rem;
  overflow-x: auto;
  margin: 1rem 0;
  font-size: 0.875rem;
  line-height: 1.7;
}

.docs-markdown pre code {
  background-color: transparent;
  padding: 0;
  color: inherit;
  font-size: inherit;
}

.docs-markdown hr {
  border: 0;
  border-top: 1px solid rgb(var(--color-gray-200));
  margin: 2rem 0;
}

.dark .docs-markdown hr {
  border-top-color: rgb(var(--color-slate-700));
}

.docs-markdown table {
  width: 100%;
  border-collapse: collapse;
  margin: 1rem 0;
  font-size: 0.9375rem;
}

.docs-markdown th,
.docs-markdown td {
  border: 1px solid rgb(var(--color-gray-200));
  padding: 0.5rem 0.75rem;
  text-align: left;
}

.dark .docs-markdown th,
.dark .docs-markdown td {
  border-color: rgb(var(--color-slate-700));
}

.docs-markdown th {
  background-color: rgb(var(--color-gray-50));
  font-weight: 600;
}

.dark .docs-markdown th {
  background-color: rgb(var(--color-slate-800));
}

.docs-markdown img {
  max-width: 100%;
  border-radius: 0.5rem;
  margin: 1rem 0;
}

/* Code block copy button */
.docs-markdown pre {
  position: relative;
}

.code-copy-btn {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
  height: 2rem;
  padding: 0 0.5rem;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 0.375rem;
  background-color: rgba(255, 255, 255, 0.08);
  color: rgb(var(--color-slate-400));
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.2s, background-color 0.2s, color 0.2s;
  font-size: 0.75rem;
  font-family: inherit;
  line-height: 1;
}

.docs-markdown pre:hover .code-copy-btn {
  opacity: 1;
}

.code-copy-btn:hover {
  background-color: rgba(255, 255, 255, 0.15);
  color: rgb(var(--color-slate-200));
}

.code-copy-btn.copied {
  opacity: 1;
  color: rgb(var(--color-green-400));
  border-color: rgba(74, 222, 128, 0.3);
  background-color: rgba(74, 222, 128, 0.1);
}

.code-copy-label {
  white-space: nowrap;
}

@media (max-width: 640px) {
  .docs-doc-copy-btn {
    position: static;
    margin-bottom: 1rem;
  }

  .docs-markdown--with-copy h1:first-child {
    max-width: 100%;
  }
}
</style>
