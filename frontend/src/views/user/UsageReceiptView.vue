<template>
  <AppLayout>
    <main class="usage-receipt-page">
      <header class="usage-receipt-page__header">
        <div>
          <p class="usage-receipt-page__eyebrow">{{ t('usageReceipt.eyebrow') }}</p>
          <h1>{{ t('usageReceipt.title') }}</h1>
          <p>{{ t('usageReceipt.description') }}</p>
        </div>
        <router-link to="/usage" class="btn btn-secondary">
          <Icon name="arrowLeft" size="sm" />
          {{ t('usageReceipt.backToUsage') }}
        </router-link>
      </header>

      <div class="usage-receipt-layout">
        <aside class="usage-receipt-controls" :aria-label="t('usageReceipt.settingsLabel')">
          <section class="card usage-receipt-controls__section">
            <div class="usage-receipt-controls__heading">
              <span>01</span>
              <div>
                <h2>{{ t('usageReceipt.periodTitle') }}</h2>
                <p>{{ t('usageReceipt.periodHint') }}</p>
              </div>
            </div>
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              @change="handleDateRangeChange"
            />
          </section>

          <section class="card usage-receipt-controls__section">
            <div class="usage-receipt-controls__heading">
              <span>02</span>
              <div>
                <h2>{{ t('usageReceipt.privacyTitle') }}</h2>
                <p>{{ t('usageReceipt.privacyHint') }}</p>
              </div>
            </div>

            <label class="receipt-option">
              <span>
                <strong>{{ t('usageReceipt.showName') }}</strong>
                <small>{{ t('usageReceipt.showNameHint') }}</small>
              </span>
              <input v-model="preferences.showDisplayName" type="checkbox" />
            </label>
            <label class="receipt-option">
              <span>
                <strong>{{ t('usageReceipt.showSavings') }}</strong>
                <small>{{ t('usageReceipt.showSavingsHint') }}</small>
              </span>
              <input v-model="preferences.showSavings" type="checkbox" />
            </label>
            <label class="receipt-option">
              <span>
                <strong>{{ t('usageReceipt.showModels') }}</strong>
                <small>{{ t('usageReceipt.showModelsHint') }}</small>
              </span>
              <input v-model="preferences.showModelBreakdown" type="checkbox" />
            </label>

            <div class="receipt-privacy-note">
              <Icon name="shield" size="md" />
              <p>{{ t('usageReceipt.safeByDefault') }}</p>
            </div>
          </section>

          <section class="card usage-receipt-controls__section">
            <div class="usage-receipt-controls__heading">
              <span>03</span>
              <div>
                <h2>{{ t('usageReceipt.shareTitle') }}</h2>
                <p>{{ inviteCode ? t('usageReceipt.inviteReady') : t('usageReceipt.inviteFallback') }}</p>
              </div>
            </div>

            <div class="usage-receipt-actions">
              <button class="btn btn-primary" :disabled="!receiptData || loading || exporting || printing" @click="shareReceipt">
                <Icon name="share" size="sm" />
                {{ sharing ? t('usageReceipt.sharing') : t('usageReceipt.shareImage') }}
              </button>
              <button class="btn btn-secondary" :disabled="!receiptData || loading || exporting || printing" @click="downloadReceipt">
                <Icon name="download" size="sm" />
                {{ exporting && !sharing ? t('usageReceipt.exporting') : t('usageReceipt.downloadImage') }}
              </button>
              <button class="btn btn-secondary" :disabled="!receiptData || loading || printing" @click="restartPrint">
                <Icon name="refresh" size="sm" />
                {{ t('usageReceipt.printAgain') }}
              </button>
              <button class="btn btn-ghost" :disabled="!inviteUrl" @click="copyInviteLink">
                <Icon name="copy" size="sm" />
                {{ copied ? t('common.copied') : t('usageReceipt.copyInvite') }}
              </button>
            </div>
          </section>
        </aside>

        <section class="usage-receipt-preview" aria-labelledby="receipt-preview-title">
          <div class="usage-receipt-preview__toolbar">
            <div>
              <span class="usage-receipt-preview__status" :class="{ 'usage-receipt-preview__status--active': printing }"></span>
              <strong id="receipt-preview-title">{{ printing ? t('usageReceipt.printing') : t('usageReceipt.previewReady') }}</strong>
            </div>
            <span>{{ t('usageReceipt.beijingTime') }}</span>
          </div>

          <div class="receipt-printer-stage">
            <div class="receipt-printer" aria-hidden="true">
              <div class="receipt-printer__topline">
                <span>{{ appStore.siteName }}</span>
                <span class="receipt-printer__lamp"></span>
              </div>
              <div class="receipt-printer__slot"></div>
            </div>

            <div v-if="loading" class="receipt-loading" role="status">
              <LoadingSpinner />
              <p>{{ t('usageReceipt.loading') }}</p>
            </div>

            <div v-else-if="loadError" class="receipt-error" role="alert">
              <Icon name="exclamationCircle" size="xl" />
              <h2>{{ t('usageReceipt.loadFailed') }}</h2>
              <p>{{ t('usageReceipt.loadFailedHint') }}</p>
              <button class="btn btn-secondary" @click="loadReceiptData">{{ t('common.retry') }}</button>
            </div>

            <div v-else-if="receiptData" class="receipt-viewport">
              <div
                :key="printRunKey"
                class="receipt-feed"
                :class="{ 'receipt-feed--printing': printing }"
                @animationend="finishPrint"
              >
                <UsageReceiptPaper
                  ref="receiptPaper"
                  :data="receiptData"
                  :preferences="preferences"
                  :site-name="appStore.siteName"
                  :site-logo="appStore.siteLogo || '/laoshirenai-icon.jpg'"
                />
              </div>
            </div>
          </div>

          <p class="usage-receipt-preview__caption">{{ t('usageReceipt.previewCaption') }}</p>
        </section>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toBlob } from 'html-to-image'
