<template>
  <!-- 页脚 -->
  <footer id="about" class="home-footer">
    <div class="home-footer__container">
      <!-- 链接分组 -->
      <div class="home-footer__grid">
        <div
          v-for="section in sections"
          :key="section.title"
          class="home-footer__column"
        >
          <h3>{{ section.title }}</h3>
          <ul>
            <li v-for="link in section.links" :key="link.label">
              <a
                :href="link.href"
                :target="link.external ? '_blank' : undefined"
                :rel="link.external ? 'noopener noreferrer' : undefined"
              >
                {{ link.label }}
              </a>
            </li>
          </ul>
        </div>
      </div>

      <!-- 底部版权 -->
      <div class="home-footer__bottom">
        <p>© 2026 DragonCode. 保留所有权利.</p>
        <p class="home-footer__company">DragonCode Team</p>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
/**
 * 页脚组件
 * - 多列链接布局
 * - 版权信息
 */
defineProps<{
  sections: Array<{
    title: string
    links: Array<{
      label: string
      href: string
      external?: boolean
    }>
  }>
}>()
</script>

<style scoped>
.home-footer {
  padding: 4rem 0 2rem;
  background: #f8fafc;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
}

.home-footer__container {
  width: min(100% - 2rem, 1280px);
  margin: 0 auto;
}

/* 链接分组网格 */
.home-footer__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 2rem;
  margin-bottom: 3rem;
}

.home-footer__column h3 {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 1rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.home-footer__column ul {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.home-footer__column a {
  font-size: 0.8125rem;
  color: #6b7280;
  text-decoration: none;
  transition: color 0.2s ease;
}

.home-footer__column a:hover {
  color: #4f8cff;
}

/* 底部版权 */
.home-footer__bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-top: 1.5rem;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
}

.home-footer__bottom p {
  font-size: 0.75rem;
  color: #9ca3af;
}

.home-footer__company {
  font-size: 0.75rem;
  color: #9ca3af;
}

/* 响应式 */
@media (max-width: 640px) {
  .home-footer__bottom {
    flex-direction: column;
    gap: 0.5rem;
    text-align: center;
  }
}
</style>
