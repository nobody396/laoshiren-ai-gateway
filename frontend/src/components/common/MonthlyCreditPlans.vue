<template>
  <section class="monthly-credit-plans" :class="`monthly-credit-plans--${variant}`">
    <div class="monthly-credit-plans__heading">
      <div>
        <p class="monthly-credit-plans__kicker">Credits Membership</p>
        <h2>{{ title }}</h2>
      </div>
      <p v-if="summary">{{ summary }}</p>
    </div>

    <div class="monthly-credit-plans__grid">
      <article
        v-for="plan in displayPlans"
        :key="plan.id"
        class="monthly-credit-card"
        :class="`monthly-credit-card--${plan.accent}`"
      >
        <div class="monthly-credit-card__top">
          <div>
            <span v-if="plan.rarityLabel" class="monthly-credit-card__rarity">{{ plan.rarityLabel }}</span>
            <h3>{{ plan.name }}</h3>
            <p>{{ plan.description }}</p>
          </div>
          <span class="monthly-credit-card__stock">正在供应</span>
        </div>

        <div class="monthly-credit-card__price">
          <strong>{{ plan.price }}</strong>
          <span>/ 月</span>
        </div>

        <div class="monthly-credit-card__credits">
          <div>
            <span>每月额度</span>
            <strong>{{ plan.displayMonthlyCreditsText }} AI credits</strong>
          </div>
        </div>

        <p v-if="plan.legendaryCopy" class="monthly-credit-card__legend">
          {{ plan.legendaryCopy }}
        </p>

        <dl v-if="showEntitlementDetails" class="monthly-credit-card__usage">
          <div>
            <dt>GPT Pro</dt>
            <dd>
              <strong>{{ plan.gptMonthlyUsage }}</strong>
              <span>{{ plan.gptMonthlyTokensText }}</span>
            </dd>
          </div>
          <div>
            <dt>Claude Max</dt>
            <dd>
              <strong>{{ plan.claudeMonthlyUsage }}</strong>
              <span>{{ plan.claudeMonthlyTokensText }}</span>
            </dd>
          </div>
        </dl>

        <button v-if="showAction" type="button" class="monthly-credit-card__button" :disabled="!plan.cardShopUrl">
          {{ plan.cardShopUrl ? '正在供应' : '等待开售' }}
        </button>
      </article>
    </div>

    <p class="monthly-credit-plans__note">
      一个订阅共享 GPT Pro 与 Claude Max 两个分组的额度池；额度按官方 API 计费折算，token 为真实长任务工作场景估算，实际随具体任务和缓存输出占比浮动。
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { monthlyCreditCardPlans, type MonthlyCreditCardPlan } from '@/constants/monthlyCreditCards'

const props = withDefaults(defineProps<{
  variant?: 'home' | 'app'
  title?: string
  summary?: string
  showEntitlementDetails?: boolean
  showAction?: boolean
  plans?: MonthlyCreditCardPlan[]
}>(), {
  variant: 'app',
  title: '开发者月卡',
  summary: '',
  showEntitlementDetails: true,
  showAction: true,
  plans: undefined
})

const displayPlans = computed(() => props.plans?.length ? props.plans : monthlyCreditCardPlans)
</script>

<style scoped>
.monthly-credit-plans {
  --membership-surface: rgb(var(--color-vellum) / 0.94);
  --membership-surface-soft: rgb(var(--color-parchment) / 0.58);
  --membership-border: rgb(var(--color-ink) / 0.14);
  --membership-border-strong: rgb(var(--color-ink) / 0.26);
  --membership-ink: rgb(var(--color-ink));
  --membership-muted: rgb(var(--color-gray-600));
  --membership-accent: rgb(var(--color-terracotta));
  --membership-shadow: 0 18px 46px rgb(var(--shadow-ink) / 0.1);
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
    linear-gradient(180deg, rgb(var(--color-vellum) / 0.46), transparent 42%),
    var(--membership-surface);
  box-shadow: var(--membership-shadow);
}

.monthly-credit-card--apex {
  grid-column: 1 / -1;
  min-height: 20rem;
  isolation: isolate;
  border-color: rgb(var(--gild-600) / 0.62);
  background:
    linear-gradient(135deg, rgb(var(--gild-400) / 0.12), rgb(var(--gild-900) / 0.08) 38%, transparent 62%),
    linear-gradient(115deg, rgb(var(--lacquer-rest-top) / 0.96), rgb(var(--color-dark-800) / 0.98) 48%, rgb(var(--lacquer-base) / 0.96));
  color: rgb(var(--gild-100));
  box-shadow:
    0 26px 70px rgb(var(--shadow-ink) / 0.22),
    inset 0 0 0 1px rgb(var(--gild-500) / 0.14);
}

