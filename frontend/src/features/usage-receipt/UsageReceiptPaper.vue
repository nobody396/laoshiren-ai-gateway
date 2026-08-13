<template>
  <article ref="receiptElement" class="usage-receipt-paper" aria-label="老实人AI 使用小票">
    <header class="receipt-brand">
      <div class="receipt-brand__mark">
        <img
          v-if="!logoFailed"
          :src="siteLogo"
          :alt="`${siteName} 图标`"
          class="receipt-brand__logo"
          @error="logoFailed = true"
        />
        <span v-else class="receipt-brand__fallback" aria-hidden="true">老</span>
      </div>
      <p class="receipt-brand__name">{{ siteName }}</p>
      <h1 class="receipt-brand__title">AI 使用小票</h1>
      <p class="receipt-brand__english">AI USAGE RECEIPT</p>
    </header>

    <div class="receipt-rule receipt-rule--heavy"></div>

    <dl class="receipt-meta">
      <div class="receipt-meta__row">
        <dt>统计周期</dt>
        <span class="receipt-meta__dots" aria-hidden="true"></span>
        <dd>{{ dateRangeLabel }}</dd>
      </div>
      <div class="receipt-meta__row">
        <dt>小票编号</dt>
        <span class="receipt-meta__dots" aria-hidden="true"></span>
        <dd>{{ data.receiptNumber }}</dd>
      </div>
      <div v-if="preferences.showDisplayName && data.displayName" class="receipt-meta__row">
        <dt>使用者</dt>
        <span class="receipt-meta__dots" aria-hidden="true"></span>
        <dd>@{{ data.displayName }}</dd>
      </div>
      <div class="receipt-meta__row">
        <dt>生成时间</dt>
        <span class="receipt-meta__dots" aria-hidden="true"></span>
        <dd>{{ generatedAtLabel }}</dd>
      </div>
    </dl>

    <section class="receipt-hero" aria-label="使用汇总">
      <p class="receipt-hero__eyebrow">本期实际消费</p>
      <p class="receipt-hero__amount">¥{{ formatCurrency(actualCost) }}</p>
      <div class="receipt-hero__metrics">
        <div>
          <span>总请求</span>
          <strong>{{ data.stats.total_requests.toLocaleString('zh-CN') }}</strong>
        </div>
        <div>
          <span>总 Token</span>
          <strong>{{ formatCompactNumber(data.stats.total_tokens) }}</strong>
        </div>
      </div>
    </section>

    <div class="receipt-rule"></div>

    <section class="receipt-token-breakdown" aria-label="Token 明细">
      <div class="receipt-section-heading">
        <span>Token 明细</span>
        <span>数量</span>
      </div>
      <div class="receipt-line-item">
        <span>输入 Token</span>
        <strong>{{ data.stats.total_input_tokens.toLocaleString('zh-CN') }}</strong>
      </div>
      <div class="receipt-line-item">
        <span>输出 Token</span>
        <strong>{{ data.stats.total_output_tokens.toLocaleString('zh-CN') }}</strong>
      </div>
      <div class="receipt-line-item">
        <span>缓存 Token</span>
        <strong>{{ data.stats.total_cache_tokens.toLocaleString('zh-CN') }}</strong>
      </div>
      <div class="receipt-line-item">
        <span>平均响应</span>
        <strong>{{ formatDuration(data.stats.average_duration_ms) }}</strong>
      </div>
    </section>

    <template v-if="preferences.showModelBreakdown">
      <div class="receipt-rule"></div>
      <section class="receipt-models" aria-label="模型消费明细">
        <div class="receipt-section-heading">
          <span>模型消费明细</span>
          <span>实付</span>
        </div>
        <div v-if="modelLines.length === 0" class="receipt-empty-line">本期暂无模型调用</div>
        <div v-for="line in modelLines" :key="line.model" class="receipt-model">
          <div class="receipt-model__heading">
            <strong>{{ line.model }}</strong>
            <strong>¥{{ formatCurrency(line.actualCost) }}</strong>
          </div>
          <p>{{ line.requests.toLocaleString('zh-CN') }} 次 · {{ formatCompactNumber(line.totalTokens) }} Token</p>
        </div>
      </section>
    </template>

    <template v-if="preferences.showSavings">
      <div class="receipt-rule receipt-rule--heavy"></div>
      <section class="receipt-totals" aria-label="价格汇总">
        <div class="receipt-line-item">
          <span>官方刊例折算</span>
          <span class="receipt-totals__original">¥{{ formatCurrency(officialCostCny) }}</span>
        </div>
        <div class="receipt-line-item receipt-line-item--total">
          <span>实际消费</span>
          <strong>¥{{ formatCurrency(actualCost) }}</strong>
        </div>
        <div class="receipt-saved">
          <span>本期已节省</span>
          <strong>¥{{ formatCurrency(savedCostCny) }}</strong>
        </div>
        <p class="receipt-totals__note">官方价按 {{ data.exchangeRate.toFixed(2) }} 汇率折算，仅作对比</p>
      </section>
    </template>

    <footer class="receipt-footer">
      <div class="receipt-footer__thanks">
        <span class="receipt-footer__stamp">BUILD WITH AI</span>
        <p>感谢每一次认真创造</p>
      </div>

      <div v-if="data.qrDataUrl" class="receipt-qr">
        <img :src="data.qrDataUrl" alt="专属邀请链接二维码" />
        <div>
          <strong>扫码加入 {{ siteName }}</strong>
          <span v-if="data.inviteCode">邀请码 {{ data.inviteCode }}</span>
          <span v-else>开启你的 AI 工作流</span>
        </div>
      </div>

      <p class="receipt-footer__privacy">仅展示汇总数据 · 不含邮箱、API Key 与请求内容</p>
      <p class="receipt-footer__signature">LAOSHIREN AI · A QUIET PLACE FOR CODE</p>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { buildReceiptModelLines, formatBeijingDateTime, formatCompactNumber, formatReceiptDateRange } from './receipt'
