<template>
  <span class="payment-method-icon" :class="`payment-method-icon--${kind}`" aria-hidden="true">
    <img v-if="kind === 'alipay'" :src="alipayIcon" alt="" />
    <img v-else-if="kind === 'wechat'" :src="wechatIcon" alt="" />
    <Icon v-else name="gift" size="sm" />
  </span>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import alipayIcon from '@/assets/payment-methods/alipay.svg'
import wechatIcon from '@/assets/payment-methods/wechat.svg'

defineProps<{
  kind: 'alipay' | 'wechat' | 'backup'
}>()
</script>

<style scoped>
.payment-method-icon {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 7px;
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
}

.payment-method-icon img {
  width: 1.15rem;
  height: 1.15rem;
  object-fit: contain;
}

.payment-method-icon--alipay {
  background: rgb(var(--color-info) / 0.1);
}

.payment-method-icon--wechat {
  background: rgb(var(--color-laurel) / 0.1);
}

.payment-method-icon--backup {
  color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
  background: rgb(var(--color-terracotta) / 0.1);
}
</style>
