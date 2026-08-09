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

    await wrapper.find('textarea').setValue('Steps to reproduce')
    await wrapper.find('textarea.input[maxlength="2000"]').setValue('request-123\nupstream timed out')
    await nextTick()

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(createFeedbackMock).toHaveBeenCalledWith({
      content: 'Steps to reproduce',
      images: [],
      request_id: 'request-123\nupstream timed out',
    })
    expect(pushMock).toHaveBeenCalledWith('/feedbacks/18')
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('input').exists()).toBe(false)
  })
})
