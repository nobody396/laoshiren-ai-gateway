<template>
  <AppLayout>
    <main class="business-finance pb-12">
      <header class="command-header overflow-hidden border border-stone-200 bg-stone-950 text-white shadow-sm dark:border-dark-700">
        <div class="command-grid px-5 py-6 sm:px-7 lg:px-8">
          <div class="flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
            <div class="max-w-3xl">
              <div class="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-amber-300">
                <Icon name="chartBar" size="sm" />
                Business finance
              </div>
              <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">经营财务中心</h1>
              <p class="mt-3 max-w-2xl text-sm leading-6 text-stone-300">
                一个入口看经营结论、现金账和用量成本。三种口径保持独立，日期统一按北京时间解释。
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <div class="rounded-lg border border-white/15 bg-white/10 px-3 py-2 text-xs text-stone-200">
                <span class="text-stone-400">统计范围</span>
                <strong class="ml-2 font-medium text-white">本月 1 日 — 现在</strong>
              </div>
              <button
                type="button"
                class="inline-flex h-10 items-center gap-2 rounded-lg bg-amber-400 px-4 text-sm font-semibold text-stone-950 transition hover:bg-amber-300 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="summaryLoading"
                data-test="refresh-summary"
                @click="loadSummary"
              >
                <Icon name="refresh" size="sm" :class="{ 'animate-spin': summaryLoading }" />
                刷新总览
              </button>
            </div>
          </div>
        </div>
      </header>

      <nav class="sticky-tabs sticky top-0 z-20 mt-4 overflow-x-auto border border-stone-200 bg-white/95 p-1.5 shadow-sm backdrop-blur dark:border-dark-700 dark:bg-dark-800/95" aria-label="经营财务中心模块">
        <div class="flex min-w-max gap-1">
          <button
            v-for="item in tabs"
            :key="item.id"
            type="button"
            :data-test="`business-tab-${item.id}`"
            class="tab-button"
            :class="activeTab === item.id ? 'tab-button-active' : ''"
            @click="selectTab(item.id)"
          >
            <Icon :name="item.icon" size="sm" />
            <span>{{ item.label }}</span>
            <small>{{ item.hint }}</small>
          </button>
        </div>
      </nav>

      <section v-if="activeTab === 'overview'" class="mt-5 space-y-5" data-test="business-overview">
        <div v-if="summaryLoading && !summaryReady" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="index in 8" :key="index" class="h-28 animate-pulse rounded-xl border border-stone-200 bg-white dark:border-dark-700 dark:bg-dark-800"></div>
        </div>

        <template v-else>
          <div v-if="summaryIssues.length" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/20 dark:text-amber-200">
            <span>部分数据暂不可用：{{ summaryIssues.join('、') }}。其余模块仍可正常使用。</span>
            <button type="button" class="font-semibold underline underline-offset-4" @click="loadSummary">重试</button>
          </div>

          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" aria-label="本月经营核心指标">
            <article class="summary-card summary-card-income">
              <div class="summary-label">现金收入</div>
              <div class="summary-value text-emerald-700 dark:text-emerald-300">{{ money(cashIncomeCNY) }}</div>
              <p>财务记账中的真实入账</p>
            </article>
            <article class="summary-card">
              <div class="summary-label">现金支出</div>
              <div class="summary-value text-red-700 dark:text-red-300">{{ money(cashExpenseCNY) }}</div>
              <p>服务器、上游充值及其他支出</p>
            </article>
            <article class="summary-card summary-card-strong">
              <div class="summary-label">现金净利润</div>
              <div class="summary-value" :class="cashNetProfitCNY < 0 ? 'text-red-700 dark:text-red-300' : 'text-stone-950 dark:text-white'">
                {{ money(cashNetProfitCNY) }}
              </div>
              <p>{{ financeSummary ? `现金利润率 ${financeSummary.margin_percent.toFixed(1)}%` : '财务汇总暂不可用' }}</p>
            </article>
            <article class="summary-card summary-card-cost">
              <div class="summary-label">实际上游成本</div>
              <div class="summary-value text-amber-700 dark:text-amber-300">{{ money(observedUpstreamCostCNY) }}</div>
              <p>{{ usageRequestCount.toLocaleString('zh-CN') }} 次真实计费请求</p>
            </article>
          </div>

          <section class="decision-grid grid gap-5 xl:grid-cols-[1.35fr_0.65fr]">
            <article class="rounded-xl border border-stone-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <p class="section-kicker">资金与消耗</p>
                  <h2 class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">本月经营对照</h2>
                  <p class="mt-1 text-sm text-stone-500 dark:text-dark-400">上游充值是预存现金；实际成本才是本月请求真正消耗的金额。</p>
                </div>
                <button type="button" class="text-sm font-semibold text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="selectTab('ledger')">
                  查看现金流水 →
                </button>
              </div>
              <div class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <div class="comparison-item">
                  <span>上游充值</span>
                  <strong>{{ money(upstreamTopupCNY) }}</strong>
                </div>
                <div class="comparison-item">
                  <span>实际消耗</span>
                  <strong>{{ money(observedUpstreamCostCNY) }}</strong>
                </div>
                <div class="comparison-item">
                  <span>上游预存差额</span>
                  <strong :class="upstreamPrepaidDeltaCNY < 0 ? 'text-red-600' : ''">{{ money(upstreamPrepaidDeltaCNY) }}</strong>
                </div>
                <div class="comparison-item">
                  <span>收入覆盖成本</span>
                  <strong>{{ observedUpstreamCostCNY > 0 ? `${(cashIncomeCNY / observedUpstreamCostCNY).toFixed(2)}×` : '—' }}</strong>
                </div>
              </div>
              <div class="mt-6 h-2 overflow-hidden rounded-full bg-stone-100 dark:bg-dark-700">
                <span class="block h-full rounded-full bg-gradient-to-r from-amber-500 to-emerald-500" :style="{ width: `${costCoveragePercent}%` }"></span>
              </div>
              <p class="mt-2 text-xs text-stone-500 dark:text-dark-400">
                实际上游成本占本月现金收入 {{ observedUpstreamCostCNY > 0 && cashIncomeCNY > 0 ? `${((observedUpstreamCostCNY / cashIncomeCNY) * 100).toFixed(1)}%` : '暂无可比数据' }}。
              </p>
            </article>

            <aside class="rounded-xl border border-stone-200 bg-stone-950 p-5 text-white shadow-sm dark:border-dark-700 sm:p-6">
              <p class="section-kicker !text-amber-300">今日看数顺序</p>
              <ol class="mt-5 space-y-4 text-sm">
                <li class="flex gap-3"><span class="step-index">1</span><p><strong>先看现金净利润</strong><br><span class="text-stone-400">确认真实收支结果</span></p></li>
                <li class="flex gap-3"><span class="step-index">2</span><p><strong>再看实际上游成本</strong><br><span class="text-stone-400">确认请求消耗有没有异常</span></p></li>
                <li class="flex gap-3"><span class="step-index">3</span><p><strong>最后看产品毛利</strong><br><span class="text-stone-400">决定分组倍率和产品价格</span></p></li>
              </ol>
            </aside>
          </section>

          <section class="rounded-xl border border-stone-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6" data-test="upstream-routing-finance">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p class="section-kicker">上游路由与资金</p>
                <h2 class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">生产分组、优先级、倍率与余额</h2>
                <p class="mt-1 text-sm text-stone-500 dark:text-dark-400">按真实调度顺序展示；同一模型不支持的账号会被跳过。观测倍率或余额未采集时明确显示待审计。</p>
              </div>
              <RouterLink to="/admin/suppliers" class="text-sm font-semibold text-primary-600 hover:text-primary-700 dark:text-primary-400">
                供应商探针 →
              </RouterLink>
            </div>

            <div class="mt-5 space-y-4">
              <article
                v-for="group in upstreamRouting"
                :key="group.group_id"
                class="overflow-hidden rounded-xl border border-stone-200 dark:border-dark-700"
              >
                <header class="flex flex-wrap items-center justify-between gap-3 bg-stone-50 px-4 py-3 dark:bg-dark-900/60">
                  <div>
                    <strong class="text-sm text-stone-950 dark:text-white">{{ group.group_name }}</strong>
                    <span class="ml-2 font-mono text-xs text-stone-500">#{{ group.group_id }}</span>
                  </div>
                  <div class="text-xs text-stone-500">
                    分组倍率 <strong class="font-mono text-stone-900 dark:text-white">{{ multiplier(group.group_rate_multiplier) }}</strong>
                  </div>
                </header>
                <div class="overflow-x-auto">
                  <table class="min-w-full divide-y divide-stone-200 text-left text-xs dark:divide-dark-700">
                    <thead class="bg-white text-stone-500 dark:bg-dark-800">
                      <tr>
                        <th class="px-4 py-2 font-medium">优先级 / 账号</th>
                        <th class="px-4 py-2 font-medium">配置倍率</th>
                        <th class="px-4 py-2 font-medium">实际倍率</th>
                        <th class="px-4 py-2 font-medium">上游余额</th>
                        <th class="px-4 py-2 font-medium">模型</th>
                        <th class="px-4 py-2 font-medium">状态</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-stone-100 dark:divide-dark-700">
                      <tr v-for="account in group.accounts" :key="account.account_id" :class="account.schedulable ? '' : 'opacity-55'">
                        <td class="px-4 py-3">
                          <span class="mr-2 inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-stone-900 px-1.5 font-mono text-white dark:bg-white dark:text-stone-950">{{ account.priority }}</span>
                          <strong class="text-stone-900 dark:text-white">{{ account.account_name }}</strong>
                          <div class="mt-1 max-w-72 truncate font-mono text-[10px] text-stone-400" :title="account.base_url">{{ account.base_url || '—' }}</div>
                        </td>
                        <td class="px-4 py-3 font-mono">{{ multiplier(account.configured_multiplier) }}</td>
                        <td class="px-4 py-3">
                          <span v-if="account.observed_multiplier != null" class="font-mono">{{ multiplier(account.observed_multiplier) }}</span>
                          <span v-else class="text-stone-400">待审计</span>
                          <span
                            v-if="account.multiplier_drift_percent != null"
                            class="ml-1 rounded px-1.5 py-0.5 text-[10px]"
                            :class="Math.abs(account.multiplier_drift_percent) >= 2 ? 'bg-red-50 text-red-700 dark:bg-red-950/30 dark:text-red-300' : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300'"
                          >{{ signedPercent(account.multiplier_drift_percent) }}</span>
                          <div class="mt-1 text-[10px] text-stone-400">{{ auditStatusLabel(account.multiplier_audit_status) }}</div>
                        </td>
                        <td class="px-4 py-3">
                          <span class="font-mono">{{ upstreamBalance(account) }}</span>
                          <div class="mt-1 text-[10px] text-stone-400">{{ balanceStatusLabel(account.balance_status) }}</div>
                        </td>
                        <td class="max-w-96 px-4 py-3 text-stone-500 dark:text-dark-300">{{ account.models.join('、') || '—' }}</td>
                        <td class="px-4 py-3">
                          <span class="rounded-full px-2 py-1 text-[10px] font-semibold" :class="account.schedulable ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300' : 'bg-stone-100 text-stone-500 dark:bg-dark-700 dark:text-dark-300'">
                            {{ account.schedulable ? '可调度' : '已暂停' }}
                          </span>
                          <div class="mt-1 whitespace-nowrap text-[10px] text-stone-400">{{ auditTime(account.audit_sampled_at) }}</div>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </article>
              <div v-if="upstreamRouting.length === 0" class="rounded-lg border border-dashed border-stone-200 px-4 py-8 text-center text-sm text-stone-500 dark:border-dark-700">
                暂无上游路由数据
              </div>
            </div>
          </section>

          <section class="grid gap-5 lg:grid-cols-3">
            <button type="button" class="module-card text-left" @click="selectTab('ledger')">
              <span class="module-icon bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300"><Icon name="dollar" /></span>
              <strong>现金流水</strong>
              <p>录入、查找和核对每一笔真实收支及凭证。</p>
              <span>进入财务记账 →</span>
            </button>
            <button type="button" class="module-card text-left" @click="selectTab('cost')">
              <span class="module-icon bg-amber-50 text-amber-700 dark:bg-amber-950/30 dark:text-amber-300"><Icon name="calculator" /></span>
              <strong>成本与毛利</strong>
              <p>查看月卡、按量分组和真实流量结构的单位经济模型。</p>
              <span>进入成本核算 →</span>
            </button>
            <RouterLink to="/admin/usage" class="module-card text-left">
              <span class="module-icon bg-blue-50 text-blue-700 dark:bg-blue-950/30 dark:text-blue-300"><Icon name="activity" /></span>
              <strong>用量明细</strong>
              <p>进一步查看模型、用户、分组和请求级消耗。</p>
              <span>进入用量统计 →</span>
            </RouterLink>
          </section>
        </template>
      </section>

      <section v-else-if="activeTab === 'ledger'" class="mt-5" data-test="business-ledger">
        <FinanceTransactionsView embedded />
      </section>

      <section v-else class="mt-5" data-test="business-cost">
        <CostAccountingView embedded />
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import type { IconName } from '@/components/icons/Icon.vue'
import FinanceTransactionsView from '@/views/admin/FinanceTransactionsView.vue'
import CostAccountingView from '@/views/admin/CostAccountingView.vue'
import { adminAPI } from '@/api/admin'
import type { CostAccountingOverview } from '@/api/admin/costAccounting'
import type { CostAccountingUpstreamAccount } from '@/api/admin/costAccounting'
import type { FinanceTransactionSummary } from '@/types'

