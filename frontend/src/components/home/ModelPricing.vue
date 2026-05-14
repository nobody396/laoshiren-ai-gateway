<template>
  <!-- 模型定价区块 - 1:1 还原设计稿 -->
  <section id="model-pricing" class="model-pricing">
    <div class="model-pricing__container mirror-reveal">
      <div class="table-wrapper">
        <table class="pricing-table">
          <thead>
            <tr>
              <th>模型名</th>
              <th>分组</th>
              <th>倍率</th>
              <th>输入（¥）</th>
              <th>输出（¥）</th>
              <th>官方（¥）</th>
              <th>折扣</th>
              <th>OpenClaw</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, index) in rows" :key="index">
              <td class="cell-name">{{ row.name }}</td>
              <td>{{ row.group }}</td>
              <td>{{ row.multiplier }}x</td>
              <td>{{ row.input }}</td>
              <td>{{ row.output }}</td>
              <td>{{ row.official }}</td>
              <td>
                <span class="discount-badge">{{ row.discount }}</span>
              </td>
              <td>{{ row.openclaw ? '是' : '否' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 中间 CTA 按钮 -->
      <div class="model-pricing__cta">
        <router-link
          :to="isAuthenticated ? '/dashboard' : '/login'"
          class="cta-btn"
        >
          免费注册 立即体验
        </router-link>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 模型定价表格 - 1:1 还原设计稿
 */
defineProps<{
  rows: Array<{
    name: string
    group: string
    multiplier: string
    input: string
    output: string
    official: string
    discount: string
    openclaw: boolean
  }>
  isAuthenticated: boolean
}>()
</script>

<style scoped>
.model-pricing {
  padding: 0.5rem 0 4rem;
  background-color: #f8f9fa;
}

.model-pricing__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
}

/* 表格包装 */
.table-wrapper {
  background: #f1f3f5;
  border-radius: 0.5rem;
  overflow: hidden;
  margin-bottom: 5rem;
}

.pricing-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}

.pricing-table th {
  padding: 1.25rem 1.5rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #1a1a2e;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

.pricing-table td {
  padding: 1.25rem 1.5rem;
  font-size: 0.875rem;
  color: rgba(26, 26, 46, 0.8);
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

.pricing-table tr:last-child td {
  border-bottom: none;
}

.pricing-table tr:hover {
  background: rgba(255, 255, 255, 0.4);
}

.cell-name {
  font-weight: 500;
  color: #1a1a2e;
}

/* 折扣标签 */
.discount-badge {
  display: inline-flex;
  padding: 0.25rem 0.625rem;
  background: #7dd3fc;
  color: #0369a1;
  border-radius: 0.375rem;
  font-size: 0.75rem;
  font-weight: 700;
}

/* CTA 按钮区 */
.model-pricing__cta {
  display: flex;
  justify-content: center;
  margin-bottom: 4rem;
}

.cta-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 4rem;
  padding: 0 4rem;
  background: #000;
  color: #fff;
  border-radius: 999px;
  font-size: 1.125rem;
  font-weight: 700;
  text-decoration: none;
  transition: all 0.2s ease;
}

.cta-btn:hover {
  transform: scale(1.02);
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.2);
}

/* 底部过渡标题 */
.model-pricing__footer-title {
  text-align: center;
  font-size: 3.5rem;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 1.5rem;
}

.model-pricing__star {
  display: inline-block;
  margin-left: 0.5rem;
  font-size: 2rem;
  color: #1a1a2e;
}

.model-pricing__footer-subtitle {
  text-align: center;
  font-size: 1.125rem;
  color: rgba(26, 26, 46, 0.6);
}

@media (max-width: 768px) {
  .table-wrapper {
    overflow-x: auto;
  }
  .pricing-table {
    min-width: 800px;
  }
  .model-pricing__footer-title {
    font-size: 2.5rem;
  }
}
</style>
