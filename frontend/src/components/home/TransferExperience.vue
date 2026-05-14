<template>
  <!-- 中转体验区域 - 1:1 还原设计稿 -->
  <section class="transfer-experience">
    <div class="transfer-experience__container mirror-reveal">
      <!-- 顶部装饰标签 -->
      <div class="transfer-experience__eyebrow">
        <span class="eyebrow-badge">核心体验</span>
      </div>

      <!-- 主标题 -->
      <h2 class="transfer-experience__title">
        专为日常编码而设计的<span class="transfer-experience__accent">一站式体验</span>
        <span class="transfer-experience__star">✦</span>
      </h2>

      <!-- 描述 -->
      <p class="transfer-experience__desc">
        更低的价格，更简单的配置，让每位开发者都能用上顶级 AI 编码模型。
      </p>

      <!-- 核心大卡片 -->
      <div 
        class="transfer-experience__card" 
        ref="cardRef" 
        @mousemove="handleMouseMove"
      >
        <p class="transfer-experience__card-intro">
          一个 DragonCode 账号，同时使用 Claude Code 和 Codex。统一的 API 接入与网络环境，把时间还给你的代码，而不是配置面板。
        </p>

        <div class="transfer-experience__inner">
          <!-- 左侧：系统环境 -->
          <div class="transfer-experience__systems">
            <div class="systems-list">
              <div class="system-item">
                <img src="/homepage-reference/macos.svg" alt="macOS" class="system-icon" />
                <span>macOS</span>
              </div>
              <div class="system-item">
                <img src="/homepage-reference/windows.svg" alt="Windows" class="system-icon" />
                <span>Windows</span>
              </div>
              <div class="system-item">
                <img src="/homepage-reference/linux.svg" alt="Linux" class="system-icon" />
                <span>Linux</span>
              </div>
            </div>
            <p class="system-note">
              统一的域名与密钥格式，覆盖常见IDE 插件与CLI工具。一键配置环境。
            </p>
          </div>

          <!-- 分隔线 -->
          <div class="transfer-experience__divider"></div>

          <!-- 右侧：支持工具 -->
          <div class="transfer-experience__tools">
            <h4 class="tools-title">统一承载，同时支持</h4>
            <div class="tools-grid">
              <div v-for="tool in tools" :key="tool.name" class="tool-item">
                <img :src="tool.logo" :alt="tool.name" class="tool-icon" />
                <span>{{ tool.name }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部过渡标题 -->
      <h3 class="transfer-experience__footer-title">
        为什么选择 <span class="transfer-experience__accent-blue">DragonCode</span>
        <span class="transfer-experience__star">✦</span>
      </h3>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'

/**
 * 中转体验区块 - 1:1 还原设计稿
 */
const tools = [
  { name: 'Claude Code', logo: '/homepage-reference/claude.svg' },
  { name: 'Codex Code', logo: '/homepage-reference/openai.svg' },
  { name: 'Gemini CLI（即将推出）', logo: '/homepage-reference/gemini.svg' },
  { name: 'OpenClaw', logo: '/homepage-reference/openclaw-logo.svg' }
]

const cardRef = ref<HTMLElement | null>(null)

const handleMouseMove = (e: MouseEvent) => {
  if (!cardRef.value) return
  const rect = cardRef.value.getBoundingClientRect()
  const x = e.clientX - rect.left
  const y = e.clientY - rect.top
  cardRef.value.style.setProperty('--mouse-x', `${x}px`)
  cardRef.value.style.setProperty('--mouse-y', `${y}px`)
}
</script>

<style scoped>
.transfer-experience {
  padding: 8rem 0;
  background-color: #f8f9fa;
}

.transfer-experience__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
}

/* 顶部小标签 */
.transfer-experience__eyebrow {
  margin-bottom: 1.5rem;
  text-align: left;
}

.eyebrow-badge {
  display: inline-flex;
  padding: 0.25rem 1rem;
  background: transparent;
  border: 1px solid rgba(26, 26, 46, 0.15);
  border-radius: 999px;
  font-size: 0.875rem;
  font-weight: 500;
  color: #1a1a2e;
}
  
.transfer-experience__title {
  font-size: 3rem;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 1.5rem;
  text-align: left;
}

.transfer-experience__accent {
  color: #4f8cff;
}

.transfer-experience__star {
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

.transfer-experience__desc {
  font-size: 1.125rem;
  color: #666;
  margin-bottom: 4rem;
  text-align: left;
}

/* 核心大卡片 */
.transfer-experience__card {
  background: #ffffff;
  border-radius: 2rem;
  padding: 4rem;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.02);
  margin-bottom: 3rem;
  position: relative;
  overflow: hidden;
  transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

.transfer-experience__card:hover {
  transform: translateY(-4px);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.06);
}

.transfer-experience__card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: radial-gradient(
    800px circle at var(--mouse-x, -500px) var(--mouse-y, -500px),
    rgba(79, 140, 255, 0.05),
    transparent 40%
  );
  z-index: 0;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.4s ease;
}

.transfer-experience__card:hover::before {
  opacity: 1;
}

.transfer-experience__card > * {
  position: relative;
  z-index: 1;
}

.transfer-experience__card-intro {
  font-size: 1.125rem;
  line-height: 1.8;
  color: #1a1a2e;
  margin: 0 0 2rem 0;
  text-align: left;
}

.transfer-experience__inner {
  display: flex;
  align-items: stretch;
  background: #f1f3f5;
  border-radius: 1rem;
  padding: 2.5rem 3rem;
}

/* 左侧系统 */
.transfer-experience__systems {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  text-align: left;
}

.systems-list {
  display: flex;
  flex-wrap: wrap;
  gap: 2rem;
  margin-bottom: 1.5rem;
}

.system-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.system-icon {
  width: 1.5rem;
  height: 1.5rem;
}

.system-item span {
  font-size: 0.9375rem;
  font-weight: 600;
  color: #1a1a2e;
}

.system-note {
  width: 100%;
  margin-top: 1rem;
  font-size: 0.875rem;
  color: rgba(26, 26, 46, 0.6);
  line-height: 1.6;
}

/* 分隔线 */
.transfer-experience__divider {
  width: 1px;
  background: rgba(0, 0, 0, 0.1);
  margin: 0 3rem;
}

/* 右侧工具 */
.transfer-experience__tools {
  flex: 1.2;
  display: flex;
  flex-direction: column;
  justify-content: center;
  text-align: center;
}

.tools-title {
  font-size: 1.125rem;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 1.5rem;
  text-align: center;
}

.tools-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 2rem;
  justify-content: center;
}

.tool-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.tool-icon {
  width: 1.5rem;
  height: 1.5rem;
}

.tool-item span {
  font-size: 0.9375rem;
  font-weight: 500;
  color: #666;
}

/* 底部过渡标题 - 居中 */
.transfer-experience__footer-title {
  font-size: 3.5rem;
  font-weight: 700;
  color: #1a1a2e;
  text-align: center;
}

.transfer-experience__accent-blue {
  color: #4f8cff;
}

@media (max-width: 968px) {
  .transfer-experience__inner {
    flex-direction: column;
    padding: 2rem;
  }
  .transfer-experience__divider {
    width: 100%;
    height: 1px;
    margin: 2rem 0;
  }
  .transfer-experience__title {
    font-size: 2.25rem;
  }
  .transfer-experience__footer-title {
    font-size: 2.5rem;
  }
}
</style>
