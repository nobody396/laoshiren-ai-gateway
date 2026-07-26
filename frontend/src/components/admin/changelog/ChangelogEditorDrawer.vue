<template>
  <Teleport to="body">
    <Transition name="changelog-drawer">
      <div v-if="show" class="changelog-drawer-layer" @click.self="emit('close')">
        <section
          class="changelog-drawer-panel"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
        >
          <header>
            <div>
              <h2 :id="titleId">
                {{ entry ? t('admin.changelog.editEntry') : t('admin.changelog.createEntry') }}
              </h2>
              <p>{{ t('admin.changelog.editorDescription') }}</p>
            </div>
            <button type="button" class="changelog-drawer-close" :aria-label="t('common.close')" @click="emit('close')">
              <Icon name="x" size="md" />
            </button>
          </header>

          <div class="changelog-drawer-body">
            <form id="changelog-editor-form" class="changelog-editor-form" @submit.prevent="submitPrimary">
              <div>
                <label class="input-label" for="changelog-title">{{ t('admin.changelog.form.title') }}</label>
                <input id="changelog-title" v-model="form.title" class="input" maxlength="200" required />
              </div>

              <div>
                <label class="input-label" for="changelog-summary">{{ t('admin.changelog.form.summary') }}</label>
                <input id="changelog-summary" v-model="form.summary" class="input" maxlength="500" required />
                <p class="input-hint">{{ t('admin.changelog.form.summaryHint') }}</p>
              </div>

              <div>
                <label class="input-label" for="changelog-rationale">{{ t('admin.changelog.form.rationale') }}</label>
                <input id="changelog-rationale" v-model="form.rationale" class="input" maxlength="500" required />
              </div>

              <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                <div>
                  <label class="input-label">{{ t('admin.changelog.form.category') }}</label>
                  <Select v-model="form.category" :options="categoryOptions" />
                </div>
                <div>
                  <label class="input-label" for="changelog-published-at">{{ t('admin.changelog.form.publishedAt') }}</label>
                  <input id="changelog-published-at" v-model="form.published_at" type="datetime-local" class="input" />
                </div>
              </div>

              <div>
                <label class="input-label" for="changelog-products">{{ t('admin.changelog.form.relatedProducts') }}</label>
                <input
                  id="changelog-products"
                  v-model="form.related_products"
                  class="input"
                  :placeholder="t('admin.changelog.form.relatedProductsPlaceholder')"
                />
                <p class="input-hint">{{ t('admin.changelog.form.relatedProductsHint') }}</p>
              </div>

              <div>
                <label class="input-label">{{ t('admin.changelog.form.content') }}</label>
                <MarkdownEditorField
                  v-model="form.content"
                  :placeholder="t('admin.changelog.form.contentPlaceholder')"
                  :fallback-message="t('admin.changelog.form.editorFallback')"
                />
              </div>

              <details class="changelog-git-details">
                <summary>{{ t('admin.changelog.form.gitTraceability') }}</summary>
                <p>{{ t('admin.changelog.form.gitTraceabilityHint') }}</p>
                <div>
                  <label class="input-label" for="changelog-slug">{{ t('admin.changelog.form.slug') }}</label>
                  <input
                    id="changelog-slug"
                    v-model="form.slug"
                    class="input font-mono"
                    maxlength="180"
                    :placeholder="t('admin.changelog.form.slugPlaceholder')"
                  />
                </div>
                <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div>
                    <label class="input-label" for="changelog-commit">Commit SHA</label>
                    <input id="changelog-commit" v-model="form.commit_sha" class="input font-mono" maxlength="64" />
                  </div>
                  <div>
                    <label class="input-label" for="changelog-pr">PR URL</label>
                    <input id="changelog-pr" v-model="form.pull_request_url" type="url" class="input" maxlength="500" />
                  </div>
                </div>
              </details>
            </form>

            <aside class="changelog-editor-preview" aria-live="polite">
              <span>{{ t('admin.changelog.publicPreview') }}</span>
              <time>{{ previewDate }}</time>
              <small>{{ categoryLabel }}</small>
              <h3>{{ form.title || t('admin.changelog.previewTitle') }}</h3>
              <p>{{ form.summary || t('admin.changelog.previewSummary') }}</p>
              <dl>
                <dt>{{ t('changelog.whyBuilt') }}</dt>
                <dd>{{ form.rationale || t('admin.changelog.previewRationale') }}</dd>
                <template v-if="parsedRelatedProducts.length">
                  <dt>{{ t('changelog.relatedProducts') }}</dt>
                  <dd>{{ parsedRelatedProducts.join('、') }}</dd>
                </template>
              </dl>
              <div v-if="form.content" class="changelog-preview-markdown">
                <MarkdownPreview :content="form.content" preview-id="changelog-admin-preview" />
              </div>
            </aside>
          </div>

          <footer>
            <button type="button" class="btn btn-secondary" :disabled="saving" @click="emit('close')">
              {{ t('common.cancel') }}
            </button>
            <button
              v-if="!entry || entry.status === 'draft'"
              type="button"
              class="btn btn-secondary"
              :disabled="saving"
              @click="submit('draft')"
            >
              {{ t('admin.changelog.saveDraft') }}
            </button>
            <button
              type="submit"
              form="changelog-editor-form"
              class="btn btn-primary"
              :disabled="saving"
            >
              <Icon name="edit" size="sm" />
              {{ primaryLabel }}
            </button>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import MarkdownEditorField from '@/components/feedback/MarkdownEditorField.vue'
