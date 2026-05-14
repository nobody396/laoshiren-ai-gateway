<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <button
            v-for="status in statuses"
            :key="status.value || 'all'"
            type="button"
            class="rounded-full px-4 py-2 text-sm transition"
            :class="activeStatus === status.value ? 'bg-primary-600 text-white' : 'bg-white text-gray-600 dark:bg-dark-800 dark:text-dark-300'"
            @click="changeStatus(status.value)"
          >
            {{ status.label }}
          </button>
        </div>
      </template>

      <template #actions>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" :disabled="loading" @click="loadFeedbacks">
            {{ t('common.refresh') }}
          </button>
          <RouterLink to="/feedbacks/new" class="btn btn-primary">
            {{ t('feedback.form.submit') }}
          </RouterLink>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-category="{ value }">
            <StatusBadge :label="t(`feedback.category.${value}`)" tone="gray" />
          </template>

          <template #cell-status="{ value }">
            <StatusBadge :label="t(`feedback.status.${value}`)" :tone="feedbackStatusTone(value)" />
          </template>

          <template #cell-priority="{ value }">
            <StatusBadge :label="t(`feedback.priority.${value}`)" :tone="feedbackPriorityTone(value)" />
          </template>

          <template #cell-title="{ row }">
            <div class="space-y-1">
              <div class="font-medium text-gray-900 dark:text-white">{{ row.title }}</div>
              <div class="text-xs text-gray-500 dark:text-dark-400">#{{ row.id }}</div>
            </div>
          </template>

          <template #cell-last_reply_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ value ? formatDateTime(value) : '-' }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex gap-2">
              <RouterLink :to="`/feedbacks/${row.id}`" class="btn btn-secondary btn-sm">
                {{ t('feedback.detail.view') }}
              </RouterLink>
              <RouterLink
                v-if="row.status !== 'closed'"
                :to="`/feedbacks/${row.id}/edit`"
                class="btn btn-secondary btn-sm"
              >
                {{ t('feedback.edit.button') }}
              </RouterLink>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('feedback.empty.title')"
              :description="t('feedback.empty.description')"
              :action-text="t('feedback.form.submit')"
              action-to="/feedbacks/new"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Column } from '@/components/common/types'
import type { FeedbackItem, FeedbackStatus } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import feedbacksAPI from '@/api/feedbacks'
import { formatDateTime } from '@/utils/format'
import { feedbackStatusOptions, feedbackPriorityTone, feedbackStatusTone } from '@/utils/feedback'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const items = ref<FeedbackItem[]>([])
const activeStatus = ref<FeedbackStatus | ''>('')
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 1,
})

const statuses = computed(() =>
  feedbackStatusOptions.map((status) => ({
    value: status,
    label: status ? t(`feedback.status.${status}`) : t('common.all'),
  }))
)

const columns = computed<Column[]>(() => [
  { key: 'category', label: t('feedback.columns.category') },
  { key: 'title', label: t('feedback.columns.title') },
  { key: 'status', label: t('feedback.columns.status') },
  { key: 'priority', label: t('feedback.columns.priority') },
  { key: 'reply_count', label: t('feedback.columns.replyCount') },
  { key: 'last_reply_at', label: t('feedback.columns.lastReplyAt') },
  { key: 'created_at', label: t('feedback.columns.createdAt') },
  { key: 'actions', label: t('common.actions') },
])

async function loadFeedbacks() {
  loading.value = true
  try {
    const response = await feedbacksAPI.list({
      page: pagination.value.page,
      pageSize: pagination.value.page_size,
      status: activeStatus.value || undefined,
    })
    items.value = response.items
    pagination.value = {
      page: response.page,
      page_size: response.page_size,
      total: response.total,
      pages: response.pages,
    }
  } catch {
    appStore.showError(t('feedback.message.loadFailed'))
  } finally {
    loading.value = false
  }
}

function changeStatus(status: FeedbackStatus | '') {
  activeStatus.value = status
  pagination.value.page = 1
  loadFeedbacks()
}

function handlePageChange(page: number) {
  pagination.value.page = page
  loadFeedbacks()
}

function handlePageSizeChange(pageSize: number) {
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  loadFeedbacks()
}

onMounted(loadFeedbacks)
</script>
