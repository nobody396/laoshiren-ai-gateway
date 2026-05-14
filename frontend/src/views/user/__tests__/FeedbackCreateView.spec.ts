import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import FeedbackCreateView from '@/views/user/FeedbackCreateView.vue'

const pushMock = vi.fn()
const createFeedbackMock = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/api/feedbacks', () => ({
  default: {
    create: (...args: unknown[]) => createFeedbackMock(...args),
    uploadImage: vi.fn(),
  },
}))

const AppLayoutStub = defineComponent({
  name: 'AppLayout',
  template: '<div><slot /></div>',
})

const SelectStub = defineComponent({
  name: 'AppSelect',
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: `
    <select :value="modelValue" @change="$emit('update:modelValue', $event.target.value)">
      <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
    </select>
  `,
})

const MarkdownEditorFieldStub = defineComponent({
  name: 'MarkdownEditorField',
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<textarea :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
})

const MultiImageUploadStub = defineComponent({
  name: 'MultiImageUpload',
  template: '<div data-testid="multi-upload" />',
})

describe('FeedbackCreateView', () => {
  beforeEach(() => {
    pushMock.mockReset()
    createFeedbackMock.mockReset()
  })

  it('submits feedback and redirects to detail page', async () => {
    createFeedbackMock.mockResolvedValue({ id: 18 })

    const wrapper = mount(FeedbackCreateView, {
      global: {
        plugins: [
          createPinia(),
        ],
        stubs: {
          AppLayout: AppLayoutStub,
          Select: SelectStub,
          MarkdownEditorField: MarkdownEditorFieldStub,
          MultiImageUpload: MultiImageUploadStub,
          RouterLink: defineComponent({
            name: 'RouterLink',
            props: ['to'],
            template: '<a href="#"><slot /></a>',
          }),
        },
      },
    })

    await wrapper.find('input[maxlength="200"]').setValue('Broken request flow')
    await wrapper.find('textarea').setValue('Steps to reproduce')
    await nextTick()

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(createFeedbackMock).toHaveBeenCalledWith({
      category: 'bug',
      title: 'Broken request flow',
      content: 'Steps to reproduce',
      images: [],
      contact: undefined,
    })
    expect(pushMock).toHaveBeenCalledWith('/feedbacks/18')
  })
})
