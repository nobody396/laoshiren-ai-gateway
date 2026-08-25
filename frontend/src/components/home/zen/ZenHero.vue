<template>
  <section class="zen-hero">
    <div class="zen-hero__inner">
      <div class="zen-hero__copy zen-reveal">
        <div class="zen-hero__eyebrow">LAOSHIRENAI · MMXXVI</div>
        <h1 class="zen-hero__title">
          {{ ui.titleLine1 }}
          <br />
          {{ ui.titleLine2 }}
          <br />
          {{ ui.titleLine3 }}
        </h1>
        <div class="zen-hero__pill">
          <span class="zen-hero__pill-text">{{ ui.pillText }}</span>
          <a class="zen-hero__pill-cta" :href="keyTarget" @click="onPush(keyTarget, $event)">
            <Icon name="sparkles" size="sm" /> {{ ui.pillCta }}
          </a>
        </div>
        <div class="zen-hero__actions">
          <a class="zen-hero__btn zen-hero__btn--dark" href="/models" @click="onPush('/models', $event)">
            {{ ui.exploreModels }} <Icon name="chevronRight" size="sm" />
          </a>
          <a class="zen-hero__btn zen-hero__btn--light" href="#access" @click="onAnchor('#access', $event)">
            {{ ui.connectTools }} <Icon name="chevronRight" size="sm" />
          </a>
        </div>
      </div>

      <div class="zen-hero__aside zen-reveal">
        <div class="zen-hero__glow" aria-hidden="true" />
        <div class="zen-hero__req-card" aria-hidden="true">
          <div class="zen-hero__req-head">
            <span class="zen-hero__req-method">POST</span>
            <span class="zen-hero__req-path">/v1/chat/completions</span>
          </div>
          <div class="zen-hero__req-row"><span>model</span><i>claude-sonnet-4.5</i></div>
          <div class="zen-hero__req-row"><span>status</span><i class="is-ok">200 OK · 1.28s</i></div>
          <div class="zen-hero__req-row"><span>tokens</span><i>1,024</i></div>
          <div class="zen-hero__req-foot">
            <span class="zen-hero__pulse" />
            {{ ui.reqFoot }}
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * Hero 主视觉(ZenMux 白底 + 希腊点缀)
 * - Cinzel eyebrow + 大号无衬线标题
 * - 渐变描边药丸(获取 API Key,按登录态跳转)
 * - 装饰性请求卡片(自动赔付保障中)
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  /** 是否已认证 */
  isAuthenticated: boolean
  /** 控制台路径 */
  dashboardPath: string
}>()

const $router = useRouter()
const { locale } = useI18n()

const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    titleLine1: 'Every model, one gateway,',
    titleLine2: 'unified access.',
    titleLine3: 'Upstream breaks? We compensate.',
    pillText: 'One API key for every mainstream model',
    pillCta: 'Get an API Key',
    exploreModels: 'Explore models',
    connectTools: 'Connect your tools',
    reqFoot: 'Auto-compensation active'
  }
  : {
    titleLine1: '万千模型，一个网关，',
    titleLine2: '统一接入。',
    titleLine3: '调用异常？我们赔付！',
    pillText: '一个 API Key，调用全球主流大模型',
    pillCta: '获取 API Key',
    exploreModels: '探索模型',
    connectTools: '接入工具',
    reqFoot: '自动赔付保障中'
  }))

const keyTarget = computed(() => (props.isAuthenticated ? props.dashboardPath : '/login'))

function onPush(path: string, event: MouseEvent): void {
  event.preventDefault()
  void $router.push(path)
}

function onAnchor(hash: string, event: MouseEvent): void {
  event.preventDefault()
  void $router.push({ path: '/', hash })
}
</script>

<style scoped>
.zen-hero {
  padding: 88px 0 56px;
  background:
    radial-gradient(720px 340px at 78% 12%, rgb(var(--zen-violet) / 0.06), transparent 65%),
    radial-gradient(560px 300px at 12% 30%, rgb(var(--zen-blue) / 0.05), transparent 60%),
    rgb(var(--zen-bg));
}

.zen-hero__inner {
  width: min(100% - 4rem, 1200px);
  margin: auto;
  display: grid;
  grid-template-columns: 1.25fr 0.75fr;
  gap: 56px;
  align-items: center;
}

.zen-hero__copy {
  min-width: 0;
}

.zen-hero__eyebrow {
  margin-bottom: 26px;
  font-family: 'Cinzel', 'Times New Roman', serif;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.22em;
  color: rgb(var(--color-terracotta));
}

.zen-hero__title {
  margin: 0;
  font-size: clamp(40px, 4.6vw, 64px);
  font-weight: 800;
  line-height: 1.16;
  letter-spacing: -0.02em;
  color: rgb(var(--zen-ink));
}

.zen-hero__pill {
  margin-top: 36px;
  display: flex;
  align-items: center;
  gap: 14px;
  width: max-content;
  max-width: 100%;
  padding: 8px 8px 8px 20px;
  border: 1px solid rgb(var(--zen-line-dark));
  border-radius: 999px;
  background: rgb(var(--zen-bg));
  box-shadow: 0 1px 2px rgb(var(--zen-ink) / 0.04), 0 8px 24px rgb(var(--zen-ink) / 0.04);
}

