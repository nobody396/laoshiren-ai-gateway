<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div v-if="loading" class="card p-6 text-sm text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</div>

      <template v-else-if="detail">
        <div class="grid gap-6 xl:grid-cols-[1fr_320px]">
          <div class="space-y-6">
            <div class="card space-y-5 p-6">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="space-y-2">
                  <div class="flex flex-wrap items-center gap-2">
                    <StatusBadge :label="t(`feedback.category.${detail.category}`)" tone="gray" />
                    <StatusBadge :label="t(`feedback.status.${detail.status}`)" :tone="feedbackStatusTone(detail.status)" />
                    <StatusBadge :label="t(`feedback.priority.${detail.priority}`)" :tone="feedbackPriorityTone(detail.priority)" />
                  </div>
                  <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ detail.title }}</h1>
                  <p class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(detail.created_at) }}</p>
                </div>
                <RouterLink to="/admin/feedbacks" class="btn btn-secondary">{{ t('common.back') }}</RouterLink>
              </div>

              <MarkdownPreview :content="detail.content" :preview-id="`admin-feedback-${detail.id}`" />
			  <div v-if="detail.request_id" class="rounded-2xl bg-gray-50 p-4 text-sm dark:bg-dark-800"><span class="text-gray-500">Request ID:</span> <code>{{ detail.request_id }}</code></div>

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
			  <div v-if="detail.events?.length" class="space-y-3">
				<div v-for="event in detail.events" :key="`event-${event.id}`" class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-800">
				  <div class="flex items-center justify-between gap-3"><span class="text-sm font-medium text-gray-900 dark:text-white">{{ event.summary }}</span><span class="text-xs text-gray-500">{{ formatDateTime(event.created_at) }}</span></div>
				</div>
			  </div>
              <div class="space-y-4">
                <div
                  v-for="reply in (detail.replies || [])"
                  :key="reply.id"
                  class="rounded-2xl border p-4"
                  :class="reply.role === 'admin' ? 'border-primary-200 bg-primary-50/60 dark:border-primary-900 dark:bg-primary-900/10' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900'"
                >
                  <div class="mb-3 flex items-center justify-between gap-3">
                    <div class="flex items-center gap-2">
                      <StatusBadge :label="reply.role === 'admin' ? t('feedback.reply.admin') : t('feedback.reply.user')" :tone="reply.role === 'admin' ? 'info' : 'gray'" />
                      <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(reply.created_at) }}</span>
                    </div>
                    <span class="text-sm text-gray-500 dark:text-dark-400">{{ reply.user?.username || reply.user?.email || '-' }}</span>
                  </div>

                  <MarkdownPreview :content="reply.content" :preview-id="`admin-reply-${reply.id}`" />

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
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('feedback.admin.replyTitle') }}</h2>
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
                <button class="btn btn-primary" :disabled="submittingReply" @click="submitReply">
                  {{ submittingReply ? t('common.processing') : t('feedback.reply.submit') }}
                </button>
              </div>
            </div>
          </div>

          <div class="space-y-6">
            <div class="card space-y-4 p-6">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('feedback.admin.userInfo') }}</h2>
              <div class="space-y-2 text-sm text-gray-600 dark:text-dark-300">
                <div>{{ t('common.user') }}: {{ detail.user?.username || '-' }}</div>
                <div>{{ t('common.email') }}: {{ detail.user?.email || '-' }}</div>
                <div v-if="detail.contact">{{ t('feedback.form.contact') }}: {{ detail.contact }}</div>
              </div>
            </div>

			<div class="card space-y-3 p-6">
			  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('feedback.admin.workflow') }}</h2>
			  <div class="space-y-2 text-sm text-gray-600 dark:text-dark-300">
				<div>{{ t('feedback.admin.triageLabel') }}：{{ detail.triage_status }} <span v-if="detail.triage_priority">· {{ detail.triage_priority }}</span></div>
				<div>{{ t('feedback.admin.decisionLabel') }}：{{ detail.owner_decision }}</div>
				<div>{{ t('feedback.admin.fixLabel') }}：{{ detail.fix_status }}</div>
				<div v-if="detail.triage_summary" class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800">{{ detail.triage_summary }}</div>
				<div v-if="detail.reward" class="rounded-xl bg-emerald-50 p-3 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300">{{ t('feedback.admin.rewardCredit', { amount: detail.reward.amount.toFixed(2), ledger: detail.reward.account_change_record_id || '-' }) }}</div>
			  </div>
			</div>

            <div class="card space-y-4 p-6">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('feedback.admin.manage') }}</h2>
              <div class="space-y-4">
                <div>
                  <label class="input-label">{{ t('feedback.columns.status') }}</label>
                  <Select v-model="editStatus" :options="statusOptions" />
                </div>
                <div>
                  <label class="input-label">{{ t('feedback.columns.priority') }}</label>
                  <Select v-model="editPriority" :options="priorityOptions" />
                </div>
                <button class="btn btn-secondary w-full" @click="saveMeta">{{ t('common.save') }}</button>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import type { FeedbackDetail, FeedbackPriority, FeedbackStatus } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import Select from '@/components/common/Select.vue'
