/**
 * Admin Settings API endpoints
 * Handles system settings management for administrators
 */

import { apiClient } from '../client'
import type { CardShopProduct, CustomMenuItem } from '@/types'

export interface DefaultSubscriptionSetting {
  group_id: number
  validity_days: number
}

/** 余额充值支付通道 */
export type TopupProvider = 'xunhu' | 'easypay'

/**
 * System settings interface
 */
export interface SystemSettings {
  // Registration settings
  registration_enabled: boolean
  email_verify_enabled: boolean
  registration_email_suffix_whitelist: string[]
  promo_code_enabled: boolean
  password_reset_enabled: boolean
  frontend_url: string
  invitation_code_enabled: boolean
  totp_enabled: boolean // TOTP 双因素认证
  totp_encryption_key_configured: boolean // TOTP 加密密钥是否已配置
  // Default settings
  default_balance: number
  default_concurrency: number
  default_subscriptions: DefaultSubscriptionSetting[]
  // OEM settings
  site_name: string
  site_logo: string
  site_subtitle: string
  api_base_url: string
  contact_info: string
  tech_support_qrcode: string
  after_sales_qrcode: string
  doc_url: string
  chatbot_url: string
  home_content: string
  landing_reports_enabled: boolean
  landing_pricing_pro_multiplier: number
  landing_pricing_max_multiplier: number
  landing_pricing_exchange_rate: number
  hide_ccs_import_button: boolean
  purchase_subscription_enabled: boolean
  purchase_subscription_url: string
  card_shop_enabled: boolean
  card_shop_products: CardShopProduct[]
  invoice_management_enabled: boolean
  feedback_management_enabled: boolean
  group_cache_hit_rate_enabled: boolean
  sora_client_enabled: boolean
  backend_mode_enabled: boolean
  balance_alert_enabled: boolean
  balance_alert_default_threshold: number
  custom_menu_items: CustomMenuItem[]
  // SMTP settings
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password_configured: boolean
  smtp_from_email: string
  smtp_from_name: string
  smtp_use_tls: boolean
  feedback_notify_email: string
  // Cloudflare Turnstile settings
  turnstile_enabled: boolean
  turnstile_site_key: string
  turnstile_secret_key_configured: boolean

  // LinuxDo Connect OAuth settings
  linuxdo_connect_enabled: boolean
  linuxdo_connect_client_id: string
  linuxdo_connect_client_secret_configured: boolean
  linuxdo_connect_redirect_url: string
  oidc_connect_enabled: boolean
  oidc_connect_provider_name: string
  oidc_connect_client_id: string
  oidc_connect_client_secret_configured: boolean
  oidc_connect_issuer_url: string
  oidc_connect_discovery_url: string
  oidc_connect_authorize_url: string
  oidc_connect_token_url: string
  oidc_connect_userinfo_url: string
  oidc_connect_jwks_url: string
  oidc_connect_scopes: string
  oidc_connect_redirect_url: string
  oidc_connect_frontend_redirect_url: string
  oidc_connect_token_auth_method: string
  oidc_connect_use_pkce: boolean
  oidc_connect_validate_id_token: boolean
  oidc_connect_allowed_signing_algs: string
  oidc_connect_clock_skew_seconds: number
  oidc_connect_require_email_verified: boolean
  oidc_connect_userinfo_email_path: string
  oidc_connect_userinfo_id_path: string
  oidc_connect_userinfo_username_path: string
  github_oauth_enabled: boolean
  github_oauth_client_id: string
  github_oauth_client_secret_configured: boolean
  github_oauth_redirect_url: string
  github_oauth_frontend_redirect_url: string

  // Model fallback configuration
  enable_model_fallback: boolean
  fallback_model_anthropic: string
  fallback_model_openai: string
  fallback_model_gemini: string
  fallback_model_antigravity: string

  // Identity patch configuration (Claude -> Gemini)
  enable_identity_patch: boolean
  identity_patch_prompt: string

  // Ops Monitoring (vNext)
  ops_monitoring_enabled: boolean
  ops_realtime_monitoring_enabled: boolean
  ops_query_mode_default: 'auto' | 'raw' | 'preagg' | string
  ops_metrics_interval_seconds: number

  // Claude Code version check
  min_claude_code_version: string
  account_quota_notify_enabled?: boolean
  web_search_emulation_enabled?: boolean
  enable_fingerprint_unification?: boolean
  enable_metadata_passthrough?: boolean
  enable_cch_signing?: boolean
  enable_anthropic_cache_ttl_1h_injection?: boolean

  // 分组隔离
  allow_ungrouped_key_scheduling: boolean

