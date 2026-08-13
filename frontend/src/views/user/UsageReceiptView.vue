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
              </div>
            </div>

            <label class="receipt-option">
              <span>
                <strong>{{ t('usageReceipt.showSignature') }}</strong>
              </span>
              <input v-model="preferences.showDisplayName" type="checkbox" />
            </label>
            <label v-if="preferences.showDisplayName" class="receipt-signature-input">
              <span>{{ t('usageReceipt.signatureLabel') }}</span>
              <input
                v-model="customDisplayName"
                type="text"
                maxlength="20"
                :placeholder="randomDisplayName"
                @input="handleDisplayNameChange"
              />
            </label>
            <label class="receipt-option">
              <span>
                <strong>{{ t('usageReceipt.showSavings') }}</strong>
              </span>
              <input v-model="preferences.showSavings" type="checkbox" />
            </label>
            <label class="receipt-option">
              <span>
                <strong>{{ t('usageReceipt.showModels') }}</strong>
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
                <p>{{ inviteStatusText }}</p>
              </div>
            </div>

            <label v-if="partnerInviteOptions.length > 0" class="receipt-invite-picker">
              <span>
                <strong>{{ t('usageReceipt.inviteLinkTitle') }}</strong>
                <small>{{ t('usageReceipt.partnerInviteHint') }}</small>
              </span>
              <select
                v-model="inviteCode"
                :disabled="loading || printing"
                @change="handleInviteLinkChange"
              >
                <option v-for="option in partnerInviteOptions" :key="option.code" :value="option.code">
                  {{ option.label }}
                </option>
              </select>
            </label>

            <div class="usage-receipt-actions">
              <button class="btn btn-primary" :disabled="!receiptData || loading || exporting || printing" @click="shareReceipt">
                <Icon name="share" size="sm" />
                {{ sharing ? t('usageReceipt.sharing') : t('usageReceipt.shareImage') }}
              </button>
              <button class="btn btn-secondary" :disabled="!receiptData || loading || exporting || printing" @click="downloadReceipt">
                <Icon name="download" size="sm" />
                {{ exporting && !sharing ? t('usageReceipt.exporting') : t('usageReceipt.downloadImage') }}
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
              <strong id="receipt-preview-title">{{ receiptStatusText }}</strong>
            </div>
            <span>{{ t('usageReceipt.beijingTime') }}</span>
          </div>

          <div
            class="receipt-printer-stage"
            :class="{
              'receipt-printer-stage--printing': isPrinting,
              'receipt-printer-stage--detaching': isDetaching,
              'receipt-printer-stage--complete': printPhase === 'complete'
            }"
          >
            <div class="receipt-printer-rig">
              <div class="receipt-printer">
                <img
                  src="/assets/usage-receipt/thermal-printer-athens-v1.png"
                  alt=""
                  draggable="false"
                  aria-hidden="true"
                />
                <span class="receipt-printer__active-lamp" aria-hidden="true"></span>
                <button
                  type="button"
                  class="receipt-printer__print-button"
                  :disabled="!receiptData || loading || printing"
                  :aria-label="t('usageReceipt.pressPrinterButton')"
                  :data-label="t('usageReceipt.pressPrinterButton')"
                  @click="restartPrint(true)"
                ></button>
              </div>

              <div v-if="receiptData && !loading && !loadError" class="receipt-viewport">
                <div
                  :key="printRunKey"
                  class="receipt-feed"
                  :class="{
                    'receipt-feed--ready': printPhase === 'ready',
                    'receipt-feed--printing': isPrinting,
                    'receipt-feed--detaching': isDetaching,
                    'receipt-feed--complete': printPhase === 'complete'
                  }"
                  @animationend.self="handlePrintAnimationEnd"
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

              <div class="receipt-printer__mouth" aria-hidden="true">
                <span class="receipt-printer__roller"></span>
                <span class="receipt-printer__cutter"></span>
              </div>
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
          </div>

        </section>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toBlob } from 'html-to-image'
import QRCode from 'qrcode'
import { usageAPI } from '@/api/usage'
import {
  getAffiliateQualification,
  getMyInviteCode,
  listAffiliateLinks,
  type AffiliateLink
} from '@/api/agent'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { resolvePartnerAccessState } from '@/features/affiliate/partnerAccess'
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
import { playThermalPrinterSound } from '@/features/usage-receipt/printerSound'
import type {
  UsageReceiptData,
  UsageReceiptPreferences
} from '@/features/usage-receipt/types'

interface ReceiptPaperExpose {
  getElement: () => HTMLElement | null
}

interface ReceiptInviteOption {
  code: string
  label: string
}

