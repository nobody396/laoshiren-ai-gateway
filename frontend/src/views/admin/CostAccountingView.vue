<template>
  <AppLayout>
    <div class="cost-page space-y-6 pb-12">
      <header class="cost-hero overflow-hidden border border-stone-200 bg-stone-950 text-white shadow-sm dark:border-dark-700">
        <div class="cost-hero-grid px-6 py-7 lg:px-8">
          <div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
            <div class="max-w-3xl">
              <div class="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.2em] text-amber-300">
                <Icon name="calculator" size="sm" />
                Unit economics
              </div>
              <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">成本核算</h1>
              <p class="mt-3 max-w-2xl text-sm leading-6 text-stone-300">
                只看当前在售的 Lite、Pro，并自动纳入全部启用的按量付费公开分组。
                这里回答“实际用了多少成本”，财务记账回答“现金收支是多少”。
              </p>
              <p v-if="overview" class="mt-4 text-xs text-stone-400">
                北京时间 {{ formatTime(overview.usage_window_start) }} — {{ formatTime(overview.usage_window_end) }}
              </p>
            </div>
            <div class="flex flex-wrap gap-2">
              <RouterLink
                to="/admin/finance-transactions"
                class="inline-flex h-10 items-center gap-2 border border-white/20 bg-white/10 px-4 text-sm font-medium text-white transition hover:bg-white/15"
              >
                财务记账
                <Icon name="arrowRight" size="sm" />
              </RouterLink>
              <button
                type="button"
                class="inline-flex h-10 items-center gap-2 bg-amber-400 px-4 text-sm font-semibold text-stone-950 transition hover:bg-amber-300 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="loading"
                @click="loadOverview"
              >
                <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
                刷新
              </button>
            </div>
          </div>
        </div>
      </header>

      <div v-if="loading && !overview" class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <div v-for="idx in 4" :key="idx" class="h-32 animate-pulse border border-stone-200 bg-white dark:border-dark-700 dark:bg-dark-800" />
      </div>

      <template v-else-if="overview">
        <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4" aria-label="成本核算核心指标">
          <article class="metric-card">
            <div class="metric-kicker">
              <Icon name="creditCard" size="sm" />
              当前在售
            </div>
            <div class="metric-value">{{ overview.monthly_cards.length }} <span>款月卡</span></div>
            <p>仅 Lite / Pro；历史产品不参与当前核算</p>
          </article>
          <article class="metric-card">
            <div class="metric-kicker">
              <Icon name="grid" size="sm" />
              按量公开分组
            </div>
            <div class="metric-value">{{ overview.pay_as_you_go.length }} <span>个分组</span></div>
            <p>从启用的公开标准计费分组自动发现</p>
          </article>
          <article class="metric-card metric-card-accent">
            <div class="metric-kicker">
              <Icon name="activity" size="sm" />
              本月实际上游成本
            </div>
            <div class="metric-value">{{ formatMoney(observedDirectCostCNY) }}</div>
            <p>
              来自真实请求；其中历史权益
              {{ formatMoney(overview.legacy_monthly_card_real_usage.observed_real_cost_cny || 0) }}
            </p>
          </article>
          <article class="metric-card">
            <div class="metric-kicker">
              <Icon name="dollar" size="sm" />
              财务账本净利润
            </div>
            <div class="metric-value" :class="financeSummary && financeSummary.net_profit_fen < 0 ? '!text-red-600' : ''">
              {{ financeSummary ? formatMoney(financeSummary.net_profit_fen / 100) : '暂不可用' }}
            </div>
            <p>真实收入减真实支出，取自财务记账</p>
          </article>
        </section>

        <section class="ledger-bridge border border-stone-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="grid lg:grid-cols-[1.2fr_1fr]">
            <div class="border-b border-stone-200 p-6 dark:border-dark-700 lg:border-b-0 lg:border-r">
              <div class="flex items-start justify-between gap-4">
                <div>
                  <p class="section-eyebrow">现金账与用量账联动</p>
                  <h2 class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">本月经营对照</h2>
                </div>
                <RouterLink to="/admin/finance-transactions" class="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400">
                  查看流水 →
                </RouterLink>
              </div>
              <div class="mt-6 grid grid-cols-2 gap-x-6 gap-y-5 sm:grid-cols-4">
                <div>
                  <div class="bridge-label">现金收入</div>
                  <div class="bridge-value text-emerald-700 dark:text-emerald-300">
                    {{ financeSummary ? formatMoney(financeSummary.total_income_fen / 100) : '—' }}
                  </div>
                </div>
                <div>
                  <div class="bridge-label">现金支出</div>
                  <div class="bridge-value text-red-700 dark:text-red-300">
                    {{ financeSummary ? formatMoney(financeSummary.total_expense_fen / 100) : '—' }}
                  </div>
                </div>
                <div>
                  <div class="bridge-label">其中上游充值</div>
                  <div class="bridge-value">{{ financeSummary ? formatMoney(upstreamTopupCNY) : '—' }}</div>
                </div>
                <div>
                  <div class="bridge-label">实际消耗成本</div>
                  <div class="bridge-value">{{ formatMoney(observedDirectCostCNY) }}</div>
                </div>
              </div>
            </div>
            <div class="bg-stone-50 p-6 dark:bg-dark-900/60">
              <p class="section-eyebrow">口径说明</p>
              <div class="mt-4 space-y-4 text-sm leading-6 text-stone-600 dark:text-dark-300">
                <div class="flex gap-3">
                  <span class="scope-index">1</span>
                  <p><strong class="text-stone-900 dark:text-white">充值</strong>是资金预存，记录在财务支出；不是充值当月就全部消耗。</p>
                </div>
                <div class="flex gap-3">
                  <span class="scope-index">2</span>
                  <p><strong class="text-stone-900 dark:text-white">实际成本</strong>按请求日志和历史账号倍率计算，才是本月真正用掉的上游成本。</p>
                </div>
                <div class="flex gap-3">
                  <span class="scope-index">3</span>
                  <p><strong class="text-stone-900 dark:text-white">订单利润</strong>仍需把收入、用户权益窗口与实际用量关联，不能用充值额代替。</p>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section>
          <div class="section-heading">
            <div>
              <p class="section-eyebrow">Monthly cards</p>
              <h2>当前月卡经济模型</h2>
              <p>同一张卡的 GPT、Claude、Grok 共享额度；真实毛利按本月实际流量结构推算。</p>
            </div>
            <span class="text-xs text-stone-500 dark:text-dark-400">渠道手续费：小铺 {{ overview.shop_channel_fee_percent.toFixed(0) }}%</span>
          </div>

          <div class="mt-4 grid gap-5 xl:grid-cols-2">
            <article
              v-for="plan in overview.monthly_cards"
              :key="plan.id"
              class="plan-card border border-stone-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800"
              :data-test="`plan-${plan.id}`"
            >
              <div class="plan-band" :class="plan.id === 'lite' ? 'plan-band-lite' : 'plan-band-pro'"></div>
              <div class="p-6">
                <div class="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <div class="flex items-center gap-3">
                      <h3 class="text-2xl font-semibold tracking-tight text-stone-950 dark:text-white">{{ plan.name }}</h3>
                      <span class="border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300">在售</span>
                    </div>
                    <p class="mt-1 text-sm text-stone-500 dark:text-dark-400">{{ formatNumber(plan.monthly_credits) }} 共享 credits / 月</p>
                  </div>
                  <div class="text-right">
                    <div class="text-xs text-stone-500 dark:text-dark-400">真实推算毛利率</div>
                    <div class="mt-1 text-3xl font-semibold tabular-nums" :class="marginClass(plan.margin_range.real_percent)">
                      {{ plan.margin_range.real_percent != null ? formatPercent(plan.margin_range.real_percent) : '暂无用量' }}
                    </div>
                  </div>
                </div>

                <div class="mt-6 grid grid-cols-2 border-y border-stone-200 py-4 dark:border-dark-700 sm:grid-cols-4">
                  <div class="price-cell">
                    <span>微信 / 支付宝</span>
                    <strong>{{ formatMoney(plan.direct_price_cny) }}</strong>
                  </div>
                  <div class="price-cell">
                    <span>链动小铺标价</span>
                    <strong>{{ formatMoney(plan.shop_price_cny) }}</strong>
                  </div>
                  <div class="price-cell">
                    <span>小铺净收入</span>
                    <strong>{{ formatMoney(plan.shop_net_price_cny) }}</strong>
                  </div>
                  <div class="price-cell">
                    <span>本月实际成本</span>
                    <strong>{{ formatMoney(plan.real_usage.observed_real_cost_cny || 0) }}</strong>
                  </div>
                </div>

                <div class="mt-5">
                  <div class="mb-2 flex items-center justify-between text-xs">
                    <span class="font-medium text-stone-700 dark:text-dark-200">真实流量结构</span>
                    <span class="text-stone-500 dark:text-dark-400">
                      {{ formatNumber(plan.real_usage.observed_request_count || 0) }} 次请求
                    </span>
                  </div>
                  <div v-if="hasUsageMix(plan)" class="flex h-2.5 w-full overflow-hidden bg-stone-100 dark:bg-dark-700">
                    <span
                      v-for="product in productKeys"
                      :key="product"
                      :class="mixColor(product)"
                      :style="{ width: `${plan.real_usage.product_mix_percent?.[product] || 0}%` }"
                    ></span>
                  </div>
                  <div v-else class="h-2.5 w-full bg-stone-100 dark:bg-dark-700"></div>
                  <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-stone-500 dark:text-dark-400">
                    <span v-for="product in productKeys" :key="product" class="flex items-center gap-1.5">
                      <i class="h-2 w-2" :class="mixColor(product)"></i>
                      {{ productLabel(product) }} {{ (plan.real_usage.product_mix_percent?.[product] || 0).toFixed(1) }}%
                    </span>
                  </div>
                </div>

                <div class="mt-6">
                  <div class="mb-2 grid grid-cols-[1fr_0.65fr_0.9fr_0.65fr] gap-3 text-[11px] font-semibold uppercase tracking-wider text-stone-400">
                    <span>路由</span><span>分组倍率</span><span>账号倍率 主 / 保守</span><span class="text-right">账号</span>
                  </div>
                  <div
                    v-for="product in productKeys"
                    :key="product"
                    class="grid grid-cols-[1fr_0.65fr_0.9fr_0.65fr] items-center gap-3 border-t border-stone-100 py-3 text-sm dark:border-dark-700"
                  >
                    <div class="min-w-0">
                      <div class="font-medium text-stone-900 dark:text-white">{{ productLabel(product) }}</div>
                      <div class="truncate text-xs text-stone-400" :title="plan.products[product].group_name">{{ plan.products[product].group_name }}</div>
                    </div>
                    <span class="font-mono tabular-nums">{{ plan.products[product].group_rate_multiplier.toFixed(4) }}</span>
                    <span class="font-mono text-xs tabular-nums text-stone-600 dark:text-dark-300">
                      {{ plan.products[product].primary_account_rate_multiplier.toFixed(4) }} /
                      {{ plan.products[product].worst_account_rate_multiplier.toFixed(4) }}
                    </span>
                    <span class="text-right font-medium">{{ plan.products[product].schedulable_account_count }}</span>
                  </div>
                </div>

                <details class="mt-4 border-t border-stone-200 pt-4 dark:border-dark-700">
                  <summary class="cursor-pointer text-sm font-medium text-stone-700 marker:text-stone-400 hover:text-primary-600 dark:text-dark-200">
                    查看满额消耗压力测试
                  </summary>
                  <div class="mt-3 overflow-x-auto">
                    <table class="w-full min-w-[34rem] text-left text-sm">
                      <thead class="text-xs text-stone-400">
                        <tr>
                          <th class="pb-2 font-medium">情景</th>
                          <th class="pb-2 font-medium">成本</th>
                          <th class="pb-2 font-medium">利润</th>
                          <th class="pb-2 text-right font-medium">毛利率</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="product in productKeys" :key="product" class="border-t border-stone-100 dark:border-dark-700">
                          <td class="py-2.5">全部使用 {{ productLabel(product) }}</td>
                          <td class="py-2.5 tabular-nums">{{ formatMoney(plan.single_product_scenarios['all_' + product].vs_direct_price.cost_cny) }}</td>
                          <td class="py-2.5 tabular-nums" :class="marginClass(plan.single_product_scenarios['all_' + product].vs_direct_price.margin_percent)">
                            {{ formatMoney(plan.single_product_scenarios['all_' + product].vs_direct_price.profit_cny) }}
                          </td>
                          <td class="py-2.5 text-right tabular-nums" :class="marginClass(plan.single_product_scenarios['all_' + product].vs_direct_price.margin_percent)">
                            {{ formatPercent(plan.single_product_scenarios['all_' + product].vs_direct_price.margin_percent) }}
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </details>
              </div>
            </article>
          </div>
        </section>

        <section>
          <div class="section-heading">
            <div>
              <p class="section-eyebrow">Pay as you go</p>
              <h2>按量付费公开分组</h2>
              <p>不再维护固定 ID 清单；新增或停用公开标准分组后，这里会自动同步。</p>
            </div>
            <RouterLink to="/admin/groups" class="inline-flex items-center gap-1 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400">
              管理分组 <Icon name="externalLink" size="sm" />
            </RouterLink>
          </div>

          <div class="mt-4 grid gap-4 lg:grid-cols-2 2xl:grid-cols-3">
            <article
              v-for="group in overview.pay_as_you_go"
              :key="group.group_id"
              class="group-card border border-stone-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800"
              :data-test="`paygo-${group.group_id}`"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-stone-400">
                    <span>{{ productLabel(group.product) }}</span>
                    <span>·</span>
                    <span>#{{ group.group_id }}</span>
                  </div>
                  <h3 class="truncate text-base font-semibold text-stone-950 dark:text-white" :title="group.group_name">{{ group.group_name }}</h3>
                </div>
                <span
                  class="shrink-0 border px-2 py-1 text-xs font-medium"
                  :class="group.schedulable_account_count > 0
                    ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300'
                    : 'border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300'"
                >
                  {{ group.schedulable_account_count }} 个可用账号
                </span>
              </div>

              <div class="mt-5 grid grid-cols-3 gap-3">
                <div class="group-stat">
                  <span>分组倍率</span>
                  <strong>{{ group.group_rate_multiplier.toFixed(4) }}</strong>
                </div>
                <div class="group-stat">
                  <span>本月请求</span>
                  <strong>{{ formatNumber(group.real_usage.observed_request_count || 0) }}</strong>
                </div>
                <div class="group-stat">
                  <span>实际成本</span>
                  <strong>{{ formatMoney(group.real_usage.observed_real_cost_cny || 0) }}</strong>
                </div>
              </div>

              <div class="mt-5 border-t border-stone-100 pt-4 dark:border-dark-700">
                <div class="flex items-baseline justify-between gap-3">
                  <span class="text-xs text-stone-500 dark:text-dark-400">¥100 小铺充值全部用完</span>
                  <strong
                    v-if="group.topup_100_cny_scenario"
                    class="text-lg tabular-nums"
                    :class="marginClass(group.topup_100_cny_scenario.margin_percent)"
                  >
                    {{ formatPercent(group.topup_100_cny_scenario.margin_percent) }}
                  </strong>
                  <span v-else class="text-sm text-stone-400">无法计算</span>
                </div>
                <p v-if="group.topup_100_cny_scenario" class="mt-1 text-xs text-stone-500 dark:text-dark-400">
                  保守利润 {{ formatMoney(group.topup_100_cny_scenario.profit_cny) }} ·
                  账号倍率 {{ group.primary_account_rate_multiplier.toFixed(4) }} / {{ group.worst_account_rate_multiplier.toFixed(4) }}
                </p>
                <p v-if="group.warning" class="mt-3 border-l-2 border-red-500 bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/20 dark:text-red-300">
                  {{ group.warning }}
                </p>
              </div>
            </article>
          </div>
        </section>

        <aside class="border-l-4 border-amber-400 bg-amber-50 px-5 py-4 text-sm leading-6 text-amber-950 dark:bg-amber-950/20 dark:text-amber-200">
          <strong>核算范围已收口：</strong>
          {{ overview.scope_note }}
          <span class="ml-1 text-amber-800/70 dark:text-amber-300/70">生成于 {{ formatTime(overview.generated_at) }}</span>
        </aside>
      </template>

      <div v-else class="border border-dashed border-stone-300 bg-white px-6 py-16 text-center dark:border-dark-600 dark:bg-dark-800">
        <Icon name="exclamationTriangle" size="xl" class="mx-auto text-amber-500" />
        <h2 class="mt-4 text-lg font-semibold text-stone-950 dark:text-white">成本核算加载失败</h2>
        <p class="mt-2 text-sm text-stone-500 dark:text-dark-400">请检查网络后重新加载。</p>
        <button class="mt-5 bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700" @click="loadOverview">重新加载</button>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type {
  CostAccountingMonthlyPlan,
  CostAccountingOverview,
  CostAccountingProduct
} from '@/api/admin/costAccounting'
import type { FinanceTransactionSummary } from '@/types'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const overview = ref<CostAccountingOverview | null>(null)
const financeSummary = ref<FinanceTransactionSummary | null>(null)
const loading = ref(false)
const productKeys: CostAccountingProduct[] = ['gpt', 'claude', 'grok']

