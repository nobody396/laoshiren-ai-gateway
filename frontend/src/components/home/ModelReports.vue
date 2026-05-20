<template>
  <section id="model-reports" class="model-reports">
    <div class="model-reports__container mirror-reveal">
      <div class="greco-divider" aria-hidden="true"></div>
      <div class="reports-heading">
        <p class="section-eyebrow">III · 检测卷宗</p>
        <h2 class="section-title">Probatio · 模型检测</h2>
        <p class="section-lede">由第三方检测工具核验模型身份、协议一致性与响应结构。</p>
      </div>

      <div class="report-board">
        <div class="report-board__summary">
          <div>
            <span>已公开报告</span>
            <strong>{{ reports.length }}</strong>
          </div>
          <div>
            <span>平均匹配度</span>
            <strong>{{ averageScore }}%</strong>
          </div>
          <div>
            <span>检测方</span>
            <strong>hvoy.ai</strong>
          </div>
        </div>

        <div class="report-grid">
          <article
            v-for="report in reports"
            :key="report.sourceUrl"
            class="report-card"
          >
            <div class="report-card__topline">
              <span class="report-badge">{{ report.provider }}</span>
              <span class="score-pill">{{ report.score }}% {{ report.verdict }}</span>
            </div>

            <div class="report-card__body">
              <h3>{{ report.title }}</h3>
              <p>{{ report.endpoint }}</p>
            </div>

            <dl class="report-facts">
              <div>
                <dt>模型 ID</dt>
                <dd>{{ report.modelId }}</dd>
              </div>
              <div>
                <dt>检测项</dt>
                <dd>{{ report.passedChecks }}/{{ report.totalChecks }} Pass</dd>
              </div>
              <div>
                <dt>检测时间</dt>
                <dd>{{ report.testedAt }}</dd>
              </div>
            </dl>

            <div class="metric-strip">
              <div>
                <span>延迟</span>
                <strong>{{ report.latency }}</strong>
              </div>
              <div>
                <span>Tokens/秒</span>
                <strong>{{ report.tps }}</strong>
              </div>
              <div>
                <span>输入</span>
                <strong>{{ report.inputTokens }}</strong>
              </div>
              <div>
                <span>输出</span>
                <strong>{{ report.outputTokens }}</strong>
              </div>
            </div>

            <a
              :href="report.sourceUrl"
              class="report-link"
              target="_blank"
              rel="noopener noreferrer"
            >
              查看完整报告 ↗
            </a>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'

/**
 * 模型检测报告区块
 */
type ModelReport = {
  provider: string
  title: string
  sourceUrl: string
  endpoint: string
  modelId: string
  score: number
  verdict: string
  testedAt: string
  passedChecks: number
  totalChecks: number
  latency: string
  tps: string
  inputTokens: string
  outputTokens: string
}

const props = defineProps<{
  reports: ModelReport[]
}>()

const averageScore = computed(() => {
  if (!props.reports.length) return 0
  const total = props.reports.reduce((sum, report) => sum + report.score, 0)
  return Math.round(total / props.reports.length)
})
</script>

<style scoped>
.model-reports {
  padding: 6rem 0;
  background: #f8f3e7;
  scroll-margin-top: 88px;
}

.model-reports__container {
  width: min(100% - 4rem, 1320px);
  margin: 0 auto;
}

.greco-divider {
  width: 15rem;
  height: 1.125rem;
  margin: 0 auto 3.75rem;
  opacity: 0.42;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 40 18'><path d='M0,9 L8,9 L8,3 L16,3 L16,15 L24,15 L24,3 L32,3 L32,9 L40,9' stroke='%233F5A3A' fill='none' stroke-width='1.5'/></svg>");
  background-repeat: repeat-x;
  background-size: 40px 18px;
}

.reports-heading {
  text-align: center;
}

.section-eyebrow {
  margin: 0 0 0.875rem;
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.section-title {
  margin: 0;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2.4rem, 4.2vw, 3.5rem);
  font-weight: 500;
  line-height: 1.1;
}