type BusinessTab = 'overview' | 'ledger' | 'cost'

const route = useRoute()
const router = useRouter()
const tabs: Array<{ id: BusinessTab; label: string; hint: string; icon: IconName }> = [
  { id: 'overview', label: '经营总览', hint: '先看结论', icon: 'chartBar' },
  { id: 'ledger', label: '现金流水', hint: '财务记账', icon: 'dollar' },
  { id: 'cost', label: '成本与毛利', hint: '用量核算', icon: 'calculator' }
]

const activeTab = ref<BusinessTab>('overview')
const financeSummary = ref<FinanceTransactionSummary | null>(null)
const costOverview = ref<CostAccountingOverview | null>(null)
const summaryLoading = ref(false)
const financeError = ref(false)
const costError = ref(false)

const summaryReady = computed(() => financeSummary.value !== null || costOverview.value !== null)
const summaryIssues = computed(() => [
  financeError.value ? '现金账' : '',
  costError.value ? '用量成本' : ''
].filter(Boolean))

const cashIncomeCNY = computed(() => (financeSummary.value?.total_income_fen || 0) / 100)
const cashExpenseCNY = computed(() => (financeSummary.value?.total_expense_fen || 0) / 100)
const cashNetProfitCNY = computed(() => (financeSummary.value?.net_profit_fen || 0) / 100)
const upstreamTopupCNY = computed(() => (financeSummary.value?.by_category || [])
  .filter((item) => item.type === 'expense' && item.category === 'upstream_topup')
  .reduce((sum, item) => sum + item.total_fen / 100, 0))

