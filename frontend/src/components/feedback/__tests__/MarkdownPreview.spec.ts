import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('md-editor-v3', () => ({
  MdPreview: {
    name: 'MdPreview',
    props: ['modelValue', 'theme', 'id'],
    template: '<div data-testid="md-preview">{{ modelValue }}</div>',
  },
}))

vi.mock('md-editor-v3/lib/preview.css', () => ({}))

vi.mock('marked', () => ({
  marked: {
    parse: (input: string) => `<p>${input}</p>`,
  },
}))

vi.mock('dompurify', () => ({
  default: {
    sanitize: (html: string) => html,
  },
}))

import MarkdownPreview from '@/components/feedback/MarkdownPreview.vue'

describe('MarkdownPreview', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('renders MdPreview component when import succeeds', async () => {
    const wrapper = mount(MarkdownPreview, {
      props: { content: '# Hello World' },
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="md-preview"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="md-preview"]').text()).toContain('# Hello World')
  })

  it('passes content as modelValue to MdPreview', async () => {
    const wrapper = mount(MarkdownPreview, {
      props: { content: '**bold text**' },
    })
    await flushPromises()

    const preview = wrapper.findComponent({ name: 'MdPreview' })
    expect(preview.props('modelValue')).toBe('**bold text**')
  })

  it('passes previewId as id to MdPreview', async () => {
    const wrapper = mount(MarkdownPreview, {
      props: { content: 'text', previewId: 'preview-123' },
    })
    await flushPromises()

    const preview = wrapper.findComponent({ name: 'MdPreview' })
    expect(preview.props('id')).toBe('preview-123')
  })

  it('applies dark theme when document has dark class', async () => {
    document.documentElement.classList.add('dark')

    const wrapper = mount(MarkdownPreview, {
      props: { content: 'test' },
    })
    await flushPromises()

    const preview = wrapper.findComponent({ name: 'MdPreview' })
    expect(preview.props('theme')).toBe('dark')

    document.documentElement.classList.remove('dark')
  })

  it('applies light theme by default', async () => {
    document.documentElement.classList.remove('dark')

    const wrapper = mount(MarkdownPreview, {
      props: { content: 'test' },
    })
    await flushPromises()

    const preview = wrapper.findComponent({ name: 'MdPreview' })
    expect(preview.props('theme')).toBe('light')
  })

  it('does not render fallback div when MdPreview is available', async () => {
    const wrapper = mount(MarkdownPreview, {
      props: { content: 'test' },
    })
    await flushPromises()

    expect(wrapper.find('.prose').exists()).toBe(false)
    expect(wrapper.find('[data-testid="md-preview"]').exists()).toBe(true)
  })
})