  // Stripe 支付设置
  stripe_enabled: boolean
  stripe_secret_key_configured: boolean
  stripe_webhook_secret_configured: boolean

  // 支付宝支付设置
  alipay_enabled: boolean
  alipay_app_id?: string
  alipay_private_key_configured?: boolean
  alipay_public_key_configured?: boolean
  alipay_notify_url?: string

  // 虎皮椒聚合支付设置
  xunhu_alipay_enabled: boolean
  xunhu_alipay_appid: string
  xunhu_alipay_key_configured: boolean
  xunhu_wechat_enabled: boolean
  xunhu_wechat_appid: string
  xunhu_wechat_key_configured: boolean
  xunhu_notify_url: string

  // 易支付（皮卡丘）聚合支付设置
  easypay_enabled: boolean
  easypay_pid: string
  easypay_api_base: string
  easypay_key_configured: boolean
  topup_alipay_provider: TopupProvider
  topup_wechat_provider: TopupProvider
}

export interface UpdateSettingsRequest {
  registration_enabled?: boolean
  email_verify_enabled?: boolean
  registration_email_suffix_whitelist?: string[]
  promo_code_enabled?: boolean
  password_reset_enabled?: boolean
  frontend_url?: string
  invitation_code_enabled?: boolean
  totp_enabled?: boolean // TOTP 双因素认证
  default_balance?: number
  default_concurrency?: number
  default_subscriptions?: DefaultSubscriptionSetting[]
  site_name?: string
  site_logo?: string
  site_subtitle?: string
  api_base_url?: string
  contact_info?: string
  tech_support_qrcode?: string
  after_sales_qrcode?: string
  doc_url?: string
  chatbot_url?: string
  home_content?: string
  landing_reports_enabled?: boolean
  landing_pricing_pro_multiplier?: number
  landing_pricing_max_multiplier?: number
  landing_pricing_exchange_rate?: number
  hide_ccs_import_button?: boolean
  purchase_subscription_enabled?: boolean
  purchase_subscription_url?: string
  card_shop_enabled?: boolean
  card_shop_products?: CardShopProduct[]
  invoice_management_enabled?: boolean
  feedback_management_enabled?: boolean
  group_cache_hit_rate_enabled?: boolean
  sora_client_enabled?: boolean
  backend_mode_enabled?: boolean
  balance_alert_enabled?: boolean
  balance_alert_default_threshold?: number
  custom_menu_items?: CustomMenuItem[]
  smtp_host?: string
  smtp_port?: number
  smtp_username?: string
  smtp_password?: string
  smtp_from_email?: string
  smtp_from_name?: string
  smtp_use_tls?: boolean
  feedback_notify_email?: string
  turnstile_enabled?: boolean
  turnstile_site_key?: string
  turnstile_secret_key?: string
  linuxdo_connect_enabled?: boolean
  linuxdo_connect_client_id?: string
  linuxdo_connect_client_secret?: string
  linuxdo_connect_redirect_url?: string
  oidc_connect_enabled?: boolean
  oidc_connect_provider_name?: string
  oidc_connect_client_id?: string
  oidc_connect_client_secret?: string
  oidc_connect_issuer_url?: string
  oidc_connect_discovery_url?: string
  oidc_connect_authorize_url?: string
  oidc_connect_token_url?: string
  oidc_connect_userinfo_url?: string
  oidc_connect_jwks_url?: string
  oidc_connect_scopes?: string
  oidc_connect_redirect_url?: string
  oidc_connect_frontend_redirect_url?: string
  oidc_connect_token_auth_method?: string
  oidc_connect_use_pkce?: boolean
  oidc_connect_validate_id_token?: boolean
  oidc_connect_allowed_signing_algs?: string
  oidc_connect_clock_skew_seconds?: number
  oidc_connect_require_email_verified?: boolean
  oidc_connect_userinfo_email_path?: string
  oidc_connect_userinfo_id_path?: string
  oidc_connect_userinfo_username_path?: string
  github_oauth_enabled?: boolean
  github_oauth_client_id?: string
  github_oauth_client_secret?: string
  github_oauth_redirect_url?: string
  github_oauth_frontend_redirect_url?: string
  enable_model_fallback?: boolean
  fallback_model_anthropic?: string
  fallback_model_openai?: string
  fallback_model_gemini?: string
  fallback_model_antigravity?: string
  enable_identity_patch?: boolean
  identity_patch_prompt?: string
  ops_monitoring_enabled?: boolean
  ops_realtime_monitoring_enabled?: boolean
  ops_query_mode_default?: 'auto' | 'raw' | 'preagg' | string
  ops_metrics_interval_seconds?: number
  min_claude_code_version?: string
  allow_ungrouped_key_scheduling?: boolean
  enable_fingerprint_unification?: boolean
  enable_metadata_passthrough?: boolean
  enable_cch_signing?: boolean
  enable_anthropic_cache_ttl_1h_injection?: boolean
  stripe_enabled?: boolean
  stripe_secret_key?: string
  stripe_webhook_secret?: string
  alipay_enabled?: boolean
  alipay_app_id?: string
  alipay_private_key?: string
  alipay_public_key?: string
  alipay_notify_url?: string