type PrintPhase = 'ready' | 'printing' | 'detaching' | 'complete'

const { t } = useI18n()
const appStore = useAppStore()
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
const partnerInviteOptions = ref<ReceiptInviteOption[]>([])
const inviteMode = ref<'ordinary' | 'partner' | 'unavailable'>('ordinary')
const DISPLAY_NAME_POOL = [
  '奥林匹斯打字员',
  '提示词炼金术士',
  '神谕编译官',
  '月桂叶调参师',
  '雅典娜的键盘手',
  '帕特农修 Bug 人',
  '阿波罗接口祭司',
  '赫尔墨斯搬砖官',
  '迷宫里的架构师',
  '特洛伊日志守夜人',
  '赛博斯巴达文书',
  '德尔斐模型观测员'
] as const
const randomDisplayName = ref(DISPLAY_NAME_POOL[Math.floor(Math.random() * DISPLAY_NAME_POOL.length)])
const customDisplayName = ref('')
const qrDataUrl = ref('')
const loading = ref(true)
const loadError = ref(false)
const printPhase = ref<PrintPhase>('ready')
const exporting = ref(false)
const sharing = ref(false)
const printRunKey = ref(0)
let stopPrinterSound: (() => void) | null = null

const isPrinting = computed(() => printPhase.value === 'printing')
const isDetaching = computed(() => printPhase.value === 'detaching')
const printing = computed(() => isPrinting.value || isDetaching.value)
const receiptStatusText = computed(() => {
  if (isPrinting.value) return t('usageReceipt.printing')
  if (isDetaching.value) return t('usageReceipt.detaching')
  if (printPhase.value === 'complete') return t('usageReceipt.previewReady')
  return t('usageReceipt.pressPrinterButton')
})
const inviteStatusText = computed(() => {
  if (inviteMode.value === 'partner') return t('usageReceipt.partnerInviteReady')
  if (inviteMode.value === 'unavailable') return t('usageReceipt.inviteFallback')
  return inviteCode.value ? t('usageReceipt.ordinaryInviteReady') : t('usageReceipt.inviteFallback')
})
const receiptDisplayName = computed(() => customDisplayName.value.trim() || randomDisplayName.value)

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

function affiliateLinkLabel(link: AffiliateLink): string {
  if (link.is_default) return t('usageReceipt.defaultPartnerLink')
  return link.channel?.trim() || link.name?.trim() || link.code
}

async function loadInviteDestination(): Promise<void> {
  const ordinaryInvite = await getMyInviteCode().catch((error) => {
    console.error('Failed to load ordinary invite code for receipt:', error)
    return { invite_code: '' }
  })

  inviteCode.value = ordinaryInvite.invite_code
  inviteMode.value = 'ordinary'
  partnerInviteOptions.value = []

  try {
    const qualification = await getAffiliateQualification()
    if (qualification.program_mode !== 'live') return

    const accessState = resolvePartnerAccessState(qualification.agent_status, qualification.risk_status)
    if (qualification.agent_status === 'active' && accessState !== 'available') {
      inviteCode.value = ''
      inviteMode.value = 'unavailable'
      return
    }
    if (accessState !== 'available') return

    const links = (await listAffiliateLinks())
      .filter(link => link.status === 'active')
      .sort((left, right) => Number(right.is_default) - Number(left.is_default) || left.id - right.id)

    if (links.length === 0) {
      inviteCode.value = ''
      inviteMode.value = 'unavailable'
      return
    }

    partnerInviteOptions.value = links.map(link => ({
      code: link.code,
      label: affiliateLinkLabel(link)
    }))
    inviteCode.value = links[0].code
    inviteMode.value = 'partner'
  } catch (error) {
    // The receipt remains usable with the ordinary invite URL if affiliate
    // eligibility is unavailable or this user is not enrolled.
    console.error('Failed to resolve affiliate link for receipt:', error)
  }
}

async function handleInviteLinkChange(): Promise<void> {
  qrDataUrl.value = await createInviteQRCode()
  if (receiptData.value) {
    receiptData.value = {
      ...receiptData.value,
      inviteCode: inviteCode.value,
      inviteUrl: inviteUrl.value,
      qrDataUrl: qrDataUrl.value
    }
    await nextTick()
    resetPrinter()
  }
}

function handleDisplayNameChange(): void {
  if (!receiptData.value) return
  receiptData.value = {
    ...receiptData.value,
    displayName: receiptDisplayName.value
  }
}

