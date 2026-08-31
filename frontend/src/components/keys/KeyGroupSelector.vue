<template>
  <div class="key-group-selector" :class="{ 'w-full': variant === 'field' }">
    <button
      ref="triggerRef"
      type="button"
      data-testid="key-group-selector-trigger"
      :class="triggerClass"
      :aria-expanded="open"
      aria-haspopup="listbox"
      @click="toggle"
    >
      <template v-if="variant === 'field'">
        <GroupBadge
          v-if="selectedOption"
          :name="selectedOption.label"
          :platform="selectedOption.platform"
          :subscription-type="selectedOption.subscriptionType"
          :rate-multiplier="shouldShowMeta(selectedOption) ? selectedOption.rate : undefined"
          :user-rate-multiplier="shouldShowMeta(selectedOption) ? selectedOption.userRate : null"
          :show-rate="shouldShowMeta(selectedOption)"
        />
        <span v-else class="text-gray-400 dark:text-gray-500">{{ placeholder }}</span>
      </template>

      <template v-else>
        <GroupBadge
          v-if="selectedOption"
          :name="selectedOption.label"
          :platform="selectedOption.platform"
          :subscription-type="selectedOption.subscriptionType"
          :rate-multiplier="selectedOption.rate"
          :user-rate-multiplier="selectedOption.userRate"
        />
        <span v-else class="text-sm text-gray-400 dark:text-dark-400">{{ t('keys.noGroup') }}</span>
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.selectGroup') }}</span>
      </template>

      <svg
        class="h-4 w-4 shrink-0 text-gray-400 transition-transform"
        :class="{ 'rotate-180': open }"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        stroke-width="2"
      >
        <path stroke-linecap="round" stroke-linejoin="round" d="m6 9 6 6 6-6" />
      </svg>
    </button>

    <Teleport to="body">
      <div
        v-if="open && position"
        ref="panelRef"
        data-testid="key-group-selector-popup"
        class="animate-in fade-in slide-in-from-top-2 fixed z-[100000020] w-[min(560px,calc(100vw-24px))] overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl shadow-black/10 duration-200 dark:border-dark-700 dark:bg-dark-800 dark:shadow-black/30"
        style="pointer-events: auto !important;"
        :style="{
          top: position.top !== undefined ? position.top + 'px' : undefined,
          bottom: position.bottom !== undefined ? position.bottom + 'px' : undefined,
          left: position.left + 'px'
        }"
        role="listbox"
      >
        <div class="border-b border-gray-100 p-2 dark:border-dark-700">
          <div class="relative">
            <svg class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 1 1-14 0 7 7 0 0 1 14 0Z" />
            </svg>
            <input
              v-model="searchQuery"
              type="text"
              data-testid="key-group-selector-search"
              class="w-full rounded-lg border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-sm text-gray-900 placeholder-gray-400 outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-300 dark:border-dark-600 dark:bg-dark-700 dark:text-white dark:placeholder-gray-500 dark:focus:border-primary-600 dark:focus:ring-primary-600"
              :placeholder="searchPlaceholder"
              @click.stop
            />
          </div>

          <div class="mt-2 flex gap-1 rounded-xl bg-gray-100 p-1 dark:bg-dark-900">
            <button
              v-for="section in sections"
              :key="section.id"
              type="button"
              :class="[
                'flex-1 rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors',
                activeSection === section.id
                  ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
                  : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
              ]"
              @click.stop="selectSection(section.id)"
            >
              {{ sectionLabel(section.id) }}
              <span class="ml-1 opacity-60">{{ section.options.length }}</span>
            </button>
          </div>

          <div v-if="families.length > 1" class="mt-2 flex gap-1 overflow-x-auto pb-0.5">
            <button
              v-for="family in families"
              :key="family.id"
              type="button"
              :class="[
                'shrink-0 rounded-full border px-3 py-1 text-xs font-medium transition-colors',
                activeFamily === family.id
                  ? 'border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                  : 'border-gray-200 bg-white text-gray-500 hover:border-gray-300 hover:text-gray-800 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400 dark:hover:text-gray-200'
              ]"
              @click.stop="activeFamily = family.id"
            >
              {{ familyLabel(family.id) }}
              <span class="ml-1 opacity-60">{{ family.options.length }}</span>
            </button>
          </div>
        </div>

        <div
          class="overflow-y-auto bg-gray-50/60 p-2 dark:bg-dark-900/40"
          :style="{ maxHeight: position.listMaxHeight + 'px' }"
        >
          <div class="space-y-1">
            <button
              v-for="option in visibleOptions"
              :key="option.value"
              type="button"
              :data-option-value="option.value"
              class="flex w-full items-center justify-between rounded-xl border px-3 py-3 text-sm transition-all"
              :class="modelValue === option.value
                ? 'border-primary-200 bg-primary-50 shadow-sm dark:border-primary-800 dark:bg-primary-900/20'
                : 'border-transparent bg-white hover:border-gray-200 hover:bg-gray-50 dark:bg-dark-800 dark:hover:border-dark-600 dark:hover:bg-dark-700'"
              :title="shouldShowMeta(option) ? option.description || undefined : undefined"
              role="option"
              :aria-selected="modelValue === option.value"
              @click="selectOption(option)"
            >
              <GroupOptionItem
                :name="option.label"
                :platform="option.platform"
                :subscription-type="option.subscriptionType"
                :rate-multiplier="shouldShowMeta(option) ? option.rate : undefined"
                :user-rate-multiplier="shouldShowMeta(option) ? option.userRate : null"
                :description="shouldShowMeta(option) ? option.description : null"
                :action-label="isMonthlyGroupOption(option) ? t('keys.groupSections.monthlyOnly') : null"
                :cache-hit-rate-pct="option.cacheHitRatePct"
                :cache-window-days="option.cacheWindowDays"
                :selected="modelValue === option.value"
              />
            </button>
          </div>

          <div v-if="visibleOptions.length === 0" class="py-8 text-center text-sm text-gray-400 dark:text-gray-500">
            {{ t('keys.noGroupFound') }}
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import {
  buildGroupOptionFamilies,
  buildGroupOptionSections,
  isMonthlyGroupOption,
  type GroupOptionFamilyId,
  type GroupOptionSectionId,
  type SectionableGroupOption
} from '@/utils/groupOptionSections'

