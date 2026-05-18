import { describe, it, expect, beforeEach, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import OpsSettingsDialog from '../OpsSettingsDialog.vue'

const mockAPI = vi.hoisted(() => ({
  getAlertRuntimeSettings: vi.fn(),
  getEmailNotificationConfig: vi.fn(),
  getWebhookNotificationConfig: vi.fn(),
  getAdvancedSettings: vi.fn(),
  getMetricThresholds: vi.fn(),
  updateAlertRuntimeSettings: vi.fn(),
  updateEmailNotificationConfig: vi.fn(),
  updateWebhookNotificationConfig: vi.fn(),
  updateAdvancedSettings: vi.fn(),
  updateMetricThresholds: vi.fn(),
  testWebhookNotification: vi.fn(),
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: mockAPI,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: vi.fn(),
    showError: vi.fn(),
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

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: { type: Boolean, default: false },
  },
  template: '<div v-if="show" class="dialog-stub"><slot /><slot name="footer" /></div>',
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: [String, Number],
      default: '',
    },
    options: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['update:modelValue'],
  template: '<select class="select-stub" />',
})

const ToggleStub = defineComponent({
  name: 'Toggle',
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['update:modelValue'],
  template: '<input class="toggle-stub" type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />',
})

function mockSettings() {
  mockAPI.getAlertRuntimeSettings.mockResolvedValue({
    evaluation_interval_seconds: 60,
    distributed_lock: { enabled: true, key: 'ops:alert:evaluator:leader', ttl_seconds: 30 },
    silencing: { enabled: false, global_until_rfc3339: '', global_reason: '', entries: [] },
    thresholds: {},
  })
  mockAPI.getEmailNotificationConfig.mockResolvedValue({
    alert: {
      enabled: true,
      recipients: ['admin@example.com'],
      min_severity: '',
      rate_limit_per_hour: 0,
      batching_window_seconds: 0,
      include_resolved_alerts: false,
    },
    report: {
      enabled: false,
      recipients: [],
      daily_summary_enabled: false,
      daily_summary_schedule: '0 9 * * *',
      weekly_summary_enabled: false,
      weekly_summary_schedule: '0 9 * * 1',
      error_digest_enabled: false,
      error_digest_schedule: '0 9 * * *',
      error_digest_min_count: 10,
      account_health_enabled: false,
      account_health_schedule: '0 9 * * *',
      account_health_error_rate_threshold: 10,
    },
  })
  mockAPI.getWebhookNotificationConfig.mockResolvedValue({
    feishu: {
      enabled: true,
      name: '飞书告警群',
      webhook_url: '',
      webhook_url_configured: true,
      secret: '',
      secret_configured: true,
      app_id: '',
      app_id_configured: false,
      app_secret: '',
      app_secret_configured: false,
      chat_id: '',
      min_severity: 'warning',
      rate_limit_per_hour: 20,
    },
    telegram: {
      enabled: true,
      name: 'Telegram 告警群',
      bot_token: '',
      bot_token_configured: true,
      chat_id: '-100123456',
      min_severity: 'critical',
      rate_limit_per_hour: 10,
    },
  })
  mockAPI.getAdvancedSettings.mockResolvedValue({
    data_retention: {
      cleanup_enabled: false,
      cleanup_schedule: '0 2 * * *',
      error_log_retention_days: 30,
      minute_metrics_retention_days: 7,
      hourly_metrics_retention_days: 90,
    },
    aggregation: { aggregation_enabled: false },
    ignore_count_tokens_errors: false,
    ignore_context_canceled: false,
    ignore_no_available_accounts: false,
    ignore_invalid_api_key_errors: false,
    ignore_insufficient_balance_errors: false,
    display_openai_token_stats: false,
    display_alert_events: true,
    auto_refresh_enabled: false,
    auto_refresh_interval_seconds: 30,
  })
  mockAPI.getMetricThresholds.mockResolvedValue({})
  mockAPI.updateAlertRuntimeSettings.mockResolvedValue({})
  mockAPI.updateEmailNotificationConfig.mockResolvedValue({})
  mockAPI.updateWebhookNotificationConfig.mockImplementation(async (config) => config)
  mockAPI.updateAdvancedSettings.mockResolvedValue({})
  mockAPI.updateMetricThresholds.mockResolvedValue({})
  mockAPI.testWebhookNotification.mockResolvedValue({ channel: 'feishu', sent: true })
}

describe('OpsSettingsDialog webhook notifications', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockSettings()
  })

  it('加载飞书和 Telegram 群通知配置，并支持测试发送与保存', async () => {
    const wrapper = mount(OpsSettingsDialog, {
      props: { show: false },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          Toggle: ToggleStub,
        },
      },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(mockAPI.getWebhookNotificationConfig).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('admin.ops.webhook.title')
    expect(wrapper.text()).toContain('admin.ops.webhook.feishuTitle')
    expect(wrapper.text()).toContain('admin.ops.webhook.telegramTitle')

    const testButtons = wrapper.findAll('button').filter((button) => button.text() === 'admin.ops.webhook.sendTest')
    expect(testButtons.length).toBe(2)
    await testButtons[0].trigger('click')
    await flushPromises()
    expect(mockAPI.testWebhookNotification).toHaveBeenCalledWith('feishu')

    const saveButton = wrapper.findAll('button').find((button) => button.text() === 'common.save')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(mockAPI.updateWebhookNotificationConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        feishu: expect.objectContaining({
          enabled: true,
          webhook_url_configured: true,
          app_id_configured: false,
          app_secret_configured: false,
        }),
        telegram: expect.objectContaining({
          enabled: true,
          bot_token_configured: true,
          chat_id: '-100123456',
        }),
      })
    )
  })
})
