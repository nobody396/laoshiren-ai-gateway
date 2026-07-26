<template>
  <ChangelogPublicShell>
    <main class="changelog-page">
      <section class="changelog-hero">
        <h1>{{ t('changelog.title') }}</h1>
        <p>{{ t('changelog.subtitle') }}</p>
        <div class="changelog-rule" aria-hidden="true">
          <span></span>
        </div>
      </section>

      <section class="changelog-browser" aria-labelledby="changelog-feed-title">
        <h2 id="changelog-feed-title" class="sr-only">{{ t('changelog.feedTitle') }}</h2>
        <div class="changelog-toolbar">
          <div class="changelog-categories" role="tablist" :aria-label="t('changelog.filterLabel')">
            <button
              v-for="category in categories"
              :key="category.value"
              type="button"
              role="tab"
              :aria-selected="selectedCategory === category.value"
              :class="{ 'is-active': selectedCategory === category.value }"
              @click="selectCategory(category.value)"
            >
              {{ category.label }}
            </button>
          </div>

          <label class="changelog-search">
            <Icon name="search" size="sm" />
            <span class="sr-only">{{ t('changelog.search') }}</span>
            <input
              v-model="searchQuery"
              type="search"
              :placeholder="t('changelog.search')"
              @input="scheduleSearch"
            />
          </label>
        </div>

        <div v-if="loading && entries.length === 0" class="changelog-loading" aria-live="polite">
          <div v-for="item in 4" :key="item" class="changelog-skeleton">
            <span></span><div><i></i><i></i><i></i></div>
          </div>
        </div>

        <div v-else-if="errorMessage" class="changelog-empty" role="alert">
          <h3>{{ t('changelog.loadFailed') }}</h3>
          <p>{{ errorMessage }}</p>
          <button type="button" @click="loadEntries(true)">{{ t('common.retry') }}</button>
        </div>

        <div v-else-if="groupedEntries.length === 0" class="changelog-empty">
          <h3>{{ t('changelog.emptyTitle') }}</h3>
          <p>{{ t('changelog.emptyDescription') }}</p>
        </div>

        <div v-else class="changelog-months">
          <section v-for="month in groupedEntries" :key="month.key" class="changelog-month">
            <h2>{{ month.label }}</h2>
            <ol class="changelog-timeline">
              <li v-for="entry in month.entries" :key="entry.id">
                <div class="changelog-date">
                  <time :datetime="entry.published_at || undefined">{{ formatDay(entry.published_at) }}</time>
                  <span v-if="isNewestEntry(entry)" class="changelog-new">NEW</span>
                </div>
                <span class="changelog-node" aria-hidden="true"></span>
                <article>
                  <div class="changelog-entry-heading">
                    <div>
                      <h3>
                        <router-link :to="`/changelog/${entry.slug}`">{{ entry.title }}</router-link>
                      </h3>
                      <span class="changelog-category">{{ categoryLabel(entry.category) }}</span>
                    </div>
                    <router-link class="changelog-detail-link" :to="`/changelog/${entry.slug}`">
                      {{ t('changelog.viewDetails') }}
                      <Icon name="chevronRight" size="sm" />
                    </router-link>
                  </div>
                  <p class="changelog-summary">{{ entry.summary }}</p>
                  <p class="changelog-rationale">
                    <strong>{{ t('changelog.whyBuilt') }}</strong>
                    {{ entry.rationale }}
                  </p>
                  <p v-if="entry.related_products.length" class="changelog-products">
                    <strong>{{ t('changelog.relatedProducts') }}</strong>
                    {{ entry.related_products.join('、') }}
                  </p>
                </article>
              </li>
            </ol>
          </section>
        </div>

        <div v-if="hasMore" class="changelog-load-more">
          <button type="button" :disabled="loading" @click="loadMore">
            <Icon name="chevronDown" size="sm" />
            {{ loading ? t('common.loading') : t('changelog.loadMore') }}
          </button>
        </div>
      </section>
    </main>
  </ChangelogPublicShell>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import ChangelogPublicShell from '@/components/changelog/ChangelogPublicShell.vue'
import Icon from '@/components/icons/Icon.vue'
import { changelogAPI } from '@/api'
import { useChangelogFreshness } from '@/composables/useChangelogFreshness'
import type { ChangelogCategory, PublicChangelogEntry } from '@/types'

type CategoryFilter = '' | ChangelogCategory

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const { markChangelogSeen } = useChangelogFreshness()

