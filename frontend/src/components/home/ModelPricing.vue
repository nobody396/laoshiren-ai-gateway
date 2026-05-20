<template>
  <section id="model-pricing" class="model-pricing">
    <div class="model-pricing__container mirror-reveal">
      <div class="greco-divider" aria-hidden="true"></div>
      <div class="pricing-heading">
        <p class="section-eyebrow">III · 价格铭文</p>
        <h2 class="section-title">Pretium · 模型定价</h2>
        <p class="section-lede">官方价 × 分组倍率；单位：每 100 万 tokens。</p>
      </div>

      <div class="discount-ledger">
        <span><strong>Max</strong> ¥4 = $1 · 约 5.7 折</span>
        <span><strong>Pro</strong> ¥1.2 = $1 · 约 1.7 折</span>
      </div>

      <div class="pricing-stack">
        <article class="pricing-frame">
          <div class="pricing-frame__header">
            <span class="provider-tag provider-tag--claude">Claude</span>
            <div>
              <h3>Claude 价格</h3>
            </div>
          </div>

          <div class="table-wrapper">
            <table class="pricing-table pricing-table--claude">
              <thead>
                <tr>
                  <th>Model</th>
                  <th>官方价格</th>
                  <th>Max 分组</th>
                  <th>折扣</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in claudeRows" :key="row.model">
                  <td class="model-name">{{ row.model }}</td>
                  <td>
                    <div class="rate-stack">
                      <span><strong>Input</strong><em>{{ row.official.input }}</em></span>
                      <span><strong>5m Write</strong><em>{{ row.official.cacheWrite5m }}</em></span>
                      <span><strong>Read</strong><em>{{ row.official.cacheRead }}</em></span>
                      <span><strong>Output</strong><em>{{ row.official.output }}</em></span>
                    </div>
                  </td>
                  <td>
                    <div class="rate-stack">
                      <span><strong>Input</strong><em>{{ row.max.input }}</em></span>
                      <span><strong>5m Write</strong><em>{{ row.max.cacheWrite5m }}</em></span>
                      <span><strong>Read</strong><em>{{ row.max.cacheRead }}</em></span>
                      <span><strong>Output</strong><em>{{ row.max.output }}</em></span>
                    </div>
                  </td>
                  <td><span class="discount-tag">{{ row.discount }}</span></td>
                </tr>
              </tbody>
            </table>
          </div>
        </article>

        <article class="pricing-frame">
          <div class="pricing-frame__header">
            <span class="provider-tag provider-tag--gpt">GPT</span>
            <div>
              <h3>GPT 价格</h3>
            </div>
          </div>

          <div class="table-wrapper">
            <table class="pricing-table pricing-table--gpt">
              <thead>
                <tr>
                  <th>Model</th>
                  <th>官方价格</th>
                  <th>Pro 分组</th>
                  <th>折扣</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in gptRows" :key="row.model">
                  <td class="model-name">{{ row.model }}</td>
                  <td>
                    <div class="rate-stack">
                      <span><strong>Input</strong><em>{{ row.official.input }}</em></span>
                      <span><strong>Cached</strong><em>{{ row.official.cachedInput }}</em></span>
                      <span><strong>Output</strong><em>{{ row.official.output }}</em></span>
                    </div>
                  </td>
                  <td>
                    <div class="rate-stack">
                      <span><strong>Input</strong><em>{{ row.pro.input }}</em></span>
                      <span><strong>Cached</strong><em>{{ row.pro.cachedInput }}</em></span>
                      <span><strong>Output</strong><em>{{ row.pro.output }}</em></span>
                    </div>
                  </td>
                  <td>
                    <span class="discount-tag">Pro {{ row.discount }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </article>
      </div>

      <p class="pricing-footnote">
        折扣按 1 USD = ¥7 估算，仅用于展示与官方人民币折算价的相对优惠。
      </p>

      <div class="model-pricing__cta">
        <router-link
          :to="isAuthenticated ? '/dashboard' : '/login'"
          class="cta-btn"
        >
          免费注册 · 立即体验
        </router-link>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 模型定价表格
 */
type ClaudePriceSet = {
  input: string
  cacheWrite5m: string
  cacheRead: string
  output: string
}

type ClaudePricingRow = {
  model: string
  official: ClaudePriceSet
  max: ClaudePriceSet
  discount: string
}

type GptPriceSet = {
  input: string
  cachedInput: string
  output: string
}

type GptPricingRow = {
  model: string
  official: GptPriceSet
  pro: GptPriceSet
  discount: string
}

defineProps<{
  claudeRows: ClaudePricingRow[]
  gptRows: GptPricingRow[]
  isAuthenticated: boolean
}>()
</script>

<style scoped>
.model-pricing {
  padding: 6rem 0 5.5rem;
  background: #f8f3e7;
  scroll-margin-top: 88px;
}

.model-pricing__container {
  width: min(100% - 4rem, 1320px);
  margin: 0 auto;
}

.greco-divider {
  width: 15rem;
  height: 1.125rem;
  margin: 0 auto 3.75rem;
  opacity: 0.42;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 40 18'><path d='M0,9 L8,9 L8,3 L16,3 L16,15 L24,15 L24,3 L32,3 L32,9 L40,9' stroke='%233F5A3A' fill='none' stroke-width='1.5'/></svg>");
  background-repeat: repeat-x;
  background-size: 40px 18px;
}

.pricing-heading {
  text-align: center;
}

.section-eyebrow {
  margin: 0 0 0.875rem;
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.section-title {
  margin: 0;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2.5rem, 4.4vw, 3.75rem);
  font-weight: 500;
  line-height: 1.1;
}

.section-lede {
  margin: 1rem auto 0;
  color: #1f1a12;
  font-size: 1.18rem;
  font-style: italic;
  line-height: 1.65;
}

.discount-ledger {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin: 2rem auto 4rem;
}

.discount-ledger span {
  display: inline-flex;
  align-items: center;
  min-height: 2.25rem;
  padding: 0.45rem 0.9rem;
  border: 1px solid rgba(63, 90, 58, 0.22);
  background: rgba(250, 246, 236, 0.72);
  color: #1f1a12;
  font-family: 'Inter', sans-serif;
  font-size: 0.78rem;
  letter-spacing: 0.02em;
}

.discount-ledger strong {
  margin-right: 0.4rem;
  color: #9a3b1f;
}

.pricing-stack {
  display: grid;
  gap: 2.25rem;
}

.pricing-frame {
  padding: 0.5rem;
  background: #faf6ec;
  border: 1px solid rgba(63, 90, 58, 0.18);
  box-shadow: 0 1px 0 rgba(63, 90, 58, 0.08), 0 0 0 1px rgba(63, 90, 58, 0.04);
}

.pricing-frame__header {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 1.25rem 1.25rem 1rem;
  border-bottom: 1px solid rgba(63, 90, 58, 0.14);
}

.pricing-frame__header h3 {
  margin: 0;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 1.35rem;
  font-weight: 600;
  line-height: 1.25;
}

.pricing-frame__header p {
  margin: 0.35rem 0 0;
  color: #6f634f;
  font-size: 0.98rem;
  line-height: 1.45;
}

.table-wrapper {
  overflow-x: auto;
}

.pricing-table {
  width: 100%;
  border-collapse: collapse;
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
}

.pricing-table--claude {
  min-width: 760px;
}

.pricing-table--gpt {
  min-width: 760px;
}

.pricing-table th {
  padding: 1.125rem 0.875rem;
  border-bottom: 1.5px solid #1f1a12;
  color: #8a7d63;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 0.7rem;
  font-weight: 500;
  letter-spacing: 0.12em;
  text-align: center;
  text-transform: uppercase;
  white-space: nowrap;
}

.pricing-table td {
  padding: 1.125rem 0.875rem;
  border-bottom: 1px solid rgba(63, 90, 58, 0.1);
  color: #1f1a12;
  font-size: 1rem;
  text-align: center;
  vertical-align: middle;
}

.pricing-table tr:nth-child(even) td {
  background: #efe6cf;
}

.pricing-table tr:hover td {
  background: #f2e9d2;
}

.pricing-table tr:last-child td {
  border-bottom: 0;
}

.model-name {
  color: #13100b;
  font-feature-settings: 'lnum' 1, 'tnum' 1;
  font-size: 1.08rem;
  font-weight: 600;
  white-space: nowrap;
}

.provider-tag,
.discount-tag {
  display: inline-flex;
  align-items: center;
  padding: 0.3rem 0.7rem;
  border-radius: 2px;
  font-family: 'Inter', sans-serif;
  font-size: 0.625rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  line-height: 1.2;
  text-transform: uppercase;
  white-space: nowrap;
}

.provider-tag--claude {
  background: rgba(154, 59, 31, 0.1);
  color: #7a2d17;
}

.provider-tag--gpt {
  background: rgba(63, 90, 58, 0.12);
  color: #26361f;
}

.discount-tag {
  background: #9a3b1f;
  color: #faf6ec;
}

.discount-tag--muted {
  background: #3f5a3a;
}

.rate-stack {
  display: inline-grid;
  justify-items: center;
  gap: 0.48rem;
}

.rate-stack span {
  display: grid;
  justify-items: center;
  gap: 0.16rem;
  color: #13100b;
  font-feature-settings: 'lnum' 1, 'tnum' 1;
}

.rate-stack strong {
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.64rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.rate-stack em {
  color: #13100b;
  font-size: 1.05rem;
  font-style: normal;
  font-weight: 600;
  line-height: 1.2;
}

.pricing-footnote {
  margin: 1.25rem 0 0;
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.78rem;
  line-height: 1.6;
  text-align: center;
}

.model-pricing__cta {
  display: flex;
  justify-content: center;
  margin-top: 4rem;
}

.cta-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 3.5rem;
  padding: 1rem 3rem;
  background: #1f1a12;
  border: 1.5px solid #1f1a12;
  color: #f8f3e7;
  font-family: 'Inter', sans-serif;
  font-size: 0.85rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-decoration: none;
  text-transform: uppercase;
  transition: background 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

.cta-btn:hover {
  background: #9a3b1f;
  border-color: #9a3b1f;
  transform: translateY(-1px);
}

@media (max-width: 768px) {
  .model-pricing__container {
    width: min(100% - 2rem, 1320px);
  }

  .pricing-frame__header {
    flex-direction: column;
  }

  .cta-btn {
    width: 100%;
    padding-inline: 1.25rem;
  }
}
</style>
