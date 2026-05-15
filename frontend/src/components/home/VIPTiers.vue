<template>
  <section class="vip-tiers">
    <div class="vip-tiers__container mirror-reveal">
      <div class="seal-row">
        <img src="/laoshirenai-icon.jpg" alt="老实人 AI" />
      </div>
      <h3 class="trusted-title">已被开发者广泛使用</h3>
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
          <span class="stat-num">6.3s</span>
          <span class="stat-label">平均耗时</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 数据指标区块
 */
</script>

<style scoped>
.vip-tiers {
  padding: 5rem 0 6rem;
  background:
    linear-gradient(rgba(31, 26, 18, 0.02), rgba(31, 26, 18, 0.02)),
    #faf6ec;
}

.vip-tiers__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
  text-align: center;
}

.seal-row {
  display: flex;
  justify-content: center;
  margin-bottom: 1.5rem;
}

.seal-row img {
  width: 4rem;
  height: 4rem;
  object-fit: cover;
  border-radius: 50%;
  box-shadow: inset 0 0 0 2px rgba(250, 246, 236, 0.42), 0 0 0 1px #7a2d17;
}

.trusted-title {
  margin: 0 0 3.5rem;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2rem, 3.4vw, 2.75rem);
  font-weight: 500;
  line-height: 1.2;
}

.stats-list {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 3.75rem;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.stat-num {
  color: #9a3b1f;
  font-family: 'Cinzel', serif;
  font-size: clamp(2.5rem, 4vw, 3.2rem);
  font-weight: 600;
  line-height: 1;
}

.stat-label {
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.stat-divider {
  width: 1px;
  height: 3.75rem;
  background: rgba(63, 90, 58, 0.18);
}

@media (max-width: 760px) {
  .vip-tiers__container {
    width: min(100% - 2rem, 1200px);
  }

  .stats-list {
    flex-direction: column;
    gap: 2rem;
  }

  .stat-divider {
    width: 6rem;
    height: 1px;
  }
}
</style>
