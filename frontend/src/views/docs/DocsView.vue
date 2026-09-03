<template>
  <DocsLayout :current-slug="slug" :toc-items="tocItems" :home="isLanding">
    <DocsHome v-if="isHome" />
    <DocsIntegrationsHome v-else-if="activeCategory?.key === 'integrations'" />
    <DocsCategoryHome v-else-if="activeCategory" :category="activeCategory" />
    <DocsModelCatalog v-else-if="slug === 'models'" />
    <DocsModelMatrix v-else-if="slug === 'model-matrix'" />
    <DocsClientMatrix v-else-if="slug === 'client-matrix'" />
    <DocsIntegrationDetail v-else-if="integrationClient" :client="integrationClient" />
    <DocsContent
      v-else
      :html="renderedHtml"
      :markdown="markdownSource"
      :loading="loading"
      :not-found="notFound"
    />
  </DocsLayout>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { defaultSlug, docsConfig, findDocItemBySlug, loadMarkdown, resolveDocSlug, type DocCategory } from '@/docs/config'
import { useMarkdownRenderer } from '@/composables/useMarkdownRenderer'
import DocsLayout from '@/components/docs/DocsLayout.vue'
import DocsContent from '@/components/docs/DocsContent.vue'
import DocsHome from '@/components/docs/DocsHome.vue'
import DocsCategoryHome from '@/components/docs/DocsCategoryHome.vue'
import DocsModelCatalog from '@/components/docs/DocsModelCatalog.vue'
import DocsModelMatrix from '@/components/docs/DocsModelMatrix.vue'
import DocsClientMatrix from '@/components/docs/DocsClientMatrix.vue'
import DocsIntegrationsHome from '@/components/docs/DocsIntegrationsHome.vue'
import DocsIntegrationDetail from '@/components/docs/DocsIntegrationDetail.vue'
import { clientMatrixBySlug } from '@/generated/clientMatrix'

const route = useRoute()
const categoryKey = computed(() => String(route.params.category || ''))
const activeCategory = computed<DocCategory | undefined>(() =>
  docsConfig.find(category => category.key === categoryKey.value),
)
const isHome = computed(() => !route.params.slug && !route.params.category)
const isLanding = computed(() => isHome.value || Boolean(activeCategory.value))
// 详情页的侧边栏高亮和内容加载统一使用规范化后的 slug。
const slug = computed(() => isLanding.value ? '' : resolveDocSlug((route.params.slug as string) || defaultSlug))
const integrationClient = computed(() => clientMatrixBySlug[slug.value])

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
    if (!newSlug) {
      loading.value = false
      notFound.value = false
      markdownSource.value = ''
      window.scrollTo({ top: 0 })
      return
    }
    loading.value = true
    notFound.value = false
    if (!findDocItemBySlug(newSlug)) {
      notFound.value = true
      markdownSource.value = ''
      loading.value = false
      window.scrollTo({ top: 0 })
      return
    }
    if (newSlug === 'models' || newSlug === 'model-matrix' || newSlug === 'client-matrix') {
      markdownSource.value = ''
      loading.value = false
      window.scrollTo({ top: 0 })
      return
    }
    if (clientMatrixBySlug[newSlug]) {
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
