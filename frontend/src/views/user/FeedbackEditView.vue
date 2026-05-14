<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div v-if="loading" class="card p-6 text-sm text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</div>

      <div v-else-if="!detail" class="card p-6 text-center text-sm text-red-500 dark:text-red-400">
        {{ t('feedback.message.detailFailed') }}
        <RouterLink to="/feedbacks" class="ml-2 underline">{{ t('common.back') }}</RouterLink>
      </div>

      <template v-else>
        <!-- Closed feedback: show hint instead of form -->
        <div v-if="detail.status === 'closed'" class="card space-y-4 p-6">
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('feedback.edit.title') }}</h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('feedback.edit.closedHint') }}</p>
          <div class="flex justify-end">
            <RouterLink to="/feedbacks" class="btn btn-secondary">{{ t('common.back') }}</RouterLink>
          </div>
        </div>

        <!-- Editable form -->
        <div v-else class="card space-y-6 p-6">
          <div class="space-y-2">
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('feedback.edit.title') }}</h1>
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('feedback.edit.description') }}</p>
          </div>

          <form class="space-y-6" @submit.prevent="handleSubmit">
            <div class="grid gap-6 md:grid-cols-2">
              <div>
                <label class="input-label">{{ t('feedback.form.category') }}</label>
                <Select v-model="form.category" :options="categoryOptions" />
              </div>
              <div>
                <label class="input-label">{{ t('feedback.form.contact') }}</label>
                <input v-model="form.contact" class="input" :placeholder="t('feedback.form.contactPlaceholder')" />
              </div>
            </div>

            <div>
              <label class="input-label">{{ t('feedback.form.titleLabel') }}</label>
              <input v-model="form.title" class="input" :maxlength="200" required />
            </div>

            <div>
              <label class="input-label">{{ t('feedback.form.content') }}</label>
              <MarkdownEditorField
                v-model="form.content"
                :placeholder="t('feedback.form.contentPlaceholder')"
                :upload-handler="uploadEditorImages"
                :fallback-message="t('feedback.form.editorFallback')"
                @paste-image-blocked="handlePasteBlocked"
              />
            </div>

            <div>
              <label class="input-label">{{ t('feedback.form.images') }}</label>
              <MultiImageUpload
                v-model="form.images"
                :upload-fn="uploadSingleImage"
                :hint="t('feedback.form.imageHint')"
                :add-button-text="t('feedback.form.addImage')"
                :uploading-text="t('feedback.form.uploading')"
                :remove-text="t('common.delete')"
                :count-template="t('feedback.form.imageCount', { count: '{count}', max: '{max}' })"
              />
            </div>

            <div class="flex justify-end gap-3">
              <RouterLink to="/feedbacks" class="btn btn-secondary">{{ t('common.cancel') }}</RouterLink>
              <button class="btn btn-primary" :disabled="submitting">
                {{ submitting ? t('common.processing') : t('feedback.edit.save') }}
              </button>
            </div>
          </form>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import MarkdownEditorField from '@/components/feedback/MarkdownEditorField.vue'
import MultiImageUpload from '@/components/feedback/MultiImageUpload.vue'
import feedbacksAPI from '@/api/feedbacks'
import { feedbackCategoryOptions } from '@/utils/feedback'
import { useAppStore } from '@/stores'
import type { FeedbackCategory, FeedbackDetail } from '@/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const detail = ref<FeedbackDetail | null>(null)

const form = reactive({
  category: 'bug' as FeedbackCategory,
  title: '',
  content: '',
  images: [] as string[],
  contact: '',
})

const categoryOptions = computed(() =>
  feedbackCategoryOptions.map((category) => ({
    value: category,
    label: t(`feedback.category.${category}`),
  }))
)

async function uploadSingleImage(file: File): Promise<string> {
  return feedbacksAPI.uploadImage(file)
}

async function uploadEditorImages(files: File[]): Promise<string[]> {
  return Promise.all(files.map((file) => uploadSingleImage(file)))
}

function handlePasteBlocked() {
  appStore.showError(t('feedback.message.pasteImageBlocked'))
}

async function loadDetail() {
  loading.value = true
  try {
    const data = await feedbacksAPI.getById(Number(route.params.id))
    detail.value = data
    form.category = data.category
    form.title = data.title
    form.content = data.content
    form.images = data.images ?? []
    form.contact = data.contact ?? ''
  } catch {
    appStore.showError(t('feedback.message.detailFailed'))
  } finally {
    loading.value = false
  }
}

async function handleSubmit() {
  submitting.value = true
  try {
    await feedbacksAPI.update(Number(route.params.id), {
      category: form.category,
      title: form.title,
      content: form.content,
      images: form.images,
      contact: form.contact || undefined,
    })
    appStore.showSuccess(t('feedback.message.updated'))
    await router.push(`/feedbacks/${route.params.id}`)
  } catch (error: any) {
    appStore.showError(error?.message || t('feedback.message.updateFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(loadDetail)
</script>