const entries = ref<PublicChangelogEntry[]>([])
const loading = ref(false)
const errorMessage = ref('')
const selectedCategory = ref<CategoryFilter>(normalizeCategory(route.query.category))
const searchQuery = ref(typeof route.query.q === 'string' ? route.query.q : '')
const page = ref(1)
const total = ref(0)
const pageSize = 12
let requestID = 0
let searchTimer: number | null = null

const categories = computed<Array<{ value: CategoryFilter; label: string }>>(() => [
  { value: '', label: t('changelog.categories.all') },
  { value: 'feature', label: t('changelog.categories.feature') },
  { value: 'model_config', label: t('changelog.categories.modelConfig') },
  { value: 'improvement', label: t('changelog.categories.improvement') },
  { value: 'fix', label: t('changelog.categories.fix') }
])

const hasMore = computed(() => entries.value.length < total.value)
const groupedEntries = computed(() => {
  const groups = new Map<string, PublicChangelogEntry[]>()
  for (const entry of entries.value) {
    const date = entry.published_at ? new Date(entry.published_at) : new Date(entry.updated_at)
    const key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
    const group = groups.get(key) ?? []
    group.push(entry)
    groups.set(key, group)
  }
  return [...groups.entries()].map(([key, monthEntries]) => {
    const [year, month] = key.split('-').map(Number)
    const date = new Date(year, month - 1, 1)
    return {
      key,
      label: new Intl.DateTimeFormat(locale.value === 'en' ? 'en-US' : 'zh-CN', {
        year: 'numeric',
        month: 'long'
      }).format(date),
      entries: monthEntries
    }
  })
})

function normalizeCategory(value: unknown): CategoryFilter {
  return ['feature', 'model_config', 'improvement', 'fix'].includes(String(value))
    ? String(value) as ChangelogCategory
    : ''
}

function categoryLabel(category: ChangelogCategory): string {
  return categories.value.find((item) => item.value === category)?.label ?? category
}

function formatDay(value: string | null): string {
  if (!value) return '--.--'
  const date = new Date(value)
  return `${String(date.getMonth() + 1).padStart(2, '0')}.${String(date.getDate()).padStart(2, '0')}`
}

function isNewestEntry(entry: PublicChangelogEntry): boolean {
  return entries.value[0]?.id === entry.id
}

async function loadEntries(reset: boolean) {
  const currentRequest = ++requestID
  if (reset) {
    page.value = 1
    errorMessage.value = ''
  }
  loading.value = true
  try {
    const result = await changelogAPI.list(page.value, pageSize, {
      category: selectedCategory.value || undefined,
      search: searchQuery.value.trim() || undefined
    })
    if (currentRequest !== requestID) return
    entries.value = reset ? result.items : [...entries.value, ...result.items]
    total.value = result.total
    if (reset) {
      markChangelogSeen(new Date().toISOString())
    }
  } catch (error: any) {
    if (currentRequest !== requestID) return
    errorMessage.value = error?.message || t('changelog.loadFailed')
  } finally {
    if (currentRequest === requestID) loading.value = false
  }
}

function updateQuery() {
  void router.replace({
    query: {
      ...(selectedCategory.value ? { category: selectedCategory.value } : {}),
      ...(searchQuery.value.trim() ? { q: searchQuery.value.trim() } : {})
    }
  })
}

function selectCategory(category: CategoryFilter) {
  if (selectedCategory.value === category) return
  selectedCategory.value = category
  updateQuery()
  void loadEntries(true)
}

function scheduleSearch() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    updateQuery()
    void loadEntries(true)
  }, 350)
}

function loadMore() {
  if (loading.value || !hasMore.value) return
  page.value += 1
  void loadEntries(false)
}

onMounted(() => void loadEntries(true))
onBeforeUnmount(() => {
  requestID += 1
  if (searchTimer) window.clearTimeout(searchTimer)
})
</script>

<style scoped>
.changelog-page {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
  padding: 7.5rem 0 6rem;
}

.changelog-hero h1 {
  margin: 0;
  color: rgb(var(--color-ink-deep));
  font-family: 'Noto Serif SC', 'EB Garamond', serif;
  font-size: clamp(3.4rem, 7vw, 6.2rem);
  font-weight: 600;
  letter-spacing: 0.06em;
  line-height: 1;
}

.changelog-hero p {
  margin: 1.25rem 0 0;
  color: rgb(var(--color-muted));
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-size: clamp(1.15rem, 2vw, 1.45rem);
  letter-spacing: 0.04em;
}