import QRCode from 'qrcode'
import { usageAPI } from '@/api/usage'
import { getMyInviteCode } from '@/api/agent'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import UsageReceiptPaper from '@/features/usage-receipt/UsageReceiptPaper.vue'
import {
  buildInviteUrl,
  createReceiptNumber,
  receiptImageFilename
} from '@/features/usage-receipt/receipt'
import type {
  UsageReceiptData,
  UsageReceiptPreferences
} from '@/features/usage-receipt/types'

interface ReceiptPaperExpose {
  getElement: () => HTMLElement | null
}

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copied, copyToClipboard } = useClipboard()

const DAY_MS = 24 * 60 * 60 * 1000
const formatBeijingDate = (offsetDays = 0): string => new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit'
}).format(new Date(Date.now() + offsetDays * DAY_MS))

const startDate = ref(formatBeijingDate(-6))
const endDate = ref(formatBeijingDate())
const receiptData = ref<UsageReceiptData | null>(null)
const receiptPaper = ref<ReceiptPaperExpose | null>(null)
const inviteCode = ref('')
const qrDataUrl = ref('')
const loading = ref(true)
const loadError = ref(false)
const printing = ref(false)
const exporting = ref(false)
const sharing = ref(false)
const printRunKey = ref(0)

const preferences = reactive<UsageReceiptPreferences>({
  showDisplayName: false,
  showSavings: true,
  showModelBreakdown: true
})

const inviteUrl = computed(() => {
  if (typeof window === 'undefined') return ''
  return buildInviteUrl(window.location.origin, inviteCode.value)
})

const exchangeRate = computed(() => {
  const value = Number(appStore.cachedPublicSettings?.landing_pricing_exchange_rate)
  return Number.isFinite(value) && value > 0 ? value : 7
})

async function createInviteQRCode(): Promise<string> {
  try {
    return await QRCode.toDataURL(inviteUrl.value, {
      errorCorrectionLevel: 'M',
      margin: 1,
      width: 176,
      color: {
        dark: '#000000',
        light: '#ffffff'
      }
    })
  } catch (error) {
    console.error('Failed to generate receipt invite QR code:', error)
    return ''
  }
}

