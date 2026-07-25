<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-4 border-b border-gray-100 px-6 py-5 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex items-start gap-4">
            <div class="flex h-12 w-12 items-center justify-center rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
              <span class="text-xl">¥</span>
            </div>
            <div>
              <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">成本核算</h1>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-300">
                <span v-if="overview">生成于 {{ formatTime(overview.generated_at) }}，真实用量窗口 {{ formatTime(overview.usage_window_start) }} ~ {{ formatTime(overview.usage_window_end) }}</span>
                <span v-else>加载中…</span>
              </p>
            </div>
          </div>
          <button
            type="button"
            class="inline-flex h-10 items-center justify-center rounded-lg border border-gray-200 px-4 text-sm font-medium text-gray-700 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700"
            :disabled="loading"
            @click="loadOverview"
          >
            刷新
          </button>
        </div>

        <div v-if="overview" class="border-b border-gray-100 px-6 py-3 text-xs text-gray-400 dark:border-dark-700">
          {{ overview.pricing_source_note }}
        </div>

        <div class="p-5">
          <div v-if="loading && !overview" class="space-y-4">
            <div v-for="idx in 3" :key="idx" class="h-56 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
          </div>

          <div v-else-if="overview" class="space-y-10">
            <!-- Monthly cards -->
            <section>
              <h2 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">月卡</h2>
              <div class="space-y-6">
                <article
                  v-for="plan in overview.monthly_cards"
                  :key="plan.id"
                  class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
                >
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <h3 class="text-xl font-semibold text-gray-900 dark:text-white">{{ plan.name }}</h3>
                    <div class="flex flex-wrap items-center gap-2 text-xs font-medium">
                      <span class="rounded-full bg-red-50 px-2.5 py-1 text-red-700 dark:bg-red-950/30 dark:text-red-300">
                        最差 {{ formatPercent(plan.margin_range.worst_percent) }}
                      </span>
                      <span class="rounded-full bg-amber-50 px-2.5 py-1 text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
                        保守 {{ formatPercent(plan.margin_range.conservative_percent) }}
                      </span>
                      <span class="rounded-full bg-emerald-50 px-2.5 py-1 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300">
                        最优 {{ formatPercent(plan.margin_range.best_percent) }}
                      </span>
                      <span
                        class="rounded-full px-2.5 py-1"
                        :class="plan.margin_range.real_percent != null
                          ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                          : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'"
                      >
                        真实 {{ plan.margin_range.real_percent != null ? formatPercent(plan.margin_range.real_percent) : '无数据' }}
                      </span>
                    </div>
                  </div>

                  <div class="mt-3 grid grid-cols-2 gap-x-6 gap-y-2 text-sm text-gray-600 dark:text-dark-300 lg:grid-cols-4">
                    <div>
                      <div class="text-xs text-gray-400">小铺价 (-{{ plan.shop_fee_percent }}%手续费)</div>
                      <div class="font-mono font-semibold text-gray-900 dark:text-white">
                        ¥{{ plan.shop_price_cny.toFixed(2) }} → ¥{{ plan.shop_net_price_cny.toFixed(2) }}
                      </div>
                    </div>
                    <div>
                      <div class="text-xs text-gray-400">微信/支付宝到手价</div>
                      <div class="font-mono font-semibold text-gray-900 dark:text-white">¥{{ plan.direct_price_cny.toFixed(2) }}</div>
                    </div>
                    <div>
                      <div class="text-xs text-gray-400">月度积分</div>
                      <div class="font-mono font-semibold text-gray-900 dark:text-white">{{ plan.monthly_credits.toFixed(0) }}</div>
                    </div>
                  </div>

                  <!-- Rates -->
                  <div class="mt-4 overflow-x-auto">
                    <table class="min-w-full text-sm">
                      <thead>
                        <tr class="border-b border-gray-100 text-left text-xs uppercase tracking-wide text-gray-400 dark:border-dark-700">
                          <th class="py-2 pr-4">产品</th>
                          <th class="py-2 pr-4">分组倍率</th>
                          <th class="py-2 pr-4">账号倍率(主用)</th>
                          <th class="py-2 pr-4">账号倍率(保守)</th>
                          <th class="py-2 pr-4">可调度账号</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="product in productKeys" :key="product" class="border-b border-gray-50 dark:border-dark-800">
                          <td class="py-2 pr-4 font-medium capitalize text-gray-700 dark:text-dark-200">{{ product }}</td>
                          <td class="py-2 pr-4">
                            <RateEditor
                              :group-id="plan.products[product].group_id"
                              :value="plan.products[product].group_rate_multiplier"
                              :saving="isSaving(plan.products[product].group_id)"
                              @save="onSaveRate"
                            />
                          </td>
                          <td class="py-2 pr-4 font-mono text-gray-600 dark:text-dark-300">
                            {{ plan.products[product].primary_account_rate_multiplier.toFixed(4) }}
                          </td>
                          <td class="py-2 pr-4 font-mono text-gray-600 dark:text-dark-300">
                            {{ plan.products[product].worst_account_rate_multiplier.toFixed(4) }}
                          </td>
                          <td class="py-2 pr-4 text-gray-600 dark:text-dark-300">{{ plan.products[product].schedulable_account_count }}</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>

                  <!-- Single-product scenarios -->
                  <div class="mt-4 overflow-x-auto">
                    <table class="min-w-full text-sm">
                      <thead>
                        <tr class="border-b border-gray-100 text-left text-xs uppercase tracking-wide text-gray-400 dark:border-dark-700">
                          <th class="py-2 pr-4">情景 (满额消耗)</th>
                          <th class="py-2 pr-4">成本/积分</th>
                          <th class="py-2 pr-4">成本</th>
                          <th class="py-2 pr-4">利润</th>
                          <th class="py-2 pr-4">毛利率</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr
                          v-for="product in productKeys"
                          :key="product"
                          class="border-b border-gray-50 dark:border-dark-800"
                        >
                          <td class="py-2 pr-4 text-gray-700 dark:text-dark-200">全部用 {{ product }}</td>
                          <template v-if="plan.single_product_scenarios['all_' + product]">
                            <td class="py-2 pr-4 font-mono text-gray-500 dark:text-dark-400">
                              {{ plan.single_product_scenarios['all_' + product].cost_per_credit_worst_account.toFixed(6) }}
                            </td>
                            <td class="py-2 pr-4 font-mono text-gray-600 dark:text-dark-300">
                              ¥{{ plan.single_product_scenarios['all_' + product].vs_direct_price.cost_cny.toFixed(2) }}
                            </td>
                            <td class="py-2 pr-4 font-mono" :class="marginTextClass(plan.single_product_scenarios['all_' + product].vs_direct_price.profit_cny)">
                              ¥{{ plan.single_product_scenarios['all_' + product].vs_direct_price.profit_cny.toFixed(2) }}
                            </td>
                            <td class="py-2 pr-4 font-mono" :class="marginTextClass(plan.single_product_scenarios['all_' + product].vs_direct_price.margin_percent)">
                              {{ formatPercent(plan.single_product_scenarios['all_' + product].vs_direct_price.margin_percent) }}
                            </td>
                          </template>
                        </tr>
                      </tbody>
                    </table>
                  </div>

                  <!-- Real usage -->
                  <div class="mt-4 rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                    <template v-if="plan.real_usage.available && plan.real_usage.observed_request_count">
                      本月真实用量：{{ plan.real_usage.observed_request_count }} 次请求，已耗积分
                      {{ plan.real_usage.observed_raw_credits_consumed?.toFixed(2) }}，产品占比
                      <span v-for="(pct, product) in plan.real_usage.product_mix_percent" :key="product" class="mr-2">
                        {{ product }}={{ pct.toFixed(1) }}%
                      </span>
                      · 满额推算利润
                      <span :class="marginTextClass(plan.real_usage.vs_direct_price?.profit_cny ?? 0)">
                        ¥{{ plan.real_usage.vs_direct_price?.profit_cny.toFixed(2) }}
                      </span>
                    </template>
                    <template v-else>
                      {{ plan.real_usage.note || '本月至今暂无真实用量数据' }}
                    </template>
                  </div>
                </article>
              </div>
            </section>

            <!-- Pay-as-you-go -->
            <section>
              <h2 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">按量付费公开分组</h2>
              <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
                <table class="min-w-full text-sm">
                  <thead>
                    <tr class="border-b border-gray-100 bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-400 dark:border-dark-700 dark:bg-dark-800">
                      <th class="px-4 py-2">分组</th>
                      <th class="px-4 py-2">产品</th>
                      <th class="px-4 py-2">分组倍率</th>
                      <th class="px-4 py-2">账号倍率(主用/保守)</th>
                      <th class="px-4 py-2">可调度账号</th>
                      <th class="px-4 py-2">¥100小铺充值(净收¥97)全用完</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="group in overview.pay_as_you_go" :key="group.group_id" class="border-b border-gray-50 dark:border-dark-800">
                      <td class="px-4 py-3 font-medium text-gray-700 dark:text-dark-200">{{ group.group_name }}</td>
                      <td class="px-4 py-3 capitalize text-gray-600 dark:text-dark-300">{{ group.product }}</td>
                      <td class="px-4 py-3">
                        <RateEditor
                          :group-id="group.group_id"
                          :value="group.group_rate_multiplier"
                          :saving="isSaving(group.group_id)"
                          @save="onSaveRate"
                        />
                      </td>
                      <td class="px-4 py-3 font-mono text-gray-600 dark:text-dark-300">
                        {{ group.primary_account_rate_multiplier.toFixed(4) }} / {{ group.worst_account_rate_multiplier.toFixed(4) }}
                      </td>
                      <td class="px-4 py-3 text-gray-600 dark:text-dark-300">{{ group.schedulable_account_count }}</td>
                      <td class="px-4 py-3">
                        <span v-if="group.warning" class="rounded-full bg-red-50 px-2.5 py-1 text-xs font-medium text-red-700 dark:bg-red-950/30 dark:text-red-300">
                          {{ group.warning }}
                        </span>
                        <span v-else-if="group.topup_100_cny_scenario" class="font-mono" :class="marginTextClass(group.topup_100_cny_scenario.profit_cny)">
                          利润 ¥{{ group.topup_100_cny_scenario.profit_cny.toFixed(2) }} ({{ formatPercent(group.topup_100_cny_scenario.margin_percent) }})
                        </span>
                        <span v-else class="text-gray-400">-</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          </div>

          <div v-else class="rounded-lg border border-dashed border-gray-300 px-6 py-12 text-center dark:border-dark-600">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">加载失败</h2>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-300">请检查网络后点击刷新重试。</p>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminAPI } from '@/api/admin'
