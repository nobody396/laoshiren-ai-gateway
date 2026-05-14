<template>
  <!-- 为什么选择区块 - 3列网格布局还原设计稿 -->
  <section class="why-choose">
    <div class="why-choose__container mirror-reveal">
      <div class="why-choose__grid">
        <!-- 第一行：卡片01 + 卡片02 + 渐变装饰图 -->
        <div class="why-choose__card">
          <div class="why-choose__number">01</div>
          <div class="why-choose__content">
            <h3 class="why-choose__title">{{ featureCards[0].title }}</h3>
            <p class="why-choose__desc">{{ featureCards[0].description }}</p>
          </div>
        </div>

        <div class="why-choose__card">
          <div class="why-choose__number">02</div>
          <div class="why-choose__content">
            <h3 class="why-choose__title">{{ featureCards[1].title }}</h3>
            <p class="why-choose__desc">{{ featureCards[1].description }}</p>
          </div>
        </div>

        <!-- 渐变流体装饰卡片 -->
        <div class="why-choose__card why-choose__card--art">
          <img src="/bg2.png" alt="" class="why-choose__art-img" />
        </div>

        <!-- 第二行：Logo装饰 + 卡片03 + 卡片04 -->
        <div class="why-choose__card why-choose__card--logo">
          <div class="why-choose__logo">
            <img src="/logo.png" alt="DragonCode" />
            <span>DragonCode</span>
          </div>
        </div>

        <div class="why-choose__card">
          <div class="why-choose__number">03</div>
          <div class="why-choose__content">
            <h3 class="why-choose__title">{{ featureCards[2].title }}</h3>
            <p class="why-choose__desc">{{ featureCards[2].description }}</p>
          </div>
        </div>

        <div class="why-choose__card">
          <div class="why-choose__number">04</div>
          <div class="why-choose__content">
            <h3 class="why-choose__title">{{ featureCards[3].title }}</h3>
            <p class="why-choose__desc">{{ featureCards[3].description }}</p>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'

/**
 * 为什么选择区块 - 3列网格还原设计稿
 * 第一行：01 + 02 + 渐变装饰
 * 第二行：Logo装饰 + 03 + 04
 */
defineProps<{
  featureCards: Array<{ title: string; description: string }>
}>()

const handleMouseMove = (e: MouseEvent) => {
  const cards = document.querySelectorAll('.why-choose__card') as NodeListOf<HTMLElement>
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
.why-choose {
  padding: 0 0 8rem;
  background-color: #f8f9fa;
}

.why-choose__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
}

/* 3列网格 */
.why-choose__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.25rem;
}

/* 通用卡片样式 */
.why-choose__card {
  position: relative;
  background: #eef0f3;
  border-radius: 1.5rem;
  padding: 2.5rem;
  min-height: 240px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

.why-choose__card:hover {
  transform: translateY(-6px);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.08);
}

.why-choose__card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: radial-gradient(
    600px circle at var(--mouse-x, -500px) var(--mouse-y, -500px),
    rgba(255, 255, 255, 0.6),
    transparent 40%
  );
  z-index: 0;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.4s ease;
}

.why-choose__card:hover::before {
  opacity: 1;
}

.why-choose__card > * {
  position: relative;
  z-index: 1;
}

/* 蓝色大数字 */
.why-choose__number {
  font-size: 3rem;
  font-weight: 800;
  color: #4f8cff;
  line-height: 1;
  margin-bottom: 1.5rem;
  transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1), color 0.4s ease;
}

.why-choose__card:hover .why-choose__number {
  transform: scale(1.05);
  color: #3b76eb;
}

.why-choose__content {
  position: relative;
  z-index: 2;
}

.why-choose__title {
  font-size: 1.25rem;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 0.75rem;
}

.why-choose__desc {
  font-size: 0.9375rem;
  line-height: 1.7;
  color: rgba(26, 26, 46, 0.55);
}

/* 渐变装饰卡片 - 右上角 */
.why-choose__card--art {
  padding: 0;
  background: #eef0f3;
}

.why-choose__art-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 1.5rem;
  animation: driftPan 20s ease-in-out infinite;
}

@keyframes driftPan {
  0% { transform: scale(1) translate(0, 0); }
  50% { transform: scale(1.1) translate(-2%, -2%); }
  100% { transform: scale(1) translate(0, 0); }
}

/* Logo 装饰卡片 - 左下角 */
.why-choose__card--logo {
  justify-content: flex-end;
  align-items: flex-start;
}

.why-choose__logo {
  display: flex;
  align-items: center;
  gap: 0.625rem;
}

.why-choose__logo img {
  width: 1.75rem;
  height: 1.75rem;
}

.why-choose__logo span {
  font-size: 1rem;
  font-weight: 600;
  color: #1a1a2e;
}

/* 响应式 */
@media (max-width: 968px) {
  .why-choose__grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .why-choose__card--art,
  .why-choose__card--logo {
    display: none;
  }
}

@media (max-width: 640px) {
  .why-choose__grid {
    grid-template-columns: 1fr;
  }
}
</style>
