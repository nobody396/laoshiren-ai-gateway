<template>
  <section class="monthly-credit-plans" :class="`monthly-credit-plans--${variant}`">
    <div class="monthly-credit-plans__heading" :class="{ 'mirror-reveal': variant === 'home' }">
      <div>
        <p class="monthly-credit-plans__kicker">Credits Membership</p>
        <h2>{{ title }}</h2>
      </div>
      <p v-if="summary">{{ summary }}</p>
    </div>

    <div class="monthly-credit-plans__grid-shell">
      <div v-if="variant === 'home'" class="monthly-credit-plans__rail" aria-hidden="true">
        <span>⚡</span>
        <span>⚡</span>
      </div>

      <div class="monthly-credit-plans__grid">
        <div
          v-for="(plan, index) in displayPlans"
          :key="plan.id"
          class="monthly-credit-card__reveal"
          :class="{ 'mirror-reveal': variant === 'home' }"
          :style="{ '--reveal-i': index + 1 }"
        >
          <article
            class="monthly-credit-card"
            :class="`monthly-credit-card--${plan.accent}`"
            @pointermove="updateCardMotion"
            @pointerleave="resetCardMotion"
          >
            <span class="monthly-credit-card__corner" aria-hidden="true">⚡</span>

            <div class="monthly-credit-card__top">
              <div class="monthly-credit-card__identity">
                <span v-if="plan.rarityLabel" class="monthly-credit-card__rarity">{{ plan.rarityLabel }}</span>
                <h3>{{ plan.name }}</h3>
                <span class="monthly-credit-card__stock">
                  <span aria-hidden="true"></span>
                  正在供应
                </span>
              </div>
              <p>{{ plan.description }}</p>
            </div>

            <div class="monthly-credit-card__price">
              <strong>{{ plan.price }}</strong>
              <span>/ 31 天</span>
            </div>
            <p class="monthly-credit-card__direct-price">直售 {{ plan.directPrice }}</p>
            <p class="monthly-credit-card__direct-help">直售请登录后联系客服</p>

            <div class="monthly-credit-card__credits">
              <span class="monthly-credit-card__credits-label">每月额度</span>
              <strong>{{ plan.displayMonthlyCreditsText }} AI credits</strong>
            </div>

            <p v-if="plan.legendaryCopy" class="monthly-credit-card__legend">
              {{ plan.legendaryCopy }}
            </p>

            <a
              v-if="plan.cardShopUrl && (showAction || variant === 'home')"
              class="monthly-credit-card__action"
              :href="plan.cardShopUrl"
              target="_blank"
              rel="noopener noreferrer"
              :aria-label="`${plan.name} ${variant === 'home' ? '查看并购买' : '立即购买'}`"
            >
              <span>{{ variant === 'home' ? '查看并购买' : '立即购买' }}</span>
              <span class="monthly-credit-card__action-arrow" aria-hidden="true">→</span>
            </a>
            <span v-else-if="showAction" class="monthly-credit-card__action monthly-credit-card__action--disabled">
              等待开售
            </span>
          </article>
        </div>
      </div>
    </div>

    <p class="monthly-credit-plans__note" :class="{ 'mirror-reveal': variant === 'home' }" style="--reveal-i: 4">
      只限制 31 天月度总额度，不设置每日或每周额度。GPT 月卡、Claude 月卡和 Grok 月卡共用同一份月度额度；
      具体支持模型请查看 <RouterLink to="/models">模型定价</RouterLink>。当前周期的计费倍率和额度不会被后续调价追溯修改。
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
  showAction?: boolean
  plans?: MonthlyCreditCardPlan[]
}>(), {
  variant: 'app',
  title: '开发者月卡',
  summary: '',
  showAction: true,
  plans: undefined
})

const displayPlans = computed(() => props.plans?.length ? props.plans : monthlyCreditCardPlans)

function updateCardMotion(event: PointerEvent): void {
  if (event.pointerType === 'touch') return

  const card = event.currentTarget as HTMLElement
  const rect = card.getBoundingClientRect()
  const x = Math.min(Math.max((event.clientX - rect.left) / rect.width, 0), 1)
  const y = Math.min(Math.max((event.clientY - rect.top) / rect.height, 0), 1)

  card.style.setProperty('--spot-x', `${(x * 100).toFixed(1)}%`)
  card.style.setProperty('--spot-y', `${(y * 100).toFixed(1)}%`)
  card.style.setProperty('--tilt-x', `${((0.5 - y) * 1.6).toFixed(2)}deg`)
  card.style.setProperty('--tilt-y', `${((x - 0.5) * 1.8).toFixed(2)}deg`)
}