async function loadReceiptData(): Promise<void> {
  loading.value = true
  loadError.value = false

  try {
    await appStore.fetchPublicSettings()

    const [stats, modelResponse] = await Promise.all([
      usageAPI.getStatsByDateRange(startDate.value, endDate.value),
      usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value }),
      loadInviteDestination()
    ])

    qrDataUrl.value = await createInviteQRCode()

    receiptData.value = {
      receiptNumber: createReceiptNumber(),
      generatedAt: new Date(),
      startDate: startDate.value,
      endDate: endDate.value,
      displayName: receiptDisplayName.value,
      inviteCode: inviteCode.value,
      inviteUrl: inviteUrl.value,
      qrDataUrl: qrDataUrl.value,
      stats,
      models: modelResponse.models || [],
      exchangeRate: exchangeRate.value
    }

    await nextTick()
    resetPrinter()
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

function restartPrint(withSound = true): void {
  if (!receiptData.value || printing.value) return
  stopPrinterSound?.()
  stopPrinterSound = withSound ? playThermalPrinterSound() : null
  printPhase.value = 'printing'
  printRunKey.value += 1
}

function resetPrinter(): void {
  stopPrinterSound?.()
  stopPrinterSound = null
  printPhase.value = 'ready'
  printRunKey.value += 1
}

