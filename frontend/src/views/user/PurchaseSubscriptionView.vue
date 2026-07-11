<template>
  <AppLayout>
    <div class="purchase-page-layout">
      <div v-if="loading" class="card flex-1 flex items-center justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <!-- Stripe Payment Mode -->
      <template v-else-if="stripeEnabled">
        <!-- Success Banner -->
        <div
          v-if="paymentSuccess"
          class="mb-4 rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20"
        >
          <div class="flex items-center gap-2">
            <Icon name="check" size="sm" class="text-green-600 dark:text-green-400" />
            <span class="text-sm font-medium text-green-800 dark:text-green-200">
              {{ t('purchase.stripe.paymentSuccess') }}
            </span>
          </div>
        </div>

        <div class="grid gap-6 lg:grid-cols-2">
          <!-- Balance Card -->
          <div class="card p-6">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('purchase.stripe.currentBalance') }}
            </h3>
            <p class="mt-2 text-3xl font-bold text-primary-600 dark:text-primary-400">
              ${{ currentBalance.toFixed(2) }}
            </p>
          </div>

          <!-- Price Tiers -->
          <div class="card p-6">
            <h3 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('purchase.stripe.topUp') }}
            </h3>
            <div class="grid grid-cols-2 gap-3">
              <button
                v-for="tier in priceTiers"
                :key="tier.amount_cents"
                :disabled="checkoutLoading"
                class="rounded-lg border-2 border-gray-200 p-4 text-center transition-all hover:border-primary-500 hover:shadow-md dark:border-dark-600 dark:hover:border-primary-500"
                :class="{ 'opacity-50 cursor-not-allowed': checkoutLoading }"
                @click="handleCheckout(tier)"
              >
                <div class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ tier.label }}
                </div>
                <div class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  {{ tier.credits }} {{ t('purchase.stripe.credits') }}
                </div>
              </button>
            </div>
            <p v-if="checkoutError" class="mt-3 text-sm text-red-600 dark:text-red-400">
              {{ checkoutError }}
            </p>
          </div>
        </div>

        <!-- Payment History -->
        <div class="card mt-6 p-6">
          <h3 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('purchase.stripe.history') }}
          </h3>
          <div v-if="paymentHistory.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('purchase.stripe.noHistory') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-200 dark:border-dark-600">
                  <th class="pb-2 text-left font-medium text-gray-500 dark:text-dark-400">{{ t('purchase.stripe.orderNo') }}</th>
                  <th class="pb-2 text-left font-medium text-gray-500 dark:text-dark-400">{{ t('purchase.stripe.amount') }}</th>
                  <th class="pb-2 text-left font-medium text-gray-500 dark:text-dark-400">{{ t('purchase.stripe.credits') }}</th>
                  <th class="pb-2 text-left font-medium text-gray-500 dark:text-dark-400">{{ t('purchase.stripe.status') }}</th>
                  <th class="pb-2 text-left font-medium text-gray-500 dark:text-dark-400">{{ t('purchase.stripe.time') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="order in paymentHistory"
                  :key="order.id"
                  class="border-b border-gray-100 dark:border-dark-700"
                >
                  <td class="py-2 font-mono text-xs">{{ order.order_no }}</td>
                  <td class="py-2">${{ (order.amount_cents / 100).toFixed(2) }}</td>
                  <td class="py-2">{{ order.credits.toFixed(2) }}</td>
                  <td class="py-2">
                    <span
                      class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium"
                      :class="statusClass(order.status)"
                    >
                      {{ t(`purchase.stripe.statusLabel.${order.status}`) }}
                    </span>
                  </td>
                  <td class="py-2 text-xs text-gray-500 dark:text-dark-400">
                    {{ formatTime(order.created_at) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>

      <!-- Legacy iframe Mode -->
      <template v-else>
        <div class="card flex-1 min-h-0 overflow-hidden">
          <div
            v-if="!purchaseEnabled"
            class="flex h-full items-center justify-center p-10 text-center"
          >
            <div class="max-w-md">
              <div
                class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
              >
                <Icon name="creditCard" size="lg" class="text-gray-400" />
              </div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('purchase.notEnabledTitle') }}
              </h3>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                {{ t('purchase.notEnabledDesc') }}
              </p>
            </div>
          </div>

          <div
            v-else-if="!isValidUrl"
            class="flex h-full items-center justify-center p-10 text-center"
          >
            <div class="max-w-md">
              <div
                class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
              >
                <Icon name="link" size="lg" class="text-gray-400" />
              </div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('purchase.notConfiguredTitle') }}
              </h3>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                {{ t('purchase.notConfiguredDesc') }}
              </p>
            </div>
          </div>

          <div v-else class="purchase-embed-shell">
            <a
              :href="purchaseUrl"
              target="_blank"
              rel="noopener noreferrer"
              referrerpolicy="no-referrer"
              class="btn btn-secondary btn-sm purchase-open-fab"
            >
              <Icon name="externalLink" size="sm" class="mr-1.5" :stroke-width="2" />
              {{ t('purchase.openInNewTab') }}
            </a>
            <iframe
              :src="purchaseUrl"
              class="purchase-embed-frame"
              referrerpolicy="no-referrer"
              sandbox="allow-scripts allow-forms allow-popups allow-same-origin"
              allow="payment"
            ></iframe>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { buildEmbeddedUrl, detectTheme } from '@/utils/embedded-url'