  // 虎皮椒聚合支付设置
  xunhu_alipay_enabled?: boolean
  xunhu_alipay_appid?: string
  xunhu_alipay_key?: string
  xunhu_wechat_enabled?: boolean
  xunhu_wechat_appid?: string
  xunhu_wechat_key?: string
  xunhu_notify_url?: string

  // 易支付（皮卡丘）聚合支付设置
  easypay_enabled?: boolean
  easypay_pid?: string
  easypay_api_base?: string
  easypay_key?: string
  topup_alipay_provider?: TopupProvider
  topup_wechat_provider?: TopupProvider
}

export interface OpenAIFastPolicyRule {
  service_tier: 'all' | 'priority' | 'flex'
  action: 'pass' | 'filter' | 'block'
  scope: 'all' | 'oauth' | 'apikey' | 'bedrock'
  error_message?: string
  model_whitelist?: string[]
  fallback_action?: 'pass' | 'filter' | 'block'
  fallback_error_message?: string
}

export interface OpenAIFastPolicySettings {
  rules: OpenAIFastPolicyRule[]
}

/**
 * Get all system settings
 * @returns System settings
 */
export async function getSettings(): Promise<SystemSettings> {
  const { data } = await apiClient.get<SystemSettings>('/admin/settings')
  return data
}

/**
 * Update system settings
 * @param settings - Partial settings to update
 * @returns Updated settings
 */
export async function updateSettings(settings: UpdateSettingsRequest): Promise<SystemSettings> {
  const { data } = await apiClient.put<SystemSettings>('/admin/settings', settings)
  return data
}

export async function getOpenAIFastPolicySettings(): Promise<OpenAIFastPolicySettings> {
  const { data } = await apiClient.get<OpenAIFastPolicySettings>('/admin/settings/openai-fast-policy')
  return data
}

export async function updateOpenAIFastPolicySettings(
  settings: OpenAIFastPolicySettings
): Promise<OpenAIFastPolicySettings> {
  const { data } = await apiClient.put<OpenAIFastPolicySettings>(
    '/admin/settings/openai-fast-policy',
    settings
  )
  return data
}

/**
 * Test SMTP connection request
 */
export interface TestSmtpRequest {
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password: string
  smtp_use_tls: boolean
}

/**
 * Test SMTP connection with provided config
 * @param config - SMTP configuration to test
 * @returns Test result message
 */
export async function testSmtpConnection(config: TestSmtpRequest): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/admin/settings/test-smtp', config)
  return data
}

/**
 * Send test email request
 */
export interface SendTestEmailRequest {
  email: string
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password: string
  smtp_from_email: string
  smtp_from_name: string
  smtp_use_tls: boolean
}

/**
 * Send test email with provided SMTP config
 * @param request - Email address and SMTP config
 * @returns Test result message
 */
export async function sendTestEmail(request: SendTestEmailRequest): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    '/admin/settings/send-test-email',
    request
  )
  return data
}

/**
 * Admin API Key status response
 */
export interface AdminApiKeyStatus {
  exists: boolean
  masked_key: string
}

/**
 * Get admin API key status
 * @returns Status indicating if key exists and masked version
 */
export async function getAdminApiKey(): Promise<AdminApiKeyStatus> {
  const { data } = await apiClient.get<AdminApiKeyStatus>('/admin/settings/admin-api-key')
  return data
}

/**
 * Regenerate admin API key
 * @returns The new full API key (only shown once)
 */
export async function regenerateAdminApiKey(): Promise<{ key: string }> {
  const { data } = await apiClient.post<{ key: string }>('/admin/settings/admin-api-key/regenerate')
  return data
}

/**
 * Delete admin API key
 * @returns Success message
 */
export async function deleteAdminApiKey(): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>('/admin/settings/admin-api-key')
  return data
}

/**
 * Stream timeout settings interface
 */
export interface StreamTimeoutSettings {
  enabled: boolean
  action: 'temp_unsched' | 'error' | 'none'
  temp_unsched_minutes: number
  threshold_count: number
  threshold_window_minutes: number
}