import type { UsageReceiptData, UsageReceiptPreferences } from './types'

const props = defineProps<{
  data: UsageReceiptData
  preferences: UsageReceiptPreferences
  siteName: string
  siteLogo: string
}>()

const receiptElement = ref<HTMLElement | null>(null)
const logoFailed = ref(false)

const actualCost = computed(() => Math.max(0, props.data.stats.total_actual_cost || 0))
const officialCostCny = computed(() => Math.max(0, (props.data.stats.total_cost || 0) * props.data.exchangeRate))
const savedCostCny = computed(() => Math.max(0, officialCostCny.value - actualCost.value))
const modelLines = computed(() => buildReceiptModelLines(props.data.models))
const dateRangeLabel = computed(() => formatReceiptDateRange(props.data.startDate, props.data.endDate))
const generatedAtLabel = computed(() => formatBeijingDateTime(props.data.generatedAt))

function formatCurrency(value: number): string {
  return Math.max(0, Number.isFinite(value) ? value : 0).toFixed(2)
}

function formatDuration(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '—'
  if (value < 1000) return `${Math.round(value)} ms`
  return `${(value / 1000).toFixed(2)} s`
}

function getElement(): HTMLElement | null {
  return receiptElement.value
}

defineExpose({ getElement })
</script>

<style scoped>
.usage-receipt-paper {
  position: relative;
  width: min(100%, 420px);
  padding: 36px 30px 34px;
  color: rgb(var(--receipt-ink));
  background:
    linear-gradient(rgb(var(--receipt-rule) / 0.035) 1px, transparent 1px),
    rgb(var(--receipt-paper));
  background-size: 100% 6px;
  box-shadow: 0 26px 70px rgb(var(--shadow-ink) / 0.28);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "PingFang SC", "Microsoft YaHei", monospace;
  isolation: isolate;
}

.usage-receipt-paper::before,
.usage-receipt-paper::after {
  position: absolute;
  left: 0;
  z-index: -1;
  width: 100%;
  height: 12px;
  content: '';
  background: linear-gradient(135deg, rgb(var(--receipt-paper)) 6px, transparent 0) 0 0 / 12px 12px repeat-x;
}

.usage-receipt-paper::before {
  top: -11px;
  transform: rotate(180deg);
}

.usage-receipt-paper::after {
  bottom: -11px;
}

.receipt-brand {
  text-align: center;
}

.receipt-brand__mark {
  width: 58px;
  height: 58px;
  margin: 0 auto 10px;
  overflow: hidden;
  border: 2px solid rgb(var(--receipt-ink));
  border-radius: 15px;
  background: rgb(var(--receipt-paper-deep));
}

