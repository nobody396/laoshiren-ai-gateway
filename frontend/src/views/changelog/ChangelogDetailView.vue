<template>
  <ChangelogPublicShell>
    <main class="changelog-detail">
      <router-link to="/changelog" class="changelog-back">
        <Icon name="chevronLeft" size="sm" />
        {{ t('changelog.backToList') }}
      </router-link>

      <div v-if="loading" class="changelog-detail-loading">
        <span></span><span></span><span></span>
      </div>

      <section v-else-if="errorMessage" class="changelog-detail-error">
        <h1>{{ t('changelog.notFound') }}</h1>
        <p>{{ errorMessage }}</p>
        <router-link to="/changelog">{{ t('changelog.backToList') }}</router-link>
      </section>

      <article v-else-if="entry">
        <header>
          <div class="changelog-detail-meta">
            <time :datetime="entry.published_at || undefined">{{ formatDate(entry.published_at) }}</time>
            <span>{{ categoryLabel(entry.category) }}</span>
          </div>
          <h1>{{ entry.title }}</h1>
          <p class="changelog-detail-summary">{{ entry.summary }}</p>
          <div class="changelog-detail-rationale">
            <strong>{{ t('changelog.whyBuilt') }}</strong>
            <span>{{ entry.rationale }}</span>
          </div>
          <p v-if="entry.related_products.length" class="changelog-detail-products">
            <strong>{{ t('changelog.relatedProducts') }}</strong>
            {{ entry.related_products.join('、') }}
          </p>
        </header>

        <div class="changelog-detail-rule" aria-hidden="true"></div>
        <div class="changelog-markdown" v-html="renderedHtml"></div>

        <footer>
          <router-link to="/changelog">
            <Icon name="chevronLeft" size="sm" />
            {{ t('changelog.backToList') }}
          </router-link>
          <button type="button" @click="copyLink">
            <Icon :name="copied ? 'check' : 'copy'" size="sm" />
            {{ copied ? t('common.copied') : t('changelog.copyLink') }}
          </button>
        </footer>
      </article>
    </main>
  </ChangelogPublicShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import ChangelogPublicShell from '@/components/changelog/ChangelogPublicShell.vue'
import Icon from '@/components/icons/Icon.vue'
import { changelogAPI } from '@/api'
import { useMarkdownRenderer } from '@/composables/useMarkdownRenderer'
import { useClipboard } from '@/composables/useClipboard'
import { useChangelogFreshness } from '@/composables/useChangelogFreshness'
import { updateRouteSeo } from '@/utils/seo'
import type { ChangelogCategory, PublicChangelogEntry } from '@/types'

const route = useRoute()
const { t, locale } = useI18n()
const entry = ref<PublicChangelogEntry | null>(null)
const markdownSource = computed(() => entry.value?.content ?? '')
const { renderedHtml } = useMarkdownRenderer(markdownSource)
const { copied, copyToClipboard } = useClipboard()
const { markChangelogSeen } = useChangelogFreshness()
const loading = ref(true)
const errorMessage = ref('')

function categoryLabel(category: ChangelogCategory): string {
  const keys: Record<ChangelogCategory, string> = {
    feature: 'changelog.categories.feature',
    model_config: 'changelog.categories.modelConfig',
    improvement: 'changelog.categories.improvement',
    fix: 'changelog.categories.fix'
  }
  return t(keys[category])
}

function formatDate(value: string | null): string {
  if (!value) return ''
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-US' : 'zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  }).format(new Date(value))
}

async function loadEntry() {
  loading.value = true
  errorMessage.value = ''
  try {
    entry.value = await changelogAPI.getBySlug(String(route.params.slug))
    markChangelogSeen(entry.value.published_at)
    updateRouteSeo(route, {
      customTitle: `${entry.value.title} - ${t('changelog.title')}`,
      customDescription: entry.value.summary
    })
  } catch (error: any) {
    errorMessage.value = error?.message || t('changelog.notFound')
  } finally {
    loading.value = false
  }
}

async function copyLink() {
  await copyToClipboard(window.location.href)
}

onMounted(() => void loadEntry())
</script>

<style scoped>
.changelog-detail {
  width: min(100% - 4rem, 860px);
  min-height: 70vh;
  margin: 0 auto;
  padding: 7.5rem 0 6rem;
}

