import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { createPinia } from 'pinia'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import FeedbackListView from '@/views/user/FeedbackListView.vue'

const listMock = vi.fn()

vi.mock('@/api/feedbacks', () => ({
  default: {
    list: (...args: unknown[]) => listMock(...args),
  },
}))

vi.mock('vue-router', () => ({
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

const AppLayoutStub = defineComponent({
  name: 'AppLayout',
  template: '<div><slot /></div>',
})

const TablePageLayoutStub = defineComponent({
  name: 'TablePageLayout',
  template: '<div><slot name="filters" /><slot name="actions" /><slot name="table" /><slot name="pagination" /></div>',
})

const DataTableStub = defineComponent({
  name: 'DataTable',
  props: ['data'],
  template: '<div><slot name="empty" v-if="!data.length" /><div v-else data-testid="rows">{{ data.length }}</div></div>',
})

describe('FeedbackListView', () => {
  beforeEach(() => {
    listMock.mockReset()
  })

  it('loads feedback list and refetches when status filter changes', async () => {
    listMock.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mount(FeedbackListView, {
      global: {
        plugins: [
          createPinia(),
        ],
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: true,
          EmptyState: true,
          StatusBadge: true,
          RouterLink: defineComponent({
            name: 'RouterLink',
            props: ['to'],
            template: '<a href="#"><slot /></a>',
          }),
        },
      },
    })

    await flushPromises()
    expect(listMock).toHaveBeenNthCalledWith(1, { page: 1, pageSize: 20, status: undefined })

    const pendingButton = wrapper.findAll('button').find((button) => button.text() === 'feedback.status.pending')
    expect(pendingButton).toBeTruthy()
    await pendingButton!.trigger('click')
    await flushPromises()

    expect(listMock).toHaveBeenNthCalledWith(2, { page: 1, pageSize: 20, status: 'pending' })
  })
})
