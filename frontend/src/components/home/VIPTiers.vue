<template>
  <!-- 数据指标区块 - VIP 卡片暂时隐藏，只保留数据统计 -->
  <section class="vip-tiers">
    <div class="vip-tiers__container mirror-reveal">
      <!-- 数据指标展示 -->
      <div class="trusted-brands">
        <h3 class="trusted-brands__title">
          已被开发者广泛使用 <span class="star-icon">✦</span>
        </h3>
        <div class="stats-list">
          <div class="stat-item">
            <span class="stat-num">2000+</span>
            <span class="stat-label">服务开发者</span>
          </div>
          <div class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-num">100万+</span>
            <span class="stat-label">累计调用次数</span>
          </div>
          <div class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-num">16.3s</span>
            <span class="stat-label">平均耗时</span>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 数据指标区块 - VIP 卡片暂时隐藏，只保留数据统计展示
 */

// VIP 卡片相关 props 暂时注释，后续恢复时取消注释
// defineProps<{
//   tiers: Array<{
//     name: string
//     discount: string
//     threshold: string
//     features: string[]
//   }>
// }>()

// /** 各等级标签渐变色止点 */
// const gradientStops = [
//   ['#434343', '#000000'],
//   ['#4facfe', '#0072ff'],
//   ['#a18cd1', '#7c3aed'],
//   ['#f093fb', '#d8328c'],
//   ['#f09819', '#ed1c24']
// ]

// /** 折扣数字颜色 */
// const numColors = [
//   '#1a1a2e',
//   '#2563eb',
//   '#7c3aed',
//   '#db2777',
//   '#dc2626'
// ]
</script>

<style scoped>
/* ── 整体区块 ── */
.vip-tiers {
  position: relative;
  padding: 100px 0 120px;
  background-image: url('/bg.png');
  background-size: cover;
  background-position: center;
  overflow: hidden;
}

.vip-tiers__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
}

/* ── 标题 ── */
.vip-tiers__title {
  text-align: center;
  font-size: 48px;
  font-weight: 800;
  color: #1a1a2e;
  margin-bottom: 12px;
  letter-spacing: -0.02em;
}

.star-icon {
  font-size: 28px;
  vertical-align: middle;
  margin-left: 4px;
}

.vip-tiers__subtitle {
  text-align: center;
  font-size: 16px;
  color: rgba(26, 26, 46, 0.6);
  margin-bottom: 100px;
}

/* ── 卡片网格 ── */
.vip-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
  margin-bottom: 140px;
}

.vip-card-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
}

/* ── 高保真 VIP 标签 ── */
.vip-tag-wrapper {
  position: relative;
  z-index: 10;
  width: 160px;
  height: 70px;
  /* 确保标签悬浮在卡片之上 */
  transform: translateY(35px);
}

/* 倾斜的背景 */
.vip-tag-bg {
  position: absolute;
  inset: 0;
  transform: skewX(-14deg);
  border-radius: 12px;
  /* 增加高光质感 (Bevel/Shine effect) */
  box-shadow: 
    inset 1px 1px 3px rgba(255, 255, 255, 0.5),
    0 10px 20px rgba(0, 0, 0, 0.15);
  transition: transform 0.3s ease, box-shadow 0.3s ease;
}

/* 端正的内容容器 */
.vip-tag-content {
  position: relative;
  z-index: 11;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #ffffff;
  /* 稍微向右偏移补偿倾斜带来的视觉错觉 */
  padding-left: 10px;
}

.vip-tag__name {
  font-size: 22px;
  font-weight: 900;
  line-height: 1;
  margin-bottom: 4px;
  letter-spacing: 0.05em;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
}

.vip-tag__threshold {
  font-size: 11px;
  font-weight: 500;
  opacity: 0.95;
  line-height: 1;
}

/* ── 卡片主体 ── */
.vip-card-body {
  width: 100%;
  background: #ffffff;
  border-radius: 16px;
  padding: 80px 20px 40px;
  box-shadow: 0 4px 30px rgba(0, 0, 0, 0.03);
  text-align: center;
  border: 1px solid rgba(0, 0, 0, 0.04);
  min-height: 330px;
  display: flex;
  flex-direction: column;
  transition: transform 0.3s ease, box-shadow 0.3s ease;
}

.vip-card-wrap:hover .vip-card-body {
  transform: translateY(-8px);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.08);
}

.vip-card-wrap:hover .vip-tag-bg {
  transform: translateY(-8px) skewX(-14deg);
  box-shadow: 
    inset 1px 1px 3px rgba(255, 255, 255, 0.5),
    0 15px 30px rgba(0, 0, 0, 0.2);
}

.vip-card-wrap:hover .vip-tag-content {
  transform: translateY(-8px);
}

.vip-discount-wrap {
  margin-bottom: 30px;
}

.vip-discount {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  gap: 2px;
}

.discount-num {
  font-size: 64px;
  font-weight: 800;
  line-height: 0.85;
  letter-spacing: -0.04em;
}

.discount-info {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  margin-top: 4px;
}

.multiplier-text {
  font-size: 14px;
  font-weight: 800;
  opacity: 1;
  line-height: 1;
}

/* ── 权益列表 ── */
.vip-features {
  display: flex;
  flex-direction: column;
  gap: 12px;
  text-align: left;
  margin-top: auto;
  padding: 0 10px;
}

.feature-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #4b5563;
  font-weight: 600;
}

.check-icon {
  flex-shrink: 0;
  color: #10b981;
}

/* ── 数据指标展示 ── */
.trusted-brands {
  text-align: center;
}

.trusted-brands__title {
  font-size: 32px;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 80px;
}

.stats-list {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 60px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.stat-num {
  font-size: 48px;
  font-weight: 800;
  color: #1a1a2e;
  letter-spacing: -0.02em;
  line-height: 1;
}

.stat-label {
  font-size: 16px;
  font-weight: 500;
  color: rgba(26, 26, 46, 0.5);
}

.stat-divider {
  width: 1px;
  height: 60px;
  background: rgba(26, 26, 46, 0.1);
}

/* ── 响应式 ── */
@media (max-width: 1200px) {
  .vip-grid {
    grid-template-columns: repeat(3, 1fr);
    gap: 24px;
  }
}

@media (max-width: 800px) {
  .vip-grid {
    grid-template-columns: 1fr;
    max-width: 300px;
    margin: 0 auto 80px;
  }
  .vip-tiers__title {
    font-size: 32px;
  }
  .stats-list {
    flex-wrap: wrap;
    gap: 40px;
  }
}
</style>
