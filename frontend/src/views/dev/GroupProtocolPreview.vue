<template>
  <main class="min-h-screen bg-[#f7f7f5] px-5 py-16 text-gray-950">
    <section class="mx-auto max-w-xl rounded-2xl border border-gray-200 bg-white p-6 shadow-[0_24px_70px_-48px_rgba(15,23,42,0.45)]">
      <header class="mb-6 border-b border-gray-100 pb-5">
        <p class="text-xs font-semibold tracking-[0.12em] text-gray-400">本地界面预览</p>
        <h1 class="mt-2 text-2xl font-semibold tracking-tight">新建 API Key</h1>
        <p class="mt-1 text-sm text-gray-500">使用当前真实国产模型分组与实际分组组件。模型图标负责识别，协议只作文字说明。</p>
      </header>

      <label class="mb-2 block text-sm font-medium text-gray-700">选择分组</label>
      <Select
        v-model="selected"
        :options="options"
        :searchable="true"
        search-placeholder="搜索分组"
        aria-label="选择分组"
      >
        <template #selected="{ option }">
          <GroupBadge
            v-if="option"
            :name="(option as PreviewOption).label"
            :platform="(option as PreviewOption).platform"
            :display-model="(option as PreviewOption).displayModel"
            :rate-multiplier="(option as PreviewOption).rate"
          />
        </template>
        <template #option="{ option, selected: optionSelected }">
          <GroupOptionItem
            :name="(option as PreviewOption).label"
            :platform="(option as PreviewOption).platform"
            :display-model="(option as PreviewOption).displayModel"
            :rate-multiplier="(option as PreviewOption).rate"
            :protocol-label="groupDisplayProtocolLabel((option as PreviewOption).displayProtocol)"
            :selected="optionSelected"
          />
        </template>
      </Select>

      <p v-if="loading" class="mt-5 text-sm text-gray-400">正在读取当前公开分组…</p>
      <p v-else-if="error" class="mt-5 text-sm text-red-600">{{ error }}</p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import type { GroupPlatform } from '@/types'
import { getPublicModelPricing } from '@/api/publicPricing'
import {
  groupDisplayProtocolLabel,
  resolveGroupDisplayProtocol,
  type GroupDisplayProtocol
} from '@/utils/groupDisplayProtocol'
import { resolveGroupDisplayModel } from '@/utils/groupDisplayModel'

interface PreviewOption extends Record<string, unknown> {
  value: number
  label: string
  platform: GroupPlatform
  displayProtocol: GroupDisplayProtocol
  displayModel: string
  rate: number
}

const selected = ref<number | null>(null)
const options = ref<PreviewOption[]>([])
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    const catalog = await getPublicModelPricing()
    options.value = catalog.groups
      .filter((group) => /(?:GLM|DeepSeek|Kimi|MiniMax|Qwen)/i.test(group.name))
      .map((group) => {
      const platform = group.platform as GroupPlatform
      const models = (group.models ?? []).filter((model) => !model.disabled).map((model) => model.model)
      return {
        value: group.group_id,
        label: group.name,
        platform,
        displayProtocol: resolveGroupDisplayProtocol({ name: group.name, platform }),
        displayModel: resolveGroupDisplayModel({ name: group.name, platform, models }),
        rate: group.rate_multiplier
      }
      })
    selected.value = options.value.find((option) => option.label.includes('GLM'))?.value ?? options.value[0]?.value ?? null
  } catch {
    error.value = '当前公开分组读取失败。'
  } finally {
    loading.value = false
  }
})
</script>
