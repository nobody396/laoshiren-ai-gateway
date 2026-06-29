<template>
  <section class="hero-section">
    <div class="hero-section__container">
      <div class="hero-section__content">
        <div class="hero-section__eyebrow mirror-reveal">{{ ui.eyebrow }}</div>
        <h1 class="hero-section__title">
          <span class="mirror-reveal" style="transition-delay: 0.08s">{{ ui.titleBrand }}</span>
          <em class="mirror-reveal" style="transition-delay: 0.16s">{{ ui.titleProduct }}</em>
        </h1>
        <p class="hero-section__desc mirror-reveal" style="transition-delay: 0.2s">
          <span v-for="line in ui.descriptionLines" :key="line">{{ line }}</span>
        </p>

        <div class="hero-section__actions mirror-reveal" style="transition-delay: 0.32s">
          <a
            :href="isAuthenticated ? dashboardPath : '/login'"
            class="hero-section__btn hero-section__btn--primary"
          >
            {{ isAuthenticated ? ui.dashboardCta : ui.beginCta }}
          </a>
          <a
            href="#model-pricing"
            class="hero-section__btn hero-section__btn--outline"
          >
            {{ ui.pricingCta }}
          </a>
        </div>
      </div>

      <aside class="hero-section__quote mirror-reveal" style="transition-delay: 0.26s">
        <p class="hero-section__quote-text">{{ ui.quoteText }}</p>
        <p class="hero-section__quote-author">{{ ui.quoteAuthor }}</p>
        <p class="hero-section__quote-greek">{{ ui.quoteGreek }}</p>
      </aside>
    </div>
  </section>
</template>

<script setup lang="ts">
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
      'Think with Claude Code, Codex, ChatGPT, and Gemini through one quiet gateway.'
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
      '通过老实人AI与 Claude Code、Codex、ChatGPT、Gemini 一起思考。'
    ],
    dashboardCta: '进入控制台',
    beginCta: '开始对谈',
    pricingCta: '查看模型价格',
    quoteText: '“未经审视的代码，不值得发布。”',
    quoteAuthor: '化用苏格拉底《申辩篇》38a',
    quoteGreek: 'ὁ δὲ ἀνεξέταστος βίος οὐ βιωτὸς ἀνθρώπῳ'
  }))

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
    radial-gradient(circle at 50% 0%, rgba(242, 233, 210, 0.9) 0%, rgba(242, 233, 210, 0) 62%),
    linear-gradient(180deg, #f8f3e7 0%, #faf6ec 100%);
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
  color: #8a7d63;
}

.hero-section__title {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.22em;
  max-width: 760px;
  margin: 0 0 1.75rem;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(4rem, 7vw, 5.9rem);
  font-weight: 600;
  line-height: 1.04;
  letter-spacing: 0;
}

.hero-section__title em {
  color: #9a3b1f;
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-style: italic;
  font-weight: 500;
}

.hero-section__desc {
  max-width: 44rem;
  margin: 0 0 2.5rem;
  color: #1f1a12;
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
  background: #9a3b1f;
  color: #faf6ec;
  border: 1.5px solid #9a3b1f;
}

.hero-section__btn--primary:hover {
  background: #7a2d17;
  transform: translateY(-1px);
}

.hero-section__btn--outline {
  background: transparent;
  color: #1f1a12;
  border: 1.5px solid #1f1a12;
}

.hero-section__btn--outline:hover {
  background: #1f1a12;
  color: #f8f3e7;
  transform: translateY(-1px);
}

.hero-section__quote {
  border-left: 2px solid #3f5a3a;
  padding: 1.65rem 1.9rem;
  background: rgba(239, 230, 207, 0.42);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.42);
}

.hero-section__quote-text {
  margin: 0 0 0.9rem;
  color: #1f1a12;
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-size: 1.45rem;
  font-style: italic;
  font-weight: 600;
  line-height: 1.45;
}

.hero-section__quote-author {
  margin: 0;
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.hero-section__quote-greek {
  margin: 1.25rem 0 0;
  color: #3f5a3a;
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