import type { CostAccountingOverview, CostAccountingProduct } from '@/api/admin/costAccounting'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const overview = ref<CostAccountingOverview | null>(null)
const loading = ref(false)
const savingGroupIds = ref<Set<number>>(new Set())

const productKeys: CostAccountingProduct[] = ['gpt', 'claude', 'grok']

async function loadOverview() {
  loading.value = true
  try {
    overview.value = await adminAPI.costAccounting.getOverview()
  } catch (error) {
    appStore.showError('加载成本核算数据失败')
  } finally {
    loading.value = false
  }
}

function isSaving(groupId: number): boolean {
  return savingGroupIds.value.has(groupId)
}

async function onSaveRate(groupId: number, newRate: number) {
  if (!Number.isFinite(newRate) || newRate <= 0) {
    appStore.showError('倍率必须是正数')
    return
  }
  const next = new Set(savingGroupIds.value)
  next.add(groupId)
  savingGroupIds.value = next
  try {
    await adminAPI.groups.update(groupId, { rate_multiplier: newRate })
    appStore.showSuccess(`分组 ${groupId} 倍率已更新为 ${newRate}`)
    await loadOverview()
  } catch (error) {
    appStore.showError('更新分组倍率失败')
  } finally {
    const after = new Set(savingGroupIds.value)
    after.delete(groupId)
    savingGroupIds.value = after
  }
}

