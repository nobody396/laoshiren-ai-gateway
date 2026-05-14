<template>
  <div class="hero-animation-shell">
    <div ref="container" class="hero-animation" :class="{ 'is-visible': isVisible }"></div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import lottie from 'lottie-web/build/player/lottie_light_canvas'

const container = ref<HTMLDivElement | null>(null)
const isVisible = ref(false)

let animation: ReturnType<typeof lottie.loadAnimation> | null = null
let startTimer: number | null = null
let aborted = false

onMounted(() => {
  startTimer = window.setTimeout(() => {
    if (!container.value) {
      return
    }

    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    fetch('/homepage-reference/reference-home-hero-lottie.json')
      .then((response) => response.json())
      .then((animationData) => {
        if (aborted || !container.value) {
          return
        }

        animation = lottie.loadAnimation({
          container: container.value,
          renderer: 'canvas',
          loop: false,
          autoplay: !reducedMotion,
          animationData,
          rendererSettings: {
            preserveAspectRatio: 'xMidYMid meet',
            progressiveLoad: false,
            clearCanvas: true
          }
        })

        const reveal = () => {
          isVisible.value = true
        }

        animation.addEventListener('DOMLoaded', reveal)
        animation.addEventListener('data_ready', reveal)
        animation.addEventListener('loaded_images', reveal)
        animation.addEventListener('complete', () => {
          animation?.goToAndStop(Math.max(animationData.op - 1, 0), true)
        })

        if (reducedMotion) {
          reveal()
          animation.goToAndStop(Math.max(animationData.op - 1, 0), true)
        }
      })
      .catch((error) => {
        console.error('Failed to load reference hero animation:', error)
      })
  }, 200)
})

onBeforeUnmount(() => {
  aborted = true
  if (startTimer !== null) {
    window.clearTimeout(startTimer)
  }
  animation?.destroy()
  animation = null
})
</script>

<style scoped>
.hero-animation-shell {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: visible;
}

.hero-animation {
  width: 100%;
  height: 100%;
  opacity: 0;
  transform: translate(-40px, 90px) scale(1.4);
  transform-origin: center center;
  transition: opacity 0.3s ease;
}

.hero-animation.is-visible {
  opacity: 1;
}
</style>