const observedDirectCostCNY = computed(() => {
  if (!overview.value) return 0
  const monthly = overview.value.monthly_cards.reduce(
    (sum, plan) => sum + (plan.real_usage.observed_real_cost_cny || 0),
    0
  )
  const paygo = overview.value.pay_as_you_go.reduce(
    (sum, group) => sum + (group.real_usage.observed_real_cost_cny || 0),
    0
  )
  const legacy = overview.value.legacy_monthly_card_real_usage.observed_real_cost_cny || 0
  return monthly + paygo + legacy
})

const upstreamTopupCNY = computed(() => {
  if (!financeSummary.value) return 0
  return financeSummary.value.by_category
    .filter((item) => item.type === 'expense' && item.category === 'upstream_topup')
    .reduce((sum, item) => sum + item.total_fen / 100, 0)
})

async function loadOverview() {
  loading.value = true
  try {
    const loaded = await adminAPI.costAccounting.getOverview()
    overview.value = loaded

    try {
      const from = Math.floor(new Date(loaded.usage_window_start).getTime() / 1000)
      const to = Math.floor(new Date(loaded.usage_window_end).getTime() / 1000)
      financeSummary.value = await adminAPI.financeTransactions.summary(from, to, 'month')
    } catch (error) {
      financeSummary.value = null
      appStore.showError('成本数据已加载，但财务记账联动暂不可用')
    }
  } catch (error) {
    overview.value = null
    financeSummary.value = null
    appStore.showError('加载成本核算数据失败')
  } finally {
    loading.value = false
  }
}