function handlePrintAnimationEnd(): void {
  if (isPrinting.value) {
    printPhase.value = 'detaching'
    return
  }

  if (isDetaching.value) {
    printPhase.value = 'complete'
    stopPrinterSound?.()
    stopPrinterSound = null
  }
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
onBeforeUnmount(() => {
  stopPrinterSound?.()
  stopPrinterSound = null
})
</script>

<style scoped>
.usage-receipt-page {
  display: grid;
  gap: 24px;
}

.usage-receipt-page::before {
  content: none !important;
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

.receipt-signature-input {
  display: grid;
  gap: 7px;
  padding: 0 0 12px;
  border-bottom: 1px solid rgb(var(--color-stone));
}

.receipt-signature-input span {
  color: rgb(var(--color-muted));
  font-size: 11px;
  font-weight: 700;
}

.receipt-signature-input input {
  width: 100%;
  min-height: 38px;
  padding: 0 10px;
  border: 1px solid rgb(var(--color-stone));
  border-radius: 3px;
  color: rgb(var(--color-ink));
  background: rgb(var(--color-vellum));
  font-size: 12px;
}

.receipt-invite-picker {
  display: grid;
  gap: 9px;
  padding: 12px;
  border: 1px solid rgb(var(--color-stone));
  background: rgb(var(--color-vellum) / 0.5);
}

.receipt-invite-picker > span {
  display: grid;
  gap: 3px;
}

.receipt-invite-picker strong {
  color: rgb(var(--color-ink));
  font-size: 13px;
}

.receipt-invite-picker small {
  color: rgb(var(--color-muted));
  font-size: 10px;
  line-height: 1.4;
}

.receipt-invite-picker select {
  width: 100%;
  min-height: 38px;
  padding: 0 34px 0 10px;
  border: 1px solid rgb(var(--color-stone));
  border-radius: 3px;
  color: rgb(var(--color-ink));
  background: rgb(var(--color-vellum));
  font-size: 12px;
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

.usage-receipt-actions .btn:last-child:nth-child(3) {
  grid-column: 1 / -1;
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
  min-height: 1530px;
  overflow: hidden;
  padding: 190px 24px 300px;
  border: 1px solid rgb(var(--color-muted) / 0.28);
  border-radius: 14px;
  background:
    radial-gradient(ellipse at 50% 56px, rgb(var(--color-ink) / 0.12), transparent 37%),
    linear-gradient(115deg, rgb(var(--color-marble) / 0.82), transparent 34%),
    linear-gradient(102deg, transparent 0 28%, rgb(var(--color-muted) / 0.08) 28.2%, transparent 28.55% 71%, rgb(var(--color-muted) / 0.06) 71.25%, transparent 71.6%),
    repeating-linear-gradient(90deg, transparent 0 31px, rgb(var(--color-muted) / 0.04) 31px 32px),
    rgb(var(--color-parchment));
  box-shadow:
    inset 0 1px 0 rgb(var(--color-vellum) / 0.78),
    inset 0 -30px 80px rgb(var(--color-ink) / 0.06),
    0 22px 60px rgb(var(--shadow-ink) / 0.14);
  perspective: 1100px;
}

.receipt-printer-stage::before {
  position: absolute;
  inset: 0;
  z-index: 0;
  border-radius: inherit;
  background:
    radial-gradient(circle at 14% 23%, rgb(var(--color-muted) / 0.08) 0 1px, transparent 1.5px),
    radial-gradient(circle at 78% 67%, rgb(var(--color-muted) / 0.065) 0 1px, transparent 1.5px);
  background-size: 48px 52px, 62px 58px;
  content: '';
  pointer-events: none;
}

.receipt-printer-stage::after {
  position: absolute;
  top: 202px;
  left: 50%;
  z-index: 1;
  width: min(640px, calc(100% - 36px));
  height: 72px;
  border-radius: 50%;
  background: rgb(var(--printer-counter-shadow) / 0.26);
  content: '';
  filter: blur(20px);
  transform: translateX(-50%);
}

.receipt-printer-rig {
  position: absolute;
  top: 18px;
  left: 50%;
  z-index: 3;
  width: min(760px, calc(100% - 30px));
  transform: translateX(-50%);
}

.receipt-printer {
  position: relative;
  z-index: 7;
  width: 100%;
  aspect-ratio: 1570 / 528;
  filter: drop-shadow(0 20px 18px rgb(var(--printer-counter-shadow) / 0.34));
  transform-origin: center bottom;
}

.receipt-printer > img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  pointer-events: none;
  user-select: none;
}

.receipt-printer__mouth {
  position: absolute;
  top: 68.75%;
  left: 12.42%;
  z-index: 10;
  width: 58.85%;
  height: 7.58%;
  overflow: hidden;
  border-radius: 3px;
  background: rgb(var(--printer-seam) / 0.94);
  box-shadow:
    inset 0 4px 7px rgb(var(--printer-seam)),
    0 -1px 0 rgb(var(--printer-metal) / 0.42),
    0 2px 3px rgb(var(--printer-seam) / 0.42);
}

.receipt-printer__roller {
  position: absolute;
  inset: 18% 2% auto;
  z-index: 2;
  height: 25%;
  border-radius: 999px;
  background: repeating-linear-gradient(90deg, rgb(var(--color-warm-700)) 0 4px, rgb(var(--printer-seam)) 4px 7px);
  box-shadow: inset 0 2px 3px rgb(var(--printer-seam));
}

.receipt-printer__cutter {
  position: absolute;
  right: 2%;
  bottom: 2%;
  left: 2%;
  z-index: 3;
  height: 22%;
  background: repeating-linear-gradient(135deg, rgb(var(--printer-metal)) 0 2px, rgb(var(--printer-seam)) 2px 4px);
  clip-path: polygon(0 0, 100% 0, 99% 100%, 98% 35%, 97% 100%, 96% 35%, 95% 100%, 4% 35%, 3% 100%, 2% 35%, 1% 100%);
}

.receipt-printer__active-lamp {
  position: absolute;
  top: 73.35%;
  left: 80.8%;
  z-index: 9;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  opacity: 0;
  background: rgb(var(--printer-paper-led));
  transform: translate(-50%, -50%);
}

.receipt-printer__print-button {
  position: absolute;
  top: 62.48%;
  right: 8.47%;
  z-index: 10;
  width: 6.5%;
  aspect-ratio: 1;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: transparent;
  cursor: pointer;
  transition:
    transform var(--duration-instant) var(--ease-standard),
    box-shadow var(--duration-fast) var(--ease-standard);
}

.receipt-printer__print-button::before {
  position: absolute;
  right: -18px;
  bottom: calc(100% + 12px);
  width: max-content;
  padding: 6px 9px;
  border: 1px solid rgb(var(--color-muted) / 0.44);
  border-radius: 3px;
  opacity: 0;
  color: rgb(var(--color-vellum));
  background: rgb(var(--color-ink));
  box-shadow: 0 7px 16px rgb(var(--shadow-ink) / 0.2);
  content: attr(data-label);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.04em;
  pointer-events: none;
  transform: translateY(4px);
  transition:
    opacity var(--duration-fast) var(--ease-standard),
    transform var(--duration-fast) var(--ease-standard);
  white-space: nowrap;
}

.receipt-printer__print-button::after {
  position: absolute;
  inset: 0;
  border: 0;
  border-radius: inherit;
  background: transparent;
  box-shadow: 0 0 0 0 rgb(var(--color-terracotta) / 0.28);
  content: '';
}

.receipt-printer__print-button:not(:disabled):hover::before,
.receipt-printer__print-button:not(:disabled):focus-visible::before {
  opacity: 1;
  transform: translateY(0);
}

.receipt-printer__print-button:focus-visible {
  outline: 2px solid rgb(var(--color-terracotta));
  outline-offset: 4px;
}

.receipt-printer__print-button:active:not(:disabled) {
  transform: translateY(2px) scale(0.94);
}

.receipt-printer__print-button:disabled {
  cursor: not-allowed;
}

.receipt-printer-stage:not(.receipt-printer-stage--printing, .receipt-printer-stage--detaching)
  .receipt-printer__print-button:not(:disabled)::after {
  animation: receipt-button-invite var(--duration-ambient) var(--ease-standard) infinite;
}

.receipt-viewport {
  position: absolute;
  top: 68.75%;
  left: 12.42%;
  z-index: 9;
  width: 58.85%;
  overflow: hidden;
}

.receipt-feed {
  display: flex;
  width: 100%;
  justify-content: center;
  transform-origin: top center;
  will-change: transform;
}

.receipt-feed--ready {
  transform: translateY(calc(-100% + 14px));
}

.receipt-feed--printing {
  animation: receipt-feed-out var(--duration-receipt-print) linear both;
}

.receipt-feed--detaching {
  animation: receipt-release var(--duration-receipt-settle) var(--ease-out) both;
}

.receipt-feed--complete {
  transform: translate3d(0, 82px, 0) rotateZ(0) scale(1);
}

.receipt-feed--detaching > :deep(.usage-receipt-paper),
.receipt-feed--complete > :deep(.usage-receipt-paper) {
  box-shadow: 0 16px 30px rgb(var(--shadow-ink) / 0.2);
}

.receipt-printer-stage--detaching .receipt-viewport,
.receipt-printer-stage--complete .receipt-viewport {
  overflow: visible;
}

.receipt-printer-stage--printing .receipt-printer {
  animation: receipt-printer-vibration 120ms linear infinite;
}

.receipt-printer-stage--printing .receipt-printer__roller {
  animation: receipt-roller-feed 240ms linear infinite;
}

.receipt-printer-stage--printing .receipt-printer__active-lamp {
  animation: receipt-paper-lamp 780ms steps(2, end) infinite;
}

.receipt-printer-stage--detaching .receipt-printer {
  animation: receipt-cutter-kick 160ms var(--ease-out) both;
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

@keyframes receipt-feed-out {
  0% {
    transform: translateY(calc(-100% + 16px));
  }
  100% { transform: translateY(0); }
}

@keyframes receipt-release {
  0% {
    transform: perspective(1100px) translate3d(0, 0, 0) rotateX(0) rotateZ(0) scale(1);
  }
  16% {
    transform: perspective(1100px) translate3d(0, -8px, 0) rotateX(0) rotateZ(0.25deg) scale(1);
  }
  42% {
    transform: perspective(1100px) translate3d(3px, 30px, 0) rotateX(4deg) rotateZ(-0.45deg) scale(0.98);
  }
  72% {
    transform: perspective(1100px) translate3d(-2px, 68px, 0) rotateX(2deg) rotateZ(0.3deg) scale(0.992);
  }
  100% {
    transform: translate3d(0, 82px, 0) rotateZ(0) scale(1);
  }
}

@keyframes receipt-printer-vibration {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(0.25px); }
}

@keyframes receipt-roller-feed {
  from { background-position-x: 0; }
  to { background-position-x: 24px; }
}

@keyframes receipt-paper-lamp {
  0%, 100% {
    opacity: 0;
    box-shadow: none;
  }
  50% {
    opacity: 1;
    box-shadow: 0 0 7px rgb(var(--printer-paper-led) / 0.7);
  }
}

@keyframes receipt-button-invite {
  0%, 100% {
    box-shadow: 0 0 0 0 rgb(var(--color-terracotta) / 0.24);
  }
  50% {
    box-shadow: 0 0 0 7px rgb(var(--color-terracotta) / 0);
  }
}

@keyframes receipt-cutter-kick {
  0%, 100% { transform: translateY(0); }
  45% { transform: translateY(1.5px); }
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
    min-height: 1260px;
    padding-top: 176px;
    padding-right: 12px;
    padding-left: 12px;
    border-radius: 10px;
  }

  .receipt-printer {
    width: 100%;
  }

  .receipt-printer-rig {
    top: 14px;
    width: max(510px, calc(100% - 8px));
  }

  .receipt-printer-stage::after {
    top: 184px;
  }

}

@media (prefers-reduced-motion: reduce) {
  .receipt-feed--printing,
  .receipt-feed--detaching,
  .receipt-printer-stage--printing .receipt-printer,
  .receipt-printer-stage--printing .receipt-printer__roller,
  .receipt-printer-stage--printing .receipt-printer__active-lamp,
  .receipt-printer-stage--detaching .receipt-printer,
  .receipt-printer__print-button::after,
  .usage-receipt-preview__status--active {
    animation-duration: 1ms;
    animation-delay: 0ms;
  }
}
</style>
