<template>
  <section class="space-y-6">
    <header class="space-y-2">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <h1 class="text-3xl font-bold text-gray-950 dark:text-white">模型目录</h1>
        <div class="flex gap-2 text-xs font-semibold">
          <a class="rounded-lg border border-gray-200 px-3 py-2 text-gray-600 hover:text-gray-950 dark:border-dark-700 dark:text-dark-300 dark:hover:text-white" href="/docs/models.md" target="_blank" rel="noopener noreferrer">查看 Markdown</a>
          <a class="rounded-lg border border-gray-200 px-3 py-2 text-gray-600 hover:text-gray-950 dark:border-dark-700 dark:text-dark-300 dark:hover:text-white" href="/docs/llms.txt" target="_blank" rel="noopener noreferrer">llms.txt</a>
        </div>
      </div>
      <p class="text-gray-600 dark:text-dark-300">
        一个模型只出现一次。不同分组只是不同的可用方案。
      </p>
      <p v-if="catalog" class="text-xs text-gray-500 dark:text-dark-400">
        更新于 {{ formatUpdatedAt(catalog.updated_at) }}
      </p>
    </header>

    <label class="block">
      <span class="sr-only">搜索模型</span>
      <input
        v-model.trim="query"
        type="search"
        class="w-full rounded-xl border border-gray-200 bg-white px-4 py-3 text-sm text-gray-900 outline-none transition focus:border-primary-400 focus:ring-2 focus:ring-primary-100 dark:border-dark-700 dark:bg-dark-900 dark:text-white dark:focus:border-primary-500 dark:focus:ring-primary-900/30"
        placeholder="搜索模型 ID 或可用方案"
      />
    </label>

    <div v-if="loading" class="rounded-xl border border-gray-200 p-6 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
      正在读取实时模型目录…
    </div>

    <div v-else-if="error" class="rounded-xl border border-red-200 p-6 text-sm text-red-700 dark:border-red-900/50 dark:text-red-300">
      <p>{{ error }}</p>
      <button type="button" class="mt-3 font-semibold underline" @click="load">重新加载</button>
    </div>

    <div v-else class="space-y-3">
      <article
        v-for="model in filteredModels"
        :key="model.id"
        class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="flex min-w-0 items-center gap-3">
            <ModelIcon :model="model.id" size="22px" />
            <div class="min-w-0">
              <h2 class="break-all text-base font-semibold text-gray-950 dark:text-white">{{ model.id }}</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ model.protocols.join(' / ') }} · 推荐 {{ model.recommendedTool }}
              </p>
            </div>
          </div>
          <span class="rounded-full bg-primary-50 px-2.5 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">
            {{ model.plans.length }} 个可用方案
          </span>
        </div>

        <ul class="mt-4 grid gap-2 sm:grid-cols-2">
          <li
            v-for="plan in model.plans"
            :key="`${model.id}-${plan.groupId}`"
            class="flex items-center justify-between gap-3 rounded-lg bg-gray-50 px-3 py-2 text-xs dark:bg-dark-800"
          >
            <span class="min-w-0 truncate text-gray-700 dark:text-dark-200">{{ plan.name }}</span>
            <span class="shrink-0 font-mono text-gray-500 dark:text-dark-400">{{ formatMultiplier(plan.multiplier) }}</span>
          </li>
        </ul>
      </article>

      <p v-if="filteredModels.length === 0" class="rounded-xl border border-gray-200 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
        没有匹配的模型。
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import { getPublicModelPricing } from '@/api/publicPricing'
import type { PublicModelPricingCatalog } from '@/api/publicPricing'

interface AvailabilityPlan {
  groupId: number
  name: string
  multiplier: number
}

interface LogicalModel {
  id: string
  protocols: string[]
  recommendedTool: string
  plans: AvailabilityPlan[]
}

const catalog = ref<PublicModelPricingCatalog | null>(null)
const loading = ref(true)
const error = ref('')
const query = ref('')

const models = computed<LogicalModel[]>(() => {
  const byID = new Map<string, { protocols: Set<string>; plans: AvailabilityPlan[] }>()
  for (const group of catalog.value?.groups ?? []) {
    for (const model of group.models ?? []) {
      if (model.disabled) continue
      const current = byID.get(model.model) ?? { protocols: new Set<string>(), plans: [] }
      current.protocols.add(protocolLabel(group.platform))
      if (!current.plans.some((plan) => plan.groupId === group.group_id)) {
        current.plans.push({
          groupId: group.group_id,
          name: group.name,
          multiplier: group.rate_multiplier,
        })
      }
      byID.set(model.model, current)
    }
  }

  return [...byID.entries()]
    .map(([id, value]) => ({
      id,
      protocols: [...value.protocols].sort(),
      recommendedTool: recommendedTool(id),
      plans: value.plans.sort((a, b) => a.groupId - b.groupId),
    }))
    .sort((a, b) => a.id.localeCompare(b.id))
})

const filteredModels = computed(() => {
  const needle = query.value.toLocaleLowerCase()
  if (!needle) return models.value
  return models.value.filter((model) =>
    model.id.toLocaleLowerCase().includes(needle)
    || model.recommendedTool.toLocaleLowerCase().includes(needle)
    || model.plans.some((plan) => plan.name.toLocaleLowerCase().includes(needle))
  )
})

function protocolLabel(platform: string): string {
  if (platform === 'anthropic') return 'Anthropic Messages'
  if (platform === 'gemini') return 'Gemini'
  if (platform === 'grok') return 'Grok / xAI'
  if (platform === 'antigravity') return 'Antigravity'
  return 'OpenAI'
}

function recommendedTool(model: string): string {
  const value = model.toLocaleLowerCase()
  if (value.startsWith('claude-')) return 'Claude Code'
  if (value.startsWith('grok-')) return 'Grok Build'
  if (value.startsWith('gemini-')) return 'Gemini CLI'
  if (value.startsWith('glm-')) return 'ZCode'
  if (value.startsWith('kimi-')) return 'Kimi Code'
  if (value.startsWith('gpt-') || value.includes('codex')) return 'Codex'
  return 'OpenCode'
}

function formatMultiplier(value: number): string {
  return `${Number(value.toFixed(4))}×`
}

function formatUpdatedAt(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    catalog.value = await getPublicModelPricing()
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : String(reason)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