interface KeyGroupSelectorOption extends SectionableGroupOption {
  value: number
  description: string | null
  cacheHitRatePct: number | null
  cacheWindowDays: number
}

interface Props {
  modelValue: number | null
  options: KeyGroupSelectorOption[]
  includeMonthly?: boolean
  variant?: 'field' | 'inline'
  placeholder?: string
  searchPlaceholder?: string
}

const props = withDefaults(defineProps<Props>(), {
  includeMonthly: false,
  variant: 'field',
  placeholder: '',
  searchPlaceholder: ''
})

const emit = defineEmits<{
  'update:modelValue': [value: number]
  open: []
  close: []
}>()

const { t } = useI18n()
const triggerRef = ref<HTMLElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const open = ref(false)
const searchQuery = ref('')
const activeSection = ref<GroupOptionSectionId>('payg')
const activeFamily = ref<GroupOptionFamilyId>('openai')
const position = ref<{
  top?: number
  bottom?: number
  left: number
  listMaxHeight: number
} | null>(null)

const sections = computed(() => buildGroupOptionSections(props.options, props.includeMonthly))
const activeSectionRow = computed(() => sections.value.find((section) => section.id === activeSection.value) ?? sections.value[0])
const families = computed(() => buildGroupOptionFamilies(activeSectionRow.value?.options ?? []))
const selectedOption = computed(() => props.options.find((option) => option.value === props.modelValue) ?? null)
const visibleOptions = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  const options = activeSectionRow.value?.options ?? []
  if (query) {
    return options.filter((option) => option.label.toLowerCase().includes(query) || option.description?.toLowerCase().includes(query))
  }
  return families.value.find((family) => family.id === activeFamily.value)?.options ?? []
})