function resetCardMotion(event: PointerEvent): void {
  const card = event.currentTarget as HTMLElement
  card.style.setProperty('--spot-x', '50%')
  card.style.setProperty('--spot-y', '0%')
  card.style.setProperty('--tilt-x', '0deg')
  card.style.setProperty('--tilt-y', '0deg')
}
</script>

<style scoped>
.monthly-credit-plans {
  --membership-surface: rgb(var(--color-vellum) / 0.96);
  --membership-surface-soft: rgb(var(--color-parchment) / 0.36);
  --membership-border: rgb(var(--color-ink) / 0.15);
  --membership-border-strong: rgb(var(--color-ink) / 0.25);
  --membership-ink: rgb(var(--color-ink));
  --membership-muted: rgb(var(--color-gray-600));
  --membership-accent: rgb(var(--color-terracotta));
  --membership-lightning: #e6a325;
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
  margin-bottom: 1.75rem;
}

.monthly-credit-plans__heading h2 {
  margin: 0;
  color: var(--membership-ink);
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2.15rem, 3.8vw, 3.35rem);
  font-weight: 600;
  line-height: 1.06;
  letter-spacing: -0.035em;
}

.monthly-credit-plans--app .monthly-credit-plans__heading h2 {
  font-family: inherit;
  font-size: clamp(1.55rem, 3vw, 2.05rem);
  font-weight: 700;
  letter-spacing: 0;
}

.monthly-credit-plans__heading > p {
  max-width: 32rem;
  margin: 0;
  color: var(--membership-muted);
  font-size: 0.92rem;
  line-height: 1.7;
}

.monthly-credit-plans__kicker {
  display: block;
  margin: 0 0 0.65rem !important;
  color: var(--membership-accent) !important;
  font-family: 'Inter', sans-serif;
  font-size: 0.72rem !important;
  font-weight: 750;
  letter-spacing: 0.16em;
  line-height: 1.2 !important;
  text-transform: uppercase;
}

.monthly-credit-plans__grid-shell {
  position: relative;
}

.monthly-credit-plans__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1.25rem;
}

.monthly-credit-plans__rail {
  position: absolute;
  top: 50%;
  right: 16.666%;
  left: 16.666%;
  height: 1px;
  pointer-events: none;
  background: linear-gradient(
    90deg,
    rgb(var(--color-laurel) / 0.72),
    var(--membership-lightning),
    rgb(var(--color-info) / 0.72),
    var(--membership-lightning),
    rgb(var(--color-terracotta) / 0.72)
  );
  background-size: 200% 100%;
  box-shadow: 0 0 12px rgb(230 163 37 / 0.2);
  animation: monthly-power-flow 4.8s linear infinite;
}

.monthly-credit-plans__rail span {
  position: absolute;
  z-index: 3;
  top: 50%;
  display: grid;
  width: 2.05rem;
  height: 2.05rem;
  place-items: center;
  border: 1px solid rgb(230 163 37 / 0.68);
  border-radius: 50%;
  background: rgb(var(--color-papyrus));
  color: var(--membership-lightning);
  font-family: 'Inter', sans-serif;
  font-size: 0.95rem;
  box-shadow: 0 6px 18px rgb(var(--shadow-ink) / 0.12);
  transform: translate(-50%, -50%);
}

.monthly-credit-plans__rail span:first-child {
  left: 25%;
}

.monthly-credit-plans__rail span:last-child {
  left: 75%;
}

.monthly-credit-card__reveal {
  position: relative;
  z-index: 1;
  min-width: 0;
}

.monthly-credit-card {
  --plan-color: rgb(var(--color-muted));
  --plan-rgb: var(--color-muted);
  --rest-lift: 0px;
  --hover-lift: -0.65rem;
  --spot-x: 50%;
  --spot-y: 0%;
  --tilt-x: 0deg;
  --tilt-y: 0deg;
  position: relative;
  display: flex;
  min-height: 31rem;
  height: 100%;
  overflow: hidden;
  flex-direction: column;
  padding: 1.75rem;
  border: 1px solid rgb(var(--plan-rgb) / 0.7);
  border-radius: 12px;
  background:
    radial-gradient(circle at var(--spot-x) var(--spot-y), rgb(var(--plan-rgb) / 0.11), transparent 38%),
    linear-gradient(180deg, rgb(var(--color-vellum) / 0.5), transparent 44%),
    var(--membership-surface);
  box-shadow:
    0 18px 50px rgb(var(--shadow-ink) / 0.09),
    inset 0 0 0 1px rgb(var(--color-vellum) / 0.56);
  transform: perspective(1100px) translate3d(0, var(--rest-lift), 0) rotateX(var(--tilt-x)) rotateY(var(--tilt-y));
  transform-style: preserve-3d;
  transition:
    transform 360ms cubic-bezier(0.2, 0.8, 0.2, 1),
    border-color 260ms ease,
    box-shadow 360ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.monthly-credit-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 2px;
  background: linear-gradient(90deg, var(--plan-color), rgb(var(--plan-rgb) / 0.28) 74%, transparent);
}