const observedUpstreamCostCNY = computed(() => {
  if (!costOverview.value) return 0
  const monthly = costOverview.value.monthly_cards.reduce(
    (sum, plan) => sum + (plan.real_usage.observed_real_cost_cny || 0),
    0
  )
  const paygo = costOverview.value.pay_as_you_go.reduce(
    (sum, group) => sum + (group.real_usage.observed_real_cost_cny || 0),
    0
  )
  return monthly + paygo + (costOverview.value.legacy_monthly_card_real_usage.observed_real_cost_cny || 0)
})

const usageRequestCount = computed(() => {
  if (!costOverview.value) return 0
  const monthly = costOverview.value.monthly_cards.reduce(
    (sum, plan) => sum + (plan.real_usage.observed_request_count || 0),
    0
  )
  const paygo = costOverview.value.pay_as_you_go.reduce(
    (sum, group) => sum + (group.real_usage.observed_request_count || 0),
    0
  )
  return monthly + paygo + (costOverview.value.legacy_monthly_card_real_usage.observed_request_count || 0)
})

const upstreamPrepaidDeltaCNY = computed(() => upstreamTopupCNY.value - observedUpstreamCostCNY.value)
const upstreamRouting = computed(() => costOverview.value?.upstream_routing || [])
const costCoveragePercent = computed(() => {
  if (cashIncomeCNY.value <= 0) return 0
  return Math.min(100, Math.max(0, observedUpstreamCostCNY.value / cashIncomeCNY.value * 100))
})

