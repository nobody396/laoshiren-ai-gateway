import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import FinanceTransactionsView from '../FinanceTransactionsView.vue'
import { formatCurrency } from '@/utils/format'

const { list, summary, create, deleteTransaction, update, uploadReceipt, getReceiptUrl, showError, showSuccess } =
  vi.hoisted(() => {
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn()
    })

    return {
      list: vi.fn(),
      summary: vi.fn(),
      create: vi.fn(),
      deleteTransaction: vi.fn(),
      update: vi.fn(),
      uploadReceipt: vi.fn(),
      getReceiptUrl: vi.fn(),
      showError: vi.fn(),
      showSuccess: vi.fn()
    }
  })

vi.mock('@/api/admin', () => ({
  adminAPI: {
    financeTransactions: {
      list,
      summary,
      create,
      delete: deleteTransaction,
      update,
      uploadReceipt,
      getReceiptUrl
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string) => fallback ?? key
    })
  }
})

vi.mock('vue-chartjs', () => ({
  Doughnut: { props: ['data'], template: '<div class="doughnut-stub" />' }
}))

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
}
const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="dialog"><slot /><slot name="footer" /></div>'
}
const ConfirmDialogStub = {
  props: ['show'],
  emits: ['confirm', 'cancel'],
  template: '<div v-if="show" data-test="confirm-dialog"><button class="do-confirm" @click="$emit(\'confirm\')">confirm</button></div>'
}
const SelectStub = { props: ['modelValue', 'options'], template: '<select></select>' }
const StatCardStub = {
  props: ['title', 'value'],
  template: '<div class="stat-card">{{ title }}:{{ value }}</div>'
}

const mountView = () =>
  mount(FinanceTransactionsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: ConfirmDialogStub,
        Select: SelectStub,
        EmptyState: true,
        StatCard: StatCardStub,
        Icon: true
      }
    }
  })

const emptySummary = {
  range_from: '2026-07-01T00:00:00Z',
  range_to: '2026-08-01T00:00:00Z',
  total_income_fen: 150000,
  total_expense_fen: 50000,
  net_profit_fen: 100000,
  margin_percent: 66.67,
  by_category: [
    { type: 'income', category: 'sale_revenue', total_fen: 150000, tx_count: 2 },
    { type: 'expense', category: 'server_cost', total_fen: 50000, tx_count: 1 }
  ]
}

beforeEach(() => {
  list.mockReset()
  summary.mockReset()
  create.mockReset()
  deleteTransaction.mockReset()
  update.mockReset()
  uploadReceipt.mockReset()
  getReceiptUrl.mockReset()
  showError.mockReset()
  showSuccess.mockReset()

  list.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
  summary.mockResolvedValue(emptySummary)
})

describe('admin FinanceTransactionsView', () => {
  it('loads the ledger list and the month summary on mount', async () => {
    mountView()
    await flushPromises()

    expect(list).toHaveBeenCalledTimes(1)
    expect(summary).toHaveBeenCalledTimes(1)
  })

  it('renders the summary stat cards from the loaded data', async () => {
    const wrapper = mountView()
    await flushPromises()

    const cards = wrapper.findAll('.stat-card').map((c) => c.text())
    expect(cards.some((c) => c.includes(formatCurrency(1500, 'CNY')))).toBe(true) // income
    expect(cards.some((c) => c.includes(formatCurrency(500, 'CNY')))).toBe(true) // expense
    expect(cards.some((c) => c.includes(formatCurrency(1000, 'CNY')))).toBe(true) // net profit
    expect(cards.some((c) => c.includes('66.7%'))).toBe(true) // margin
  })

  it('converts the yuan input to fen and creates a transaction on save', async () => {
    create.mockResolvedValue({ id: 1 })
    const wrapper = mountView()
    await flushPromises()

    // Open the create dialog.
    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()

    const amountInput = wrapper.find('input[type="number"]')
    await amountInput.setValue('128.50')

    const form = wrapper.find('form#finance-transaction-form')
    await form.trigger('submit')
    await flushPromises()

    expect(create).toHaveBeenCalledTimes(1)
    const payload = create.mock.calls[0][0]
    expect(payload.amount_fen).toBe(12850)
    expect(payload.type).toBe('expense')
    expect(showSuccess).toHaveBeenCalled()
    // Reloads both the list and the summary after a successful save.
    expect(list).toHaveBeenCalledTimes(2)
    expect(summary).toHaveBeenCalledTimes(2)
  })

  it('rejects a zero/empty amount without calling the API', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()

    const form = wrapper.find('form#finance-transaction-form')
    await form.trigger('submit')
    await flushPromises()

    expect(create).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalled()
  })

  it('deletes a transaction after confirmation', async () => {
    list.mockResolvedValue({
      items: [
        {
          id: 42,
          type: 'expense',
          category: 'server_cost',
          amount_fen: 2000,
          occurred_at: '2026-07-10T00:00:00Z',
          source: 'manual',
          created_at: '2026-07-10T00:00:00Z',
          updated_at: '2026-07-10T00:00:00Z'
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    deleteTransaction.mockResolvedValue({ message: 'ok' })

    const wrapper = mountView()
    await flushPromises()

    // Find the delete (trash) button rendered in the actions cell for row 42.
    const trashButtons = wrapper.findAll('button[title]').filter((b) => b.attributes('title') === 'common.delete')
    expect(trashButtons.length).toBeGreaterThan(0)
    await trashButtons[0].trigger('click')
    await flushPromises()

    await wrapper.find('button.do-confirm').trigger('click')
    await flushPromises()

    expect(deleteTransaction).toHaveBeenCalledWith(42)
    expect(showSuccess).toHaveBeenCalled()
  })
})