async function loadReceiptData(): Promise<void> {
  loading.value = true
  loadError.value = false

  try {
    await appStore.fetchPublicSettings()

    const [stats, modelResponse, inviteResult] = await Promise.all([
      usageAPI.getStatsByDateRange(startDate.value, endDate.value),
      usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value }),
      getMyInviteCode().catch((error) => {
        console.error('Failed to load invite code for receipt:', error)
        return { invite_code: '' }
      })
    ])

    inviteCode.value = inviteResult.invite_code
    qrDataUrl.value = await createInviteQRCode()

    receiptData.value = {
      receiptNumber: createReceiptNumber(),
      generatedAt: new Date(),
      startDate: startDate.value,
      endDate: endDate.value,
      displayName: authStore.user?.username?.trim() || '',
      inviteCode: inviteCode.value,
      inviteUrl: inviteUrl.value,
      qrDataUrl: qrDataUrl.value,
      stats,
      models: modelResponse.models || [],
      exchangeRate: exchangeRate.value
    }

    await nextTick()
    restartPrint()
  } catch (error) {
    console.error('Failed to load usage receipt:', error)
    receiptData.value = null
    loadError.value = true
  } finally {
    loading.value = false
  }
}

async function handleDateRangeChange(range: { startDate: string; endDate: string }): Promise<void> {
  startDate.value = range.startDate
  endDate.value = range.endDate
  await loadReceiptData()
}

function restartPrint(): void {
  if (!receiptData.value) return
  printing.value = true
  printRunKey.value += 1
}

function finishPrint(): void {
  printing.value = false
}

async function renderReceiptBlob(): Promise<Blob> {
  const element = receiptPaper.value?.getElement()
  if (!element) throw new Error('Receipt is not ready')

  if (document.fonts?.ready) await document.fonts.ready
  const logo = element.querySelector('img.receipt-brand__logo') as HTMLImageElement | null
  if (logo && typeof logo.decode === 'function') {
    await logo.decode().catch(() => undefined)
  }

  const previousWidth = element.style.width
  const previousMaxWidth = element.style.maxWidth
  element.style.width = '420px'
  element.style.maxWidth = 'none'

  try {
    const blob = await toBlob(element, {
      cacheBust: true,
      pixelRatio: 2.5
    })
    if (!blob) throw new Error('Receipt image could not be created')
    return blob
  } finally {
    element.style.width = previousWidth
    element.style.maxWidth = previousMaxWidth
  }
}

function triggerDownload(blob: Blob): void {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = receiptImageFilename(startDate.value, endDate.value)
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

async function downloadReceipt(): Promise<void> {
  if (!receiptData.value || exporting.value) return
  exporting.value = true
  try {
    const blob = await renderReceiptBlob()
    triggerDownload(blob)
    appStore.showSuccess(t('usageReceipt.downloaded'))
  } catch (error) {
    console.error('Failed to export usage receipt:', error)
    appStore.showError(t('usageReceipt.exportFailed'))
  } finally {
    exporting.value = false
  }
}

async function shareReceipt(): Promise<void> {
  if (!receiptData.value || exporting.value) return
  exporting.value = true
  sharing.value = true

  try {
    const blob = await renderReceiptBlob()
    const file = new File([blob], receiptImageFilename(startDate.value, endDate.value), {
      type: 'image/png'
    })
    const sharePayload = {
      title: t('usageReceipt.sharePayloadTitle'),
      text: t('usageReceipt.sharePayloadText'),
      files: [file]
    }
    const canShareFiles = typeof navigator.share === 'function'
      && (typeof navigator.canShare !== 'function' || navigator.canShare(sharePayload))

    if (canShareFiles) {
      await navigator.share(sharePayload)
      appStore.showSuccess(t('usageReceipt.shared'))
    } else {
      triggerDownload(blob)
      appStore.showInfo(t('usageReceipt.shareFallback'))
    }
  } catch (error) {
    if ((error as { name?: string }).name !== 'AbortError') {
      console.error('Failed to share usage receipt:', error)
      appStore.showError(t('usageReceipt.shareFailed'))
    }
  } finally {
    exporting.value = false
    sharing.value = false
  }
}

async function copyInviteLink(): Promise<void> {
  if (!inviteUrl.value) return
  await copyToClipboard(inviteUrl.value, t('usageReceipt.inviteCopied'))
}

onMounted(loadReceiptData)
</script>

<style scoped>
.usage-receipt-page {
  display: grid;
  gap: 24px;
}

.usage-receipt-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}

