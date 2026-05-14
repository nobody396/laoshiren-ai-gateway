<template>
  <!-- 模型矩阵区块 - 2x2 网格，沿用原设计稿风格 -->
  <section class="claude-combinations">
    <!-- 左侧蓝色装饰竖线 -->
    <div class="claude-combinations__accent-line"></div>
    <div class="claude-combinations__container mirror-reveal">
      <!-- 顶部装饰标签 -->
      <div class="claude-combinations__eyebrow">
        <span>模型矩阵</span>
      </div>

      <!-- 主标题 -->
      <h2 class="claude-combinations__title">
        三大 AI，各司其职
        <span class="claude-combinations__star">✦</span>
      </h2>

      <!-- 描述 -->
      <p class="claude-combinations__desc">
        Claude · ChatGPT · Gemini 强强联合，不同任务交给最擅长的模型。
      </p>

      <!-- 2x2 网格 -->
      <div class="claude-combinations__grid">
        <!-- 左上：第一个模型卡片 -->
        <div class="model-card">
          <div class="model-card__header">
            <span class="model-card__badge badge--blue">{{ models[0].eyebrow }}</span>
            <h3 class="model-card__name">{{ models[0].name }}</h3>
          </div>
          <p class="model-card__subtitle">{{ models[0].subtitle }}</p>
          <p class="model-card__description">{{ models[0].description }}</p>
          <p class="model-card__sub-models">{{ models[0].subModels }}</p>
        </div>

        <!-- 右上：更多模型即将接入 -->
        <div class="model-card model-card--coming">
          <div class="model-card__header">
            <span class="model-card__badge badge--gray">敬请期待</span>
            <h3 class="model-card__name">更多模型</h3>
          </div>
          <p class="model-card__subtitle">持续接入中</p>
          <p class="model-card__description">更多顶尖 AI 模型正在接入，覆盖更广泛的编码场景与工作流。</p>
          <div class="model-card__dots">
            <span></span><span></span><span></span>
          </div>
        </div>

        <!-- 左下：第二个模型卡片 -->
        <div class="model-card">
          <div class="model-card__header">
            <span class="model-card__badge badge--indigo">{{ models[1].eyebrow }}</span>
            <h3 class="model-card__name">{{ models[1].name }}</h3>
          </div>
          <p class="model-card__subtitle">{{ models[1].subtitle }}</p>
          <p class="model-card__description">{{ models[1].description }}</p>
          <p class="model-card__sub-models">{{ models[1].subModels }}</p>
        </div>

        <!-- 右下：第三个模型卡片 -->
        <div class="model-card">
          <div class="model-card__header">
            <span class="model-card__badge badge--purple">{{ models[2].eyebrow }}</span>
            <h3 class="model-card__name">{{ models[2].name }}</h3>
          </div>
          <p class="model-card__subtitle">{{ models[2].subtitle }}</p>
          <p class="model-card__description">{{ models[2].description }}</p>
          <p class="model-card__sub-models">{{ models[2].subModels }}</p>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'

/**
 * 模型矩阵区块 - 2x2 网格，沿用原设计稿风格
 * 左上 Claude / 右上 更多模型 / 左下 ChatGPT / 右下 Gemini
 */
defineProps<{
  models: Array<{
    eyebrow: string
    name: string
    subtitle: string
    description: string
    subModels: string
  }>
}>()

/* 鼠标跟随高光效果 */
const handleMouseMove = (e: MouseEvent) => {
  const cards = document.querySelectorAll('.claude-combinations .model-card') as NodeListOf<HTMLElement>
  cards.forEach(card => {
    const rect = card.getBoundingClientRect()
    const x = e.clientX - rect.left
    const y = e.clientY - rect.top
    card.style.setProperty('--mouse-x', `${x}px`)
    card.style.setProperty('--mouse-y', `${y}px`)
  })
}

onMounted(() => {
  window.addEventListener('mousemove', handleMouseMove, { passive: true })
})

onUnmounted(() => {
  window.removeEventListener('mousemove', handleMouseMove)
})
</script>

<style scoped>
/* 整个区块：全宽浅蓝底色，左侧蓝色竖线装饰 */
.claude-combinations {
  position: relative;
  padding: 6rem 0;
  background: #edf3ff;
}

/* 左侧蓝色装饰竖线 */
.claude-combinations__accent-line {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 5px;
  background: #3b82f6;
  border-radius: 0 4px 4px 0;
}