function normalizeTab(value: unknown): BusinessTab {
  const raw = Array.isArray(value) ? value[0] : value
  return raw === 'ledger' || raw === 'cost' ? raw : 'overview'
}

function selectTab(tab: BusinessTab) {
  activeTab.value = tab
  void router.replace({ query: tab === 'overview' ? {} : { tab } })
}

async function loadSummary() {
  summaryLoading.value = true
  financeError.value = false
  costError.value = false
  const financeRange = currentShanghaiMonthRange()
  const [financeResult, costResult] = await Promise.allSettled([
    adminAPI.financeTransactions.summary(financeRange.from, financeRange.to, 'month'),
    adminAPI.costAccounting.getOverview()
  ])
  if (financeResult.status === 'fulfilled') financeSummary.value = financeResult.value
  else financeError.value = true
  if (costResult.status === 'fulfilled') costOverview.value = costResult.value
  else costError.value = true
  summaryLoading.value = false
}

function currentShanghaiMonthRange(): { from: number; to: number } {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: 'numeric'
  }).formatToParts(new Date())
  const year = Number(parts.find((part) => part.type === 'year')?.value)
  const month = Number(parts.find((part) => part.type === 'month')?.value)
  const shanghaiOffsetMs = 8 * 60 * 60 * 1000
  return {
    from: Math.floor((Date.UTC(year, month - 1, 1) - shanghaiOffsetMs) / 1000),
    to: Math.floor((Date.UTC(year, month, 1) - shanghaiOffsetMs) / 1000)
  }
}