function formatMoney(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(Number.isFinite(value) ? value : 0)
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(value || 0)
}

function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

function formatTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23'
  }).format(date)
}

function marginClass(value?: number | null): string {
  if (value == null) return 'text-stone-400'
  if (value < 0) return 'text-red-600 dark:text-red-300'
  if (value < 20) return 'text-amber-600 dark:text-amber-300'
  return 'text-emerald-700 dark:text-emerald-300'
}

function productLabel(product: string): string {
  const labels: Record<string, string> = {
    gpt: 'GPT',
    claude: 'Claude',
    grok: 'Grok',
    glm: 'GLM'
  }
  return labels[product.toLowerCase()] || product
}

function mixColor(product: CostAccountingProduct): string {
  return {
    gpt: 'bg-blue-500',
    claude: 'bg-amber-500',
    grok: 'bg-fuchsia-500'
  }[product]
}

function hasUsageMix(plan: CostAccountingMonthlyPlan): boolean {
  return productKeys.some((product) => (plan.real_usage.product_mix_percent?.[product] || 0) > 0)
}

onMounted(() => {
  void loadOverview()
})
</script>

<style scoped>
.cost-hero,
.metric-card,
.ledger-bridge,
.plan-card,
.group-card {
  border-radius: 0.75rem;
}