import MarkdownPreview from '@/components/feedback/MarkdownPreview.vue'
import type {
  AdminChangelogEntry,
  ChangelogCategory,
  ChangelogStatus,
  CreateChangelogRequest
} from '@/types'

const props = defineProps<{
  show: boolean
  entry: AdminChangelogEntry | null
  saving: boolean
}>()

const emit = defineEmits<{
  close: []
  save: [payload: CreateChangelogRequest]
}>()

const { t, locale } = useI18n()
const titleId = `changelog-editor-${Math.random().toString(36).slice(2)}`

const form = reactive({
  slug: '',
  title: '',
  summary: '',
  rationale: '',
  content: '',
  category: 'feature' as ChangelogCategory,
  related_products: '',
  published_at: '',
  commit_sha: '',
  pull_request_url: ''
})

const categoryOptions = computed(() => [
  { value: 'feature', label: t('changelog.categories.feature') },
  { value: 'model_config', label: t('changelog.categories.modelConfig') },
  { value: 'improvement', label: t('changelog.categories.improvement') },
  { value: 'fix', label: t('changelog.categories.fix') }
])

const parsedRelatedProducts = computed(() =>
  [...new Set(form.related_products.split(/[,，]/).map((item) => item.trim()).filter(Boolean))]
)
const categoryLabel = computed(() =>
  categoryOptions.value.find((item) => item.value === form.category)?.label ?? ''
)
const previewDate = computed(() => {
  const date = form.published_at ? new Date(form.published_at) : new Date()
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-US' : 'zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  }).format(date)
})
const primaryLabel = computed(() => {
  if (props.saving) return t('common.saving')
  if (props.entry?.status === 'published') return t('admin.changelog.updatePublished')
  if (props.entry?.status === 'archived') return t('admin.changelog.republish')
  return t('admin.changelog.publish')
})

