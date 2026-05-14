<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div v-if="loading" class="card p-6 text-sm text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</div>

      <div v-else-if="!detail" class="card p-6 text-center text-sm text-red-500 dark:text-red-400">
        {{ t('feedback.message.detailFailed') }}
        <RouterLink to="/feedbacks" class="ml-2 underline">{{ t('common.back') }}</RouterLink>
      </div>

      <template v-else>
        <div class="card space-y-5 p-6">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="space-y-2">
              <div class="flex flex-wrap items-center gap-2">
                <StatusBadge :label="t(`feedback.category.${detail.category}`)" tone="gray" />
                <StatusBadge :label="t(`feedback.status.${detail.status}`)" :tone="feedbackStatusTone(detail.status)" />
                <StatusBadge :label="t(`feedback.priority.${detail.priority}`)" :tone="feedbackPriorityTone(detail.priority)" />
              </div>
              <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ detail.title }}</h1>
              <p class="text-sm text-gray-500 dark:text-dark-400">
                {{ t('feedback.detail.meta', { createdAt: formatDateTime(detail.created_at), replies: detail.reply_count }) }}
              </p>
            </div>
            <RouterLink to="/feedbacks" class="btn btn-secondary">{{ t('common.back') }}</RouterLink>
            <RouterLink
              v-if="detail.status !== 'closed'"
              :to="`/feedbacks/${detail.id}/edit`"
              class="btn btn-primary"
            >
              {{ t('feedback.edit.button') }}
            </RouterLink>
          </div>

          <div v-if="detail.contact" class="rounded-2xl bg-gray-50 px-4 py-3 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-300">
            {{ t('feedback.form.contact') }}: {{ detail.contact }}
          </div>

          <MarkdownPreview :content="detail.content" :preview-id="`feedback-${detail.id}`" />

          <div v-if="detail.images?.length" class="grid grid-cols-2 gap-3 md:grid-cols-4">
            <img
              v-for="(image, index) in (detail.images || [])"
              :key="`${image}-${index}`"
              :src="image"
              alt=""
              class="rounded-2xl border border-gray-200 object-cover dark:border-dark-700"
            />
          </div>
        </div>

        <div class="card space-y-4 p-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('feedback.detail.timeline') }}</h2>
          <div class="space-y-4">
            <div
              v-for="reply in detail.replies"
              :key="reply.id"
              class="rounded-2xl border p-4"
              :class="reply.role === 'admin' ? 'border-primary-200 bg-primary-50/60 dark:border-primary-900 dark:bg-primary-900/10' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900'"
            >
              <div class="mb-3 flex items-center justify-between gap-3">
                <div class="flex items-center gap-2">
                  <StatusBadge
                    :label="reply.role === 'admin' ? t('feedback.reply.admin') : t('feedback.reply.user')"
                    :tone="reply.role === 'admin' ? 'info' : 'gray'"
                  />
                  <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(reply.created_at) }}</span>
                </div>
                <span v-if="reply.user?.username" class="text-sm text-gray-500 dark:text-dark-400">{{ reply.user.username }}</span>
              </div>

              <MarkdownPreview :content="reply.content" :preview-id="`reply-${reply.id}`" />

              <div v-if="reply.images?.length" class="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4">
                <img
                  v-for="(image, index) in (reply.images || [])"
                  :key="`${image}-${index}`"
                  :src="image"
                  alt=""
                  class="rounded-2xl border border-gray-200 object-cover dark:border-dark-700"
                />
              </div>
            </div>
          </div>
        </div>

        <div v-if="detail.status !== 'closed'" class="card space-y-5 p-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('feedback.detail.addReply') }}</h2>
          <MarkdownEditorField
            v-model="replyContent"
            :placeholder="t('feedback.reply.placeholder')"
            :upload-handler="uploadEditorImages"
            :fallback-message="t('feedback.form.editorFallback')"
            @paste-image-blocked="handlePasteBlocked"
          />
          <MultiImageUpload
            v-model="replyImages"
            :upload-fn="uploadSingleImage"
            :hint="t('feedback.form.imageHint')"
            :add-button-text="t('feedback.form.addImage')"
            :uploading-text="t('feedback.form.uploading')"
            :remove-text="t('common.delete')"
            :count-template="t('feedback.form.imageCount', { count: '{count}', max: '{max}' })"
          />
          <div class="flex justify-end">
            <button class="btn btn-primary" :disabled="replySubmitting" @click="submitReply">
              {{ replySubmitting ? t('common.processing') : t('feedback.reply.submit') }}
            </button>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import MarkdownPreview from '@/components/feedback/MarkdownPreview.vue'
import MarkdownEditorField from '@/components/feedback/MarkdownEditorField.vue'
import MultiImageUpload from '@/components/feedback/MultiImageUpload.vue'
import feedbacksAPI from '@/api/feedbacks'
import { formatDateTime } from '@/utils/format'
import { feedbackPriorityTone, feedbackStatusTone } from '@/utils/feedback'
import type { FeedbackDetail } from '@/types'
import { useAppStore } from '@/stores'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const replySubmitting = ref(false)
const detail = ref<FeedbackDetail | null>(null)
const replyContent = ref('')
const replyImages = ref<string[]>([])

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
    detail.value = await feedbacksAPI.getById(Number(route.params.id))
  } catch {
    appStore.showError(t('feedback.message.detailFailed'))
  } finally {
    loading.value = false
  }
}

async function submitReply() {
  if (!replyContent.value.trim()) {
    appStore.showError(t('feedback.reply.empty'))
    return
  }
  replySubmitting.value = true
  try {
    await feedbacksAPI.createReply(Number(route.params.id), {
      content: replyContent.value,
      images: replyImages.value,
    })
    appStore.showSuccess(t('feedback.message.replyCreated'))
    replyContent.value = ''
    replyImages.value = []
    await loadDetail()
  } catch (error: any) {
    appStore.showError(error?.message || t('feedback.message.replyFailed'))
  } finally {
    replySubmitting.value = false
  }
}

onMounted(loadDetail)
</script>
