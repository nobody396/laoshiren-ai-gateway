<template>
  <section class="billing-example">
    <h2 class="billing-example__title">{{ t('modelPricing.billing.title') }}</h2>
    <p class="billing-example__note">{{ t('modelPricing.billing.rateNote') }}</p>

    <h3 class="billing-example__subtitle">{{ t('modelPricing.billing.example') }}</h3>
    <p class="billing-example__sub">{{ t('modelPricing.billing.exampleSub') }}</p>

    <table class="billing-example__table">
      <thead>
        <tr>
          <th>{{ t('modelPricing.billing.colItem') }}</th>
          <th>{{ t('modelPricing.billing.colUsage') }}</th>
          <th>{{ t('modelPricing.billing.colUnitPrice') }}</th>
          <th>{{ t('modelPricing.billing.colAmount') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.label">
          <td>{{ row.label }}</td>
          <td>{{ row.usage }}</td>
          <td>{{ row.unit }}</td>
          <td>{{ row.amount }}</td>
        </tr>
        <tr class="billing-example__total-row">
          <td colspan="3">{{ t('modelPricing.billing.total') }}</td>
          <td>{{ officialTotal }}</td>
        </tr>
      </tbody>
    </table>

    <ol class="billing-example__steps">
      <li>{{ t('modelPricing.billing.stepRate') }}</li>
      <li>{{ t('modelPricing.billing.stepTopup') }}</li>
    </ol>
    <p class="billing-example__bottom">{{ t('modelPricing.billing.bottomLine') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const rows = [
  { label: t('modelPricing.billing.rowInput'), usage: '2,000,000', unit: '$5 / 百万', amount: '$10.00' },
  { label: t('modelPricing.billing.rowOutput'), usage: '500,000', unit: '$30 / 百万', amount: '$15.00' },
  { label: t('modelPricing.billing.rowCache'), usage: '2,000,000', unit: '$0.5 / 百万', amount: '$1.00' }
]
const officialTotal = '$26.00'
</script>

<style scoped>
.billing-example {
  margin-bottom: 2rem;
  padding: 1.5rem 1.5rem 1.25rem;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #f9fafb;
}

.billing-example__title {
  margin: 0 0 0.5rem;
  color: #111827;
  font-size: 1.125rem;
  font-weight: 700;
}

.billing-example__note {
  margin: 0;
  color: #374151;
  font-size: 0.9rem;
  line-height: 1.6;
}

.billing-example__subtitle {
  margin: 1.25rem 0 0.25rem;
  color: #111827;
  font-size: 0.95rem;
  font-weight: 600;
}

.billing-example__sub {
  margin: 0 0 0.75rem;
  color: #6b7280;
  font-size: 0.8125rem;
}

.billing-example__table {
  width: 100%;
  max-width: 640px;
  border-collapse: collapse;
  font-size: 0.8125rem;
}

.billing-example__table th {
  padding: 0.5rem 0.75rem;
  text-align: right;
  color: #6b7280;
  font-weight: 600;
  border-bottom: 1px solid #e5e7eb;
}

.billing-example__table th:first-child,
.billing-example__table td:first-child {
  text-align: left;
}

.billing-example__table td {
  padding: 0.5rem 0.75rem;
  text-align: right;
  color: #111827;
  border-bottom: 1px solid #f3f4f6;
  font-variant-numeric: tabular-nums;
}

.billing-example__total-row td {
  font-weight: 700;
  color: #111827;
  border-bottom: none;
}

.billing-example__steps {
  margin: 1rem 0 0.5rem;
  padding-left: 1.25rem;
  color: #374151;
  font-size: 0.875rem;
  line-height: 1.8;
}

.billing-example__bottom {
  margin: 0;
  padding-top: 0.75rem;
  border-top: 1px solid #e5e7eb;
  color: #111827;
  font-size: 0.875rem;
  font-weight: 600;
}
</style>