/* 内容容器 */
.claude-combinations__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
  text-align: center;
  padding: 2rem 0;
}

/* 顶部标签 */
.claude-combinations__eyebrow {
  margin-bottom: 1.5rem;
}

.claude-combinations__eyebrow span {
  display: inline-flex;
  padding: 0.35rem 1.25rem;
  border: 1px solid #1a1a2e;
  border-radius: 999px;
  font-size: 0.875rem;
  font-weight: 500;
  color: #1a1a2e;
}

.claude-combinations__title {
  font-size: 3rem;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 1.5rem;
}

.claude-combinations__star {
  display: inline-block;
  margin-left: 0.5rem;
  font-size: 2rem;
  color: #1a1a2e;
  animation: twinkleStar 4s ease-in-out infinite;
}

@keyframes twinkleStar {
  0%, 100% {
    transform: scale(0.9) rotate(0deg);
    opacity: 0.8;
  }
  50% {
    transform: scale(1.1) rotate(15deg);
    opacity: 1;
  }
}

.claude-combinations__desc {
  font-size: 1.125rem;
  color: rgba(26, 26, 46, 0.6);
  margin-bottom: 3rem;
}

/* 2列2行网格 */
.claude-combinations__grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1.25rem;
}

/* 卡片样式 - 白色底与浅蓝容器形成层次 */
.model-card {
  background: #ffffff;
  border-radius: 1.25rem;
  padding: 2rem 2.5rem;
  text-align: left;
  display: flex;
  flex-direction: column;
  border: 1px solid #e8ecf2;
  position: relative;
  overflow: hidden;
  transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.4s cubic-bezier(0.16, 1, 0.3, 1), border-color 0.4s ease;
}

.model-card:hover {
  transform: translateY(-6px);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.08);
  border-color: transparent;
}

/* Hover Spotlight */
.model-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: radial-gradient(
    600px circle at var(--mouse-x, -500px) var(--mouse-y, -500px),
    rgba(59, 130, 246, 0.06),
    transparent 40%
  );
  z-index: 0;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.4s ease;
}

.model-card:hover::before {
  opacity: 1;
}

.model-card > * {
  position: relative;
  z-index: 1;
}

/* "更多模型"卡片 - 虚线边框 + 浅灰底 */
.model-card--coming {
  background: #f9fafb;
  border: 1.5px dashed #d1d5db;
}

.model-card--coming:hover {
  border-color: #93c5fd;
}

/* 三个动画小圆点 */
.model-card__dots {
  display: flex;
  gap: 0.5rem;
  margin-top: auto;
  padding-top: 1rem;
}

.model-card__dots span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(26, 26, 46, 0.2);
  animation: dotPulse 1.8s ease-in-out infinite;
}

.model-card__dots span:nth-child(2) {
  animation-delay: 0.3s;
}

.model-card__dots span:nth-child(3) {
  animation-delay: 0.6s;
}

@keyframes dotPulse {
  0%, 100% { opacity: 0.3; transform: scale(1); }
  50% { opacity: 1; transform: scale(1.3); }
}

.badge--gray { background: #f3f4f6; color: #6b7280; }

/* badge 与模型名同行显示 */
.model-card__header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1.25rem;
}

.model-card__badge {
  display: inline-flex;
  padding: 0.25rem 0.75rem;
  border-radius: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
  transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

.model-card:hover .model-card__badge {
  transform: scale(1.05);
}

.badge--blue { background: #e0f2fe; color: #0ea5e9; }
.badge--indigo { background: #e0e7ff; color: #6366f1; }
.badge--purple { background: #f3e8ff; color: #a855f7; }

.model-card__name {
  font-size: 1.5rem;
  font-weight: 700;
  color: #1a1a2e;
}

.model-card__subtitle {
  font-size: 0.9375rem;
  font-weight: 500;
  color: rgba(26, 26, 46, 0.7);
  margin-bottom: 1rem;
}

.model-card__description {
  font-size: 0.9375rem;
  line-height: 1.7;
  color: rgba(26, 26, 46, 0.55);
}

/* 子模型列表 */
.model-card__sub-models {
  margin-top: auto;
  padding-top: 1rem;
  font-size: 0.8125rem;
  color: rgba(26, 26, 46, 0.35);
  letter-spacing: 0.02em;
}

@media (max-width: 768px) {
  .claude-combinations__grid {
    grid-template-columns: 1fr;
  }
}
</style>
