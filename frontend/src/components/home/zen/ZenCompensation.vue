<template>
  <section id="compensation" class="zen-comp">
    <div class="zen-comp__grid">
      <div class="zen-comp__copy zen-reveal">
        <Icon name="shield" size="xl" class="zen-comp__shield" />
        <small class="zen-comp__eyebrow">AUTOMATIC COMPENSATION</small>
        <h2 class="zen-comp__title">{{ ui.title }}</h2>
        <p class="zen-comp__desc">{{ ui.description }}</p>
      </div>
      <div class="zen-comp__steps">
        <article
          v-for="(step, i) in ui.steps"
          :key="step.num"
          class="zen-reveal"
          :style="{ '--zen-reveal-i': i }"
        >
          <span>{{ step.num }}</span>
          <h3>{{ step.title }}</h3>
          <p>{{ step.description }}</p>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 自动赔付章节:盾牌图标 + Cinzel eyebrow + 三步流程(识别/计算/到账)
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    title: 'When something breaks, you never have to ask.',
    description: 'The gateway observes the final outcome of every call. Once an upstream failure is confirmed, compensation is generated automatically and credited straight to your balance — no tickets, no applications.',
    steps: [
      { num: '01', title: 'Detect', description: 'The gateway confirms the call\u2019s final failure, filtering out client-side interruptions.' },
      { num: '02', title: 'Calculate', description: 'Compensation is computed by public rules — fully traceable and reconcilable.' },
      { num: '03', title: 'Credit', description: 'Compensation lands in your balance automatically, with a unique, queryable record.' }
    ]
  }
  : {
    title: '异常发生后，不必再追问。',
    description: '网关持续观测每一次调用的最终结果。确认为上游异常后，系统按规则自动生成赔付，直接记入余额 — 无需工单，无需申请。',
    steps: [
      { num: '01', title: '识别', description: '网关确认调用的最终失败，排除客户端中断等干扰。' },
      { num: '02', title: '计算', description: '按公开规则计算赔付金额，全程留痕、可对账。' },
      { num: '03', title: '到账', description: '赔付自动记入账户余额，记录可查、结果唯一。' }
    ]
  }))
</script>

<style scoped>
.zen-comp {
  padding: 110px 0;
}

.zen-comp__grid {
  width: min(100% - 4rem, 1200px);
  margin: auto;
  display: grid;
  grid-template-columns: 0.9fr 1.1fr;
  gap: 96px;
}

.zen-comp__shield {
  color: rgb(var(--zen-ink));
}

.zen-comp__eyebrow {
  display: block;
  margin-top: 22px;
  font-family: 'Cinzel', 'Times New Roman', serif;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.22em;
  color: rgb(var(--color-terracotta));
}

.zen-comp__title {
  margin: 16px 0 0;
  font-size: clamp(36px, 3.6vw, 52px);
  font-weight: 800;
  line-height: 1.18;
  letter-spacing: -0.02em;
  color: rgb(var(--zen-ink));
}

.zen-comp__desc {
  margin: 22px 0 0;
  max-width: 420px;
  font-size: 15.5px;
  line-height: 1.75;
  color: rgb(var(--zen-muted));
}

.zen-comp__steps {
  border-top: 2px solid rgb(var(--zen-ink));
}

.zen-comp__steps article {
  display: grid;
  grid-template-columns: 56px 130px 1fr;
  gap: 16px;
  align-items: baseline;
  padding: 30px 12px;
  border-bottom: 1px solid rgb(var(--zen-line));
  transition: background 0.25s ease;
}

.zen-comp__steps article:hover {
  background: rgb(var(--zen-bg-soft));
}

.zen-comp__steps span {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.1em;
  color: rgb(var(--zen-muted-light));
}

.zen-comp__steps h3 {
  margin: 0;
  font-size: 22px;
  font-weight: 750;
  letter-spacing: -0.01em;
  color: rgb(var(--zen-ink));
}

.zen-comp__steps p {
  margin: 0;
  font-size: 14.5px;
  line-height: 1.7;
  color: rgb(var(--zen-muted));
}

@media (max-width: 900px) {
  .zen-comp {
    padding: 80px 0;
  }

  .zen-comp__grid {
    width: min(100% - 2.5rem, 1200px);
    grid-template-columns: 1fr;
    gap: 56px;
  }

  .zen-comp__steps article {
    grid-template-columns: 44px 1fr;
    gap: 8px 14px;
  }

  .zen-comp__steps p {
    grid-column: 2;
  }
}
</style>