.usage-receipt-page__eyebrow {
  color: rgb(var(--color-terracotta));
  font-family: Cinzel, serif;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.usage-receipt-page__header h1 {
  margin-top: 4px;
  color: rgb(var(--color-ink-deep));
  font-family: "EB Garamond", "Noto Serif SC", "Songti SC", serif;
  font-size: clamp(32px, 4vw, 48px);
  font-weight: 700;
  line-height: 1;
}

.usage-receipt-page__header p:last-child {
  max-width: 650px;
  margin-top: 10px;
  color: rgb(var(--color-muted));
  font-size: 14px;
  line-height: 1.7;
}

.usage-receipt-layout {
  display: grid;
  grid-template-columns: minmax(280px, 360px) minmax(0, 1fr);
  gap: 24px;
  align-items: start;
}

.usage-receipt-controls {
  display: grid;
  gap: 16px;
}

.usage-receipt-controls__section {
  display: grid;
  gap: 16px;
  padding: 20px;
}

.usage-receipt-controls__heading {
  display: flex;
  gap: 12px;
}

.usage-receipt-controls__heading > span {
  color: rgb(var(--color-terracotta));
  font-family: Cinzel, serif;
  font-size: 11px;
  font-weight: 800;
}

.usage-receipt-controls__heading h2 {
  color: rgb(var(--color-ink));
  font-size: 14px;
  font-weight: 750;
}

.usage-receipt-controls__heading p {
  margin-top: 3px;
  color: rgb(var(--color-muted));
  font-size: 11px;
  line-height: 1.5;
}

.receipt-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-top: 1px solid rgb(var(--color-stone));
  cursor: pointer;
}

.receipt-option span {
  display: grid;
  gap: 3px;
}

.receipt-option strong {
  color: rgb(var(--color-ink));
  font-size: 13px;
}

.receipt-option small {
  color: rgb(var(--color-muted));
  font-size: 10px;
  line-height: 1.4;
}

.receipt-option input {
  width: 18px;
  height: 18px;
  flex: 0 0 auto;
  accent-color: rgb(var(--color-terracotta));
}

.receipt-privacy-note {
  display: flex;
  gap: 10px;
  padding: 12px;
  border: 1px solid rgb(var(--color-green-200));
  background: rgb(var(--color-green-50));
  color: rgb(var(--color-green-800));
  font-size: 11px;
  line-height: 1.55;
}

.receipt-privacy-note svg {
  flex: 0 0 auto;
}

.usage-receipt-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.usage-receipt-actions .btn {
  width: 100%;
  padding-right: 10px;
  padding-left: 10px;
}

.usage-receipt-preview {
  position: sticky;
  top: 20px;
  min-width: 0;
}

.usage-receipt-preview__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 10px;
  color: rgb(var(--color-muted));
  font-size: 11px;
}

.usage-receipt-preview__toolbar > div {
  display: flex;
  align-items: center;
  gap: 8px;
  color: rgb(var(--color-ink));
}

.usage-receipt-preview__status {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: rgb(var(--color-success));
}

.usage-receipt-preview__status--active {
  animation: receipt-status-pulse var(--duration-ambient) var(--ease-standard) infinite;
}

.receipt-printer-stage {
  position: relative;
  min-height: 700px;
  overflow: hidden;
  padding: 102px 24px 48px;
  border: 1px solid rgb(var(--color-dark-600));
  border-radius: 24px;
  background: linear-gradient(145deg, rgb(var(--lacquer-rest-top)), rgb(var(--lacquer-rest-mid)) 48%, rgb(var(--lacquer-base)));
  box-shadow: inset 0 1px 0 rgb(var(--color-dark-300) / 0.14), 0 22px 60px rgb(var(--shadow-ink) / 0.2);
}

