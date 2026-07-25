<template>
  <Teleport to="body">
    <Transition name="popup-fade">
      <div
        v-if="popupBatch && currentPopup"
        data-testid="announcement-popup"
        class="fixed inset-0 z-[120] flex items-start justify-center overflow-y-auto bg-gradient-to-br from-black/70 via-black/60 to-black/70 p-4 pt-[8vh] backdrop-blur-md"
      >
        <div
          class="w-full max-w-[680px] overflow-hidden rounded-3xl bg-white shadow-2xl ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
          @click.stop
        >
          <div class="relative overflow-hidden border-b border-amber-100/80 bg-gradient-to-br from-amber-50/80 via-orange-50/50 to-yellow-50/30 px-8 py-6 dark:border-dark-700/50 dark:from-amber-900/20 dark:via-orange-900/10 dark:to-yellow-900/5">
            <div class="absolute right-0 top-0 h-full w-64 bg-gradient-to-l from-orange-100/30 to-transparent dark:from-orange-900/20"></div>
            <div class="absolute -right-8 -top-8 h-32 w-32 rounded-full bg-gradient-to-br from-amber-400/20 to-orange-500/20 blur-3xl"></div>

            <button
              type="button"
              class="absolute right-5 top-5 z-20 flex h-9 w-9 items-center justify-center rounded-xl bg-white/60 text-gray-500 shadow-sm backdrop-blur-sm transition hover:bg-white hover:text-gray-800 dark:bg-dark-700/60 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-white"
              :aria-label="t('common.close')"
              @click="announcementStore.dismissPopup"
            >
              <span class="text-xl leading-none">×</span>
            </button>

            <div class="relative z-10 pr-10">
              <div class="mb-3 flex items-center gap-2">
                <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500 to-orange-600 text-white shadow-lg shadow-amber-500/30">
                  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                  </svg>
                </div>
                <span class="inline-flex items-center rounded-lg bg-gradient-to-r from-amber-500 to-orange-600 px-2.5 py-1 text-xs font-medium text-white shadow-lg shadow-amber-500/30">
                  {{ t('announcements.popupNotice') }}
                </span>
              </div>

              <h2 class="mb-2 text-2xl font-bold leading-tight text-gray-900 dark:text-white">
                {{ isBatch
                  ? t('announcements.newCount', { count: popupBatch.total })
                  : currentPopup.title
                }}
              </h2>
              <p v-if="isBatch" class="text-sm text-gray-600 dark:text-gray-400">
                {{ t('announcements.batchHint') }}
              </p>
              <div v-else class="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-400">
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <time>{{ formatRelativeWithDateTime(currentPopup.created_at) }}</time>
              </div>
            </div>
          </div>

          <div class="max-h-[52vh] overflow-y-auto bg-white px-8 py-7 dark:bg-dark-800">
            <div v-if="isBatch" class="space-y-3" data-testid="announcement-popup-batch">
              <div
                v-for="(announcement, index) in popupBatch.announcements"
                :key="announcement.id"
                class="flex items-start gap-4 rounded-2xl border border-amber-100 bg-amber-50/40 px-5 py-4 dark:border-amber-900/30 dark:bg-amber-900/10"
              >
                <span class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-amber-500 text-xs font-bold text-white">
                  {{ index + 1 }}
                </span>
                <div class="min-w-0 flex-1">
                  <h3 class="text-sm font-semibold leading-6 text-gray-900 dark:text-white">
                    {{ announcement.title }}
                  </h3>
                  <time class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                    {{ formatRelativeWithDateTime(announcement.created_at) }}
                  </time>
                </div>
              </div>

              <div
                v-if="remainingCount > 0"
                class="rounded-2xl border border-dashed border-gray-300 px-5 py-4 text-center text-sm text-gray-600 dark:border-dark-600 dark:text-gray-400"
              >
                {{ t('announcements.moreInCenter', { count: remainingCount }) }}
              </div>
            </div>

            <div v-else class="relative">
              <div class="absolute bottom-0 left-0 top-0 w-1 rounded-full bg-gradient-to-b from-amber-500 via-orange-500 to-yellow-500"></div>
              <div class="pl-6">
                <div
                  class="markdown-body prose prose-sm max-w-none dark:prose-invert"
                  v-html="renderedContent"
                ></div>
              </div>
            </div>
          </div>

          <div class="border-t border-gray-100 bg-gray-50/50 px-8 py-5 dark:border-dark-700 dark:bg-dark-900/30">
            <div v-if="isBatch" class="flex flex-col-reverse justify-end gap-3 sm:flex-row">
              <button
                type="button"
                class="rounded-xl border border-gray-300 bg-white px-5 py-2.5 text-sm font-medium text-gray-700 shadow-sm transition hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
                @click="announcementStore.dismissPopup"
              >
                {{ t('announcements.viewLater') }}
              </button>
              <button
                type="button"
                class="rounded-xl bg-gradient-to-r from-amber-500 to-orange-600 px-6 py-2.5 text-sm font-medium text-white shadow-lg shadow-amber-500/30 transition hover:scale-[1.02] hover:shadow-xl"
                @click="handleViewAll"
              >
                {{ t('announcements.viewAllWithCount', { count: popupBatch.total }) }}
              </button>
            </div>

            <div v-else class="flex justify-end">
              <button
                type="button"
                class="rounded-xl bg-gradient-to-r from-amber-500 to-orange-600 px-6 py-2.5 text-sm font-medium text-white shadow-lg shadow-amber-500/30 transition hover:scale-[1.02] hover:shadow-xl"
                @click="handleAcknowledge"
              >
                {{ t('announcements.acknowledge') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useBodyScrollLock } from '@/composables/useBodyScrollLock'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeWithDateTime } from '@/utils/format'

const { t } = useI18n()
const announcementStore = useAnnouncementStore()
const { popupBatch, currentPopup } = storeToRefs(announcementStore)

marked.setOptions({
  breaks: true,
  gfm: true,
})

const isBatch = computed(() => (popupBatch.value?.total ?? 0) > 1)
const remainingCount = computed(() =>
  Math.max(0, (popupBatch.value?.total ?? 0) - (popupBatch.value?.announcements.length ?? 0))
)
const renderedContent = computed(() => {
  const content = currentPopup.value?.content
  if (!content) return ''
  return DOMPurify.sanitize(marked.parse(content) as string)
})

function handleViewAll() {
  announcementStore.dismissPopup()
  announcementStore.openCenter()
}

async function handleAcknowledge() {
  try {
    await announcementStore.acknowledgeCurrentPopup()
  } catch (err) {
    console.error('Failed to acknowledge announcement:', err)
    announcementStore.dismissPopup()
  }
}

useBodyScrollLock(computed(() => Boolean(popupBatch.value)))
</script>

<style scoped>
.popup-fade-enter-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.popup-fade-leave-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 1, 1);
}

.popup-fade-enter-from,
.popup-fade-leave-to {
  opacity: 0;
}

.popup-fade-enter-from > div {
  transform: scale(0.94) translateY(-12px);
  opacity: 0;
}

.popup-fade-leave-to > div {
  transform: scale(0.96) translateY(-8px);
  opacity: 0;
}
</style>
