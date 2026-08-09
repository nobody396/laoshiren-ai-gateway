import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import FeedbackEditView from '@/views/user/FeedbackEditView.vue'

const pushMock = vi.fn()
const getByIdMock = vi.fn()
const updateMock = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
  useRoute: () => ({ params: { id: '42' } }),
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
    getById: (...args: unknown[]) => getByIdMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
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
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<div data-testid="multi-upload" />',
})

const baseFeedbackDetail = {
  id: 42,
  user_id: 1,
  category: 'suggestion',
  title: 'Add dark mode',
  content: 'Please add dark mode support',
  request_id: 'request-old',
  images: ['https://example.com/img1.png'],
  contact: 'user@example.com',
  priority: 'normal',
  status: 'pending',
  reply_count: 0,
  created_at: '2026-04-10T12:00:00Z',
  updated_at: '2026-04-10T12:00:00Z',
  replies: [],
}

function mountEditView() {
  return mount(FeedbackEditView, {
    global: {
      plugins: [createPinia()],
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
}

describe('FeedbackEditView', () => {
  beforeEach(() => {
    pushMock.mockReset()
    getByIdMock.mockReset()
    updateMock.mockReset()
  })

  it('loads feedback data and pre-fills the form', async () => {
    getByIdMock.mockResolvedValue(baseFeedbackDetail)

    const wrapper = mountEditView()
    await flushPromises()

    expect(getByIdMock).toHaveBeenCalledWith(42)

    // Content should be pre-filled via textarea stub
    const textarea = wrapper.find('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toBe('Please add dark mode support')

    const requestContext = wrapper.find('textarea.input[maxlength="2000"]')
    expect((requestContext.element as HTMLTextAreaElement).value).toBe('request-old')
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('input').exists()).toBe(false)
  })

  it('submits updated feedback and redirects to detail page', async () => {
    getByIdMock.mockResolvedValue(baseFeedbackDetail)
    updateMock.mockResolvedValue({ ...baseFeedbackDetail, content: 'Updated details' })

    const wrapper = mountEditView()
    await flushPromises()

    await wrapper.find('textarea').setValue('Updated details')
    await wrapper.find('textarea.input[maxlength="2000"]').setValue('request-new\nerror details')
    await nextTick()

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateMock).toHaveBeenCalledWith(42, {
      content: 'Updated details',
      images: ['https://example.com/img1.png'],
      request_id: 'request-new\nerror details',
    })
    expect(pushMock).toHaveBeenCalledWith('/feedbacks/42')
  })

  it('shows loading state while fetching', async () => {
    getByIdMock.mockReturnValue(new Promise(() => {})) // never resolves

    const wrapper = mountEditView()
    await nextTick()

    expect(wrapper.text()).toContain('common.loading')
  })

  it('does not render form for closed feedback', async () => {
    getByIdMock.mockResolvedValue({ ...baseFeedbackDetail, status: 'closed' })

    const wrapper = mountEditView()
    await flushPromises()

    // Should show a message instead of the form
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.text()).toContain('feedback.edit.closedHint')
  })
})