.monthly-credit-card--apex::after {
  content: "";
  position: absolute;
  z-index: -1;
  inset: 0;
  opacity: 0.92;
  background:
    linear-gradient(90deg, transparent 0%, rgb(var(--gild-500) / 0.18) 46%, transparent 58%),
    repeating-linear-gradient(90deg, rgb(var(--gild-500) / 0.08) 0 1px, transparent 1px 3.1rem);
  transform: translateX(-42%);
  animation: monthly-apex-sheen 6.8s ease-in-out infinite;
}

.monthly-credit-card::before {
  content: "";
  position: absolute;
  inset: 0 0 auto;
  height: 0.28rem;
  background: rgb(var(--color-muted));
}

.monthly-credit-card--lite::before {
  background: rgb(var(--color-laurel));
}

.monthly-credit-card--pro::before {
  background: rgb(var(--color-info));
}

.monthly-credit-card--max::before {
  background: rgb(var(--color-terracotta));
}

.monthly-credit-card--ultra::before {
  background: rgb(var(--color-ink-deep));
}

.monthly-credit-card--apex::before {
  height: 0.34rem;
  background: linear-gradient(90deg, rgb(var(--gild-900)), rgb(var(--gild-600)) 24%, rgb(var(--gild-400)) 50%, rgb(var(--gild-700)) 76%, rgb(var(--lacquer-base)));
  box-shadow: 0 0 22px rgb(var(--gild-600) / 0.46);
}

.monthly-credit-card__rarity {
  display: inline-flex;
  margin-bottom: 0.55rem;
  color: rgb(var(--gild-500));
  font-size: 0.7rem;
  font-weight: 900;
  letter-spacing: 0.16em;
  line-height: 1;
  text-transform: uppercase;
}