function money(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(Number.isFinite(value) ? value : 0)
}

function multiplier(value: number): string {
  return Number.isFinite(value) ? Number(value).toFixed(4).replace(/0+$/, '').replace(/\.$/, '') : '—'
}

function upstreamBalance(account: CostAccountingUpstreamAccount): string {
  if (account.balance_value == null) return '待采集'
  const value = Number(account.balance_value).toLocaleString('zh-CN', { maximumFractionDigits: 4 })
  return account.balance_currency ? `${value} ${account.balance_currency}` : value
}

function signedPercent(value: number): string {
  const prefix = value > 0 ? '+' : ''
  return `${prefix}${value.toFixed(1)}%`
}

function auditStatusLabel(status: string): string {
  return ({ ok: '账单/余额差实测', published: '供应商报价待账单复核', drift: '倍率漂移告警', manual: '人工核对', unobserved: '未观测' } as Record<string, string>)[status] || status
}

function balanceStatusLabel(status: string): string {
  return ({ ok: '实时余额', manual: '人工读取', stale: '余额已过期', unobserved: '余额待采集' } as Record<string, string>)[status] || status
}

function auditTime(value?: string): string {
  if (!value) return '未采集'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false
  }).format(date)
}

watch(() => route.query.tab, (value) => {
  activeTab.value = normalizeTab(value)
})

onMounted(() => {
  activeTab.value = normalizeTab(route.query.tab)
  void loadSummary()
})
</script>

<style scoped>
.command-header,
.sticky-tabs,
.summary-card,
.module-card {
  border-radius: 0.85rem;
}

.command-grid {
  background:
    radial-gradient(circle at 82% 10%, rgb(245 158 11 / 0.2), transparent 24rem),
    linear-gradient(115deg, rgb(255 255 255 / 0.04) 1px, transparent 1px);
  background-size: auto, 2rem 2rem;
}

