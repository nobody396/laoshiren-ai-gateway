<template>
  <section class="hero-section">
    <div class="hero-section__container">
      <div class="hero-section__content">
        <div class="hero-section__eyebrow mirror-reveal">{{ ui.eyebrow }}</div>
        <h1 class="hero-section__title">
          <span class="mirror-reveal" style="transition-delay: calc(var(--reveal-stagger) * 1)">{{ ui.titleBrand }}</span>
          <em class="mirror-reveal" style="transition-delay: calc(var(--reveal-stagger) * 2)">{{ ui.titleProduct }}</em>
        </h1>
        <p class="hero-section__desc mirror-reveal" style="transition-delay: calc(var(--reveal-stagger) * 3)">
          <span v-for="line in ui.descriptionLines" :key="line">{{ line }}</span>
        </p>

        <div class="hero-section__actions mirror-reveal" style="transition-delay: calc(var(--reveal-stagger) * 4)">
          <a
            :href="isAuthenticated ? dashboardPath : '/login'"
            class="hero-section__btn hero-section__btn--primary"
          >
            {{ isAuthenticated ? ui.dashboardCta : ui.beginCta }}
          </a>
          <a
            href="/models"
            class="hero-section__btn hero-section__btn--outline"
          >
            {{ ui.pricingCta }}
          </a>
        </div>
      </div>

      <aside class="hero-section__quote mirror-reveal" style="transition-delay: calc(var(--reveal-stagger) * 3)">
        <p class="hero-section__quote-text">{{ ui.quoteText }}</p>
        <p class="hero-section__quote-author">{{ ui.quoteAuthor }}</p>
        <!-- 逐字符浮现：拆成 span，每个字符按 index 递增延迟。
             空格用 &nbsp; 保证 inline-block 下不塌陷。 -->
        <p class="hero-section__quote-greek" :aria-label="ui.quoteGreek">
          <span
            v-for="(ch, i) in greekChars"
            :key="`${i}-${ch}`"
            aria-hidden="true"
            :style="{ transitionDelay: `calc(var(--reveal-stagger) * 3 + ${i * 26}ms)` }"
          >{{ ch === ' ' ? NBSP : ch }}</span>
        </p>
      </aside>
    </div>
  </section>
</template>

<script setup lang="ts">
// 逐字拆开后每个字符都是 inline-block，普通空格会被折叠掉，词与词粘连。
// 用不断行空格顶住。写成常量而不是模板里的字面量：U+00A0 在源码里不可见，
// eslint 的 no-irregular-whitespace 会报错，而且下一个人看不出它是有意的。
const NBSP = '\u00A0'

/**
 * Hero 主视觉区域组件
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')

const ui = computed(() => (isEnglish.value
  ? {
    eyebrow: 'LaoshirenAI · AI Coding Gateway · MMXXVI',
    titleBrand: 'LaoshirenAI',
    titleProduct: 'Coding Gateway',
    descriptionLines: [
      'Code with clarity. Every line should stand up to scrutiny.',
      'Think with Claude Code, Codex, ChatGPT, and Grok through one quiet gateway.'
    ],
    dashboardCta: 'Dashboard',
    beginCta: 'Begin the dialogue',
    pricingCta: 'View pricing',
    quoteText: '"The unexamined code is not worth shipping."',
    quoteAuthor: 'After Socrates, Apology 38a',
    quoteGreek: 'ὁ δὲ ἀνεξέταστος βίος οὐ βιωτὸς ἀνθρώπῳ'
  }
  : {
    eyebrow: '老实人AI · AI 编码网关 · MMXXVI',
    titleBrand: '老实人AI',
    titleProduct: '编码网关',
    descriptionLines: [
      '让每一行代码都经得起审视。',
      '像柏拉图与门徒在柱廊下对谈一样，',
      '通过老实人AI与 Claude Code、Codex、ChatGPT、Grok 一起思考。'
    ],
    dashboardCta: '进入控制台',
    beginCta: '开始对谈',
    pricingCta: '查看模型价格',
    quoteText: '“未经审视的代码，不值得发布。”',
    quoteAuthor: '化用苏格拉底《申辩篇》38a',
    quoteGreek: 'ὁ δὲ ἀνεξέταστος βίος οὐ βιωτὸς ἀνθρώπῳ'
  }))

/** 希腊文引文拆成字符数组，供逐字浮现使用。
 *  用 Array.from 而不是 split('') —— 希腊文有组合字符，split 会拆坏。 */