.zen-hero__pill-text {
  font-size: 14px;
  color: rgb(var(--zen-muted));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.zen-hero__pill-cta {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: none;
  padding: 9px 16px;
  border-radius: 999px;
  background: rgb(var(--zen-bg));
  font-size: 13px;
  font-weight: 600;
  color: rgb(var(--zen-ink));
  text-decoration: none;
  transition: background 0.2s ease;
}

/* 旋转渐变描边:violet → blue → cyan,沿药丸一周流动 */
.zen-hero__pill-cta::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 999px;
  padding: 1.5px;
  background: linear-gradient(
    100deg,
    rgb(var(--zen-violet)),
    rgb(var(--zen-blue)),
    rgb(var(--zen-cyan)),
    rgb(var(--zen-violet))
  );
  background-size: 300% 100%;
  /* mask 只需要不透明色,复用 zen-bg 令牌 */
  -webkit-mask: linear-gradient(rgb(var(--zen-bg)) 0 0) content-box, linear-gradient(rgb(var(--zen-bg)) 0 0);
  mask: linear-gradient(rgb(var(--zen-bg)) 0 0) content-box, linear-gradient(rgb(var(--zen-bg)) 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  animation: zenSpinBorder 5s linear infinite;
}

.zen-hero__pill-cta:hover {
  background: rgb(var(--zen-bg-soft));
}

@keyframes zenSpinBorder {
  to {
    background-position: 300% 0;
  }
}

.zen-hero__actions {
  display: flex;
  gap: 12px;
  margin-top: 28px;
}

.zen-hero__btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 44px;
  padding: 0 22px;
  border-radius: 11px;
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  transition: transform 0.18s ease, box-shadow 0.18s ease, background 0.18s ease;
}

.zen-hero__btn:active {
  transform: scale(0.97);
}

.zen-hero__btn--dark {
  background: rgb(var(--zen-ink));
  color: rgb(var(--zen-bg));
  box-shadow: 0 1px 2px rgb(var(--zen-ink) / 0.15);
}

.zen-hero__btn--dark:hover {
  background: rgb(var(--zen-text-strong));
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgb(var(--zen-ink) / 0.14);
}

.zen-hero__btn--light {
  background: rgb(var(--zen-digit-face));
  color: rgb(var(--zen-ink));
}

.zen-hero__btn--light:hover {
  background: rgb(var(--zen-hover-fill));
  transform: translateY(-1px);
}

/* 装饰性请求卡片:mono 字体 + 毛玻璃 + 上下漂浮 */
.zen-hero__aside {
  position: relative;
  display: flex;
  justify-content: center;
}

.zen-hero__glow {
  position: absolute;
  inset: -40px;
  background: radial-gradient(closest-side, rgb(var(--zen-violet) / 0.14), transparent 70%);
  filter: blur(10px);
  animation: zenBreathe 6s ease-in-out infinite;
}

@keyframes zenBreathe {
  50% {
    opacity: 0.55;
    transform: scale(1.06);
  }
}

.zen-hero__req-card {
  position: relative;
  width: min(340px, 100%);
  padding: 18px 20px;
  border: 1px solid rgb(var(--zen-line-dark));
  border-radius: 16px;
  background: rgb(var(--zen-bg) / 0.86);
  backdrop-filter: blur(8px);
  box-shadow: 0 24px 48px -12px rgb(var(--zen-ink) / 0.12);
  font-family: 'SF Mono', ui-monospace, Menlo, Consolas, monospace;
  font-size: 12.5px;
  animation: zenFloat 7s ease-in-out infinite;
}

@keyframes zenFloat {
  50% {
    transform: translateY(-10px);
  }
}

.zen-hero__req-head {
  display: flex;
  gap: 10px;
  align-items: center;
  padding-bottom: 12px;
  margin-bottom: 12px;
  border-bottom: 1px dashed rgb(var(--zen-line-dark));
}

.zen-hero__req-method {
  font-weight: 700;
  color: rgb(var(--zen-violet));
}

.zen-hero__req-path {
  color: rgb(var(--zen-text));
}

.zen-hero__req-row {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  color: rgb(var(--zen-muted-light));
}

.zen-hero__req-row i {
  font-style: normal;
  color: rgb(var(--zen-ink));
}

.zen-hero__req-row i.is-ok {
  color: rgb(var(--zen-ok));
}

.zen-hero__req-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px dashed rgb(var(--zen-line-dark));
  color: rgb(var(--zen-text));
  font-family: 'Inter', 'PingFang SC', sans-serif;
  font-size: 12px;
}

.zen-hero__pulse {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgb(var(--zen-ok));
  animation: zenPulse 2s ease-out infinite;
}

@keyframes zenPulse {
  0% {
    box-shadow: 0 0 0 0 rgb(var(--zen-ok) / 0.45);
  }
  70% {
    box-shadow: 0 0 0 9px rgb(var(--zen-ok) / 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgb(var(--zen-ok) / 0);
  }
}

@media (max-width: 900px) {
  .zen-hero {
    padding: 56px 0 44px;
  }

  .zen-hero__inner {
    width: min(100% - 2.5rem, 1200px);
    grid-template-columns: 1fr;
    gap: 44px;
  }

  .zen-hero__title {
    font-size: clamp(30px, 8.2vw, 38px);
  }

  .zen-hero__aside {
    justify-content: flex-start;
  }

  .zen-hero__req-card {
    width: min(320px, 100%);
    animation: none;
  }

  .zen-hero__pill {
    width: 100%;
    padding-left: 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .zen-hero__pill-cta::before,
  .zen-hero__glow,
  .zen-hero__req-card,
  .zen-hero__pulse {
    animation: none;
  }

  .zen-hero__btn,
  .zen-hero__pill-cta {
    transition-duration: 1ms;
  }
}
</style>
