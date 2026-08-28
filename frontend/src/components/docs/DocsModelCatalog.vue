<template>
  <section class="space-y-6">
    <header class="space-y-2">
      <h1 class="text-3xl font-bold text-gray-950 dark:text-white">模型目录</h1>
      <p class="text-gray-600 dark:text-dark-300">
        同一个模型可以由多个分组提供。请根据预算选择对应分组。
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
        placeholder="搜索模型 ID 或可用分组"
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
            {{ model.plans.length }} 个可用分组
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

    <section class="space-y-3 border-t border-gray-200 pt-6 dark:border-dark-700" aria-labelledby="monthly-plan-heading">
      <div>
        <h2 id="monthly-plan-heading" class="text-xl font-bold text-gray-950 dark:text-white">月卡 Plus / Pro / Max</h2>
        <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">
          一张月卡同时开放对应档位的 GPT、Claude 和 Grok 月卡分组，三类分组共用同一份 31 天额度。
        </p>
      </div>

      <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
        <table class="min-w-full divide-y divide-gray-200 text-left text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-gray-600 dark:bg-dark-800 dark:text-dark-300">
            <tr>
              <th class="px-4 py-3 font-semibold">档位</th>
              <th class="px-4 py-3 font-semibold">31 天额度</th>
              <th class="px-4 py-3 font-semibold">购买页价格</th>
              <th class="px-4 py-3 font-semibold">站内直付价</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 bg-white text-gray-700 dark:divide-dark-800 dark:bg-dark-900 dark:text-dark-200">
            <tr v-for="plan in monthlyCreditCardPlans" :key="plan.id">
              <td class="px-4 py-3 font-semibold text-gray-950 dark:text-white">{{ plan.name }}</td>
              <td class="px-4 py-3">{{ plan.displayMonthlyCreditsText }} AI credits</td>
              <td class="px-4 py-3">{{ plan.price }}</td>
              <td class="px-4 py-3">{{ plan.directPrice }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <p class="text-xs text-gray-500 dark:text-dark-400">
        创建 Key 时请选择准备使用的模型系列和档位；每把 Key 只使用所选分组的模型、倍率和共享额度。价格与权益以购买页实时显示为准。
      </p>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import { getPublicModelPricing } from '@/api/publicPricing'
import type { PublicModelPricingCatalog } from '@/api/publicPricing'
import { monthlyCreditCardPlans } from '@/constants/monthlyCreditCards'

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
    const addPlan = (modelID: string, protocol: string) => {
      const current = byID.get(modelID) ?? { protocols: new Set<string>(), plans: [] }
      current.protocols.add(protocol)
      if (!current.plans.some((plan) => plan.groupId === group.group_id)) {
        current.plans.push({
          groupId: group.group_id,
          name: group.name,
          multiplier: group.rate_multiplier,
        })
      }
      byID.set(modelID, current)
    }

    for (const model of group.models ?? []) {
      if (model.disabled) continue
      addPlan(model.model, protocolLabel(group.platform))
    }

    if (group.image_generation && (group.models ?? []).length === 0) {
      addPlan('gpt-image-2', 'Images API')
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
  if (value === 'gpt-image-2') return 'Images API'
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
