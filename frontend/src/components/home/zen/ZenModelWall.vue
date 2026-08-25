<template>
  <section id="models" class="zen-model-wall">
    <p class="zen-model-wall__caption zen-reveal">{{ ui.caption }}</p>
    <div
      v-for="row in rows"
      :key="row.key"
      class="zen-model-wall__marquee"
    >
      <div
        class="zen-model-wall__track"
        :class="{ 'zen-model-wall__track--reverse': row.reverse }"
      >
        <div
          v-for="(item, i) in [...row.items, ...row.items, ...row.items, ...row.items]"
          :key="`${item.name}-${i}`"
          class="zen-model-wall__brand"
        >
          <img :src="item.asset" :alt="item.name" loading="lazy" />
          <span>{{ item.name }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 模型墙:国际行 + 国产行两条无缝 marquee(全彩图标)
 * - 图标统一来自 public/brand/client-tools(来源与授权见该目录 README)
 * - hover 暂停滚动,图标轻微放大;两端渐隐遮罩
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

interface BrandItem {
  name: string
  asset: string
}

const intlModels: BrandItem[] = [
  { name: 'Claude', asset: '/brand/client-tools/claude.svg' },
  { name: 'ChatGPT', asset: '/brand/client-tools/openai.svg' },
  { name: 'Grok', asset: '/brand/client-tools/grok.svg' },
  { name: 'Gemini', asset: '/brand/client-tools/gemini.svg' }
]

const cnModels: BrandItem[] = [
  { name: 'DeepSeek', asset: '/brand/client-tools/deepseek.svg' },
  { name: 'Kimi', asset: '/brand/client-tools/kimi.svg' },
  { name: '通义千问', asset: '/brand/client-tools/qwen.svg' },
  { name: '智谱 GLM', asset: '/brand/client-tools/glm.svg' },
  { name: '豆包', asset: '/brand/client-tools/doubao.svg' },
  { name: 'MiniMax', asset: '/brand/client-tools/minimax.svg' }
]

const rows = [
  { key: 'intl', items: intlModels, reverse: false },
  { key: 'cn', items: cnModels, reverse: true }
]

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? { caption: 'One key connects you to every mainstream model' }
  : { caption: '一个密钥，连接全球主流模型' }))
</script>

<style scoped>
.zen-model-wall {
  padding: 84px 0 88px;
  background: rgb(var(--zen-bg-soft));
  border-top: 1px solid rgb(var(--zen-line));
  border-bottom: 1px solid rgb(var(--zen-line));
  overflow: hidden;
}

.zen-model-wall__caption {
  margin: 0 0 44px;
  text-align: center;
  font-size: 15px;
  color: rgb(var(--zen-muted));
}

.zen-model-wall__marquee {
  overflow: hidden;
  /* 遮罩只需要不透明色,复用 zen-ink 令牌 */
  mask-image: linear-gradient(90deg, transparent, rgb(var(--zen-ink)) 12%, rgb(var(--zen-ink)) 88%, transparent);
  -webkit-mask-image: linear-gradient(90deg, transparent, rgb(var(--zen-ink)) 12%, rgb(var(--zen-ink)) 88%, transparent);
}

.zen-model-wall__marquee + .zen-model-wall__marquee {
  margin-top: 40px;
}

.zen-model-wall__track {
  display: flex;
  gap: 88px;
  width: max-content;
  animation: zenMarquee 32s linear infinite;
}

.zen-model-wall__track--reverse {
  animation-direction: reverse;
  animation-duration: 40s;
}

.zen-model-wall__marquee:hover .zen-model-wall__track {
  animation-play-state: paused;
}

@keyframes zenMarquee {
  to {
    transform: translateX(-25%);
  }
}

.zen-model-wall__brand {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 17px;
  font-weight: 650;
  white-space: nowrap;
  color: rgb(var(--zen-text-strong));
  transition: transform 0.25s ease, color 0.2s ease;
}

.zen-model-wall__brand img {
  width: 32px;
  height: 32px;
  object-fit: contain;
  transition: transform 0.25s ease;
}

.zen-model-wall__brand:hover {
  color: rgb(var(--zen-ink));
  transform: translateY(-2px);
}

.zen-model-wall__brand:hover img {
  transform: scale(1.12);
}

@media (prefers-reduced-motion: reduce) {
  .zen-model-wall__track {
    animation: none;
  }

  .zen-model-wall__brand,
  .zen-model-wall__brand img {
    transition-duration: 1ms;
  }
}
</style>