function formatPercent(value: number): string {
  return `${value.toFixed(2)}%`
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', month: '2-digit', day: '2-digit' })
}

function marginTextClass(value: number): string {
  if (value < 0) return 'text-red-600 dark:text-red-300'
  return 'text-emerald-600 dark:text-emerald-300'
}

// Small inline rate-editor cell: number input + save button, matching this
// page's operational-dashboard density rather than opening a modal per edit.
const RateEditor = defineComponent({
  name: 'RateEditor',
  props: {
    groupId: { type: Number, required: true },
    value: { type: Number, required: true },
    saving: { type: Boolean, default: false }
  },
  emits: ['save'],
  setup(props, { emit }) {
    const draft = ref(props.value)
    const dirty = computed(() => Math.abs(draft.value - props.value) > 1e-9)
    return () =>
      h('div', { class: 'flex items-center gap-1.5' }, [
        h('input', {
          type: 'number',
          step: '0.0001',
          class:
            'w-24 rounded border border-gray-200 bg-white px-2 py-1 font-mono text-sm text-gray-900 focus:border-primary-500 focus:outline-none dark:border-dark-600 dark:bg-dark-900 dark:text-white',
          value: draft.value,
          disabled: props.saving,
          onInput: (event: Event) => {
            draft.value = Number((event.target as HTMLInputElement).value)
          }
        }),
        dirty.value
          ? h(
              'button',
              {
                type: 'button',
                class:
                  'rounded bg-primary-600 px-2 py-1 text-xs font-medium text-white hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-60',
                disabled: props.saving,
                onClick: () => emit('save', props.groupId, draft.value)
              },
              props.saving ? '保存中…' : '保存'
            )
          : null
      ])
  }
})

onMounted(() => {
  void loadOverview()
})
</script>