.section-lede {
  margin: 1rem auto 4rem;
  color: #1f1a12;
  font-size: 1.18rem;
  font-style: italic;
  line-height: 1.65;
}

.report-board {
  padding: 0.5rem;
  background: #faf6ec;
  border: 1px solid rgba(63, 90, 58, 0.18);
  box-shadow: 0 1px 0 rgba(63, 90, 58, 0.08), 0 0 0 1px rgba(63, 90, 58, 0.04);
}

.report-board__summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1px;
  background: rgba(63, 90, 58, 0.14);
  border-bottom: 1px solid rgba(63, 90, 58, 0.14);
}

.report-board__summary div {
  min-height: 7.5rem;
  padding: 1.35rem 1.5rem;
  background: #faf6ec;
}

.report-board__summary span,
.report-facts dt,
.metric-strip span {
  display: block;
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.64rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.report-board__summary strong {
  display: block;
  margin-top: 0.6rem;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2rem, 4vw, 3.1rem);
  font-weight: 600;
  line-height: 1;
}

.report-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1px;
  background: rgba(63, 90, 58, 0.14);
}

.report-card {
  display: grid;
  grid-template-rows: auto auto 1fr auto auto;
  min-height: 31rem;
  padding: 1.35rem;
  background: #faf6ec;
}

.report-card__topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.report-badge,
.score-pill {
  display: inline-flex;
  align-items: center;
  min-height: 1.6rem;
  padding: 0.25rem 0.58rem;
  font-family: 'Inter', sans-serif;
  font-size: 0.62rem;
  font-weight: 800;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.report-badge {
  background: rgba(63, 90, 58, 0.12);
  color: #26361f;
}

.score-pill {
  background: #1f1a12;
  color: #f8f3e7;
  white-space: nowrap;
}

.report-card__body {
  margin-top: 2.25rem;
  min-height: 8.5rem;
}

.report-card__body h3 {
  margin: 0;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(1.45rem, 2.1vw, 2rem);
  font-weight: 500;
  line-height: 1.12;
}

.report-card__body p {
  margin: 1rem 0 0;
  color: #6f634f;
  font-family: 'Inter', sans-serif;
  font-size: 0.8rem;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.report-facts {
  display: grid;
  gap: 0.9rem;
  margin: 0;
  padding: 1.25rem 0;
  border-top: 1px solid rgba(63, 90, 58, 0.14);
  border-bottom: 1px solid rgba(63, 90, 58, 0.14);
}

.report-facts div {
  display: grid;
  gap: 0.35rem;
}

.report-facts dd {
  margin: 0;
  color: #13100b;
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-size: 1rem;
  font-weight: 700;
  overflow-wrap: anywhere;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  margin-top: 1.25rem;
  background: rgba(63, 90, 58, 0.12);
}

.metric-strip div {
  min-height: 4.35rem;
  padding: 0.8rem;
  background: #efe6cf;
}

.metric-strip strong {
  display: block;
  margin-top: 0.35rem;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 1.12rem;
  font-weight: 600;
  line-height: 1.1;
}

.report-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.75rem;
  margin-top: 1.25rem;
  padding: 0.75rem 1rem;
  border: 1.5px solid #1f1a12;
  color: #1f1a12;
  font-family: 'Inter', sans-serif;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-decoration: none;
  text-transform: uppercase;
  white-space: nowrap;
  transition: background 0.2s ease, color 0.2s ease;
}

.report-link:hover {
  background: #1f1a12;
  color: #f8f3e7;
}

@media (max-width: 900px) {
  .model-reports__container {
    width: min(100% - 2rem, 1320px);
  }

  .report-board__summary,
  .report-grid {
    grid-template-columns: 1fr;
  }

  .report-card {
    min-height: auto;
  }

  .report-card__body {
    min-height: 0;
  }

  .metric-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 520px) {
  .section-title {
    font-size: 2rem;
  }

  .section-lede {
    margin-bottom: 2.75rem;
    font-size: 1rem;
  }

  .metric-strip {
    grid-template-columns: 1fr;
  }
}
</style>
