<template>
  <!-- IDE 协同区块 - 1:1 还原设计稿 -->
  <section class="ide-integration">
    <div class="ide-integration__container">
      <div class="ide-integration__grid">
        <!-- 左侧：IDE 截图 -->
        <div class="ide-integration__visual mirror-reveal" ref="visualRef" @mousemove="handleMouseMove" @mouseleave="handleMouseLeave">
          <div class="ide-integration__glow"></div>
          <img src="/ide.png" alt="Claude Code in IDE" class="ide-screenshot" ref="screenshotRef" />
        </div>

        <!-- 右侧：文案内容 -->
        <div class="ide-integration__content mirror-reveal" style="transition-delay: 0.15s">
          <h2 class="ide-integration__title">
            与您的IDE 协同工作 <span class="ide-integration__star">✦</span>
          </h2>

          <p class="ide-integration__desc">
            通过 DragonCode 配置，一键在 VS Code 和 JetBrains 中启用 Claude 编码助手。它能理解你的整个项目结构与代码模式，直接在编辑器中给出贴合上下文的建议，无需复制粘贴，专注构建。
          </p>

          <div class="ide-integration__platforms">
            <div class="platform-item">
              <img src="/homepage-reference/vscode.svg" alt="VS Code" class="platform-logo" />
              <span>VS Code</span>
            </div>
            <div class="platform-item">
              <img src="/homepage-reference/jetbrains.svg" alt="JetBrains" class="platform-logo" />
              <span>JetBrains</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部过渡标题 -->
      <h3 class="ide-integration__footer-title mirror-reveal" style="transition-delay: 0.3s">
        模型定价 <span class="ide-integration__star">✦</span>
      </h3>
      <p class="ide-integration__footer-subtitle mirror-reveal" style="transition-delay: 0.4s">
        我们的价格以人民币（¥）计价•官方原价以美元（$）标注•汇率按1:7折算（单位：百万tokens）
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'

/**
 * IDE 协同区块 - 1:1 还原设计稿
 */
const visualRef = ref<HTMLElement | null>(null)
const screenshotRef = ref<HTMLElement | null>(null)

const handleMouseMove = (e: MouseEvent) => {
  if (!visualRef.value || !screenshotRef.value) return
  
  const rect = visualRef.value.getBoundingClientRect()
  const x = e.clientX - rect.left
  const y = e.clientY - rect.top
  
  const centerX = rect.width / 2
  const centerY = rect.height / 2
  
  const rotateX = (centerY - y) / 20
  const rotateY = (x - centerX) / 20
  
  screenshotRef.value.style.transform = `perspective(1000px) rotateX(${rotateX}deg) rotateY(${rotateY}deg) scale(1.05)`
}

const handleMouseLeave = () => {
  if (!screenshotRef.value) return
  screenshotRef.value.style.transform = `perspective(1000px) rotateX(0deg) rotateY(0deg) scale(1)`
}
</script>

<style scoped>
.ide-integration {
  padding: 8rem 0;
  background-color: #f8f9fa;
  overflow: hidden;
}

.ide-integration__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
}

.ide-integration__grid {
  display: grid;
  grid-template-columns: 1fr 0.85fr;
  align-items: center;
  gap: 6rem;
  margin-bottom: 10rem;
}

/* 左侧截图 */
.ide-integration__visual {
  position: relative;
  perspective: 1000px;
  display: flex;
  justify-content: center;
  align-items: center;
}

.ide-integration__glow {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 80%;
  height: 80%;
  background: radial-gradient(circle, rgba(79, 140, 255, 0.15) 0%, transparent 70%);
  filter: blur(40px);
  z-index: -1;
  animation: pulseGlow 8s ease-in-out infinite;
}

@keyframes pulseGlow {
  0%, 100% { opacity: 0.5; transform: translate(-50%, -50%) scale(1); }
  50% { opacity: 0.8; transform: translate(-50%, -50%) scale(1.1); }
}

.ide-screenshot {
  width: 100%;
  border-radius: 1rem;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.1);
  animation: floatImage 8s ease-in-out infinite;
  transition: transform 0.2s ease-out, box-shadow 0.4s ease;
  will-change: transform;
}

.ide-screenshot:hover {
  box-shadow: 0 40px 80px rgba(0, 0, 0, 0.2);
  animation-play-state: paused;
}

@keyframes floatImage {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-12px); }
}

/* 右侧文案 */
.ide-integration__content {
  text-align: left;
}

.ide-integration__title {
  font-size: 3rem;
  font-weight: 800;
  color: #1a1a2e;
  margin-bottom: 2rem;
  line-height: 1.2;
  white-space: nowrap;
}

.ide-integration__star {
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

.ide-integration__desc {
  font-size: 1.125rem;
  line-height: 1.8;
  color: rgba(26, 26, 46, 0.7);
  margin-bottom: 3rem;
}

/* 平台图标 */
.ide-integration__platforms {
  display: flex;
  gap: 3rem;
}

.platform-item {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem 1.25rem;
  border-radius: 0.75rem;
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  cursor: default;
  background: transparent;
}

.platform-item:hover {
  background: #ffffff;
  transform: translateY(-4px);
  box-shadow: 0 12px 24px rgba(0, 0, 0, 0.04);
}

.platform-item:hover .platform-logo {
  animation: logoShine 1s ease-in-out infinite;
}

@keyframes logoShine {
  0% { filter: brightness(1) drop-shadow(0 0 0 rgba(79, 140, 255, 0)); }
  50% { filter: brightness(1.2) drop-shadow(0 0 8px rgba(79, 140, 255, 0.3)); }
  100% { filter: brightness(1) drop-shadow(0 0 0 rgba(79, 140, 255, 0)); }
}

.platform-logo {
  width: 2.5rem;
  height: 2.5rem;
  object-fit: cover;
  object-position: left center;
}

.platform-item span {
  font-size: 1.125rem;
  font-weight: 500;
  color: #1a1a2e;
}

/* 底部标题 */
.ide-integration__footer-title {
  text-align: center;
  font-size: 3.5rem;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 1.5rem;
}

.ide-integration__footer-subtitle {
  text-align: center;
  font-size: 1.125rem;
  color: rgba(26, 26, 46, 0.6);
}

@media (max-width: 1024px) {
  .ide-integration__grid {
    grid-template-columns: 1fr;
    gap: 4rem;
  }
  .ide-integration__title {
    font-size: 2.5rem;
  }
}
</style>
