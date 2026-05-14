<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center gap-3">
      <label class="btn btn-secondary cursor-pointer">
        <input
          type="file"
          class="hidden"
          accept="image/jpeg,image/png,image/gif,image/webp"
          multiple
          :disabled="uploading || modelValue.length >= maxCount"
          @change="handleFileChange"
        />
        {{ uploading ? uploadingText : addButtonText }}
      </label>
      <span class="text-sm text-gray-500 dark:text-dark-400">
        {{ countText }}
      </span>
    </div>

    <p v-if="hint" class="text-xs text-gray-500 dark:text-dark-400">{{ hint }}</p>
    <p v-if="errorMessage" class="text-xs text-red-500">{{ errorMessage }}</p>

    <div v-if="modelValue.length > 0" class="grid grid-cols-2 gap-3 md:grid-cols-5">
      <div
        v-for="(image, index) in modelValue"
        :key="`${image}-${index}`"
        class="group relative overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
      >
        <img :src="image" alt="" class="h-28 w-full object-cover" />
        <button
          type="button"
          class="absolute right-2 top-2 rounded-full bg-black/65 px-2 py-1 text-xs text-white opacity-0 transition group-hover:opacity-100"
          @click="removeImage(index)"
        >
          {{ removeText }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: string[]
  uploadFn: (file: File) => Promise<string>
  maxCount?: number
  maxSizeMB?: number
  hint?: string
  addButtonText?: string
  uploadingText?: string
  removeText?: string
  countTemplate?: string
}>(), {
  maxCount: 5,
  maxSizeMB: 5,
  hint: '',
  addButtonText: 'Upload',
  uploadingText: 'Uploading...',
  removeText: 'Remove',
  countTemplate: '{count}/{max}',
})

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const uploading = ref(false)
const errorMessage = ref('')

const countText = computed(() =>
  props.countTemplate
    .replace('{count}', String(props.modelValue.length))
    .replace('{max}', String(props.maxCount))
)

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  errorMessage.value = ''

  if (files.length === 0) {
    return
  }
  if (props.modelValue.length + files.length > props.maxCount) {
    errorMessage.value = `Too many images. Max ${props.maxCount}.`
    return
  }

  // Validate all file sizes upfront before uploading any
  const oversized = files.find(f => f.size > props.maxSizeMB * 1024 * 1024)
  if (oversized) {
    errorMessage.value = `File too large: ${oversized.name}`
    return
  }

  uploading.value = true
  try {
    const nextImages = [...props.modelValue]
    for (const file of files) {
      nextImages.push(await props.uploadFn(file))
    }
    emit('update:modelValue', nextImages)
  } catch (error: any) {
    errorMessage.value = error?.message || 'Upload failed'
  } finally {
    uploading.value = false
  }
}

function removeImage(index: number) {
  emit('update:modelValue', props.modelValue.filter((_, currentIndex) => currentIndex !== index))
}
</script>