.receipt-brand__logo {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.receipt-brand__fallback {
  display: grid;
  width: 100%;
  height: 100%;
  place-items: center;
  font-family: "EB Garamond", "Songti SC", serif;
  font-size: 30px;
  font-weight: 800;
}

.receipt-brand__name {
  font-family: Cinzel, "Noto Serif SC", "Songti SC", serif;
  font-size: 20px;
  font-weight: 800;
  letter-spacing: 0.08em;
}

.receipt-brand__title {
  margin-top: 2px;
  font-size: 24px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.receipt-brand__english {
  margin-top: 4px;
  color: rgb(var(--receipt-muted));
  font-family: Cinzel, serif;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.24em;
}

.receipt-rule {
  height: 0;
  margin: 18px 0;
  border-top: 1px dashed rgb(var(--receipt-rule));
}

.receipt-rule--heavy {
  border-top: 2px solid rgb(var(--receipt-ink));
}

.receipt-meta {
  display: grid;
  gap: 7px;
  margin: 0;
  font-size: 11px;
}

.receipt-meta__row {
  display: grid;
  grid-template-columns: auto minmax(12px, 1fr) auto;
  gap: 6px;
  align-items: baseline;
}

.receipt-meta__row dt,
.receipt-meta__row dd {
  margin: 0;
}

.receipt-meta__row dd {
  max-width: 225px;
  overflow-wrap: anywhere;
  text-align: right;
}

.receipt-meta__dots {
  height: 1px;
  border-bottom: 1px dotted rgb(var(--receipt-rule));
}

.receipt-hero {
  margin-top: 22px;
  padding: 18px 16px 14px;
  border: 1px solid rgb(var(--receipt-ink));
  text-align: center;
}

.receipt-hero__eyebrow {
  color: rgb(var(--receipt-muted));
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.16em;
}

.receipt-hero__amount {
  margin-top: 3px;
  font-family: "EB Garamond", "Songti SC", serif;
  font-size: 45px;
  font-weight: 800;
  line-height: 1;
}

.receipt-hero__metrics {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  margin-top: 16px;
  border-top: 1px dashed rgb(var(--receipt-rule));
}

.receipt-hero__metrics > div {
  display: grid;
  gap: 2px;
  padding-top: 11px;
}

.receipt-hero__metrics > div + div {
  border-left: 1px dashed rgb(var(--receipt-rule));
}

.receipt-hero__metrics span {
  color: rgb(var(--receipt-muted));
  font-size: 9px;
}

.receipt-hero__metrics strong {
  font-size: 15px;
}

.receipt-section-heading,
.receipt-line-item,
.receipt-model__heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 14px;
}

.receipt-section-heading {
  margin-bottom: 10px;
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 0.12em;
}

.receipt-line-item {
  padding: 4px 0;
  color: rgb(var(--receipt-muted));
  font-size: 11px;
}

.receipt-line-item strong {
  color: rgb(var(--receipt-ink));
}

.receipt-model + .receipt-model {
  margin-top: 11px;
}

.receipt-model__heading {
  padding-bottom: 4px;
  border-bottom: 1px dotted rgb(var(--receipt-rule));
  font-size: 11px;
}

.receipt-model__heading strong:first-child {
  min-width: 0;
  overflow-wrap: anywhere;
}

.receipt-model p,
.receipt-empty-line {
  margin-top: 4px;
  color: rgb(var(--receipt-muted));
  font-size: 9px;
}

.receipt-line-item--total {
  margin-top: 2px;
  color: rgb(var(--receipt-ink));
  font-size: 14px;
  font-weight: 900;
}

.receipt-totals__original {
  text-decoration: line-through;
}

.receipt-saved {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px solid rgb(var(--receipt-stamp));
  color: rgb(var(--receipt-stamp));
  font-size: 12px;
  font-weight: 900;
}

.receipt-totals__note {
  margin-top: 7px;
  color: rgb(var(--receipt-muted));
  font-size: 8px;
  text-align: right;
}

.receipt-footer {
  margin-top: 22px;
  padding-top: 18px;
  border-top: 2px dashed rgb(var(--receipt-rule));
  text-align: center;
}

.receipt-footer__thanks p {
  margin-top: 7px;
  font-family: "EB Garamond", "Songti SC", serif;
  font-size: 16px;
  font-weight: 700;
}

.receipt-footer__stamp {
  display: inline-block;
  padding: 4px 7px 3px;
  border: 2px solid rgb(var(--receipt-accent));
  color: rgb(var(--receipt-accent));
  font-size: 9px;
  font-weight: 900;
  letter-spacing: 0.12em;
  transform: rotate(-2deg);
}

.receipt-qr {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 18px 0 0;
  padding: 10px;
  border: 1px solid rgb(var(--receipt-ink));
  text-align: left;
}

.receipt-qr img {
  width: 86px;
  height: 86px;
  flex: 0 0 auto;
  background: white;
}

.receipt-qr div {
  display: grid;
  gap: 5px;
}

.receipt-qr strong {
  font-size: 11px;
}

.receipt-qr span {
  color: rgb(var(--receipt-muted));
  font-size: 9px;
}

.receipt-footer__privacy {
  margin-top: 16px;
  color: rgb(var(--receipt-muted));
  font-size: 8px;
}

.receipt-footer__signature {
  margin-top: 8px;
  font-family: Cinzel, serif;
  font-size: 7px;
  font-weight: 700;
  letter-spacing: 0.14em;
}

@media (max-width: 460px) {
  .usage-receipt-paper {
    padding-right: 22px;
    padding-left: 22px;
  }

  .receipt-meta__row dd {
    max-width: 180px;
  }
}
</style>