.changelog-back,
.changelog-detail footer a,
.changelog-detail footer button {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border: 0;
  background: transparent;
  color: #3f5a3a;
  font-size: 0.94rem;
  text-decoration: none;
  cursor: pointer;
}

.changelog-detail article { margin-top: 2.5rem; }

.changelog-detail-meta {
  display: flex;
  align-items: center;
  gap: 1rem;
  color: #3f5a3a;
  font-family: 'Inter', sans-serif;
  font-size: 0.78rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.changelog-detail-meta time {
  color: #8a7d63;
  font-family: 'EB Garamond', serif;
  font-size: 1rem;
  text-transform: none;
}

.changelog-detail h1 {
  margin: 1rem 0 0;
  color: #13100b;
  font-family: 'Noto Serif SC', 'EB Garamond', serif;
  font-size: clamp(2.5rem, 6vw, 4.5rem);
  font-weight: 600;
  letter-spacing: 0.025em;
  line-height: 1.15;
}

.changelog-detail-summary {
  margin: 1.25rem 0 0;
  color: #514a3c;
  font-family: 'Noto Serif SC', serif;
  font-size: 1.2rem;
  line-height: 1.8;
}

.changelog-detail-rationale {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.9rem;
  margin-top: 1.4rem;
  border-left: 2px solid #3f5a3a;
  padding: 0.2rem 0 0.2rem 1rem;
  color: #4d4536;
  line-height: 1.7;
}

.changelog-detail-rationale strong,
.changelog-detail-products strong {
  color: #2d291f;
  font-weight: 600;
}

.changelog-detail-products { color: #6d6453; font-size: 0.92rem; }

.changelog-detail-rule {
  margin: 2.8rem 0;
  border-top: 1px solid rgba(138, 125, 99, 0.45);
}

.changelog-markdown {
  color: #312d24;
  font-size: 1.02rem;
  line-height: 1.85;
}

.changelog-markdown :deep(h2) {
  margin: 2.8rem 0 1rem;
  color: #13100b;
  font-family: 'Noto Serif SC', serif;
  font-size: 1.65rem;
  font-weight: 600;
}

.changelog-markdown :deep(h3) {
  margin: 2rem 0 0.75rem;
  color: #1e1a13;
  font-family: 'Noto Serif SC', serif;
  font-size: 1.3rem;
}

.changelog-markdown :deep(p),
.changelog-markdown :deep(ul),
.changelog-markdown :deep(ol) { margin: 1rem 0; }
.changelog-markdown :deep(a) { color: #3f5a3a; text-underline-offset: 0.2rem; }
.changelog-markdown :deep(code) {
  border: 1px solid rgba(138, 125, 99, 0.25);
  background: rgba(255, 255, 255, 0.34);
  color: #9a3b1f;
  padding: 0.14rem 0.32rem;
}
.changelog-markdown :deep(pre) {
  overflow-x: auto;
  background: #1b1914;
  color: #f8f3e7;
  padding: 1.2rem;
}
.changelog-markdown :deep(pre code) { border: 0; background: transparent; color: inherit; padding: 0; }
.changelog-markdown :deep(blockquote) {
  margin: 1.5rem 0;
  border-left: 2px solid #9a3b1f;
  padding-left: 1rem;
  color: #655d4e;
}
.changelog-markdown :deep(img) { max-width: 100%; }

.changelog-detail footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 4rem;
  border-top: 1px solid rgba(138, 125, 99, 0.38);
  padding-top: 1.4rem;
}

.changelog-detail-loading {
  display: grid;
  gap: 1rem;
  margin-top: 4rem;
}
.changelog-detail-loading span {
  height: 1rem;
  background: rgba(138, 125, 99, 0.16);
}
.changelog-detail-loading span:first-child { width: 65%; height: 3.5rem; }
.changelog-detail-loading span:nth-child(2) { width: 90%; }
.changelog-detail-loading span:nth-child(3) { width: 74%; }

.changelog-detail-error { padding: 6rem 0; text-align: center; }
.changelog-detail-error h1 { font-size: 2.4rem; }
.changelog-detail-error p { color: #6d6453; }
.changelog-detail-error a { color: #3f5a3a; }

@media (max-width: 640px) {
  .changelog-detail {
    width: min(100% - 2rem, 860px);
    padding-top: 6.5rem;
  }
  .changelog-detail-rationale { grid-template-columns: 1fr; gap: 0.2rem; }
}
</style>