.tab-button {
  display: grid;
  grid-template-columns: auto auto;
  align-items: center;
  column-gap: 0.45rem;
  min-width: 9.5rem;
  border-radius: 0.65rem;
  padding: 0.65rem 0.85rem;
  color: rgb(87 83 78);
  text-align: left;
  transition: background-color 150ms, color 150ms, box-shadow 150ms;
}

.tab-button small {
  grid-column: 2;
  color: rgb(168 162 158);
  font-size: 0.65rem;
}

.tab-button-active {
  background: rgb(28 25 23);
  color: white;
  box-shadow: 0 1px 2px rgb(28 25 23 / 0.18);
}

.tab-button-active small {
  color: rgb(214 211 209);
}

:global(.dark) .tab-button {
  color: rgb(209 213 219);
}

:global(.dark) .tab-button-active {
  background: rgb(245 158 11);
  color: rgb(28 25 23);
}

:global(.dark) .tab-button-active small {
  color: rgb(68 64 60);
}

.summary-card {
  min-height: 7.5rem;
  border: 1px solid rgb(231 229 228);
  background: white;
  padding: 1.15rem;
  box-shadow: 0 1px 2px rgb(28 25 23 / 0.04);
}

:global(.dark) .summary-card {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55);
}

.summary-card-income { border-top: 3px solid rgb(16 185 129); }
.summary-card-cost { border-top: 3px solid rgb(245 158 11); }
.summary-card-strong { border-color: rgb(168 162 158); }

.summary-label,
.comparison-item span {
  color: rgb(120 113 108);
  font-size: 0.72rem;
  font-weight: 600;
}

.summary-value {
  margin-top: 0.7rem;
  font-size: 1.65rem;
  font-weight: 680;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.summary-card p {
  margin-top: 0.6rem;
  color: rgb(120 113 108);
  font-size: 0.7rem;
}

.section-kicker {
  color: rgb(180 83 9);
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.comparison-item {
  border-left: 2px solid rgb(231 229 228);
  padding-left: 0.8rem;
}

:global(.dark) .comparison-item { border-color: rgb(75 85 99); }

.comparison-item strong {
  display: block;
  margin-top: 0.35rem;
  color: rgb(41 37 36);
  font-size: 1rem;
  font-variant-numeric: tabular-nums;
}

:global(.dark) .comparison-item strong { color: rgb(243 244 246); }

.step-index {
  display: inline-flex;
  height: 1.45rem;
  width: 1.45rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(245 158 11 / 0.55);
  color: rgb(253 230 138);
  font-size: 0.68rem;
  font-weight: 700;
}

.module-card {
  display: flex;
  min-height: 11.5rem;
  flex-direction: column;
  border: 1px solid rgb(231 229 228);
  background: white;
  padding: 1.25rem;
  box-shadow: 0 1px 2px rgb(28 25 23 / 0.04);
  transition: transform 150ms, border-color 150ms, box-shadow 150ms;
}

.module-card:hover {
  transform: translateY(-2px);
  border-color: rgb(245 158 11);
  box-shadow: 0 8px 20px rgb(28 25 23 / 0.08);
}

:global(.dark) .module-card {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55);
}

.module-icon {
  display: inline-flex;
  height: 2.2rem;
  width: 2.2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.6rem;
}

.module-card strong {
  margin-top: 1rem;
  color: rgb(28 25 23);
  font-size: 1rem;
}

:global(.dark) .module-card strong { color: white; }

.module-card p {
  margin-top: 0.4rem;
  flex: 1;
  color: rgb(120 113 108);
  font-size: 0.78rem;
  line-height: 1.5;
}

.module-card > span:last-child {
  margin-top: 1rem;
  color: rgb(180 83 9);
  font-size: 0.78rem;
  font-weight: 600;
}

@media (max-width: 640px) {
  .sticky-tabs { top: 0.5rem; }
  .tab-button { min-width: 8.4rem; }
}
</style>
