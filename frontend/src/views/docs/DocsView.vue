<template>
  <DocsLayout :current-slug="slug" :toc-items="tocItems">
    <DocsModelCatalog v-if="slug === 'models'" />
    <DocsContent
      v-else
      :html="renderedHtml"
      :markdown="markdownSource"
      :slug="slug"
      :loading="loading"
      :not-found="notFound"
    />
  </DocsLayout>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { defaultSlug, loadMarkdown, resolveDocSlug } from '@/docs/config'
import { useMarkdownRenderer } from '@/composables/useMarkdownRenderer'
import DocsLayout from '@/components/docs/DocsLayout.vue'
import DocsContent from '@/components/docs/DocsContent.vue'
import DocsModelCatalog from '@/components/docs/DocsModelCatalog.vue'

const route = useRoute()
// 侧边栏高亮和内容加载都统一使用规范化后的 slug。
const slug = computed(() => resolveDocSlug((route.params.slug as string) || defaultSlug))

const markdownSource = ref('')
const loading = ref(true)
const notFound = ref(false)

const { renderedHtml, tocItems } = useMarkdownRenderer(markdownSource)

/**
 * 根据当前 slug 加载对应 Markdown 原文，并同步驱动渲染结果。
 */
watch(
  slug,
  async (newSlug) => {
    loading.value = true
    notFound.value = false
    if (newSlug === 'models') {
      markdownSource.value = ''
      loading.value = false
      window.scrollTo({ top: 0 })
      return
    }
    const content = await loadMarkdown(newSlug)
    if (content === null) {
      notFound.value = true
      markdownSource.value = ''
    } else {
      markdownSource.value = content
    }
    loading.value = false
    // Scroll to top on doc change
    window.scrollTo({ top: 0 })
  },
  { immediate: true },
)
</script>