.receipt-printer {
  position: absolute;
  top: 24px;
  left: 50%;
  z-index: 5;
  width: min(520px, calc(100% - 48px));
  height: 92px;
  padding: 18px 24px;
  border: 1px solid rgb(var(--color-dark-500));
  border-radius: 18px 18px 8px 8px;
  background: linear-gradient(180deg, rgb(var(--lacquer-hover-mid)), rgb(var(--lacquer-base)));
  box-shadow: 0 18px 36px rgb(var(--lacquer-base) / 0.52);
  transform: translateX(-50%);
}

.receipt-printer__topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: rgb(var(--color-dark-300));
  font-family: Cinzel, serif;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.receipt-printer__lamp {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: rgb(var(--color-success));
  box-shadow: 0 0 12px rgb(var(--color-success) / 0.78);
}

.receipt-printer__slot {
  height: 12px;
  margin-top: 17px;
  border: 1px solid rgb(var(--color-dark-600));
  border-radius: 999px;
  background: rgb(var(--lacquer-base));
  box-shadow: inset 0 4px 7px rgb(var(--lacquer-base) / 0.9), 0 1px 0 rgb(var(--color-dark-400) / 0.18);
}

.receipt-viewport {
  display: flex;
  justify-content: center;
  overflow: hidden;
  padding: 12px 0 24px;
}

.receipt-feed {
  display: flex;
  width: min(100%, 420px);
  justify-content: center;
  transform-origin: top center;
  will-change: clip-path, transform;
}

.receipt-feed--printing {
  animation: receipt-feed-out calc(var(--duration-receipt-print) + var(--duration-receipt-settle))
    var(--ease-receipt-feed) both;
}

.receipt-loading,
.receipt-error {
  display: grid;
  min-height: 520px;
  place-items: center;
  align-content: center;
  gap: 12px;
  color: rgb(var(--color-dark-200));
  text-align: center;
}

.receipt-loading p,
.receipt-error p {
  max-width: 360px;
  color: rgb(var(--color-dark-400));
  font-size: 12px;
}

.receipt-error h2 {
  font-family: "EB Garamond", "Noto Serif SC", serif;
  font-size: 24px;
}

.usage-receipt-preview__caption {
  margin-top: 12px;
  color: rgb(var(--color-muted));
  font-size: 10px;
  line-height: 1.6;
  text-align: center;
}

@keyframes receipt-feed-out {
  0% {
    clip-path: inset(0 0 100% 0);
    transform: translateY(-10px);
  }
  8% {
    clip-path: inset(0 0 92% 0);
  }
  88% {
    clip-path: inset(0 0 0 0);
    transform: translateY(0);
  }
  94% { transform: translateY(0); }
  97% { transform: translateY(7px); }
  100% { clip-path: inset(0 0 0 0); transform: translateY(0); }
}

@keyframes receipt-status-pulse {
  0%, 100% { opacity: 0.45; transform: scale(0.88); }
  50% { opacity: 1; transform: scale(1); }
}

@media (max-width: 1100px) {
  .usage-receipt-layout {
    grid-template-columns: 1fr;
  }

  .usage-receipt-controls {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .usage-receipt-preview {
    position: static;
  }
}

@media (max-width: 850px) {
  .usage-receipt-controls {
    grid-template-columns: 1fr;
  }

  .usage-receipt-page__header {
    flex-direction: column;
  }
}

@media (max-width: 520px) {
  .usage-receipt-actions {
    grid-template-columns: 1fr;
  }

  .receipt-printer-stage {
    min-height: 620px;
    padding-right: 12px;
    padding-left: 12px;
    border-radius: 18px;
  }

  .receipt-printer {
    width: calc(100% - 24px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .receipt-feed--printing,
  .usage-receipt-preview__status--active {
    animation-duration: 1ms;
    animation-delay: 0ms;
  }
}
</style>
