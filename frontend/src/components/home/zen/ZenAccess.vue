<template>
  <section id="access" class="zen-access">
    <div class="zen-access__title zen-reveal">
      <small class="zen-access__eyebrow">YOUR WORKFLOW</small>
      <h2>{{ ui.title }}</h2>
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
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 接入网格:真实客户端工具(5×2)
 * - 图标统一来自 public/brand/client-tools(来源与授权见该目录 README)
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const tools = [
  { name: 'Claude Code', asset: '/brand/client-tools/claude.svg' },
  { name: 'Codex', asset: '/brand/client-tools/codex-light.png' },
  { name: 'Kimi Code', asset: '/brand/client-tools/kimi.svg' },
  { name: 'Z Code', asset: '/brand/client-tools/glm.svg' },
  { name: 'Gemini CLI', asset: '/brand/client-tools/gemini.svg' },
  { name: 'AntiGravity', asset: '/brand/client-tools/antigravity.png' },
  { name: 'Grok Build', asset: '/brand/client-tools/grok.svg' },
  { name: 'Hermes Agent', asset: '/brand/client-tools/hermes.png' },
  { name: 'opencode', asset: '/brand/client-tools/opencode.svg' },
  { name: 'OpenClaw', asset: '/brand/client-tools/openclaw.svg' }
]

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    title: 'Plug into your apps and workflows'
  }
  : {
    title: '接入你的应用与工作流'
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

.zen-access__grid {
  width: min(100% - 4rem, 1200px);
  margin: auto;
  display: grid;
  grid-template-columns: repeat(5, 1fr);
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

.zen-access__cell:nth-child(5n) {
  border-right: 0;
}

.zen-access__cell:nth-child(n + 6) {
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

  /* 用同等特异性的 nth-child 覆盖桌面 5 列的边框规则,避免中间行丢线 */
  .zen-access__cell:nth-child(n) {
    border-right: 1px solid rgb(var(--zen-line));
    border-bottom: 1px solid rgb(var(--zen-line));
  }

  .zen-access__cell:nth-child(2n) {
    border-right: 0;
  }

  .zen-access__cell:nth-child(n + 9) {
    border-bottom: 0;
  }

  .zen-access__cell span {
    font-size: 13px;
  }
}
</style>