.monthly-credit-card:hover,
.monthly-credit-card:focus-within {
  z-index: 4;
  border-color: var(--plan-color);
  box-shadow:
    0 28px 74px rgb(var(--plan-rgb) / 0.18),
    0 8px 22px rgb(var(--shadow-ink) / 0.1),
    inset 0 0 0 1px rgb(var(--color-vellum) / 0.72);
  transform: perspective(1100px) translate3d(0, var(--hover-lift), 0) rotateX(var(--tilt-x)) rotateY(var(--tilt-y));
}

.monthly-credit-card--plus {
  --plan-color: rgb(var(--color-laurel));
  --plan-rgb: var(--color-laurel);
}

.monthly-credit-card--pro {
  --plan-color: rgb(var(--color-info));
  --plan-rgb: var(--color-info);
  --rest-lift: -0.35rem;
  --hover-lift: -0.85rem;
  box-shadow:
    0 24px 60px rgb(var(--color-info) / 0.14),
    inset 0 0 0 1px rgb(var(--color-vellum) / 0.72);
}

.monthly-credit-card--max {
  --plan-color: rgb(var(--color-terracotta));
  --plan-rgb: var(--color-terracotta);
}

.monthly-credit-card__corner {
  position: absolute;
  top: 0;
  right: 0;
  display: grid;
  width: 5.1rem;
  height: 2.45rem;
  place-items: center end;
  padding-right: 1.05rem;
  clip-path: polygon(35% 0, 100% 0, 100% 100%, 54% 100%);
  background: linear-gradient(90deg, rgb(var(--plan-rgb) / 0.82), var(--plan-color));
  color: #fffdf7;
  font-family: 'Inter', sans-serif;
  font-size: 1rem;
  line-height: 1;
}

.monthly-credit-card__top {
  padding-right: 3rem;
}