.cost-hero-grid {
  background:
    radial-gradient(circle at 85% 20%, rgb(var(--gild-600) / 0.18), transparent 26rem),
    linear-gradient(115deg, rgb(var(--color-vellum) / 0.04) 1px, transparent 1px);
  background-size: auto, 2rem 2rem;
}

.metric-card {
  min-height: 8rem;
  border: 1px solid rgb(231 229 228);
  background: white;
  padding: 1.25rem;
  box-shadow: 0 1px 2px rgb(28 25 23 / 0.04);
}

:global(.dark) .metric-card {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55);
}

.metric-card-accent {
  border-top: 3px solid rgb(245 158 11);
}

.metric-kicker {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  color: rgb(120 113 108);
  font-size: 0.75rem;
  font-weight: 600;
}

.metric-value {
  margin-top: 0.7rem;
  color: rgb(28 25 23);
  font-size: 1.65rem;
  font-weight: 650;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

:global(.dark) .metric-value {
  color: white;
}

.metric-value span {
  color: rgb(120 113 108);
  font-size: 0.8rem;
  font-weight: 500;
}

.metric-card p {
  margin-top: 0.55rem;
  color: rgb(120 113 108);
  font-size: 0.72rem;
  line-height: 1.4;
}

.section-eyebrow {
  color: rgb(180 83 9);
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.section-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
}

.section-heading h2 {
  margin-top: 0.2rem;
  color: rgb(28 25 23);
  font-size: 1.2rem;
  font-weight: 650;
}

:global(.dark) .section-heading h2 {
  color: white;
}

.section-heading p:not(.section-eyebrow) {
  margin-top: 0.35rem;
  color: rgb(120 113 108);
  font-size: 0.78rem;
}

.bridge-label,
.group-stat span {
  color: rgb(120 113 108);
  font-size: 0.7rem;
}

.bridge-value {
  margin-top: 0.3rem;
  color: rgb(41 37 36);
  font-size: 1.05rem;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
}

:global(.dark) .bridge-value {
  color: rgb(243 244 246);
}

.scope-index {
  display: inline-flex;
  height: 1.35rem;
  width: 1.35rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(214 211 209);
  color: rgb(120 113 108);
  font-size: 0.65rem;
  font-weight: 700;
}

.plan-band {
  height: 0.3rem;
}

.plan-band-lite {
  background: linear-gradient(90deg, rgb(14 165 233), rgb(34 197 94));
}

.plan-band-pro {
  background: linear-gradient(90deg, rgb(124 58 237), rgb(236 72 153));
}

.price-cell {
  padding: 0 0.8rem;
  border-left: 1px solid rgb(231 229 228);
}

.price-cell:first-child {
  padding-left: 0;
  border-left: 0;
}

:global(.dark) .price-cell {
  border-color: rgb(55 65 81);
}

.price-cell span {
  display: block;
  color: rgb(120 113 108);
  font-size: 0.68rem;
}

.price-cell strong {
  display: block;
  margin-top: 0.3rem;
  color: rgb(41 37 36);
  font-size: 0.9rem;
  font-variant-numeric: tabular-nums;
}

:global(.dark) .price-cell strong {
  color: rgb(243 244 246);
}

.group-stat {
  border-left: 2px solid rgb(231 229 228);
  padding-left: 0.75rem;
}

:global(.dark) .group-stat {
  border-color: rgb(75 85 99);
}

.group-stat strong {
  display: block;
  margin-top: 0.25rem;
  color: rgb(41 37 36);
  font-size: 0.92rem;
  font-variant-numeric: tabular-nums;
}

:global(.dark) .group-stat strong {
  color: rgb(243 244 246);
}

@media (max-width: 640px) {
  .section-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .price-cell:nth-child(3) {
    margin-top: 1rem;
    padding-left: 0;
    border-left: 0;
  }

  .price-cell:nth-child(4) {
    margin-top: 1rem;
  }
}
</style>