.changelog-rule {
  position: relative;
  height: 2rem;
  margin-top: 1.6rem;
}

.changelog-rule::before,
.changelog-rule::after {
  content: '';
  position: absolute;
  top: 50%;
  width: calc(50% - 2.2rem);
  border-top: 1px solid rgb(var(--color-muted) / 0.8);
}

.changelog-rule::before { left: 0; }
.changelog-rule::after { right: 0; }

.changelog-rule span {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 1.2rem;
  height: 1.2rem;
  border: 1px solid rgb(var(--color-muted));
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

.changelog-rule span::before,
.changelog-rule span::after {
  content: '';
  position: absolute;
  inset: 50% auto auto 50%;
  width: 3rem;
  border-top: 1px solid rgb(var(--color-muted));
  transform: translate(-50%, -50%) rotate(90deg);
}

.changelog-rule span::after { transform: translate(-50%, -50%); }

.changelog-browser { margin-top: 1.4rem; }

.changelog-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 2rem;
  padding-bottom: 1.7rem;
}

.changelog-categories {
  display: flex;
  align-items: center;
  gap: 2.2rem;
}

.changelog-categories button {
  position: relative;
  border: 0;
  background: transparent;
  color: rgb(var(--color-ink));
  font-family: 'Noto Serif SC', 'EB Garamond', serif;
  font-size: 1rem;
  cursor: pointer;
  padding: 0.6rem 0;
}

.changelog-categories button::after {
  content: '';
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 2px;
  background: rgb(var(--color-laurel));
  transform: scaleX(0);
  transition: transform 180ms ease;
}

.changelog-categories button:hover,
.changelog-categories button.is-active { color: rgb(var(--color-laurel)); }
.changelog-categories button.is-active::after { transform: scaleX(1); }

.changelog-search {
  display: flex;
  width: min(100%, 18rem);
  align-items: center;
  gap: 0.65rem;
  border: 1px solid rgb(var(--color-muted) / 0.52);
  padding: 0.72rem 0.9rem;
  color: rgb(var(--color-muted));
}

.changelog-search input {
  min-width: 0;
  flex: 1;
  border: 0;
  outline: none;
  background: transparent;
  color: rgb(var(--color-ink-deep));
  font-size: 0.95rem;
}

.changelog-search:focus-within {
  border-color: rgb(var(--color-laurel));
  box-shadow: 0 0 0 2px rgb(var(--color-laurel) / 0.12);
}

.changelog-month + .changelog-month { margin-top: 4.5rem; }

.changelog-month > h2 {
  margin: 0 0 0.6rem;
  color: rgb(var(--color-ink-deep));
  font-family: 'Noto Serif SC', 'EB Garamond', serif;
  font-size: 1.3rem;
  font-weight: 600;
  letter-spacing: 0.12em;
}

.changelog-timeline {
  margin: 0;
  padding: 0;
  list-style: none;
}

.changelog-timeline > li {
  position: relative;
  display: grid;
  grid-template-columns: 7.2rem 2rem minmax(0, 1fr);
  min-height: 10rem;
}

.changelog-date {
  padding: 1.45rem 1.1rem 1.3rem 0;
  text-align: left;
}

.changelog-date time {
  display: block;
  font-family: 'EB Garamond', serif;
  font-size: 1.5rem;
  letter-spacing: 0.06em;
}

.changelog-new {
  display: inline-block;
  margin-top: 0.55rem;
  border: 1px solid rgb(var(--color-terracotta));
  color: rgb(var(--color-terracotta));
  font-family: 'Cinzel', serif;
  font-size: 0.68rem;
  letter-spacing: 0.12em;
  padding: 0.2rem 0.36rem;
  transform: rotate(-2deg);
}

.changelog-node {
  position: relative;
  border-left: 1px solid rgb(var(--color-muted) / 0.68);
}

.changelog-node::before {
  content: '';
  position: absolute;
  top: 1.72rem;
  left: 0;
  width: 0.72rem;
  height: 0.72rem;
  border: 2px solid rgb(var(--color-papyrus));
  border-radius: 50%;
  background: rgb(var(--color-laurel));
  box-shadow: 0 0 0 1px rgb(var(--color-laurel));
  transform: translateX(-50%);
}

.changelog-timeline article {
  padding: 1.35rem 0 1.65rem 1rem;
  border-bottom: 1px solid rgb(var(--color-laurel) / 0.28);
}

.changelog-entry-heading,
.changelog-entry-heading > div {
  display: flex;
  align-items: center;
  gap: 0.9rem;
}