import { getPriceTiers, createCheckoutSession, getPaymentHistory } from '@/api/payment'
import type { PriceTier, PaymentOrder } from '@/api/payment'

const { t, locale } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(false)
const checkoutLoading = ref(false)
const checkoutError = ref('')
const paymentSuccess = ref(false)
const priceTiers = ref<PriceTier[]>([])
const paymentHistory = ref<PaymentOrder[]>([])
const purchaseTheme = ref<'light' | 'dark'>('light')
let themeObserver: MutationObserver | null = null

const stripeEnabled = computed(() => {
  return appStore.cachedPublicSettings?.stripe_enabled ?? false
})

const purchaseEnabled = computed(() => {
  return appStore.cachedPublicSettings?.purchase_subscription_enabled ?? false
})

const currentBalance = computed(() => {
  return authStore.user?.balance ?? 0
})

const purchaseUrl = computed(() => {
  const baseUrl = (appStore.cachedPublicSettings?.purchase_subscription_url || '').trim()
  return buildEmbeddedUrl(baseUrl, purchaseTheme.value, locale.value)
})

const isValidUrl = computed(() => {
  const url = purchaseUrl.value
  return url.startsWith('https://') || (import.meta.env.DEV && url.startsWith('http://'))
})

function statusClass(status: string): string {
  switch (status) {
    case 'completed':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    case 'pending':
      return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'expired':
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-400'
    case 'failed':
      return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-400'
  }
}

function formatTime(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

async function handleCheckout(tier: PriceTier) {
  checkoutLoading.value = true
  checkoutError.value = ''
  try {
    const currentUrl = window.location.href.split('?')[0]
    const result = await createCheckoutSession(
      tier.amount_cents,
      `${currentUrl}?status=success`,
      `${currentUrl}?status=cancel`
    )
    window.location.href = result.checkout_url
  } catch (e: any) {
    checkoutError.value = e?.message || 'Failed to create checkout session'
  } finally {
    checkoutLoading.value = false
  }
}

async function loadStripeData() {
  try {
    const [tiers, history] = await Promise.all([
      getPriceTiers(),
      getPaymentHistory()
    ])
    priceTiers.value = tiers
    paymentHistory.value = history
  } catch {
    // Silently fail, UI shows empty state
  }
}

onMounted(async () => {
  purchaseTheme.value = detectTheme()

  if (typeof document !== 'undefined') {
    themeObserver = new MutationObserver(() => {
      purchaseTheme.value = detectTheme()
    })
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })
  }

  if (!appStore.publicSettingsLoaded) {
    loading.value = true
    try {
      await appStore.fetchPublicSettings()
    } finally {
      loading.value = false
    }
  }

  // Check for payment callback
  if (route.query.status === 'success') {
    paymentSuccess.value = true
    // Refresh user profile to get updated balance
    await authStore.refreshUser()
  }

  // Load Stripe data if enabled
  if (appStore.cachedPublicSettings?.stripe_enabled) {
    await loadStripeData()
  }
})

onUnmounted(() => {
  if (themeObserver) {
    themeObserver.disconnect()
    themeObserver = null
  }
})
</script>

<style scoped>
.purchase-page-layout {
  @apply flex flex-col;
  min-height: calc(100vh - 64px - 4rem);
}

.purchase-embed-shell {
  @apply relative;
  @apply h-full w-full overflow-hidden rounded-2xl;
  @apply bg-gradient-to-b from-gray-50 to-white dark:from-dark-900 dark:to-dark-950;
  @apply p-0;
}

.purchase-open-fab {
  @apply absolute right-3 top-3 z-10;
  @apply shadow-sm backdrop-blur supports-[backdrop-filter]:bg-white/80 dark:supports-[backdrop-filter]:bg-dark-800/80;
}

.purchase-embed-frame {
  display: block;
  margin: 0;
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: 0;
  box-shadow: none;
  background: transparent;
}
</style>
