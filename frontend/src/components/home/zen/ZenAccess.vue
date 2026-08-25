<template>
  <section id="access" class="zen-access">
    <div class="zen-access__title zen-reveal">
      <small class="zen-access__eyebrow">YOUR WORKFLOW</small>
      <h2>{{ ui.title }}</h2>
      <p>{{ ui.subtitle }}</p>
    </div>
    <div class="zen-access__grid zen-reveal">
      <div
        v-for="item in tools"
        :key="item.name"
        class="zen-access__cell"
      >
        <img :src="item.asset" :alt="item.name" loading="lazy" />
        <span>{{ item.name }}</span>
      </div>
      <div class="zen-access__cell zen-access__cell--more">
        <Icon name="grid" size="lg" class="zen-access__cell-more-icon" />
        <span>{{ ui.moreLabel }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 接入网格:真实客户端工具(4×2)
 * - 图标统一来自 public/brand/client-tools(来源与授权见该目录 README)
 * - 最后一格是"任意 OpenAI 兼容客户端"通用格
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const tools = [
  { name: 'Claude Code', asset: '/brand/client-tools/claude.svg' },
  { name: 'Codex', asset: '/brand/client-tools/codex-light.png' },
  { name: 'Gemini CLI', asset: '/brand/client-tools/gemini.svg' },
  { name: 'Grok Build', asset: '/brand/client-tools/grok.svg' },
  { name: 'Hermes Agent', asset: '/brand/client-tools/hermes.png' },
  { name: 'opencode', asset: '/brand/client-tools/opencode.svg' },
  { name: 'OpenClaw', asset: '/brand/client-tools/openclaw.svg' }
]

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    title: 'Plug into your apps and workflows',
    subtitle: 'One key, compatible with both OpenAI and Anthropic protocols — import with CC Switch in one click, or change a single Base URL manually.',
    moreLabel: 'Any OpenAI-compatible client'
  }
  : {
    title: '接入你的应用与工作流',
    subtitle: '一个 Key，兼容 OpenAI 与 Anthropic 协议 — CC Switch 一键导入，或改一行 Base URL 手动接入。',
    moreLabel: '任意 OpenAI 兼容客户端'
  }))
</script>

<style scoped>
.zen-access {
  padding: 0 0 110px;
}

.zen-access__title {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto 44px;
}

.zen-access__eyebrow {
  font-family: 'Cinzel', 'Times New Roman', serif;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.22em;
  color: rgb(var(--color-terracotta));
}

.zen-access__title h2 {
  margin: 14px 0 0;
  font-size: clamp(32px, 3.2vw, 46px);
  font-weight: 800;
  letter-spacing: -0.02em;
  color: rgb(var(--zen-ink));
}

.zen-access__title p {
  margin: 14px 0 0;
  font-size: 15px;
  color: rgb(var(--zen-muted));
}

.zen-access__grid {
  width: min(100% - 4rem, 1200px);
  margin: auto;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  border: 1px solid rgb(var(--zen-line));
  border-radius: 16px;
  overflow: hidden;
  background: rgb(var(--zen-bg));
}

.zen-access__cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 32px 12px;
  border-right: 1px solid rgb(var(--zen-line));
  border-bottom: 1px solid rgb(var(--zen-line));
  font-size: 15px;
  font-weight: 650;
  white-space: nowrap;
  color: rgb(var(--zen-text-strong));
  transition: background 0.2s ease;
}

.zen-access__cell:nth-child(4n) {
  border-right: 0;
}

.zen-access__cell:nth-child(n + 5) {
  border-bottom: 0;
}

.zen-access__cell:hover {
  background: rgb(var(--zen-bg-soft));
}

.zen-access__cell img {
  width: 32px;
  height: 32px;
  object-fit: contain;
}

.zen-access__cell--more {
  color: rgb(var(--zen-muted));
  white-space: normal;
  text-align: center;
  line-height: 1.45;
}

.zen-access__cell-more-icon {
  color: rgb(var(--zen-muted-light));
  flex: none;
}

@media (max-width: 900px) {
  .zen-access {
    padding-bottom: 80px;
  }

  .zen-access__title,
  .zen-access__grid {
    width: min(100% - 2.5rem, 1200px);
  }

  .zen-access__grid {
    grid-template-columns: 1fr 1fr;
  }

  /* 用同等特异性的 nth-child 覆盖桌面 4 列的边框规则,避免中间行丢线 */
  .zen-access__cell:nth-child(n) {
    border-right: 1px solid rgb(var(--zen-line));
    border-bottom: 1px solid rgb(var(--zen-line));
  }

  .zen-access__cell:nth-child(2n) {
    border-right: 0;
  }

  .zen-access__cell:nth-child(n + 7) {
    border-bottom: 0;
  }

  .zen-access__cell span {
    font-size: 13px;
  }
}
</style>
