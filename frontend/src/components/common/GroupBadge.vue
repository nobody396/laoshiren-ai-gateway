<template>
  <span
    :class="[
      'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium transition-colors',
      badgeClass
    ]"
  >
    <!-- The model family is the visual identity; protocol is secondary text. -->
    <ModelIcon v-if="displayModel" :model="displayModel" size="16px" />
    <PlatformIcon v-else-if="platform" :platform="platform" size="sm" />
    <!-- Group name -->
    <span class="truncate">{{ displayName }}</span>
    <!-- Right side label -->
    <span v-if="showLabel" :class="labelClass">
      <template v-if="hasCustomRate">
        <!-- 原倍率删除线 + 专属倍率高亮 -->
        <span class="line-through opacity-50 mr-0.5">{{ rateMultiplier }}x</span>
        <span class="font-bold">{{ userRateMultiplier }}x</span>
      </template>
      <template v-else>
        {{ labelText }}
      </template>
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionType, GroupPlatform } from '@/types'
import { publicGroupDisplayName } from '@/utils/groupDisplayName'
import PlatformIcon from './PlatformIcon.vue'
import ModelIcon from './ModelIcon.vue'

interface Props {
  name: string
  platform?: GroupPlatform
  displayModel?: string
  subscriptionType?: SubscriptionType
  rateMultiplier?: number
  userRateMultiplier?: number | null // 用户专属倍率
  showRate?: boolean
  daysRemaining?: number | null // 剩余天数（订阅类型时使用）
}

const props = withDefaults(defineProps<Props>(), {
  subscriptionType: 'standard',
  showRate: true,
  daysRemaining: null,
  userRateMultiplier: null
})

const { t } = useI18n()

const isSubscription = computed(() => props.subscriptionType === 'subscription' || props.subscriptionType === 'credit')
const displayName = computed(() =>
  isSubscription.value ? publicGroupDisplayName(props.name) : props.name
)

// 是否有专属倍率（且与默认倍率不同）
const hasCustomRate = computed(() => {
  return (
    props.userRateMultiplier !== null &&
    props.userRateMultiplier !== undefined &&
    props.rateMultiplier !== undefined &&
    props.userRateMultiplier !== props.rateMultiplier
  )
})

// 是否显示右侧标签
const showLabel = computed(() => {
  if (!props.showRate) return false
  // 订阅类型：显示天数或"订阅"
  if (isSubscription.value) return true
  // 标准类型：显示倍率（包括专属倍率）
  return props.rateMultiplier !== undefined || hasCustomRate.value
})

// Label text
const labelText = computed(() => {
  if (isSubscription.value) {
    // 如果有剩余天数，显示天数
    if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
      if (props.daysRemaining <= 0) {
        return t('admin.users.expired')
      }
      return t('admin.users.daysRemaining', { days: props.daysRemaining })
    }
    if (props.subscriptionType === 'credit') {
      return 'Credits'
    }
    // 否则显示"订阅"
    return t('groups.subscription')
  }
  return props.rateMultiplier !== undefined ? `${props.rateMultiplier}x` : ''
})

// Label style based on type and days remaining
const labelClass = computed(() => {
  const base = 'px-1.5 py-0.5 rounded text-[10px] font-semibold'

  if (!isSubscription.value) {
    // Standard: subtle background (不再为专属倍率使用不同的背景色)
    return `${base} bg-black/10 dark:bg-white/10`
  }

  // 订阅类型：根据剩余天数显示不同颜色
  if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
    if (props.daysRemaining <= 0 || props.daysRemaining <= 3) {
      // 已过期或紧急（<=3天）：红色
      return `${base} bg-red-200/80 text-red-800 dark:bg-red-800/50 dark:text-red-300`
    }
    if (props.daysRemaining <= 7) {
      // 警告（<=7天）：橙色
      return `${base} bg-amber-200/80 text-amber-800 dark:bg-amber-800/50 dark:text-amber-300`
    }
  }

  if (props.displayModel) return `${base} bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300`

  // Legacy callers without a model identity keep their platform theme.
  if (props.platform === 'anthropic') {
    return `${base} bg-orange-200/60 text-orange-800 dark:bg-orange-800/40 dark:text-orange-300`
  }
  if (props.platform === 'openai') {
    return `${base} bg-emerald-200/60 text-emerald-800 dark:bg-emerald-800/40 dark:text-emerald-300`
  }
  if (props.platform === 'gemini') {
    return `${base} bg-blue-200/60 text-blue-800 dark:bg-blue-800/40 dark:text-blue-300`
  }
  if (props.platform === 'gpt-image') {
    return `${base} bg-yellow-200/70 text-yellow-800 dark:bg-yellow-800/40 dark:text-yellow-300`
  }
  if (props.platform === 'grok') {
    return `${base} bg-zinc-300/70 text-zinc-800 dark:bg-zinc-700/60 dark:text-zinc-200`
  }
  return `${base} bg-violet-200/60 text-violet-800 dark:bg-violet-800/40 dark:text-violet-300`
})

// Badge color based on platform and subscription type
const badgeClass = computed(() => {
  if (props.displayModel) {
    const model = props.displayModel.toLowerCase()
    if (model.startsWith('claude')) return 'bg-orange-50 text-orange-800 dark:bg-orange-950/30 dark:text-orange-300'
    if (model.startsWith('glm')) return 'bg-sky-50 text-sky-800 dark:bg-sky-950/30 dark:text-sky-300'
    if (model.startsWith('deepseek')) return 'bg-blue-50 text-blue-800 dark:bg-blue-950/30 dark:text-blue-300'
    if (model.startsWith('qwen')) return 'bg-indigo-50 text-indigo-800 dark:bg-indigo-950/30 dark:text-indigo-300'
    if (model.startsWith('minimax')) return 'bg-rose-50 text-rose-800 dark:bg-rose-950/30 dark:text-rose-300'
    if (model.startsWith('gemini')) return 'bg-blue-50 text-blue-800 dark:bg-blue-950/30 dark:text-blue-300'
    if (model.startsWith('gpt')) return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-300'
    if (model.startsWith('grok') || model.startsWith('kimi')) return 'bg-zinc-100 text-zinc-800 dark:bg-zinc-800 dark:text-zinc-200'
    return 'bg-gray-50 text-gray-800 dark:bg-dark-800 dark:text-dark-200'
  }

  if (props.platform === 'anthropic') {
    // Claude: orange theme
    return isSubscription.value
      ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
      : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
  } else if (props.platform === 'openai') {
    // OpenAI: green theme
    return isSubscription.value
      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
      : 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
  }
  if (props.platform === 'gemini') {
    return isSubscription.value
      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
      : 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-400'
  }
  if (props.platform === 'gpt-image') {
    return isSubscription.value
      ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
      : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
  }
  if (props.platform === 'grok') {
    return isSubscription.value
      ? 'bg-zinc-200 text-zinc-800 dark:bg-zinc-700 dark:text-zinc-100'
      : 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200'
  }
  // Fallback: original colors
  return isSubscription.value
    ? 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'
    : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
})
</script>
