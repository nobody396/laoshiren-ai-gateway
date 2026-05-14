<template>
  <!-- Hero 主视觉区域 -->
  <section class="hero-section">
    <!-- 背景底图层：放大后缓慢漂移，产生流动感 -->
    <div class="hero-section__bg"></div>

    <div class="hero-section__container">
      <div class="hero-section__content">
        <!-- 标题行 - 错落入场 -->
        <h1 class="hero-section__title">
          <span class="hero-section__kicker mirror-reveal">开发者首选</span>
          <span class="hero-section__main mirror-reveal" style="transition-delay: 0.1s">AI 编码工作台</span>
        </h1>

        <!-- 描述 -->
        <p class="hero-section__desc mirror-reveal" style="transition-delay: 0.2s">
          一个账号、一条线路，统一调用 Claude Code、Codex 和 Gemini CLI。<br />
          更低价格、更稳链路、更透明计费。
        </p>

        <!-- CTA 按钮组 -->
        <div class="hero-section__actions mirror-reveal" style="transition-delay: 0.32s">
          <a
            :href="isAuthenticated ? dashboardPath : '/login'"
            class="hero-section__btn hero-section__btn--primary"
          >
            {{ isAuthenticated ? '进入控制台' : '立即体验' }}
          </a>
          <a
            href="#model-pricing"
            class="hero-section__btn hero-section__btn--outline"
          >
            查看定价
          </a>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * Hero 主视觉区域组件
 * - 居中排版
 * - bg.png 底图 + 缓慢漂移微动画
 */
defineProps<{
  isAuthenticated: boolean
  dashboardPath: string
}>()
</script>

<style scoped>
/* Hero 区域 - 全屏居中布局 */
.hero-section {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding-top: 80px;
  overflow: hidden;
  background: #fff;
}

/* 背景底图 — 放大 130% 留出漂移空间，沿对角线缓慢游动 */
.hero-section__bg {
  position: absolute;
  top: -15%;
  left: -15%;
  width: 130%;
  height: 130%;
  background-image: url('/bg.png');
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
  z-index: 0;
  animation: bgDrift 10s ease-in-out infinite alternate;
  will-change: transform;
}

.hero-section__container {
  position: relative;
  z-index: 1;
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
  text-align: center;
}

.hero-section__content {
  display: flex;
  flex-direction: column;
  align-items: center;
  max-width: 900px;
  margin: 0 auto;
}

/* 主标题样式 */
.hero-section__title {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1.5rem;
  margin-bottom: 2rem;
  line-height: 1.2;
}

/* "开发者首选"胶囊标签 */
.hero-section__kicker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.5rem 1.75rem;
  border: 1px solid #1a1a2e;
  border-radius: 999px;
  font-size: 2.25rem;
  font-weight: 500;
  color: #1a1a2e;
  letter-spacing: -0.02em;
}

/* "AI 编码工作台" */
.hero-section__main {
  font-size: 4.5rem;
  font-weight: 700;
  color: #1a1a2e;
  letter-spacing: -0.01em;
  background: linear-gradient(
    to right,
    #1a1a2e,
    #1a1a2e 45%,
    #4f8cff 50%,
    #1a1a2e 55%,
    #1a1a2e
  );
  background-size: 200% auto;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: textShimmer 6s linear infinite;
}

@keyframes textShimmer {
  to { background-position: 200% center; }
}

/* 描述文字 */
.hero-section__desc {
  font-size: 1.125rem;
  line-height: 1.8;
  color: rgba(26, 26, 46, 0.6);
  margin-bottom: 3.5rem;
}

/* 按钮组 */
.hero-section__actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1.25rem;
}

/* 通用按钮基础 */
.hero-section__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 3.5rem;
  padding: 0 2.5rem;
  border-radius: 999px;
  font-size: 1rem;
  font-weight: 600;
  text-decoration: none;
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  cursor: pointer;
}

/* 主要按钮 - 黑色填充 */
.hero-section__btn--primary {
  background: #000;
  color: #fff;
}

.hero-section__btn--primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 12px 24px rgba(0, 0, 0, 0.15);
}

/* 次要按钮 - 描边样式 */
.hero-section__btn--outline {
  background: transparent;
  color: #1a1a2e;
  border: 1px solid rgba(26, 26, 46, 0.3);
}

.hero-section__btn--outline:hover {
  border-color: #1a1a2e;
  background: rgba(0, 0, 0, 0.02);
  transform: translateY(-2px);
}

/* 响应式适配 */
@media (max-width: 1024px) {
  .hero-section__main {
    font-size: 3.5rem;
  }
  .hero-section__kicker {
    font-size: 1.75rem;
    padding: 0.4rem 1.25rem;
  }
}

@media (max-width: 768px) {
  .hero-section__title {
    flex-direction: column;
    gap: 1rem;
  }
  .hero-section__main {
    font-size: 2.5rem;
  }
  .hero-section__desc br {
    display: none;
  }
}

/* 无障碍：减弱动画偏好 */
@media (prefers-reduced-motion: reduce) {
  .hero-section__bg {
    animation: none !important;
  }
}
</style>

<!-- 非 scoped：keyframes 避免 Vue scoped hash 问题 -->
<style>
/* 背景漂移：沿对角线平移 + 缩放，四段路径形成环形游动 */
@keyframes bgDrift {
  0% {
    transform: translate(0, 0) scale(1);
  }
  100% {
    transform: translate(8%, 5%) scale(1.07);
  }
}
</style>