const greekChars = computed(() => Array.from(ui.value.quoteGreek ?? ''))

defineProps<{
  isAuthenticated: boolean
  dashboardPath: string
}>()
</script>

<style scoped>
.hero-section {
  position: relative;
  min-height: 720px;
  padding: 11rem 0 5rem;
  overflow: hidden;
  background:
    radial-gradient(circle at 50% 0%, rgb(var(--color-parchment) / 0.9) 0%, rgb(var(--color-parchment) / 0) 62%),
    linear-gradient(180deg, rgb(var(--color-papyrus)) 0%, rgb(var(--color-marble)) 100%);
}

/* ── L1 环境层 ────────────────────────────────────────────────
 * 两层纯 CSS 的环境动效，零素材、零依赖。
 * 目的不是"加动画"，是让这张纸有生命感 —— 真实纸张在光下本来就有
 * 纤维颗粒和缓慢移动的光斑。这是从 OKX 的 film-grain 手法移植过来的，
 * 但语义换了：他们做胶片感，我们做纸感。
 * ─────────────────────────────────────────────────────────── */

/* 辉光漂移：朱红与月桂在纸下缓慢游走，像侧光照着纸面 */
.hero-section::before {
  content: '';
  position: absolute;
  inset: -20%;
  z-index: 0;
  pointer-events: none;
  background:
    radial-gradient(38% 44% at 22% 32%, rgb(var(--color-terracotta) / 0.055) 0%, rgb(var(--color-terracotta) / 0) 70%),
    radial-gradient(34% 40% at 78% 62%, rgb(var(--color-laurel) / 0.05) 0%, rgb(var(--color-laurel) / 0) 70%);
  animation: heroGlowDrift 26s var(--ease-standard, ease-in-out) infinite alternate;
}

/* 纸纤维颗粒：SVG feTurbulence 生成，不是图片。
 * steps(6) 而不是平滑过渡 —— 平滑会像"呼吸"，跳帧才像纸的颗粒感 */
.hero-section::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  opacity: 0.05;
  mix-blend-mode: multiply;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.82' numOctaves='3' stitchTiles='stitch'/%3E%3CfeColorMatrix type='saturate' values='0'/%3E%3C/filter%3E%3Crect width='180' height='180' filter='url(%23n)'/%3E%3C/svg%3E");
  animation: heroGrain 3s steps(6) infinite;
}

@keyframes heroGlowDrift {
  from { transform: translate3d(0, 0, 0) scale(1); }
  to   { transform: translate3d(2.5%, -2%, 0) scale(1.06); }
}

@keyframes heroGrain {
  0%   { transform: translate3d(0, 0, 0); }
  20%  { transform: translate3d(-3%, 2%, 0); }
  40%  { transform: translate3d(2%, -3%, 0); }
  60%  { transform: translate3d(-2%, -2%, 0); }
  80%  { transform: translate3d(3%, 1%, 0); }
  100% { transform: translate3d(0, 0, 0); }
}

@media (prefers-reduced-motion: reduce) {
  .hero-section::before,
  .hero-section::after {
    animation: none;
  }
}

.hero-section__container {
  position: relative;
  z-index: 2;
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
  display: grid;
  grid-template-columns: 1.12fr 0.88fr;
  gap: 5rem;
  align-items: center;
  min-width: 0;
}

.hero-section__content {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
  width: 100%;
}

.hero-section__eyebrow {
  margin-bottom: 1.5rem;
  font-family: 'Inter', sans-serif;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgb(var(--color-muted));
}