.changelog-entry-heading { justify-content: space-between; }

.changelog-entry-heading h3 {
  margin: 0;
  font-family: 'Noto Serif SC', 'EB Garamond', serif;
  font-size: 1.45rem;
  font-weight: 600;
}

.changelog-entry-heading h3 a {
  color: rgb(var(--color-ink-deep));
  text-decoration: none;
}

.changelog-entry-heading h3 a:hover { color: rgb(var(--color-laurel)); }

.changelog-category {
  color: rgb(var(--color-laurel));
  font-size: 0.78rem;
  white-space: nowrap;
}

.changelog-detail-link {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: rgb(var(--color-laurel));
  font-family: 'Noto Serif SC', serif;
  font-size: 0.92rem;
  text-decoration: underline;
  text-underline-offset: 0.2rem;
  white-space: nowrap;
}

.changelog-summary,
.changelog-rationale,
.changelog-products {
  margin: 0.7rem 0 0;
  color: rgb(var(--color-muted));
  font-size: 0.96rem;
  line-height: 1.75;
}

.changelog-rationale,
.changelog-products { margin-top: 0.24rem; }
.changelog-rationale strong,
.changelog-products strong { color: rgb(var(--color-ink)); font-weight: 600; }

.changelog-load-more {
  display: flex;
  justify-content: center;
  padding: 2.5rem 0 0;
}

.changelog-load-more button,
.changelog-empty button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  border: 0;
  background: transparent;
  color: rgb(var(--color-laurel));
  font-family: 'Noto Serif SC', serif;
  font-size: 1rem;
  cursor: pointer;
}

.changelog-load-more button:disabled { opacity: 0.55; cursor: wait; }

.changelog-empty {
  border-block: 1px solid rgb(var(--color-muted) / 0.4);
  padding: 5rem 1rem;
  text-align: center;
}

.changelog-empty h3 { margin: 0; font-family: 'Noto Serif SC', serif; font-size: 1.4rem; }
.changelog-empty p { margin: 0.8rem auto 1rem; color: rgb(var(--color-muted)); }

.changelog-skeleton {
  display: grid;
  grid-template-columns: 8rem 1fr;
  gap: 2rem;
  padding: 1.5rem 0;
  border-bottom: 1px solid rgb(var(--color-muted) / 0.2);
}

.changelog-skeleton > span,
.changelog-skeleton i {
  display: block;
  height: 1rem;
  background: rgb(var(--color-muted) / 0.16);
  animation: changelog-pulse 1.5s ease-in-out infinite;
}

.changelog-skeleton > div { display: grid; gap: 0.8rem; }
.changelog-skeleton i:nth-child(1) { width: 45%; height: 1.5rem; }
.changelog-skeleton i:nth-child(2) { width: 82%; }
.changelog-skeleton i:nth-child(3) { width: 64%; }

@keyframes changelog-pulse { 50% { opacity: 0.45; } }

@media (prefers-reduced-motion: reduce) {
  .changelog-skeleton > span,
  .changelog-skeleton i { animation: none; }
  .changelog-categories button::after { transition: none; }
}

@media (max-width: 850px) {
  .changelog-toolbar { align-items: stretch; flex-direction: column; }
  .changelog-categories { overflow-x: auto; gap: 1.5rem; }
  .changelog-categories button { white-space: nowrap; }
  .changelog-search { width: 100%; }
  .changelog-timeline > li { grid-template-columns: 5.2rem 1.4rem minmax(0, 1fr); }
  .changelog-entry-heading { align-items: flex-start; gap: 1rem; }
  .changelog-entry-heading > div { align-items: flex-start; flex-direction: column; gap: 0.25rem; }
}

@media (max-width: 640px) {
  .changelog-page {
    width: min(100% - 2rem, 1200px);
    padding-top: 6.5rem;
  }
  .changelog-hero h1 { font-size: 3.2rem; }
  .changelog-timeline > li {
    grid-template-columns: 1fr;
    padding-left: 1.2rem;
    border-left: 1px solid rgb(var(--color-muted) / 0.68);
  }
  .changelog-date { padding: 1.4rem 0 0.4rem 1rem; }
  .changelog-date time { display: inline; }
  .changelog-new { margin: 0 0 0 0.6rem; }
  .changelog-node { display: none; }
  .changelog-timeline article { padding: 0 0 1.5rem 1rem; }
  .changelog-entry-heading { flex-direction: column; }
  .changelog-detail-link { align-self: flex-start; }
}
</style>
