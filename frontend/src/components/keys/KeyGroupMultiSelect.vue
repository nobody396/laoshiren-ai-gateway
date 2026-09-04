<template>
  <section class="space-y-3" aria-label="授权分组与优先顺序">
    <p class="text-sm text-gray-600 dark:text-dark-300">已选 {{ modelValue.length }} 个分组。同模型按下方顺序匹配；不会因为失败或月卡额度耗尽自动转扣其他分组。</p>
    <div class="flex items-center gap-3">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="disabled" @click="emit('update:modelValue', defaultKeyGroupIds(groups))">全选可用分组</button>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="disabled" @click="emit('update:modelValue', [])">清空</button>
    </div>
    <ol v-if="modelValue.length" class="max-h-56 space-y-2 overflow-y-auto rounded-lg border border-gray-200 p-3 dark:border-dark-600" aria-label="分组优先级">
      <li v-for="(id, index) in modelValue" :key="id" class="flex items-center gap-2 text-sm">
        <span class="text-gray-500">{{ index + 1 }}.</span>
        <span class="min-w-0 flex-1">{{ nameFor(id) }}</span>
        <button type="button" class="btn btn-secondary btn-sm" :aria-label="`上移 ${nameFor(id)}`" :disabled="disabled || index === 0" @click="emit('update:modelValue', moveKeyGroup(modelValue, id, -1))">↑</button>
        <button type="button" class="btn btn-secondary btn-sm" :aria-label="`下移 ${nameFor(id)}`" :disabled="disabled || index === modelValue.length - 1" @click="emit('update:modelValue', moveKeyGroup(modelValue, id, 1))">↓</button>
      </li>
    </ol>
    <div class="max-h-64 space-y-2 overflow-y-auto rounded-lg border border-gray-200 p-3 dark:border-dark-600">
      <label v-for="group in groups" :key="group.id" class="flex cursor-pointer items-start gap-3 rounded p-2 hover:bg-gray-50 dark:hover:bg-dark-700">
        <input type="checkbox" :disabled="disabled" :checked="modelValue.includes(group.id)" :aria-label="publicGroupDisplayName(group.name)" @change="toggle(group.id)" />
        <span class="flex-1 text-sm">{{ publicGroupDisplayName(group.name) }}<span class="ml-2 text-gray-500">{{ group.subscription_type === 'subscription' || group.subscription_type === 'credit' ? '订阅权益' : `${rates[group.id] ?? group.rate_multiplier}× 分组倍率` }}</span></span>
      </label>
    </div>
    <p v-if="modelValue.some(id => !groups.some(group => group.id === id))" class="text-sm text-amber-700 dark:text-amber-300">部分原授权分组已不可用。请核对后重新全选，或保留原设置退出。</p>
    <p class="text-xs text-gray-500">默认优先已有订阅；新增分组不会自动加入。当前多分组支持文本 HTTP 接口；图片、视频和实时连接请继续使用单分组 Key。</p>
  </section>
</template>

<script setup lang="ts">
import type { Group } from '@/types'
import { publicGroupDisplayName } from '@/utils/groupDisplayName'
import { defaultKeyGroupIds, moveKeyGroup } from '@/utils/keyGroupSelection'
const props = withDefaults(defineProps<{ modelValue: number[]; groups: Group[]; rates?: Record<number, number>; disabled?: boolean }>(), { rates: () => ({}) })
const emit = defineEmits<{ 'update:modelValue': [number[]] }>()
const nameFor = (id: number) => {
  const group = props.groups.find(item => item.id === id)
  return group ? publicGroupDisplayName(group.name) : `原分组 #${id}（已不可用）`
}
const toggle = (id: number) => emit('update:modelValue', props.modelValue.includes(id) ? props.modelValue.filter(item => item !== id) : [...props.modelValue, id])
</script>