import MarkdownPreview from '@/components/feedback/MarkdownPreview.vue'
import MarkdownEditorField from '@/components/feedback/MarkdownEditorField.vue'
import MultiImageUpload from '@/components/feedback/MultiImageUpload.vue'
import adminFeedbacksAPI from '@/api/admin/feedbacks'
import feedbacksAPI from '@/api/feedbacks'
import { formatDateTime } from '@/utils/format'
import { feedbackPriorityOptions, feedbackPriorityTone, feedbackStatusOptions, feedbackStatusTone } from '@/utils/feedback'
import { useAppStore } from '@/stores'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const submittingReply = ref(false)
const detail = ref<FeedbackDetail | null>(null)
const replyContent = ref('')
const replyImages = ref<string[]>([])
const editStatus = ref<FeedbackStatus>('pending')
const editPriority = ref<FeedbackPriority>('normal')

const statusOptions = computed(() =>
  feedbackStatusOptions
    .filter((item): item is FeedbackStatus => item !== '')
    .map((item) => ({ value: item, label: t(`feedback.status.${item}`) }))
)

const priorityOptions = computed(() =>
  feedbackPriorityOptions
    .filter((item): item is FeedbackPriority => item !== '')
    .map((item) => ({ value: item, label: t(`feedback.priority.${item}`) }))
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
    const loaded = await adminFeedbacksAPI.getById(Number(route.params.id))
    detail.value = loaded
    editStatus.value = loaded.status
    editPriority.value = loaded.priority
  } catch {
    appStore.showError(t('feedback.message.detailFailed'))
  } finally {
    loading.value = false
  }
}

async function saveMeta() {
  if (!detail.value) return
  try {
    await Promise.all([
      adminFeedbacksAPI.updateStatus(detail.value.id, editStatus.value),
      adminFeedbacksAPI.updatePriority(detail.value.id, editPriority.value),
    ])
    appStore.showSuccess(t('feedback.message.updated'))
    await loadDetail()
  } catch {
    appStore.showError(t('feedback.message.updateFailed'))
  }
}

async function submitReply() {
  if (!replyContent.value.trim()) {
    appStore.showError(t('feedback.reply.empty'))
    return
  }
  submittingReply.value = true
  try {
    await adminFeedbacksAPI.createReply(Number(route.params.id), {
      content: replyContent.value,
      images: replyImages.value,
    })
    appStore.showSuccess(t('feedback.message.replyCreated'))
    replyContent.value = ''
    replyImages.value = []
    await loadDetail()
  } catch {
    appStore.showError(t('feedback.message.replyFailed'))
  } finally {
    submittingReply.value = false
  }
}

onMounted(loadDetail)
</script>