function toLocalDateTime(value?: string | null): string {
  const date = value ? new Date(value) : new Date()
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

function resetForm() {
  const entry = props.entry
  form.slug = entry?.slug ?? ''
  form.title = entry?.title ?? ''
  form.summary = entry?.summary ?? ''
  form.rationale = entry?.rationale ?? ''
  form.content = entry?.content ?? ''
  form.category = entry?.category ?? 'feature'
  form.related_products = entry?.related_products.join('，') ?? ''
  form.published_at = entry?.published_at ? toLocalDateTime(entry.published_at) : ''
  form.commit_sha = entry?.commit_sha ?? ''
  form.pull_request_url = entry?.pull_request_url ?? ''
}

function submit(status: ChangelogStatus) {
  const payload: CreateChangelogRequest = {
    ...(form.slug.trim() ? { slug: form.slug.trim() } : {}),
    title: form.title.trim(),
    summary: form.summary.trim(),
    rationale: form.rationale.trim(),
    content: form.content.trim(),
    category: form.category,
    related_products: parsedRelatedProducts.value,
    status,
    ...(form.published_at ? { published_at: new Date(form.published_at).toISOString() } : {}),
    commit_sha: form.commit_sha.trim(),
    pull_request_url: form.pull_request_url.trim()
  }
  emit('save', payload)
}

function submitPrimary() {
  submit('published')
}

watch(() => [props.show, props.entry?.id], ([show]) => {
  if (show) resetForm()
}, { immediate: true })
</script>

<style scoped>
.changelog-drawer-layer {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  justify-content: flex-end;
  background: rgba(15, 23, 42, 0.42);
  backdrop-filter: blur(2px);
}

.changelog-drawer-panel {
  display: flex;
  width: min(1100px, 86vw);
  height: 100%;
  flex-direction: column;
  background: #fff;
  box-shadow: -20px 0 60px rgba(15, 23, 42, 0.16);
}

.dark .changelog-drawer-panel { background: #111827; }

.changelog-drawer-panel > header,
.changelog-drawer-panel > footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-color: #e5e7eb;
  padding: 1rem 1.5rem;
}
.changelog-drawer-panel > header { border-bottom: 1px solid #e5e7eb; }
.changelog-drawer-panel > footer { justify-content: flex-end; border-top: 1px solid #e5e7eb; }
.dark .changelog-drawer-panel > header,
.dark .changelog-drawer-panel > footer { border-color: #334155; }

.changelog-drawer-panel h2 { margin: 0; color: #111827; font-size: 1.15rem; font-weight: 700; }
.changelog-drawer-panel header p { margin: 0.2rem 0 0; color: #6b7280; font-size: 0.78rem; }
.dark .changelog-drawer-panel h2 { color: #f9fafb; }

.changelog-drawer-close {
  display: grid;
  width: 2.25rem;
  height: 2.25rem;
  place-items: center;
  border: 0;
  border-radius: 0.5rem;
  background: transparent;
  color: #6b7280;
}
.changelog-drawer-close:hover { background: #f3f4f6; color: #111827; }

.changelog-drawer-body {
  display: grid;
  min-height: 0;
  flex: 1;
  grid-template-columns: minmax(0, 2.2fr) minmax(250px, 0.8fr);
  overflow: hidden;
}

.changelog-editor-form {
  display: grid;
  align-content: start;
  gap: 1.15rem;
  overflow-y: auto;
  padding: 1.5rem;
}

.changelog-editor-preview {
  overflow-y: auto;
  border-left: 1px solid #e5e7eb;
  background: #fafafa;
  padding: 1.5rem;
}
.dark .changelog-editor-preview { border-color: #334155; background: #0f172a; }

.changelog-editor-preview > span {
  display: block;
  color: #6b7280;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.changelog-editor-preview time { display: block; margin-top: 1.5rem; color: #8a7d63; font-family: 'EB Garamond', serif; }
.changelog-editor-preview small { display: block; margin-top: 0.35rem; color: #3f5a3a; }
.changelog-editor-preview h3 { margin: 0.75rem 0 0; color: #111827; font-family: 'Noto Serif SC', serif; font-size: 1.35rem; line-height: 1.4; }
.changelog-editor-preview > p { color: #4b5563; font-size: 0.9rem; line-height: 1.65; }
.changelog-editor-preview dl { margin-top: 1.5rem; border-left: 2px solid #3f5a3a; padding-left: 0.8rem; }
.changelog-editor-preview dt { color: #374151; font-size: 0.75rem; font-weight: 700; }
.changelog-editor-preview dd { margin: 0.25rem 0 1rem; color: #6b7280; font-size: 0.84rem; line-height: 1.55; }
.dark .changelog-editor-preview h3 { color: #f9fafb; }
.dark .changelog-editor-preview > p,
.dark .changelog-editor-preview dd { color: #cbd5e1; }

.changelog-preview-markdown {
  margin-top: 1.5rem;
  border-top: 1px solid #e5e7eb;
  padding-top: 1rem;
}
.changelog-preview-markdown :deep(.md-editor-preview-wrapper) { padding: 0; background: transparent; }
.changelog-preview-markdown :deep(.md-editor-preview) { font-size: 0.82rem; }

.changelog-git-details {
  border: 1px solid #e5e7eb;
  border-radius: 0.75rem;
  padding: 0.9rem;
}
.dark .changelog-git-details { border-color: #334155; }
.changelog-git-details summary { color: #374151; font-size: 0.86rem; font-weight: 600; cursor: pointer; }
.dark .changelog-git-details summary { color: #e5e7eb; }
.changelog-git-details > p { margin: 0.55rem 0 1rem; color: #6b7280; font-size: 0.75rem; }
.changelog-git-details > div + div { margin-top: 1rem; }

.changelog-drawer-enter-active,
.changelog-drawer-leave-active { transition: opacity 180ms ease; }
.changelog-drawer-enter-active .changelog-drawer-panel,
.changelog-drawer-leave-active .changelog-drawer-panel { transition: transform 180ms ease; }
.changelog-drawer-enter-from,
.changelog-drawer-leave-to { opacity: 0; }
.changelog-drawer-enter-from .changelog-drawer-panel,
.changelog-drawer-leave-to .changelog-drawer-panel { transform: translateX(100%); }

@media (prefers-reduced-motion: reduce) {
  .changelog-drawer-enter-active,
  .changelog-drawer-leave-active,
  .changelog-drawer-enter-active .changelog-drawer-panel,
  .changelog-drawer-leave-active .changelog-drawer-panel { transition: none; }
}

@media (max-width: 900px) {
  .changelog-drawer-panel { width: 100vw; }
  .changelog-drawer-body { grid-template-columns: 1fr; overflow-y: auto; }
  .changelog-editor-form { overflow: visible; }
  .changelog-editor-preview { border-top: 1px solid #e5e7eb; border-left: 0; }
}
</style>