.monthly-credit-card--apex .monthly-credit-card__top {
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
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

.monthly-credit-card--apex h3 {
  color: rgb(var(--gild-200));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(1.45rem, 2.5vw, 2rem);
  letter-spacing: 0;
  text-shadow: 0 0 18px rgb(var(--gild-500) / 0.2);
}

.monthly-credit-card p {
  min-height: 3.6rem;
  margin: 0.55rem 0 0;
  color: var(--membership-muted);
  font-size: 0.82rem;
  line-height: 1.55;
}

.monthly-credit-card--apex p {
  max-width: 45rem;
  color: rgb(var(--gild-200) / 0.74);
}

.monthly-credit-card__stock {
  justify-self: start;
  display: inline-flex;
  align-items: center;
  min-height: 1.55rem;
  padding: 0.2rem 0.55rem;
  border: 1px solid rgb(var(--color-terracotta) / 0.22);
  border-radius: 999px;
  background: rgb(var(--color-terracotta) / 0.08);
  color: rgb(var(--color-terracotta));
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1;
}

.monthly-credit-card--apex .monthly-credit-card__stock {
  border-color: rgb(var(--gild-500) / 0.42);
  background: rgb(var(--gild-500) / 0.1);
  color: rgb(var(--gild-500));
  box-shadow: inset 0 0 18px rgb(var(--gild-500) / 0.08);
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

.monthly-credit-card--apex .monthly-credit-card__price strong {
  color: rgb(var(--gild-300));
  font-size: clamp(2.2rem, 4vw, 3rem);
}

.monthly-credit-card__price span {
  color: var(--membership-muted);
  font-size: 0.9rem;
}

.monthly-credit-card--apex .monthly-credit-card__price span {
  color: rgb(var(--gild-200) / 0.7);
}

.monthly-credit-card__credits {
  display: grid;
  gap: 0.65rem;
  margin-top: 1.1rem;
  padding: 0.8rem;
  border: 1px solid var(--membership-border);
  border-radius: 6px;
  background: var(--membership-surface-soft);
}

.monthly-credit-card--apex .monthly-credit-card__credits {
  margin-top: 1.25rem;
  border-color: rgb(var(--gild-500) / 0.28);
  background:
    linear-gradient(135deg, rgb(var(--gild-500) / 0.16), rgb(var(--color-primary-400) / 0.08)),
    rgb(var(--lacquer-base) / 0.44);
}

.monthly-credit-card__credits div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.monthly-credit-card__credits span {
  color: var(--membership-muted);
  font-size: 0.76rem;
  font-weight: 700;
}

.monthly-credit-card--apex .monthly-credit-card__credits span {
  color: rgb(var(--gild-200) / 0.72);
}

.monthly-credit-card__credits strong {
  color: var(--membership-ink);
  font-size: 0.98rem;
  font-weight: 800;
  line-height: 1.15;
}

.monthly-credit-card--apex .monthly-credit-card__credits strong {
  color: rgb(var(--gild-100));
  font-size: clamp(1rem, 2vw, 1.22rem);
}

.monthly-credit-card__legend {
  min-height: 0 !important;
  margin-top: 1rem !important;
  border-left: 2px solid rgb(var(--gild-500) / 0.54);
  padding-left: 0.8rem;
  color: rgb(var(--gild-300) / 0.86) !important;
  font-size: 0.86rem !important;
  font-weight: 650;
  line-height: 1.8 !important;
}

.monthly-credit-card__usage {
  display: grid;
  gap: 0.55rem;
  margin: 1rem 0 0;
}

.monthly-credit-card--apex .monthly-credit-card__usage {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.8rem;
}

.monthly-credit-card__usage div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 2.2rem;
  border-bottom: 1px solid rgb(var(--color-ink) / 0.08);
}

.monthly-credit-card--apex .monthly-credit-card__usage div {
  min-height: 4.2rem;
  border: 1px solid rgb(var(--gild-500) / 0.22);
  border-radius: 7px;
  background: rgb(var(--gild-200) / 0.06);
  padding: 0.85rem;
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

.monthly-credit-card--apex .monthly-credit-card__usage dt {
  color: rgb(var(--gild-200) / 0.72);
}

.monthly-credit-card__usage dd {
  display: grid;
  gap: 0.18rem;
  color: var(--membership-ink);
  text-align: right;
}

.monthly-credit-card__usage dd strong {
  color: var(--membership-ink);
  font-size: 0.8rem;
  font-weight: 800;
}

.monthly-credit-card--apex .monthly-credit-card__usage dd strong {
  color: rgb(var(--gild-100));
}

.monthly-credit-card__usage dd span {
  color: var(--membership-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.monthly-credit-card--apex .monthly-credit-card__usage dd span {
  color: rgb(var(--color-terracotta-dark));
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
  background: rgb(var(--color-ink) / 0.08);
  color: var(--membership-muted);
  font-size: 0.88rem;
  font-weight: 800;
  cursor: not-allowed;
}

.monthly-credit-card--apex .monthly-credit-card__button {
  border-color: rgb(var(--gild-500) / 0.42);
  background: rgb(var(--gild-500) / 0.12);
  color: rgb(var(--gild-500));
}

.monthly-credit-plans__note {
  margin: 1rem 0 0;
  color: var(--membership-muted);
  font-size: 0.84rem;
  line-height: 1.75;
}

.monthly-credit-plans--app {
  --membership-surface: var(--admin-control, rgb(var(--color-vellum) / 0.95));
  --membership-surface-soft: var(--admin-surface-soft, rgb(var(--color-stone) / 0.72));
  --membership-border: var(--admin-border, rgb(var(--color-ink) / 0.14));
  --membership-border-strong: var(--admin-border-strong, rgb(var(--color-ink) / 0.32));
  --membership-ink: var(--admin-ink, rgb(var(--color-ink)));
  --membership-muted: var(--admin-muted, rgb(var(--color-muted)));
  --membership-accent: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
  --membership-shadow: var(--admin-shadow-sm, 0 8px 24px rgb(var(--shadow-ink) / 0.08));
}

.dark .monthly-credit-plans--app {
  --membership-surface: rgb(var(--color-indigo-900) / 0.96);
  --membership-surface-soft: rgb(var(--color-indigo-800) / 0.82);
  --membership-border: rgb(var(--color-vellum) / 0.1);
  --membership-border-strong: rgb(var(--color-vellum) / 0.18);
  --membership-ink: rgb(var(--color-slate-50));
  --membership-muted: rgb(var(--color-gray-400));
  --membership-accent: rgb(var(--color-amber-500));
  --membership-shadow: 0 12px 32px rgb(var(--lacquer-base) / 0.22);
}

.dark .monthly-credit-card__usage div {
  border-bottom-color: rgb(var(--color-vellum) / 0.08);
}

@keyframes monthly-apex-sheen {
  0%,
  62% {
    transform: translateX(-46%);
    opacity: 0.42;
  }
  82% {
    transform: translateX(42%);
    opacity: 0.88;
  }
  100% {
    transform: translateX(46%);
    opacity: 0.42;
  }
}

@media (prefers-reduced-motion: reduce) {
  .monthly-credit-card--apex::after {
    animation: none;
    transform: none;
  }
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

  .monthly-credit-card--apex .monthly-credit-card__usage {
    grid-template-columns: 1fr;
  }

  .monthly-credit-card p {
    min-height: auto;
  }
}
</style>
