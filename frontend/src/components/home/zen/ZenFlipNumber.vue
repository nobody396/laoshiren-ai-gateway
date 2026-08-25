<template>
  <div class="zen-flip-number" :aria-label="`${prefix}${text}`" role="text">
    <b v-if="prefix" class="zen-flip-number__prefix">{{ prefix }}</b>
    <template v-for="(char, i) in chars" :key="`${i}-${char}`">
      <span v-if="isFlipDigit(char)" class="zen-flip-number__digit">
        <span
          class="zen-flip-number__col"
          :style="{ transform: `translateY(-${Number(char)}em)`, transitionDelay: `${i * 45}ms` }"
        >
          <i v-for="d in 10" :key="d">{{ d - 1 }}</i>
        </span>
        <em class="zen-flip-number__line" />
      </span>
      <span v-else class="zen-flip-number__sep">{{ char }}</span>
    </template>
  </div>
</template>

<script setup lang="ts">
/**
 * 翻牌/里程表数字:每个数字位是一列 0-9 的滚轮,
 * 通过 translateY 滚动到目标值;字符级 45ms 错峰。
 * 数值变化(轮询刷新)时同样平滑滚动。
 */
import { computed } from 'vue'
import { formatFlipNumber, isFlipDigit } from './flipNumber'

const props = withDefaults(defineProps<{
  value: number
  /** 前缀(如 ¥),不参与滚动 */
  prefix?: string
  /** 小数位数 */
  decimals?: number
  /** 整数部分最小位数(前导补零) */
  pad?: number
}>(), {
  prefix: '',
  decimals: 2,
  pad: 5
})

const text = computed(() => formatFlipNumber(props.value, { pad: props.pad, decimals: props.decimals }))
const chars = computed(() => Array.from(text.value))
</script>

<style scoped>
.zen-flip-number {
  display: flex;
  align-items: flex-end;
  gap: 5px;
}

.zen-flip-number__prefix {
  margin: 0 4px 8px 0;
  font-size: 22px;
  font-weight: 700;
  color: rgb(var(--zen-muted-light));
}

.zen-flip-number__digit {
  position: relative;
  --cell-h: 70px;
  width: 52px;
  height: var(--cell-h);
  border: 1px solid rgb(var(--zen-line-dark));
  border-radius: 8px;
  background: linear-gradient(
    180deg,
    rgb(var(--zen-bg)) 0%,
    rgb(var(--zen-bg-soft)) 49%,
    rgb(var(--zen-digit-face)) 50%,
    rgb(var(--zen-bg)) 100%
  );
  box-shadow: 0 1px 2px rgb(var(--zen-ink) / 0.06), inset 0 1px 0 rgb(var(--zen-bg));
  overflow: hidden;
}

.zen-flip-number__col {
  display: flex;
  flex-direction: column;
  font-size: var(--cell-h);
  line-height: 1;
  transition: transform 1s cubic-bezier(0.23, 1, 0.32, 1);
}

.zen-flip-number__col i {
  display: flex;
  align-items: center;
  justify-content: center;
  height: var(--cell-h);
  font-style: normal;
  font-size: 44px;
  font-weight: 700;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
  color: rgb(var(--zen-ink));
}

/* 翻牌中缝 */
.zen-flip-number__line {
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  height: 1px;
  background: rgb(var(--zen-ink) / 0.07);
}

.zen-flip-number__sep {
  align-self: flex-end;
  padding-bottom: 10px;
  font-size: 26px;
  font-weight: 700;
  color: rgb(var(--zen-muted-light));
}

@media (max-width: 900px) {
  .zen-flip-number__digit {
    --cell-h: 60px;
    width: min(10.5vw, 44px);
  }

  .zen-flip-number__col i {
    font-size: 36px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .zen-flip-number__col {
    transition-duration: 1ms;
    transition-delay: 0ms !important;
  }
}
</style>