/**
 * Get stream timeout settings
 * @returns Stream timeout settings
 */
export async function getStreamTimeoutSettings(): Promise<StreamTimeoutSettings> {
  const { data } = await apiClient.get<StreamTimeoutSettings>('/admin/settings/stream-timeout')
  return data
}

/**
 * Update stream timeout settings
 * @param settings - Stream timeout settings to update
 * @returns Updated settings
 */
export async function updateStreamTimeoutSettings(
  settings: StreamTimeoutSettings
): Promise<StreamTimeoutSettings> {
  const { data } = await apiClient.put<StreamTimeoutSettings>(
    '/admin/settings/stream-timeout',
    settings
  )
  return data
}

// ==================== Rectifier Settings ====================

/**
 * Rectifier settings interface
 */
export interface RectifierSettings {
  enabled: boolean
  thinking_signature_enabled: boolean
  thinking_budget_enabled: boolean
}

/**
 * Get rectifier settings
 * @returns Rectifier settings
 */
export async function getRectifierSettings(): Promise<RectifierSettings> {
  const { data } = await apiClient.get<RectifierSettings>('/admin/settings/rectifier')
  return data
}

/**
 * Update rectifier settings
 * @param settings - Rectifier settings to update
 * @returns Updated settings
 */
export async function updateRectifierSettings(
  settings: RectifierSettings
): Promise<RectifierSettings> {
  const { data } = await apiClient.put<RectifierSettings>(
    '/admin/settings/rectifier',
    settings
  )
  return data
}

// ==================== Beta Policy Settings ====================

/**
 * Beta policy rule interface
 */
export interface BetaPolicyRule {
  beta_token: string
  action: 'pass' | 'filter' | 'block'
  scope: 'all' | 'oauth' | 'apikey' | 'bedrock'
  error_message?: string
}

/**
 * Beta policy settings interface
 */
export interface BetaPolicySettings {
  rules: BetaPolicyRule[]
}

/**
 * Get beta policy settings
 * @returns Beta policy settings
 */
export async function getBetaPolicySettings(): Promise<BetaPolicySettings> {
  const { data } = await apiClient.get<BetaPolicySettings>('/admin/settings/beta-policy')
  return data
}

/**
 * Update beta policy settings
 * @param settings - Beta policy settings to update
 * @returns Updated settings
 */
export async function updateBetaPolicySettings(
  settings: BetaPolicySettings
): Promise<BetaPolicySettings> {
  const { data } = await apiClient.put<BetaPolicySettings>(
    '/admin/settings/beta-policy',
    settings
  )
  return data
}

export interface WebSearchProviderConfig {
  api_key?: string
  api_key_configured?: boolean
  quota_limit?: number | null
  quota_used?: number
  subscribed_at?: string
  proxy_id?: number | null
}

export interface WebSearchEmulationConfig {
  enabled: boolean
  providers: WebSearchProviderConfig[]
}

export interface WebSearchTestResult {
  provider: string
  results: unknown[]
}

export async function getWebSearchEmulationConfig(): Promise<WebSearchEmulationConfig> {
  const { data } = await apiClient.get<WebSearchEmulationConfig>(
    '/admin/settings/web-search-emulation'
  )
  return data
}

export async function updateWebSearchEmulationConfig(
  config: WebSearchEmulationConfig
): Promise<WebSearchEmulationConfig> {
  const { data } = await apiClient.put<WebSearchEmulationConfig>(
    '/admin/settings/web-search-emulation',
    config
  )
  return data
}

export async function testWebSearchEmulation(query: string): Promise<WebSearchTestResult> {
  const { data } = await apiClient.post<WebSearchTestResult>(
    '/admin/settings/web-search-emulation/test',
    { query }
  )
  return data
}

export async function resetWebSearchUsage(payload: {
  provider_index?: number
} = {}): Promise<void> {
  await apiClient.post('/admin/settings/web-search-emulation/reset-usage', payload)
}

export const settingsAPI = {
  getSettings,
  updateSettings,
  testSmtpConnection,
  sendTestEmail,
  getAdminApiKey,
  regenerateAdminApiKey,
  deleteAdminApiKey,
  getStreamTimeoutSettings,
  updateStreamTimeoutSettings,
  getRectifierSettings,
  updateRectifierSettings,
  getBetaPolicySettings,
  updateBetaPolicySettings,
  getOpenAIFastPolicySettings,
  updateOpenAIFastPolicySettings,
  getWebSearchEmulationConfig,
  updateWebSearchEmulationConfig,
  testWebSearchEmulation,
  resetWebSearchUsage
}

export default settingsAPI
