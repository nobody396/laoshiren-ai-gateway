<template>
  <div class="public-info-page">
    <header class="public-info-header">
      <router-link to="/" class="public-info-brand">
        <img src="/laoshirenai-icon.jpg" alt="老实人AI" />
        <span>老实人AI</span>
      </router-link>
      <nav>
        <router-link to="/enterprise">企业</router-link>
        <router-link to="/security">安全</router-link>
        <router-link to="/status">状态</router-link>
        <router-link to="/changelog">更新日志</router-link>
        <router-link to="/legal/terms">条款</router-link>
        <router-link to="/docs">文档</router-link>
        <router-link to="/login">登录</router-link>
      </nav>
    </header>

    <main class="public-info-main">
      <DocsContent
        :html="renderedHtml"
        :markdown="markdownSource"
        :loading="loading"
        :not-found="notFound"
      />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import DocsContent from '@/components/docs/DocsContent.vue'
import { useMarkdownRenderer } from '@/composables/useMarkdownRenderer'
import { loadMarkdown } from '@/docs/config'

const route = useRoute()
const slug = computed(() => String(route.meta.publicDocSlug || ''))
const markdownSource = ref('')
const loading = ref(true)
const notFound = ref(false)
const { renderedHtml } = useMarkdownRenderer(markdownSource)

watch(
  slug,
  async (newSlug) => {
    loading.value = true
    notFound.value = false
    const content = await loadMarkdown(newSlug)
    if (content === null) {
      markdownSource.value = ''
      notFound.value = true
    } else {
      markdownSource.value = content
    }
    loading.value = false
    window.scrollTo({ top: 0 })
  },
  { immediate: true },
)
</script>

<style scoped>
.public-info-page {
  min-height: 100vh;
  background: #ffffff;
  color: rgb(var(--color-gray-700));
  color-scheme: light;
}

.public-info-header {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  min-height: 4rem;
  align-items: center;
  justify-content: space-between;
  gap: 1.5rem;
  border-bottom: 1px solid rgb(var(--color-gray-200));
  background: rgba(255, 255, 255, 0.92);
  padding: 0 2rem;
  backdrop-filter: blur(10px);
}

.public-info-brand {
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
  color: rgb(var(--color-gray-900));
  font-weight: 700;
  text-decoration: none;
}

.public-info-brand img {
  width: 2rem;
  height: 2rem;
  border-radius: 999px;
  object-fit: cover;
}

.public-info-header nav {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}

.public-info-header nav a {
  color: rgb(var(--color-gray-600));
  font-size: 0.95rem;
  font-weight: 600;
  text-decoration: none;
}

.public-info-header nav a:hover,
.public-info-header nav a.router-link-active {
  color: #d87757;
}

.public-info-main {
  width: min(100% - 2rem, 860px);
  margin: 0 auto;
  padding: 3rem 0 5rem;
}

.public-info-page :deep(.docs-doc-copy-btn) {
  border-color: rgb(var(--color-gray-200));
  background: rgba(255, 255, 255, 0.92);
  color: rgb(var(--color-gray-700));
}

.public-info-page :deep(.docs-doc-copy-btn:hover) {
  border-color: #d87757;
  background: #fff7f3;
  color: #d87757;
}

.public-info-page :deep(.docs-markdown) {
  color: rgb(var(--color-gray-700));
}

.public-info-page :deep(.docs-markdown h1) {
  color: rgb(var(--color-gray-900));
}

.public-info-page :deep(.docs-markdown h2),
.public-info-page :deep(.docs-markdown h3) {
  color: rgb(var(--color-gray-800));
}

.public-info-page :deep(.docs-markdown h2) {
  border-bottom-color: rgb(var(--color-gray-200));
}

.public-info-page :deep(.docs-markdown h4) {
  color: rgb(var(--color-gray-700));
}

.public-info-page :deep(.docs-markdown strong) {
  color: rgb(var(--color-gray-900));
}

.public-info-page :deep(.docs-markdown blockquote) {
  background-color: #fdf5f2;
  color: #6f3829;
}

.public-info-page :deep(.docs-markdown code) {
  background-color: rgb(var(--color-gray-100));
  color: #d87757;
}

.public-info-page :deep(.docs-markdown pre) {
  background-color: rgb(var(--color-slate-800));
  color: rgb(var(--color-slate-200));
}

.public-info-page :deep(.docs-markdown pre code) {
  background-color: transparent;
  color: inherit;
}

.public-info-page :deep(.docs-markdown hr) {
  border-top-color: rgb(var(--color-gray-200));
}

.public-info-page :deep(.docs-markdown th),
.public-info-page :deep(.docs-markdown td) {
  border-color: rgb(var(--color-gray-200));
}

.public-info-page :deep(.docs-markdown th) {
  background-color: rgb(var(--color-gray-50));
}

@media (max-width: 640px) {
  .public-info-header {
    align-items: flex-start;
    flex-direction: column;
    padding: 0.9rem 1rem;
  }
}
</style>
