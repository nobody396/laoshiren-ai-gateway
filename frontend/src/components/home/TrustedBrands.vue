<template>
  <!-- 企业信赖品牌板块 - 蓝色渐变背景 -->
  <section class="trusted-brands">
    <!-- 蓝色渐变背景 -->
    <div class="trusted-brands__bg">
      <img src="/bg.png" alt="" class="trusted-brands__bg-img" />
    </div>

    <div class="trusted-brands__container mirror-reveal">
      <h2 class="trusted-brands__title">
        受企业工程师信赖
        <span class="trusted-brands__star">✦</span>
      </h2>
      <div class="trusted-brands__grid">
        <div
          v-for="logo in logos"
          :key="logo.name"
          class="trusted-brands__item"
        >
          <img :src="logo.logo" :alt="logo.name" />
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 企业信赖品牌展示板块
 * - 蓝色渐变波浪背景
 * - 品牌 logo 横排展示
 */
defineProps<{
  logos: Array<{ name: string; logo: string }>
}>()
</script>

<style scoped>
.trusted-brands {
  position: relative;
  padding: 8rem 0 6rem;
  overflow: hidden;
}

/* 蓝色渐变背景图 */
.trusted-brands__bg {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 60%;
  overflow: hidden;
  z-index: 0;
}

.trusted-brands__bg-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0.4;
}

.trusted-brands__container {
  position: relative;
  z-index: 1;
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
  text-align: center;
}

/* 标题 - 大号加粗 */
.trusted-brands__title {
  font-size: 2.5rem;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 3rem;
}

.trusted-brands__star {
  display: inline-block;
  margin-left: 0.5rem;
  font-size: 1.5rem;
  color: #1a1a2e;
}

/* logo 网格 */
.trusted-brands__grid {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5rem;
  flex-wrap: wrap;
}

.trusted-brands__item {
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0.5;
  transition: opacity 0.3s ease;
  filter: grayscale(100%);
}

.trusted-brands__item:hover {
  opacity: 0.8;
  filter: grayscale(0%);
}

.trusted-brands__item img {
  height: 4rem;
  width: auto;
  object-fit: contain;
}
</style>
