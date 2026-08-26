<template>
  <footer id="about" class="zen-footer">
    <div class="zen-footer__container">
      <div class="zen-footer__grid">
        <div
          v-for="section in sections"
          :key="section.title"
          class="zen-footer__column"
        >
          <h3>{{ section.title }}</h3>
          <ul>
            <li v-for="link in section.links" :key="link.label">
              <a
                :href="link.href"
                :target="link.external ? '_blank' : undefined"
                :rel="link.external ? 'noopener noreferrer' : undefined"
                @click="onLinkClick(link, $event)"
              >
                {{ link.label }}
              </a>
            </li>
          </ul>
        </div>
      </div>

      <div class="zen-footer__bottom">
        <span>{{ brandName }} · MMXXVI</span>
        <span class="zen-footer__greek">λόγος · ἀλήθεια · πίστις</span>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
/**
 * 落地页页脚(白底 Zen 风格)
 * 链接分组与旧页脚完全一致(产品/高意图指南/服务承诺/合规条款),
 * 底部一行:品牌 · MMXXVI + 希腊文点缀。
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { scrollToHash } from './scrollToHash'

const $router = useRouter()
const { locale } = useI18n()

const isEnglish = computed(() => locale.value === 'en')
const brandName = computed(() => (isEnglish.value ? 'LaoshirenAI' : '老实人AI'))

interface FooterLink {
  label: string
  href: string
  external?: boolean
}

defineProps<{
  sections: Array<{
    title: string
    links: FooterLink[]
  }>
}>()

function onLinkClick(link: FooterLink, event: MouseEvent): void {
  if (link.external) return
  event.preventDefault()
  if (link.href.startsWith('#')) {
    scrollToHash(link.href)
  } else {
    void $router.push(link.href)
  }
}
</script>

<style scoped>
.zen-footer {
  border-top: 1px solid rgb(var(--zen-line));
  background: rgb(var(--zen-bg));
}

.zen-footer__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
}

.zen-footer__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 2.5rem;
  padding: 56px 0 48px;
}

.zen-footer__column h3 {
  margin: 0 0 14px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: rgb(var(--zen-muted-light));
}

.zen-footer__column ul {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.zen-footer__column a {
  color: rgb(var(--zen-text));
  font-size: 14px;
  text-decoration: none;
  transition: color 0.2s ease;
}

.zen-footer__column a:hover {
  color: rgb(var(--zen-ink));
}

.zen-footer__bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  min-height: 88px;
  border-top: 1px solid rgb(var(--zen-line));
  font-size: 13px;
  color: rgb(var(--zen-muted-light));
}

.zen-footer__greek {
  font-family: 'EB Garamond', 'Times New Roman', serif;
  font-style: italic;
  font-size: 14px;
  letter-spacing: 0.06em;
}

@media (max-width: 900px) {
  .zen-footer__container {
    width: min(100% - 2.5rem, 1200px);
  }

  .zen-footer__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 2rem;
    padding: 44px 0 36px;
  }
}

@media (max-width: 560px) {
  .zen-footer__grid {
    grid-template-columns: 1fr;
  }

  .zen-footer__bottom {
    flex-direction: column;
    align-items: flex-start;
    justify-content: center;
    gap: 6px;
    padding: 20px 0;
  }
}
</style>