.hero-section__title {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.22em;
  max-width: 760px;
  margin: 0 0 1.75rem;
  color: rgb(var(--color-ink-deep));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(4rem, 7vw, 5.9rem);
  font-weight: 600;
  line-height: 1.04;
  letter-spacing: 0;
}

.hero-section__title em {
  color: rgb(var(--color-terracotta));
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-style: italic;
  font-weight: 500;
}

.hero-section__desc {
  max-width: 44rem;
  margin: 0 0 2.5rem;
  color: rgb(var(--color-ink));
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-size: 1.45rem;
  font-style: italic;
  line-height: 1.55;
}

.hero-section__desc span {
  display: block;
}

.hero-section__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
}

.hero-section__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 3.25rem;
  padding: 0.95rem 1.9rem;
  border-radius: 0;
  font-family: 'Inter', sans-serif;
  font-size: 0.8125rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  line-height: 1.2;
  text-transform: uppercase;
  text-decoration: none;
  transition: background 0.2s ease, color 0.2s ease, transform 0.2s ease;
  cursor: pointer;
}

.hero-section__btn--primary {
  background: rgb(var(--color-terracotta));
  color: rgb(var(--color-marble));
  border: 1.5px solid rgb(var(--color-terracotta));
}

.hero-section__btn--primary:hover {
  background: rgb(var(--color-terracotta-dark));
  transform: translateY(-1px);
}

.hero-section__btn--outline {
  background: transparent;
  color: rgb(var(--color-ink));
  border: 1.5px solid rgb(var(--color-ink));
}

.hero-section__btn--outline:hover {
  background: rgb(var(--color-ink));
  color: rgb(var(--color-papyrus));
  transform: translateY(-1px);
}

.hero-section__quote {
  border-left: 2px solid rgb(var(--color-laurel));
  padding: 1.65rem 1.9rem;
  background: rgb(var(--color-stone) / 0.42);
  box-shadow: inset 0 1px 0 rgb(var(--color-vellum) / 0.42);
}

.hero-section__quote-text {
  margin: 0 0 0.9rem;
  color: rgb(var(--color-ink));
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-size: 1.45rem;
  font-style: italic;
  font-weight: 600;
  line-height: 1.45;
}

.hero-section__quote-author {
  margin: 0;
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.hero-section__quote-greek {
  margin: 1.25rem 0 0;
  color: rgb(var(--color-laurel));
  font-family: 'EB Garamond', serif;
  font-size: 1rem;
  font-style: italic;
  text-align: right;
}

@media (max-width: 1024px) {
  .hero-section {
    min-height: auto;
    padding-top: 8.5rem;
  }

  .hero-section__container {
    grid-template-columns: 1fr;
    gap: 3rem;
  }
}

@media (max-width: 768px) {
  .hero-section {
    padding: 7.5rem 0 4rem;
  }

  .hero-section__container {
    width: min(100% - 2rem, 1200px);
  }

  .hero-section__title {
    display: block;
    max-width: 100%;
    font-size: clamp(2.4rem, 12vw, 3rem);
    overflow-wrap: anywhere;
  }

  .hero-section__title span,
  .hero-section__title em {
    display: block;
    max-width: 100%;
  }

  .hero-section__desc {
    max-width: 100%;
    white-space: normal;
    word-break: break-all;
    line-break: anywhere;
  }

  .hero-section__desc span {
    display: block;
  }

  .hero-section__desc,
  .hero-section__quote-text {
    font-size: 1.2rem;
    overflow-wrap: anywhere;
  }

  .hero-section__actions,
  .hero-section__btn {
    width: 100%;
  }

  .hero-section__btn {
    padding-inline: 1rem;
    letter-spacing: 0.08em;
  }

  .hero-section__quote {
    max-width: 100%;
    padding-inline: 1.2rem;
  }

  .hero-section__quote-greek {
    overflow-wrap: anywhere;
    text-align: left;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hero-section__btn {
    transition-duration: 1ms;
  }
}
</style>
