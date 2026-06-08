<template>
  <section class="monthly-credit-plans" :class="`monthly-credit-plans--${variant}`">
    <div class="monthly-credit-plans__heading">
      <div>
        <p class="monthly-credit-plans__kicker">Credits Membership</p>
        <h2>{{ title }}</h2>
      </div>
      <p>{{ summary }}</p>
    </div>

    <div class="monthly-credit-plans__grid">
      <article
        v-for="plan in monthlyCreditCardPlans"
        :key="plan.id"
        class="monthly-credit-card"
        :class="`monthly-credit-card--${plan.accent}`"
      >
        <div class="monthly-credit-card__top">
          <div>
            <h3>{{ plan.name }}</h3>
            <p>{{ plan.description }}</p>
          </div>
          <span class="monthly-credit-card__stock">暂时缺货</span>
        </div>

        <div class="monthly-credit-card__price">
          <strong>{{ plan.price }}</strong>
          <span>/ 月</span>
        </div>

        <div class="monthly-credit-card__credits">
          <span>每天</span>
          <strong>{{ plan.dailyCredits }} credits</strong>
        </div>

        <dl v-if="showEntitlementDetails" class="monthly-credit-card__usage">
          <div>
            <dt>GPT Pro</dt>
            <dd>{{ plan.gptUsage }}</dd>
          </div>
          <div>
            <dt>Claude Max</dt>
            <dd>{{ plan.claudeUsage }}</dd>
          </div>
        </dl>

        <button v-if="showAction" type="button" class="monthly-credit-card__button" disabled>
          暂时缺货
        </button>
      </article>
    </div>

    <p class="monthly-credit-plans__note">
      每月更新，不结转。
    </p>
  </section>
</template>

<script setup lang="ts">
import { monthlyCreditCardPlans } from '@/constants/monthlyCreditCards'

withDefaults(defineProps<{
  variant?: 'home' | 'app'
  title?: string
  summary?: string
  showEntitlementDetails?: boolean
  showAction?: boolean
}>(), {
  variant: 'app',
  title: '开发者月卡',
  summary: '固定每日 credits 池，GPT Pro 与 Claude Max 共用。',
  showEntitlementDetails: true,
  showAction: true
})
</script>

<style scoped>
.monthly-credit-plans {
  --membership-surface: rgba(255, 252, 245, 0.94);
  --membership-surface-soft: rgba(242, 233, 210, 0.58);
  --membership-border: rgba(31, 26, 18, 0.14);
  --membership-border-strong: rgba(31, 26, 18, 0.26);
  --membership-ink: #1f1a12;
  --membership-muted: #7b705d;
  --membership-accent: #9a3b1f;
  --membership-shadow: 0 18px 46px rgba(49, 38, 20, 0.1);
  color: var(--membership-ink);
}

.monthly-credit-plans--home {
  margin: 0 auto 2.5rem;
}

.monthly-credit-plans--app {
  margin-top: 1.25rem;
}

.monthly-credit-plans__heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1.5rem;
  margin-bottom: 1.25rem;
}

.monthly-credit-plans__heading h2 {
  margin: 0;
  color: var(--membership-ink);
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(1.8rem, 3vw, 2.45rem);
  font-weight: 600;
  line-height: 1.12;
  letter-spacing: 0;
}

.monthly-credit-plans--app .monthly-credit-plans__heading h2 {
  font-family: inherit;
  font-size: clamp(1.55rem, 3vw, 2.05rem);
  font-weight: 700;
}

.monthly-credit-plans__heading p {
  max-width: 32rem;
  margin: 0;
  color: var(--membership-muted);
  font-size: 0.92rem;
  line-height: 1.7;
}

.monthly-credit-plans__kicker {
  display: block;
  margin: 0 0 0.55rem !important;
  color: var(--membership-accent) !important;
  font-family: 'Inter', sans-serif;
  font-size: 0.72rem !important;
  font-weight: 700;
  letter-spacing: 0.12em;
  line-height: 1.2 !important;
  text-transform: uppercase;
}

.monthly-credit-plans__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.9rem;
}

.monthly-credit-card {
  position: relative;
  overflow: hidden;
  min-height: 22rem;
  padding: 1rem;
  border: 1px solid var(--membership-border);
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.46), transparent 42%),
    var(--membership-surface);
  box-shadow: var(--membership-shadow);
}

.monthly-credit-card::before {
  content: "";
  position: absolute;
  inset: 0 0 auto;
  height: 0.28rem;
  background: #8a7d63;
}

.monthly-credit-card--lite::before {
  background: #3f5a3a;
}

.monthly-credit-card--pro::before {
  background: #4b6faf;
}

.monthly-credit-card--max::before {
  background: #9a3b1f;
}

