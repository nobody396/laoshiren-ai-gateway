<template>
  <div class="effort-field">
    <div class="effort-heading">
      <span>{{ label }}</span>
      <div><strong>{{ selected.label }}</strong><em v-if="selected.mode">{{ selected.mode }}</em></div>
    </div>
    <div class="effort-slider">
      <div class="effort-range">
        <div class="effort-rail" :style="{ '--effort-progress': `${progress}%` }">
          <span class="effort-dots" :style="{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }" aria-hidden="true">
            <i v-for="(_, itemIndex) in options" :key="itemIndex" :class="{ passed: itemIndex <= index }" />
          </span>
        </div>
        <input :value="index" type="range" min="0" :max="Math.max(0, options.length - 1)" step="1" :aria-label="label" @input="selectIndex" />
      </div>
      <div class="effort-labels" :style="{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }">
        <button v-for="(item, itemIndex) in options" :key="item.id" type="button" :class="{ active: index === itemIndex }" @click="$emit('update:modelValue', item.id)">{{ item.shortLabel }}</button>
      </div>
    </div>
    <small v-if="selected.description">{{ selected.description }}</small>
    <small v-if="mappingText" class="effort-mapping">{{ mappingText }}</small>
    <small v-if="supportText" class="effort-support">{{ supportText }}</small>
    <small v-if="warningText" class="effort-warning">{{ warningText }}</small>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

export interface DocsReasoningOption {
  id: string
  label: string
  shortLabel: string
  mode?: string
  description?: string
}

const props = withDefaults(defineProps<{
  modelValue: string
  options: readonly DocsReasoningOption[]
  label?: string
  mappingText?: string
  supportText?: string
  warningText?: string
}>(), {
  label: '推理强度',
  mappingText: '',
  supportText: '',
  warningText: '',
})

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const fallback: DocsReasoningOption = { id: 'auto', label: '自动', shortLabel: '自动' }
const index = computed(() => Math.max(0, props.options.findIndex(item => item.id === props.modelValue)))
const selected = computed(() => props.options[index.value] ?? props.options[0] ?? fallback)
const progress = computed(() => props.options.length > 1 ? (index.value / (props.options.length - 1)) * 100 : 0)

function selectIndex(event: Event): void {
  const item = props.options[Number((event.target as HTMLInputElement).value)]
  if (item) emit('update:modelValue', item.id)
}
</script>

<style scoped>
.effort-field{max-width:28rem;margin-top:1.15rem;border:1px solid #e4e4e7;border-radius:.85rem;background:#fafafa;padding:.72rem .8rem}.effort-heading{display:flex;align-items:center;justify-content:space-between;gap:1rem}.effort-heading>span{color:#3f3f46;font-size:.66rem;font-weight:700}.effort-heading>div{display:flex;align-items:center;gap:.45rem}.effort-heading strong{color:#1267d6;font-size:.7rem}.effort-heading em{border-radius:999px;background:#eaf3ff;color:#1267d6;padding:.2rem .42rem;font-size:.54rem;font-style:normal;font-weight:700}.effort-slider{padding:0}.effort-range{position:relative;height:1.7rem;margin-top:.62rem}.effort-rail{position:absolute;inset:0;border-radius:999px;background:linear-gradient(90deg,#3699f5 0 var(--effort-progress),#e5e7eb var(--effort-progress) 100%);box-shadow:inset 0 0 0 1px #d4d4d8}.effort-dots{position:absolute;inset:0 .82rem;display:grid;align-items:center}.effort-dots i{justify-self:center;width:.28rem;height:.28rem;border-radius:999px;background:#b6b8bd}.effort-dots i.passed{background:#c7e3ff}.effort-range>input{position:absolute;inset:0;width:100%;height:100%;box-sizing:border-box;margin:0;appearance:none;border:0;background:transparent;padding:0;box-shadow:none;cursor:pointer}.effort-range>input::-webkit-slider-runnable-track{height:1.7rem;border-radius:999px;background:transparent}.effort-range>input::-webkit-slider-thumb{width:1.7rem;height:1.7rem;margin-top:0;appearance:none;border:1px solid #d4d4d8;border-radius:999px;background:#fff;box-shadow:0 1px 5px #18181b30}.effort-range>input::-moz-range-track{height:1.7rem;border-radius:999px;background:transparent}.effort-range>input::-moz-range-progress{background:transparent}.effort-range>input::-moz-range-thumb{width:1.58rem;height:1.58rem;border:1px solid #d4d4d8;border-radius:999px;background:#fff;box-shadow:0 1px 5px #18181b30}.effort-labels{display:grid;margin-top:.3rem}.effort-labels button{border:0;background:transparent;color:#a1a1aa;padding:.14rem 0;font-size:.52rem;font-weight:700;cursor:pointer}.effort-labels button:hover{color:#52525b}.effort-labels button.active{color:#1267d6}.effort-field small{display:block;margin-top:.35rem;color:#71717a;font-size:.61rem;line-height:1.55}.effort-field .effort-support{color:#1267d6;font-weight:600}.effort-field .effort-mapping{color:#b45309;font-weight:600}.effort-field .effort-warning{color:#b45309;font-weight:600}
</style>
