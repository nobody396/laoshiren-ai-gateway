<template>
  <div v-if="shouldRender" class="card p-6 space-y-5">
    <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('user.savings.title') }}</h3>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <!-- 我的累计消费 -->
      <div class="rounded-lg bg-gray-50 dark:bg-dark-800 p-4 text-center">
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('user.savings.totalSpent') }}</p>
        <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">¥{{ formatCny(dActual) }}</p>
      </div>

      <!-- 官方价（刊例价 USD × 汇率，纯展示层换算） -->
      <div class="rounded-lg bg-gray-50 dark:bg-dark-800 p-4 text-center">
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('user.savings.officialPrice') }}</p>
        <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">¥{{ formatCny(dOfficial) }}</p>
      </div>

      <!-- 已为你节省（hero） -->
      <div class="rounded-lg bg-green-50 dark:bg-green-950/40 p-4 text-center">
        <p class="text-xs text-green-600 dark:text-green-400">{{ t('user.savings.saved') }}</p>
        <p class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">¥{{ formatCny(dSaved) }}</p>
        <p v-if="discountText" class="mt-1 text-xs text-green-500 dark:text-green-500">{{ discountText }}</p>
      </div>

      <!-- 累计已返余额 -->
      <div class="rounded-lg bg-gray-50 dark:bg-dark-800 p-4 text-center">
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('user.savings.totalRebate') }}</p>
        <p class="mt-1 text-2xl font-bold text-primary-600 dark:text-primary-400">¥{{ formatCny(dRebate) }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI, type UserDashboardStats } from '@/api/usage'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const stats = ref<UserDashboardStats | null>(null)

// 数字口径：total_actual_cost / total_customer_rebate 单位为人民币元（1 余额单位 = 1 元）；
// total_cost 为官方刊例价 USD，官方价 = total_cost × 汇率，仅在此展示层做 USD→CNY 换算。
const exchangeRate = computed(() => {
  const rate = Number(appStore.cachedPublicSettings?.landing_pricing_exchange_rate)
  return Number.isFinite(rate) && rate > 0 ? rate : 7
})

const actualCny = computed(() => stats.value?.total_actual_cost ?? 0)
const officialCny = computed(() => (stats.value?.total_cost ?? 0) * exchangeRate.value)
const savedCny = computed(() => Math.max(0, officialCny.value - actualCny.value))
const rebateCny = computed(() => stats.value?.total_customer_rebate ?? 0)

// 无消费且无返利记录时整卡不渲染
const shouldRender = computed(() => stats.value !== null && ((stats.value?.total_cost ?? 0) > 0 || rebateCny.value > 0))

// ---- 数字动画 ----
// 每天首次加载：四个数字从 0 快速滚到目标值；之后当天再进页面直接显示。
// 自动刷新（5s）拿到新值时：数字从旧值平滑滚动到新值（只增不减的场景就是"往上爬"）。
const dActual = ref(0)
const dOfficial = ref(0)
const dSaved = ref(0)
const dRebate = ref(0)

const rafIds = new Map<object, number>()
function cancelTween(r: object) {
  const id = rafIds.get(r)
  if (id !== undefined) {
    cancelAnimationFrame(id)
    rafIds.delete(r)
  }
}
function tweenTo(r: { value: number }, to: number, duration: number) {
  cancelTween(r)
  const from = r.value
  if (from === to || duration <= 0) {
    r.value = to
    return
  }
  const start = performance.now()
  const step = (now: number) => {
    const p = Math.min(1, (now - start) / duration)
    const eased = 1 - Math.pow(1 - p, 4) // easeOutQuart：起步很快，越接近目标越慢
    r.value = from + (to - from) * eased
    if (p < 1) {
      rafIds.set(r, requestAnimationFrame(step))
    } else {
      r.value = to
      rafIds.delete(r)
    }
  }
  rafIds.set(r, requestAnimationFrame(step))
}

const COUNTUP_KEY = 'savings-countup-date'
function shouldCountUp(): boolean {
  try {
    const today = new Date().toISOString().slice(0, 10)
    if (localStorage.getItem(COUNTUP_KEY) !== today) {
      localStorage.setItem(COUNTUP_KEY, today)
      return true
    }
  } catch {
    // localStorage 不可用则每次都滚动，无伤大雅
    return true
  }
  return false
}

const initialized = ref(false)
watch([actualCny, officialCny, savedCny, rebateCny], ([a, o, s, r]) => {
  if (stats.value === null) return
  if (!initialized.value) {
    initialized.value = true
    const duration = shouldCountUp() ? 1600 : 0
    tweenTo(dActual, a, duration)
    tweenTo(dOfficial, o, duration)
    tweenTo(dSaved, s, duration)
    tweenTo(dRebate, r, duration)
  } else {
    tweenTo(dActual, a, 700)
    tweenTo(dOfficial, o, 700)
    tweenTo(dSaved, s, 700)
    tweenTo(dRebate, r, 700)
  }
})

// 相当于 N 折 = 实际消费 / 官方价 × 10；官方价为 0 时不显示
const discountText = computed(() => {
  if (dOfficial.value <= 0) return ''
  const discount = (dActual.value / dOfficial.value) * 10
  return t('user.savings.discountEquivalent', {
    discount: discount.toFixed(1),
    percent: (discount * 10).toFixed(0)
  })
})

const formatCny = (value: number) => value.toFixed(2)

// ---- 数据加载与自动刷新 ----
const POLL_INTERVAL_MS = 5000
let pollTimer: number | undefined

async function loadStats() {
  try {
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    // 刷新失败保留旧数字，不打扰用户
    console.error('Failed to load savings stats:', error)
  }
}

onMounted(async () => {
  appStore.fetchPublicSettings()
  await loadStats()
  pollTimer = window.setInterval(loadStats, POLL_INTERVAL_MS)
})

onUnmounted(() => {
  if (pollTimer !== undefined) window.clearInterval(pollTimer)
  cancelTween(dActual)
  cancelTween(dOfficial)
  cancelTween(dSaved)
  cancelTween(dRebate)
})
</script>
