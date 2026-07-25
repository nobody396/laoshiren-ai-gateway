<template>
  <Teleport to="body">
    <Transition name="lightbox">
      <div
        v-if="show"
        class="fixed inset-0 z-[80] flex min-h-[100dvh] flex-col bg-black/90 p-3 backdrop-blur-sm sm:p-6"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        data-test="image-lightbox"
        @click.self="emit('close')"
      >
        <div class="flex shrink-0 items-center justify-between gap-3 text-white">
          <h3 :id="titleId" class="min-w-0 truncate text-sm font-semibold sm:text-base">
            {{ title }}
          </h3>
          <div class="flex shrink-0 items-center gap-2">
            <a
              v-if="src"
              :href="src"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex min-h-10 items-center gap-1.5 rounded-lg bg-white/10 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-white/20 focus:outline-none focus:ring-2 focus:ring-white/70"
              :aria-label="openExternalLabel"
              :title="openExternalLabel"
              data-test="image-lightbox-external"
            >
              <Icon name="externalLink" size="sm" />
              <span class="hidden sm:inline">{{ openExternalLabel }}</span>
            </a>
            <button
              ref="closeButtonRef"
              type="button"
              class="inline-flex h-10 w-10 items-center justify-center rounded-lg bg-white/10 text-white transition-colors hover:bg-white/20 focus:outline-none focus:ring-2 focus:ring-white/70"
              :aria-label="closeLabel"
              :title="closeLabel"
              data-test="image-lightbox-close"
              @click="emit('close')"
            >
              <Icon name="x" size="md" />
            </button>
          </div>
        </div>

        <div
          class="flex min-h-0 flex-1 items-center justify-center py-3 sm:py-5"
          data-test="image-lightbox-stage"
          @click.self="emit('close')"
        >
          <div v-if="imageFailed" class="max-w-md rounded-xl bg-white/10 p-6 text-center text-sm text-white">
            {{ failedLabel }}
          </div>
          <img
            v-else-if="src"
            :src="src"
            :alt="alt || title"
            class="max-h-[calc(100dvh-6.75rem)] max-w-full select-none object-contain sm:max-h-[calc(100dvh-8rem)]"
            data-test="image-lightbox-image"
            @error="handleImageError"
          />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'

let lightboxIdCounter = 0
const titleId = `image-lightbox-title-${++lightboxIdCounter}`

const props = withDefaults(defineProps<{
  show: boolean
  src: string
  title: string
  alt?: string
  closeLabel: string
  openExternalLabel: string
  failedLabel: string
}>(), {
  alt: ''
})

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'error'): void
}>()

const closeButtonRef = ref<HTMLButtonElement | null>(null)
const imageFailed = ref(false)
let previousActiveElement: HTMLElement | null = null
let ownsBodyScrollLock = false

function restorePageState() {
  if (ownsBodyScrollLock) {
    document.body.classList.remove('modal-open')
    ownsBodyScrollLock = false
  }
  previousActiveElement?.focus()
  previousActiveElement = null
}

watch(
  () => props.show,
  async (isOpen) => {
    if (!isOpen) {
      restorePageState()
      return
    }

    imageFailed.value = false
    previousActiveElement = document.activeElement as HTMLElement
    ownsBodyScrollLock = !document.body.classList.contains('modal-open')
    if (ownsBodyScrollLock) {
      document.body.classList.add('modal-open')
    }
    await nextTick()
    closeButtonRef.value?.focus()
  },
  { immediate: true }
)

watch(
  () => props.src,
  () => {
    imageFailed.value = false
  }
)

function handleImageError() {
  imageFailed.value = true
  emit('error')
}

function handleEscape(event: KeyboardEvent) {
  if (!props.show || event.key !== 'Escape') return
  event.preventDefault()
  event.stopImmediatePropagation()
  emit('close')
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleEscape, true)
  restorePageState()
})
</script>

<style scoped>
.lightbox-enter-active,
.lightbox-leave-active {
  transition: opacity 160ms ease;
}

.lightbox-enter-from,
.lightbox-leave-to {
  opacity: 0;
}
</style>
