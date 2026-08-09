import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { createPinia } from 'pinia'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const getByIdMock = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '42' } }),
  RouterLink: defineComponent({
    name: 'RouterLink',
    props: ['to'],
    template: '<a href="#"><slot /></a>',
  }),
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
    uploadImage: vi.fn(),
    createReply: vi.fn(),
  },
}))

vi.mock('md-editor-v3', () => ({
  MdEditor: defineComponent({ name: 'MdEditor', props: ['modelValue'], template: '<div />' }),
  MdPreview: defineComponent({ name: 'MdPreview', props: ['modelValue', 'id', 'theme'], template: '<div />' }),
}))
vi.mock('md-editor-v3/lib/style.css', () => ({}))
vi.mock('md-editor-v3/lib/preview.css', () => ({}))

import FeedbackDetailView from '@/views/user/FeedbackDetailView.vue'

const AppLayoutStub = defineComponent({
  name: 'AppLayout',
  template: '<div><slot /></div>',
})

const StatusBadgeStub = defineComponent({
  name: 'StatusBadge',
  props: ['label', 'tone'],
  template: '<span>{{ label }}</span>',
})

const MarkdownPreviewStub = defineComponent({
  name: 'MarkdownPreview',
  props: ['content', 'previewId'],
  template: '<div />',
})

const MarkdownEditorFieldStub = defineComponent({
  name: 'MarkdownEditorField',
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<textarea />',
})

const MultiImageUploadStub = defineComponent({
  name: 'MultiImageUpload',
  template: '<div />',
})

function makeFeedbackDetail(overrides: Record<string, unknown> = {}) {
  return {
    id: 42,
    user_id: 1,
    category: 'bug',
    title: 'Test feedback',
    content: 'Some content',
    images: null, // <-- API returns null, not []
    status: 'pending',
    priority: 'normal',
    reply_count: 1,
    created_at: '2026-04-14T00:00:00Z',
    updated_at: '2026-04-14T00:00:00Z',
    replies: [
      {
        id: 1,
        feedback_id: 42,
        user_id: 1,
        role: 'admin',
        content: 'Reply text',
        images: null, // <-- Also null
        created_at: '2026-04-14T01:00:00Z',
        user: { id: 1, email: 'admin@test.com', username: 'admin' },
      },
    ],
    ...overrides,
  }
}

const globalStubs = {
  AppLayout: AppLayoutStub,
  RouterLink: defineComponent({ template: '<a><slot /></a>' }),
  StatusBadge: StatusBadgeStub,
  MarkdownPreview: MarkdownPreviewStub,
  MarkdownEditorField: MarkdownEditorFieldStub,
  MultiImageUpload: MultiImageUploadStub,
}

describe('User FeedbackDetailView', () => {
  beforeEach(() => {
    getByIdMock.mockReset()
  })

  it('renders without crash when detail.images is null', async () => {
    getByIdMock.mockResolvedValue(makeFeedbackDetail({ images: null }))

    const wrapper = mount(FeedbackDetailView, {
      global: {
        plugins: [createPinia()],
        stubs: globalStubs,
      },
    })
    await flushPromises()

    // Should not crash — no image grid rendered
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('h1').text()).toBe('feedback.detail.ticketTitle')
  })

  it('renders without crash when reply.images is null', async () => {
    getByIdMock.mockResolvedValue(
      makeFeedbackDetail({
        images: [],
        replies: [
          {
            id: 1,
            feedback_id: 42,
            user_id: 1,
            role: 'admin',
            content: 'Reply text',
            images: null,
            created_at: '2026-04-14T01:00:00Z',
            user: { id: 1, email: 'admin@test.com', username: 'admin' },
          },
        ],
      }),
    )

    const wrapper = mount(FeedbackDetailView, {
      global: {
        plugins: [createPinia()],
        stubs: globalStubs,
      },
    })
    await flushPromises()

    expect(wrapper.find('h1').text()).toBe('feedback.detail.ticketTitle')
    expect(wrapper.findAll('img')).toHaveLength(0)
  })

  it('renders images when detail.images is a non-empty array', async () => {
    getByIdMock.mockResolvedValue(
      makeFeedbackDetail({
        images: ['https://example.com/img1.png', 'https://example.com/img2.png'],
        replies: [],
      }),
    )

    const wrapper = mount(FeedbackDetailView, {
      global: {
        plugins: [createPinia()],
        stubs: globalStubs,
      },
    })
    await flushPromises()

    const images = wrapper.findAll('img')
    expect(images).toHaveLength(2)
    expect(images[0].attributes('src')).toBe('https://example.com/img1.png')
  })

  it('shows request and error details as preserved multiline context without the optional form label', async () => {
    getByIdMock.mockResolvedValue(makeFeedbackDetail({
      request_id: 'request-123\n报错信息：connection reset',
      replies: [],
    }))

    const wrapper = mount(FeedbackDetailView, {
      global: {
        plugins: [createPinia()],
        stubs: globalStubs,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('feedback.detail.requestContext')
    expect(wrapper.text()).not.toContain('feedback.form.requestId')
    expect(wrapper.get('[data-testid="feedback-request-context"]').text()).toContain('request-123\n报错信息：connection reset')
  })
})
