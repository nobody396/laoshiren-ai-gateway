<template>
  <!-- 客服入口按钮：点击弹出客服联系方式弹窗 -->
  <div v-if="hasCustomerServiceContact">
    <button
      @click="openModal"
      class="relative flex items-center gap-1 rounded-lg px-2 py-1.5 text-xs font-medium text-gray-600 transition-all hover:scale-105 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-800 dark:hover:text-white"
      :aria-label="t('common.customerService')"
      :title="t('common.customerService')"
    >
      <Icon name="headphones" size="sm" />
      <span class="hidden sm:inline">{{ t('common.customerService') }}</span>
    </button>

    <!-- 客服二维码弹窗 -->
    <Teleport to="body">
      <Transition name="cs-modal-fade">
        <div
          v-if="isOpen"
          class="fixed inset-0 z-[100] flex items-start justify-center overflow-y-auto bg-gradient-to-br from-black/70 via-black/60 to-black/70 p-4 pt-[10vh] backdrop-blur-md"
          @click="closeModal"
        >
          <div
            class="w-full max-w-[560px] overflow-hidden rounded-3xl bg-white shadow-2xl ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
            @click.stop
          >
            <!-- 头部 -->
            <div
              class="relative overflow-hidden border-b border-gray-100/80 bg-gradient-to-br from-blue-50/50 to-indigo-50/30 px-6 py-5 dark:border-dark-700/50 dark:from-blue-900/10 dark:to-indigo-900/5"
            >
              <div class="relative z-10 flex items-start justify-between">
                <div class="flex items-center gap-2">
                  <div
                    class="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-blue-500 to-indigo-600 text-white shadow-lg shadow-blue-500/30"
                  >
                    <Icon name="headphones" size="sm" />
                  </div>
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ t('common.customerService') }}
                  </h2>
                </div>
                <button
                  @click="closeModal"
                  class="flex h-9 w-9 items-center justify-center rounded-lg bg-white/50 text-gray-500 backdrop-blur-sm transition-all hover:bg-white hover:text-gray-700 dark:bg-dark-700/50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-300"
                  :aria-label="t('common.close')"
                >
                  <Icon name="x" size="sm" />
                </button>
              </div>
            </div>

            <!-- 内容：仅配置微信号时显示单个客服入口；配置二维码时区分售后/技术客服 -->
            <div v-if="showSingleContact" class="p-6">
              <div
                class="flex flex-col items-center rounded-2xl border border-gray-100 bg-gray-50/50 p-6 text-center dark:border-dark-700 dark:bg-dark-900/30"
              >
                <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
                  <Icon name="headphones" size="lg" />
                </div>
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('common.wechatId') }}
                </span>
                <span class="mt-2 break-all text-2xl font-semibold text-gray-900 dark:text-white">
                  {{ supportContact }}
                </span>
              </div>
            </div>

            <div v-else class="grid gap-6 p-6 sm:grid-cols-2">
              <!-- 售后客服 -->
              <div
                v-if="showAfterSalesContact"
                class="flex flex-col items-center rounded-2xl border border-gray-100 bg-gray-50/50 p-5 dark:border-dark-700 dark:bg-dark-900/30"
              >
                <div class="mb-3 text-center">
                  <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('common.afterSalesTitle') }}
                  </h3>
                  <p class="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
                    {{ t('common.afterSalesDesc') }}
                  </p>
                </div>
                <img
                  v-if="afterSalesQRCode"
                  :src="afterSalesQRCode"
                  :alt="t('common.afterSalesTitle')"
                  class="h-44 w-44 rounded-lg border border-gray-200 bg-white object-contain p-2 dark:border-dark-600"
                  @error="onImgError($event, 'afterSales')"
                />
                <div
                  v-else-if="supportContact"
                  class="flex min-h-44 w-full flex-col items-center justify-center rounded-lg border border-gray-200 bg-white p-4 text-center dark:border-dark-600 dark:bg-dark-800"
                >
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('common.wechatId') }}
                  </span>
                  <span class="mt-2 break-all text-lg font-semibold text-gray-900 dark:text-white">
                    {{ supportContact }}
                  </span>
                </div>
              </div>

              <!-- 技术客服 -->
              <div
                v-if="showTechSupportContact"
                class="flex flex-col items-center rounded-2xl border border-gray-100 bg-gray-50/50 p-5 dark:border-dark-700 dark:bg-dark-900/30"
              >
                <div class="mb-3 text-center">
                  <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('common.techSupportTitle') }}
                  </h3>
                  <p class="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
                    {{ t('common.techSupportDesc') }}
                  </p>
                </div>
                <img
                  v-if="techSupportQRCode"
                  :src="techSupportQRCode"
                  :alt="t('common.techSupportTitle')"
                  class="h-44 w-44 rounded-lg border border-gray-200 bg-white object-contain p-2 dark:border-dark-600"
                  @error="onImgError($event, 'techSupport')"
                />
                <div
                  v-else-if="supportContact"
                  class="flex min-h-44 w-full flex-col items-center justify-center rounded-lg border border-gray-200 bg-white p-4 text-center dark:border-dark-600 dark:bg-dark-800"
                >
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('common.wechatId') }}
                  </span>
                  <span class="mt-2 break-all text-lg font-semibold text-gray-900 dark:text-white">
                    {{ supportContact }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
/**
 * 顶部栏客服入口按钮
 * 点击弹出弹窗，展示售后客服 + 技术客服联系方式
 * 二维码图片来源于 admin 配置的 public settings
 */
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import { useBodyScrollLock } from '@/composables/useBodyScrollLock'

const { t } = useI18n()
const appStore = useAppStore()
const { contactInfo, techSupportQRCode, afterSalesQRCode } = storeToRefs(appStore)

const supportContact = computed(() => contactInfo.value.trim())
const showAfterSalesContact = computed(() => !!afterSalesQRCode.value || !!supportContact.value)
const showTechSupportContact = computed(() => !!techSupportQRCode.value || !!supportContact.value)
const showSingleContact = computed(
  () => !!supportContact.value && !afterSalesQRCode.value && !techSupportQRCode.value
)

// 至少有一个二维码或客服联系方式配置时才显示按钮
const hasCustomerServiceContact = computed(
  () => showAfterSalesContact.value || showTechSupportContact.value
)

// 弹窗开关状态
const isOpen = ref(false)

function openModal(): void {
  isOpen.value = true
}

function closeModal(): void {
  isOpen.value = false
}

// 图片加载失败时隐藏该二维码（避免显示坏图）
function onImgError(e: Event, type: 'afterSales' | 'techSupport'): void {
  const target = e.target as HTMLImageElement
  target.style.display = 'none'
  // 可选：在此也可以清空对应 ref，但为了让管理员修复 URL 后无需刷新页面，仅隐藏 img 本身
  void type
}

// ESC 关闭
function handleEscape(e: KeyboardEvent): void {
  if (e.key === 'Escape' && isOpen.value) {
    closeModal()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleEscape)
})

useBodyScrollLock(isOpen)
</script>

<style scoped>
.cs-modal-fade-enter-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.cs-modal-fade-leave-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 1, 1);
}
.cs-modal-fade-enter-from,
.cs-modal-fade-leave-to {
  opacity: 0;
}
.cs-modal-fade-enter-from > div {
  transform: scale(0.94) translateY(-12px);
  opacity: 0;
}
.cs-modal-fade-leave-to > div {
  transform: scale(0.96) translateY(-8px);
  opacity: 0;
}
</style>