.monthly-credit-card__identity {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.monthly-credit-card h3 {
  margin: 0;
  color: var(--plan-color);
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(1.65rem, 2.6vw, 2.25rem);
  font-weight: 700;
  line-height: 1.08;
  letter-spacing: -0.025em;
}

.monthly-credit-card__rarity {
  display: inline-flex;
  margin-bottom: 0.55rem;
  color: var(--plan-color);
  font-size: 0.7rem;
  font-weight: 850;
  letter-spacing: 0.14em;
  line-height: 1;
  text-transform: uppercase;
}

.monthly-credit-card__top > p {
  min-height: 3.9rem;
  margin: 0.95rem 0 0;
  color: var(--membership-muted);
  font-size: 0.9rem;
  line-height: 1.7;
}

.monthly-credit-card__stock {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 0.42rem;
  color: var(--plan-color);
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.monthly-credit-card__stock > span {
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 0 3px rgb(var(--plan-rgb) / 0.12);
}

.monthly-credit-card__price {
  display: flex;
  align-items: baseline;
  gap: 0.45rem;
  margin-top: 1.45rem;
  padding-top: 1.3rem;
  border-top: 1px dashed rgb(var(--plan-rgb) / 0.23);
}

.monthly-credit-card__price strong {
  color: var(--membership-ink);
  font-family: 'Inter', 'EB Garamond', serif;
  font-size: clamp(2.35rem, 4vw, 3.2rem);
  font-weight: 750;
  font-feature-settings: 'lnum' 1, 'tnum' 1;
  letter-spacing: -0.055em;
  line-height: 0.95;
}

.monthly-credit-card__price span {
  color: var(--membership-muted);
  font-size: 0.9rem;
  white-space: nowrap;
}

.monthly-credit-card__direct-price {
  margin: 0.65rem 0 0;
  color: var(--membership-muted);
  font-size: 0.86rem;
  line-height: 1.5;
}

.monthly-credit-card__direct-help {
  margin: 0.2rem 0 0;
  color: var(--membership-muted);
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  font-size: 0.7rem;
  line-height: 1.5;
  opacity: 0.72;
}

.monthly-credit-card__credits {
  display: grid;
  grid-template-columns: 1fr;
  row-gap: 0.35rem;
  margin-top: auto;
  padding: 1.25rem 0 1.1rem;
  border-top: 1px solid rgb(var(--plan-rgb) / 0.18);
  border-bottom: 1px solid rgb(var(--plan-rgb) / 0.18);
}

.monthly-credit-card__credits-label {
  color: var(--membership-muted);
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.1em;
}

.monthly-credit-card__credits strong {
  color: var(--membership-ink);
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  font-size: clamp(2rem, 3vw, 2.6rem);
  font-weight: 780;
  font-feature-settings: 'lnum' 1, 'tnum' 1;
  letter-spacing: -0.045em;
  line-height: 1;
}

.monthly-credit-card__credits strong::first-letter {
  color: var(--membership-lightning);
}

.monthly-credit-card__credits small {
  color: var(--membership-muted);
  font-size: 0.8rem;
  line-height: 1.55;
}

.monthly-credit-card__legend {
  margin: 0.85rem 0 0;
  border-left: 2px solid rgb(var(--plan-rgb) / 0.5);
  padding-left: 0.75rem;
  color: var(--membership-muted);
  font-size: 0.82rem;
  line-height: 1.65;
}

.monthly-credit-card__action {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.65rem;
  min-height: 2.75rem;
  margin-top: 0.85rem;
  border-radius: 7px;
  color: var(--plan-color);
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  font-size: 0.84rem;
  font-weight: 750;
  line-height: 1;
  text-decoration: none;
  transition:
    gap 240ms ease,
    background-color 240ms ease,
    color 240ms ease;
}

.monthly-credit-card__action:hover {
  gap: 0.95rem;
  background: rgb(var(--plan-rgb) / 0.09);
}

.monthly-credit-card__action:focus-visible {
  outline: 2px solid var(--plan-color);
  outline-offset: 3px;
}

.monthly-credit-card__action-arrow {
  font-size: 1.05rem;
  transition: transform 240ms ease;
}

.monthly-credit-card__action:hover .monthly-credit-card__action-arrow {
  transform: translateX(0.2rem);
}

.monthly-credit-card__action--disabled {
  color: var(--membership-muted);
  cursor: not-allowed;
  opacity: 0.72;
}

.monthly-credit-plans__note {
  margin: 1.4rem 0 0;
  color: var(--membership-muted);
  font-size: 0.84rem;
  line-height: 1.8;
}

.monthly-credit-plans__note a {
  color: var(--membership-ink);
  text-decoration-color: rgb(var(--color-terracotta) / 0.5);
  text-underline-offset: 0.22em;
}

.monthly-credit-plans--app {
  --membership-surface: var(--admin-control, rgb(var(--color-vellum) / 0.95));
  --membership-surface-soft: var(--admin-surface-soft, rgb(var(--color-stone) / 0.72));
  --membership-border: var(--admin-border, rgb(var(--color-ink) / 0.14));
  --membership-border-strong: var(--admin-border-strong, rgb(var(--color-ink) / 0.32));
  --membership-ink: var(--admin-ink, rgb(var(--color-ink)));
  --membership-muted: var(--admin-muted, rgb(var(--color-muted)));
  --membership-accent: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
}

.dark .monthly-credit-plans--app {
  --membership-surface: rgb(var(--color-indigo-900) / 0.96);
  --membership-surface-soft: rgb(var(--color-indigo-800) / 0.82);
  --membership-border: rgb(var(--color-vellum) / 0.1);
  --membership-border-strong: rgb(var(--color-vellum) / 0.18);
  --membership-ink: rgb(var(--color-slate-50));
  --membership-muted: rgb(var(--color-gray-400));
  --membership-accent: rgb(var(--color-amber-500));
}

@keyframes monthly-power-flow {
  to {
    background-position: -200% 0;
  }
}

@media (max-width: 1100px) {
  .monthly-credit-plans__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .monthly-credit-plans__rail {
    display: none;
  }

  .monthly-credit-card--pro {
    --rest-lift: 0px;
  }
}

@media (max-width: 720px) {
  .monthly-credit-plans__heading {
    display: grid;
    align-items: start;
    margin-bottom: 1.35rem;
  }

  .monthly-credit-plans__heading h2 {
    font-size: clamp(2rem, 12vw, 2.75rem);
  }

  .monthly-credit-plans__grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }

  .monthly-credit-card {
    min-height: auto;
    padding: 1.35rem;
  }

  .monthly-credit-card__top {
    padding-right: 2.4rem;
  }

  .monthly-credit-card__top > p {
    min-height: auto;
  }

  .monthly-credit-card__credits {
    margin-top: 1.35rem;
  }
}

@media (hover: none) {
  .monthly-credit-card,
  .monthly-credit-card:hover,
  .monthly-credit-card:focus-within {
    --rest-lift: 0px;
    --hover-lift: 0px;
    transform: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .monthly-credit-plans__rail {
    animation: none;
  }

  .monthly-credit-card,
  .monthly-credit-card:hover,
  .monthly-credit-card:focus-within,
  .monthly-credit-card__action,
  .monthly-credit-card__action-arrow {
    transition-duration: 0.01ms;
    transform: none;
  }
}
</style>