.monthly-credit-card--ultra::before {
  background: #13100b;
}

.monthly-credit-card__top {
  display: grid;
  gap: 0.75rem;
}

.monthly-credit-card h3 {
  margin: 0;
  color: var(--membership-ink);
  font-size: 1.1rem;
  font-weight: 800;
  line-height: 1.25;
}

.monthly-credit-card p {
  min-height: 3.6rem;
  margin: 0.55rem 0 0;
  color: var(--membership-muted);
  font-size: 0.82rem;
  line-height: 1.55;
}

.monthly-credit-card__stock {
  justify-self: start;
  display: inline-flex;
  align-items: center;
  min-height: 1.55rem;
  padding: 0.2rem 0.55rem;
  border: 1px solid rgba(154, 59, 31, 0.22);
  border-radius: 999px;
  background: rgba(154, 59, 31, 0.08);
  color: #9a3b1f;
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1;
}

.monthly-credit-card__price {
  display: flex;
  align-items: baseline;
  gap: 0.35rem;
  margin-top: 1.45rem;
}

.monthly-credit-card__price strong {
  color: var(--membership-ink);
  font-size: 2.1rem;
  font-weight: 850;
  line-height: 1;
  font-feature-settings: 'lnum' 1, 'tnum' 1;
}

.monthly-credit-card__price span {
  color: var(--membership-muted);
  font-size: 0.9rem;
}

.monthly-credit-card__credits {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-top: 1.1rem;
  padding: 0.8rem;
  border: 1px solid var(--membership-border);
  border-radius: 6px;
  background: var(--membership-surface-soft);
}

.monthly-credit-card__credits span {
  color: var(--membership-muted);
  font-size: 0.76rem;
  font-weight: 700;
}

.monthly-credit-card__credits strong {
  color: var(--membership-ink);
  font-size: 0.98rem;
  font-weight: 800;
  line-height: 1.15;
}

.monthly-credit-card__usage {
  display: grid;
  gap: 0.55rem;
  margin: 1rem 0 0;
}

.monthly-credit-card__usage div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 2.2rem;
  border-bottom: 1px solid rgba(31, 26, 18, 0.08);
}

.monthly-credit-card__usage dt,
.monthly-credit-card__usage dd {
  margin: 0;
  font-size: 0.8rem;
}

.monthly-credit-card__usage dt {
  color: var(--membership-muted);
  font-weight: 700;
}

.monthly-credit-card__usage dd {
  color: var(--membership-ink);
  font-weight: 800;
  text-align: right;
}

.monthly-credit-card__button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 2.7rem;
  margin-top: 1.1rem;
  border: 1px solid var(--membership-border-strong);
  border-radius: 6px;
  background: rgba(31, 26, 18, 0.08);
  color: var(--membership-muted);
  font-size: 0.88rem;
  font-weight: 800;
  cursor: not-allowed;
}

.monthly-credit-plans__note {
  margin: 1rem 0 0;
  color: var(--membership-muted);
  font-size: 0.84rem;
  line-height: 1.75;
}

.monthly-credit-plans--app {
  --membership-surface: var(--admin-control, rgba(255, 252, 245, 0.95));
  --membership-surface-soft: var(--admin-surface-soft, rgba(239, 230, 207, 0.72));
  --membership-border: var(--admin-border, rgba(31, 26, 18, 0.14));
  --membership-border-strong: var(--admin-border-strong, rgba(31, 26, 18, 0.32));
  --membership-ink: var(--admin-ink, #1f1a12);
  --membership-muted: var(--admin-muted, #8a7d63);
  --membership-accent: var(--admin-terracotta-dark, #7a2d17);
  --membership-shadow: var(--admin-shadow-sm, 0 8px 24px rgba(49, 38, 20, 0.08));
}

.dark .monthly-credit-plans--app {
  --membership-surface: rgba(26, 31, 38, 0.96);
  --membership-surface-soft: rgba(42, 47, 55, 0.82);
  --membership-border: rgba(255, 255, 255, 0.1);
  --membership-border-strong: rgba(255, 255, 255, 0.18);
  --membership-ink: #f8fafc;
  --membership-muted: #9ca3af;
  --membership-accent: #f59e0b;
  --membership-shadow: 0 12px 32px rgba(0, 0, 0, 0.22);
}

.dark .monthly-credit-card__usage div {
  border-bottom-color: rgba(255, 255, 255, 0.08);
}

@media (max-width: 1100px) {
  .monthly-credit-plans__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .monthly-credit-plans__heading {
    display: grid;
    align-items: start;
  }

  .monthly-credit-plans__grid {
    grid-template-columns: 1fr;
  }

  .monthly-credit-card {
    min-height: auto;
  }

  .monthly-credit-card p {
    min-height: auto;
  }
}
</style>
