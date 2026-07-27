<template>
  <footer id="about" class="home-footer">
    <div class="home-footer__container">
      <div class="greco-divider" aria-hidden="true"></div>
      <div class="home-footer__grid">
        <div class="home-footer__brand">
          <div class="brand-row">
            <img src="/laoshirenai-icon.jpg" :alt="ui.brandName" />
            <div>
              <span class="brand-name">{{ ui.brandName }}</span>
              <span class="brand-tag">{{ ui.brandTag }}</span>
            </div>
          </div>
          <p>{{ ui.description }}</p>
        </div>

        <div
          v-for="section in sections"
          :key="section.title"
          class="home-footer__column"
        >
          <h3>{{ section.title }}</h3>
          <ul>
            <li v-for="link in section.links" :key="link.label">
              <a
                :href="link.href"
                :target="link.external ? '_blank' : undefined"
                :rel="link.external ? 'noopener noreferrer' : undefined"
              >
                {{ link.label }}
              </a>
            </li>
          </ul>
        </div>
      </div>

      <div class="home-footer__bottom">
        <p>{{ ui.copyright }}</p>
        <p class="home-footer__company">γνῶθι σεαυτόν</p>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
/**
 * 页脚组件
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    brandName: 'LaoshirenAI',
    brandTag: 'A Quiet Place for Code',
    description: 'We build a quiet place where code can be examined and trusted.',
    copyright: '© MMXXVI · LaoshirenAI · Made with reverence for craft.'
  }
  : {
    brandName: '老实人AI',
    brandTag: '安静的代码工作台',
    description: '我们以工匠之心，造一处让代码可被审视、可被信赖的安静之地。',
    copyright: '© MMXXVI · 老实人AI · 以敬畏之心打磨工程。'
  }))

defineProps<{
  sections: Array<{
    title: string
    links: Array<{
      label: string
      href: string
      external?: boolean
    }>
  }>
}>()
</script>

<style scoped>
.home-footer {
  padding: 4rem 0 2rem;
  background: rgb(var(--color-ink-deep));
  color: rgb(var(--color-papyrus));
}

.home-footer__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
}

.greco-divider {
  width: 15rem;
  height: 1.125rem;
  margin: 0 auto 4rem;
  opacity: 0.55;
  filter: invert(0.9) hue-rotate(60deg);
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 40 18'><path d='M0,9 L8,9 L8,3 L16,3 L16,15 L24,15 L24,3 L32,3 L32,9 L40,9' stroke='%233F5A3A' fill='none' stroke-width='1.5'/></svg>");
  background-repeat: repeat-x;
  background-size: 40px 18px;
}

.home-footer__grid {
  display: grid;
  grid-template-columns: minmax(280px, 1.35fr) repeat(4, minmax(120px, 1fr));
  gap: 2.5rem;
  margin-bottom: 3.5rem;
}

.brand-row {
  display: flex;
  align-items: center;
  gap: 0.9rem;
}

.brand-row img {
  width: 2.5rem;
  height: 2.5rem;
  object-fit: cover;
  border-radius: 50%;
  box-shadow: inset 0 0 0 2px rgb(var(--color-marble) / 0.35), 0 0 0 1px rgb(var(--color-terracotta-dark));
}

.brand-name,
.brand-tag {
  display: block;
}

.brand-name {
  color: rgb(var(--color-papyrus));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 1.1rem;
  font-weight: 600;
  letter-spacing: 0.04em;
}

.brand-tag {
  margin-top: 0.125rem;
  color: rgb(var(--color-papyrus) / 0.55);
  font-family: 'Inter', sans-serif;
  font-size: 0.62rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.home-footer__brand p {
  max-width: 32ch;
  margin: 1rem 0 0;
  color: rgb(var(--color-papyrus) / 0.7);
  font-size: 1rem;
  font-style: italic;
  line-height: 1.55;
}

.home-footer__column h3 {
  margin: 0 0 1.125rem;
  color: rgb(var(--color-papyrus) / 0.58);
  font-family: 'Inter', sans-serif;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.home-footer__column ul {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
}

.home-footer__column a {
  color: rgb(var(--color-papyrus));
  font-size: 0.98rem;
  text-decoration: none;
  transition: color 0.2s ease;
}

.home-footer__column a:hover {
  color: rgb(var(--color-primary-300));
}

.home-footer__bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding-top: 1.5rem;
  border-top: 1px solid rgb(var(--color-papyrus) / 0.15);
}

.home-footer__bottom p {
  margin: 0;
  color: rgb(var(--color-papyrus) / 0.55);
  font-size: 0.9rem;
  font-style: italic;
}

@media (max-width: 1040px) {
  .home-footer__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .home-footer__brand {
    grid-column: 1 / -1;
  }
}

@media (max-width: 640px) {
  .home-footer__container {
    width: min(100% - 2rem, 1200px);
  }

  .home-footer__grid {
    grid-template-columns: 1fr;
  }

  .home-footer__bottom {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