const triggerClass = computed(() => props.variant === 'field'
  ? 'flex min-h-12 w-full items-center justify-between gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3 text-left text-sm text-gray-900 transition-colors hover:border-gray-300 focus:border-primary-400 focus:outline-none focus:ring-2 focus:ring-primary-100 dark:border-dark-600 dark:bg-dark-800 dark:text-white dark:hover:border-dark-500 dark:focus:border-primary-600 dark:focus:ring-primary-900/30'
  : '-mx-2 -my-1 flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1 text-left transition-all duration-200 hover:bg-gray-100 dark:hover:bg-dark-700'
)

const sectionLabel = (section: GroupOptionSectionId) => section === 'monthly'
  ? t('keys.groupSections.monthly')
  : t('keys.groupSections.payg')
const familyLabel = (family: GroupOptionFamilyId) => t(`keys.groupFamilies.${family}`)
const shouldShowMeta = (option: KeyGroupSelectorOption) => !isMonthlyGroupOption(option)

const selectSection = (section: GroupOptionSectionId) => {
  activeSection.value = section
  activeFamily.value = buildGroupOptionFamilies(
    sections.value.find((candidate) => candidate.id === section)?.options ?? []
  )[0]?.id ?? 'openai'
}

const syncActiveFilters = () => {
  const selected = selectedOption.value
  if (selected) {
    activeSection.value = isMonthlyGroupOption(selected) ? 'monthly' : 'payg'
    activeFamily.value = selected.familyKey
    return
  }
  selectSection(sections.value[0]?.id ?? 'payg')
}

const updatePosition = () => {
  const trigger = triggerRef.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const dropdownWidth = Math.min(560, window.innerWidth - 24)
  const safeLeft = Math.min(
    Math.max(12, rect.left),
    Math.max(12, window.innerWidth - dropdownWidth - 12)
  )
  const spaceBelow = window.innerHeight - rect.bottom
  const spaceAbove = rect.top
  const openUpward = spaceBelow < 460 && spaceAbove > spaceBelow
  const availableHeight = openUpward ? spaceAbove : spaceBelow
  const listMaxHeight = Math.max(160, Math.min(480, availableHeight - 156))
  position.value = openUpward
    ? { bottom: window.innerHeight - rect.top + 4, left: safeLeft, listMaxHeight }
    : { top: rect.bottom + 4, left: safeLeft, listMaxHeight }
}

const close = () => {
  if (!open.value) return
  open.value = false
  position.value = null
  emit('close')
}

const show = async () => {
  searchQuery.value = ''
  syncActiveFilters()
  open.value = true
  emit('open')
  await nextTick()
  updatePosition()
}

const toggle = () => open.value ? close() : show()
const selectOption = (option: KeyGroupSelectorOption) => {
  emit('update:modelValue', option.value)
  close()
}

const handleOutsidePointer = (event: PointerEvent) => {
  const target = event.target as Node
  if (!triggerRef.value?.contains(target) && !panelRef.value?.contains(target)) close()
}
const handleViewportChange = () => {
  if (open.value) updatePosition()
}

watch(sections, () => {
  if (!sections.value.some((section) => section.id === activeSection.value)) syncActiveFilters()
})

onMounted(() => {
  document.addEventListener('pointerdown', handleOutsidePointer)
  window.addEventListener('resize', handleViewportChange)
  window.addEventListener('scroll', handleViewportChange, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handleOutsidePointer)
  window.removeEventListener('resize', handleViewportChange)
  window.removeEventListener('scroll', handleViewportChange, true)
})
</script>
