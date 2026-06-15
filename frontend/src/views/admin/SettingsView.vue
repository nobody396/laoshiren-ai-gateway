<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <!-- Settings Form -->
      <form v-else @submit.prevent="saveSettings" class="space-y-6" novalidate>
        <!-- Tab Navigation -->
        <div class="sticky top-0 z-10 overflow-x-auto settings-tabs-scroll">
          <nav class="settings-tabs">
            <button
              v-for="tab in settingsTabs"
              :key="tab.key"
              type="button"
              :class="['settings-tab', activeTab === tab.key && 'settings-tab-active']"
              @click="activeTab = tab.key"
            >
              <span class="settings-tab-icon">
                <Icon :name="tab.icon" size="sm" />
              </span>
              <span>{{ t(`admin.settings.tabs.${tab.key}`) }}</span>
            </button>
          </nav>
        </div>

        <!-- Tab: Security — Admin API Key -->
        <div v-show="activeTab === 'security'" class="space-y-6">
        <!-- Admin API Key Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.adminApiKey.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.adminApiKey.description') }}
            </p>
          </div>
          <div class="space-y-4 p-6">
            <!-- Security Warning -->
            <div
              class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20"
            >
              <div class="flex items-start">
                <Icon
                  name="exclamationTriangle"
                  size="md"
                  class="mt-0.5 flex-shrink-0 text-amber-500"
                />
                <p class="ml-3 text-sm text-amber-700 dark:text-amber-300">
                  {{ t('admin.settings.adminApiKey.securityWarning') }}
                </p>
              </div>
            </div>

            <!-- Loading State -->
            <div v-if="adminApiKeyLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <!-- No Key Configured -->
            <div v-else-if="!adminApiKeyExists" class="flex items-center justify-between">
              <span class="text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.adminApiKey.notConfigured') }}
              </span>
              <button
                type="button"
                @click="createAdminApiKey"
                :disabled="adminApiKeyOperating"
                class="btn btn-primary btn-sm"
              >
                <svg
                  v-if="adminApiKeyOperating"
                  class="mr-1 h-4 w-4 animate-spin"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                {{
                  adminApiKeyOperating
                    ? t('admin.settings.adminApiKey.creating')
                    : t('admin.settings.adminApiKey.create')
                }}
              </button>
            </div>

            <!-- Key Exists -->
            <div v-else class="space-y-4">
              <div class="flex items-center justify-between">
                <div>
                  <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.adminApiKey.currentKey') }}
                  </label>
                  <code
                    class="rounded bg-gray-100 px-2 py-1 font-mono text-sm text-gray-900 dark:bg-dark-700 dark:text-gray-100"
                  >
                    {{ adminApiKeyMasked }}
                  </code>
                </div>
                <div class="flex gap-2">
                  <button
                    type="button"
                    @click="regenerateAdminApiKey"
                    :disabled="adminApiKeyOperating"
                    class="btn btn-secondary btn-sm"
                  >
                    {{
                      adminApiKeyOperating
                        ? t('admin.settings.adminApiKey.regenerating')
                        : t('admin.settings.adminApiKey.regenerate')
                    }}
                  </button>
                  <button
                    type="button"
                    @click="deleteAdminApiKey"
                    :disabled="adminApiKeyOperating"
                    class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                  >
                    {{ t('admin.settings.adminApiKey.delete') }}
                  </button>
                </div>
              </div>

              <!-- Newly Generated Key Display -->
              <div
                v-if="newAdminApiKey"
                class="space-y-3 rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20"
              >
                <p class="text-sm font-medium text-green-700 dark:text-green-300">
                  {{ t('admin.settings.adminApiKey.keyWarning') }}
                </p>
                <div class="flex items-center gap-2">
                  <code
                    class="flex-1 select-all break-all rounded border border-green-300 bg-white px-3 py-2 font-mono text-sm dark:border-green-700 dark:bg-dark-800"
                  >
                    {{ newAdminApiKey }}
                  </code>
                  <button
                    type="button"
                    @click="copyNewKey"
                    class="btn btn-primary btn-sm flex-shrink-0"
                  >
                    {{ t('admin.settings.adminApiKey.copyKey') }}
                  </button>
                </div>
                <p class="text-xs text-green-600 dark:text-green-400">
                  {{ t('admin.settings.adminApiKey.usage') }}
                </p>
              </div>
            </div>
          </div>
        </div>
        </div><!-- /Tab: Security — Admin API Key -->

        <!-- Tab: Gateway — Stream Timeout -->
        <div v-show="activeTab === 'gateway'" class="space-y-6">
        <!-- Stream Timeout Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.streamTimeout.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.streamTimeout.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Loading State -->
            <div v-if="streamTimeoutLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <template v-else>
              <!-- Enable Stream Timeout -->
              <div class="flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t('admin.settings.streamTimeout.enabled')
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.enabledHint') }}
                  </p>
                </div>
                <Toggle v-model="streamTimeoutForm.enabled" />
              </div>

              <!-- Settings - Only show when enabled -->
              <div
                v-if="streamTimeoutForm.enabled"
                class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <!-- Action -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.streamTimeout.action') }}
                  </label>
                  <select v-model="streamTimeoutForm.action" class="input w-64">
                    <option value="temp_unsched">{{ t('admin.settings.streamTimeout.actionTempUnsched') }}</option>
                    <option value="error">{{ t('admin.settings.streamTimeout.actionError') }}</option>
                    <option value="none">{{ t('admin.settings.streamTimeout.actionNone') }}</option>
                  </select>
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.actionHint') }}
                  </p>
                </div>

                <!-- Temp Unsched Minutes (only show when action is temp_unsched) -->
                <div v-if="streamTimeoutForm.action === 'temp_unsched'">
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.streamTimeout.tempUnschedMinutes') }}
                  </label>
                  <input
                    v-model.number="streamTimeoutForm.temp_unsched_minutes"
                    type="number"
                    min="1"
                    max="60"
                    class="input w-32"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.tempUnschedMinutesHint') }}
                  </p>
                </div>

                <!-- Threshold Count -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.streamTimeout.thresholdCount') }}
                  </label>
                  <input
                    v-model.number="streamTimeoutForm.threshold_count"
                    type="number"
                    min="1"
                    max="10"
                    class="input w-32"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.thresholdCountHint') }}
                  </p>
                </div>

                <!-- Threshold Window Minutes -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.streamTimeout.thresholdWindowMinutes') }}
                  </label>
                  <input
                    v-model.number="streamTimeoutForm.threshold_window_minutes"
                    type="number"
                    min="1"
                    max="60"
                    class="input w-32"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.thresholdWindowMinutesHint') }}
                  </p>
                </div>
              </div>

              <!-- Save Button -->
              <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
                <button
                  type="button"
                  @click="saveStreamTimeoutSettings"
                  :disabled="streamTimeoutSaving"
                  class="btn btn-primary btn-sm"
                >
                  <svg
                    v-if="streamTimeoutSaving"
                    class="mr-1 h-4 w-4 animate-spin"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    ></circle>
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                  {{ streamTimeoutSaving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </template>
          </div>
        </div>

        <!-- Request Rectifier Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.rectifier.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.rectifier.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Loading State -->
            <div v-if="rectifierLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <template v-else>
              <!-- Master Toggle -->
              <div class="flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t('admin.settings.rectifier.enabled')
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.rectifier.enabledHint') }}
                  </p>
                </div>
                <Toggle v-model="rectifierForm.enabled" />
              </div>

              <!-- Sub-toggles (only show when master is enabled) -->
              <div
                v-if="rectifierForm.enabled"
                class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <!-- Thinking Signature Rectifier -->
                <div class="flex items-center justify-between">
                  <div>
                    <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{
                      t('admin.settings.rectifier.thinkingSignature')
                    }}</label>
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.settings.rectifier.thinkingSignatureHint') }}
                    </p>
                  </div>
                  <Toggle v-model="rectifierForm.thinking_signature_enabled" />
                </div>

                <!-- Thinking Budget Rectifier -->
                <div class="flex items-center justify-between">
                  <div>
                    <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{
                      t('admin.settings.rectifier.thinkingBudget')
                    }}</label>
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.settings.rectifier.thinkingBudgetHint') }}
                    </p>
                  </div>
                  <Toggle v-model="rectifierForm.thinking_budget_enabled" />
                </div>
              </div>

              <!-- Save Button -->
              <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
                <button
                  type="button"
                  @click="saveRectifierSettings"
                  :disabled="rectifierSaving"
                  class="btn btn-primary btn-sm"
                >
                  <svg
                    v-if="rectifierSaving"
                    class="mr-1 h-4 w-4 animate-spin"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    ></circle>
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                  {{ rectifierSaving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </template>
          </div>
        </div>
        <!-- Beta Policy Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.betaPolicy.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.betaPolicy.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Loading State -->
            <div v-if="betaPolicyLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <template v-else>
              <!-- Rule Cards -->
              <div
                v-for="rule in betaPolicyForm.rules"
                :key="rule.beta_token"
                class="rounded-lg border border-gray-200 p-4 dark:border-dark-600"
              >
                <div class="mb-3 flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ getBetaDisplayName(rule.beta_token) }}
                  </span>
                  <span class="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">
                    {{ rule.beta_token }}
                  </span>
                </div>

                <div class="grid grid-cols-2 gap-4">
                  <!-- Action -->
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.settings.betaPolicy.action') }}
                    </label>
                    <Select
                      :modelValue="rule.action"
                      @update:modelValue="rule.action = $event as any"
                      :options="betaPolicyActionOptions"
                    />
                  </div>

                  <!-- Scope -->
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.settings.betaPolicy.scope') }}
                    </label>
                    <Select
                      :modelValue="rule.scope"
                      @update:modelValue="rule.scope = $event as any"
                      :options="betaPolicyScopeOptions"
                    />
                  </div>
                </div>

                <!-- Error Message (only when action=block) -->
                <div v-if="rule.action === 'block'" class="mt-3">
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.betaPolicy.errorMessage') }}
                  </label>
                  <input
                    v-model="rule.error_message"
                    type="text"
                    class="input"
                    :placeholder="t('admin.settings.betaPolicy.errorMessagePlaceholder')"
                  />
                  <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    {{ t('admin.settings.betaPolicy.errorMessageHint') }}
                  </p>
                </div>
              </div>

              <!-- Save Button -->
              <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
                <button
                  type="button"
                  @click="saveBetaPolicySettings"
                  :disabled="betaPolicySaving"
                  class="btn btn-primary btn-sm"
                >
                  <svg
                    v-if="betaPolicySaving"
                    class="mr-1 h-4 w-4 animate-spin"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    ></circle>
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                  {{ betaPolicySaving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </template>
          </div>
        </div>

        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              OpenAI Fast/Flex Policy
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              控制 OpenAI 请求中的 service_tier 字段，支持透传、过滤或拦截 fast/flex。
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div v-if="openAIFastPolicyLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <template v-else>
              <div
                v-for="(rule, index) in openAIFastPolicyForm.rules"
                :key="`openai-fast-rule-${index}`"
                class="rounded-lg border border-gray-200 p-4 dark:border-dark-600"
              >
                <div class="mb-3 flex items-center justify-between gap-3">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">
                    规则 {{ index + 1 }}
                  </span>
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                    @click="removeOpenAIFastPolicyRule(index)"
                  >
                    删除
                  </button>
                </div>

                <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      service_tier
                    </label>
                    <Select
                      :modelValue="rule.service_tier"
                      @update:modelValue="rule.service_tier = $event as any"
                      :options="openAIFastPolicyTierOptions"
                    />
                  </div>

                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      动作
                    </label>
                    <Select
                      :modelValue="rule.action"
                      @update:modelValue="rule.action = $event as any"
                      :options="betaPolicyActionOptions"
                    />
                  </div>

                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      范围
                    </label>
                    <Select
                      :modelValue="rule.scope"
                      @update:modelValue="rule.scope = $event as any"
                      :options="betaPolicyScopeOptions"
                    />
                  </div>
                </div>

                <div class="mt-3">
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    模型白名单
                  </label>
                  <input
                    v-model="rule.model_whitelist_text"
                    type="text"
                    class="input"
                    placeholder="留空表示全部模型，多个模式用逗号分隔，例如 gpt-5.5,gpt-5.5*"
                  />
                </div>

                <div v-if="rule.action === 'block'" class="mt-3">
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    错误消息
                  </label>
                  <input v-model="rule.error_message" type="text" class="input" />
                </div>
              </div>

              <div class="flex flex-wrap justify-between gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="addOpenAIFastPolicyRule"
                >
                  添加规则
                </button>
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="openAIFastPolicySaving"
                  @click="saveOpenAIFastPolicySettings"
                >
                  {{ openAIFastPolicySaving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </template>
          </div>
        </div>

        </div><!-- /Tab: Gateway -->

        <!-- Tab: Security — Registration, Turnstile, LinuxDo -->
        <div v-show="activeTab === 'security'" class="space-y-6">
        <!-- Registration Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.registration.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.registration.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Enable Registration -->
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.enableRegistration')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.enableRegistrationHint') }}
                </p>
              </div>
              <Toggle v-model="form.registration_enabled" />
            </div>

            <!-- Email Verification -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.emailVerification')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.emailVerificationHint') }}
                </p>
              </div>
              <Toggle v-model="form.email_verify_enabled" />
            </div>

            <!-- Email Suffix Whitelist -->
            <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
              <label class="font-medium text-gray-900 dark:text-white">{{
                t('admin.settings.registration.emailSuffixWhitelist')
              }}</label>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.registration.emailSuffixWhitelistHint') }}
              </p>
              <div
                class="mt-3 rounded-lg border border-gray-300 bg-white p-2 dark:border-dark-500 dark:bg-dark-700"
              >
                <div class="flex flex-wrap items-center gap-2">
                  <span
                    v-for="suffix in registrationEmailSuffixWhitelistTags"
                    :key="suffix"
                    class="inline-flex items-center gap-1 rounded bg-gray-100 px-2 py-1 text-xs font-mono text-gray-700 dark:bg-dark-600 dark:text-gray-200"
                  >
                    <span class="text-gray-400 dark:text-gray-500">@</span>
                    <span>{{ suffix }}</span>
                    <button
                      type="button"
                      class="rounded-full text-gray-500 hover:bg-gray-200 hover:text-gray-700 dark:text-gray-300 dark:hover:bg-dark-500 dark:hover:text-white"
                      @click="removeRegistrationEmailSuffixWhitelistTag(suffix)"
                    >
                      <Icon name="x" size="xs" class="h-3.5 w-3.5" :stroke-width="2" />
                    </button>
                  </span>

                  <div
                    class="flex min-w-[220px] flex-1 items-center gap-1 rounded border border-transparent px-2 py-1 focus-within:border-primary-300 dark:focus-within:border-primary-700"
                  >
                    <span class="font-mono text-sm text-gray-400 dark:text-gray-500">@</span>
                    <input
                      v-model="registrationEmailSuffixWhitelistDraft"
                      type="text"
                      class="w-full bg-transparent text-sm font-mono text-gray-900 outline-none placeholder:text-gray-400 dark:text-white dark:placeholder:text-gray-500"
                      :placeholder="t('admin.settings.registration.emailSuffixWhitelistPlaceholder')"
                      @input="handleRegistrationEmailSuffixWhitelistDraftInput"
                      @keydown="handleRegistrationEmailSuffixWhitelistDraftKeydown"
                      @blur="commitRegistrationEmailSuffixWhitelistDraft"
                      @paste="handleRegistrationEmailSuffixWhitelistPaste"
                    />
                  </div>
                </div>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.registration.emailSuffixWhitelistInputHint') }}
              </p>
            </div>

            <!-- Promo Code -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.promoCode')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.promoCodeHint') }}
                </p>
              </div>
              <Toggle v-model="form.promo_code_enabled" />
            </div>

            <!-- Invitation Code -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.invitationCode')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.invitationCodeHint') }}
                </p>
              </div>
              <Toggle v-model="form.invitation_code_enabled" />
            </div>
            <!-- Password Reset - Only show when email verification is enabled -->
            <div
              v-if="form.email_verify_enabled"
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.passwordReset')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.passwordResetHint') }}
                </p>
              </div>
              <Toggle v-model="form.password_reset_enabled" />
            </div>
            <!-- Frontend URL - Only show when password reset is enabled -->
            <div
              v-if="form.email_verify_enabled && form.password_reset_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.registration.frontendUrl') }}
              </label>
              <input
                v-model="form.frontend_url"
                type="url"
                class="input"
                :placeholder="t('admin.settings.registration.frontendUrlPlaceholder')"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.registration.frontendUrlHint') }}
              </p>
            </div>

            <!-- TOTP 2FA -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.totp')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.totpHint') }}
                </p>
                <!-- Warning when encryption key not configured -->
                <p
                  v-if="!form.totp_encryption_key_configured"
                  class="mt-2 text-sm text-amber-600 dark:text-amber-400"
                >
                  {{ t('admin.settings.registration.totpKeyNotConfigured') }}
                </p>
              </div>
              <Toggle
                v-model="form.totp_enabled"
                :disabled="!form.totp_encryption_key_configured"
              />
            </div>
          </div>
        </div>

        <!-- Cloudflare Turnstile Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.turnstile.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.turnstile.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Enable Turnstile -->
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.turnstile.enableTurnstile')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.turnstile.enableTurnstileHint') }}
                </p>
              </div>
              <Toggle v-model="form.turnstile_enabled" />
            </div>

            <!-- Turnstile Keys - Only show when enabled -->
            <div
              v-if="form.turnstile_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div class="grid grid-cols-1 gap-6">
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.turnstile.siteKey') }}
                  </label>
                  <input
                    v-model="form.turnstile_site_key"
                    type="text"
                    class="input font-mono text-sm"
                    placeholder="0x4AAAAAAA..."
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.turnstile.siteKeyHint') }}
                    <a
                      href="https://dash.cloudflare.com/"
                      target="_blank"
                      class="text-primary-600 hover:text-primary-500"
                      >{{ t('admin.settings.turnstile.cloudflareDashboard') }}</a
                    >
                  </p>
                </div>
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.turnstile.secretKey') }}
                  </label>
                  <input
                    v-model="form.turnstile_secret_key"
                    type="password"
                    class="input font-mono text-sm"
                    placeholder="0x4AAAAAAA..."
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.turnstile_secret_key_configured
                        ? t('admin.settings.turnstile.secretKeyConfiguredHint')
                        : t('admin.settings.turnstile.secretKeyHint')
                    }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- LinuxDo Connect OAuth 登录 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.linuxdo.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.linuxdo.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.linuxdo.enable')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.linuxdo.enableHint') }}
                </p>
              </div>
              <Toggle v-model="form.linuxdo_connect_enabled" />
            </div>

            <div
              v-if="form.linuxdo_connect_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div class="grid grid-cols-1 gap-6">
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.linuxdo.clientId') }}
                  </label>
                  <input
                    v-model="form.linuxdo_connect_client_id"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.linuxdo.clientIdPlaceholder')"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.linuxdo.clientIdHint') }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.linuxdo.clientSecret') }}
                  </label>
                  <input
                    v-model="form.linuxdo_connect_client_secret"
                    type="password"
                    class="input font-mono text-sm"
                    :placeholder="
                      form.linuxdo_connect_client_secret_configured
                        ? t('admin.settings.linuxdo.clientSecretConfiguredPlaceholder')
                        : t('admin.settings.linuxdo.clientSecretPlaceholder')
                    "
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.linuxdo_connect_client_secret_configured
                        ? t('admin.settings.linuxdo.clientSecretConfiguredHint')
                        : t('admin.settings.linuxdo.clientSecretHint')
                    }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.linuxdo.redirectUrl') }}
                  </label>
                  <input
                    v-model="form.linuxdo_connect_redirect_url"
                    type="url"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.linuxdo.redirectUrlPlaceholder')"
                  />
                  <div class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm w-fit"
                      @click="setAndCopyLinuxdoRedirectUrl"
                    >
                      {{ t('admin.settings.linuxdo.quickSetCopy') }}
                    </button>
                    <code
                      v-if="linuxdoRedirectUrlSuggestion"
                      class="select-all break-all rounded bg-gray-50 px-2 py-1 font-mono text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                    >
                      {{ linuxdoRedirectUrlSuggestion }}
                    </code>
                  </div>
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.linuxdo.redirectUrlHint') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Google OAuth 登录 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.google.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.google.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">
                  {{ t('admin.settings.google.enable') }}
                </label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.google.enableHint') }}
                </p>
              </div>
              <Toggle v-model="form.oidc_connect_enabled" />
            </div>

            <div
              v-if="form.oidc_connect_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div class="grid grid-cols-1 gap-6">
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.google.providerName') }}
                  </label>
                  <input
                    v-model="form.oidc_connect_provider_name"
                    type="text"
                    class="input"
                    :placeholder="t('admin.settings.google.providerNamePlaceholder')"
                  />
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.google.clientId') }}
                  </label>
                  <input
                    v-model="form.oidc_connect_client_id"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.google.clientIdPlaceholder')"
                  />
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.google.clientSecret') }}
                  </label>
                  <input
                    v-model="form.oidc_connect_client_secret"
                    type="password"
                    class="input font-mono text-sm"
                    :placeholder="
                      form.oidc_connect_client_secret_configured
                        ? t('admin.settings.google.clientSecretConfiguredPlaceholder')
                        : t('admin.settings.google.clientSecretPlaceholder')
                    "
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.oidc_connect_client_secret_configured
                        ? t('admin.settings.google.clientSecretConfiguredHint')
                        : t('admin.settings.google.clientSecretHint')
                    }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.google.redirectUrl') }}
                  </label>
                  <input
                    v-model="form.oidc_connect_redirect_url"
                    type="url"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.google.redirectUrlPlaceholder')"
                  />
                  <div class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm w-fit"
                      @click="setAndCopyGoogleRedirectUrl"
                    >
                      {{ t('admin.settings.google.quickSetCopy') }}
                    </button>
                    <code
                      v-if="googleRedirectUrlSuggestion"
                      class="select-all break-all rounded bg-gray-50 px-2 py-1 font-mono text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                    >
                      {{ googleRedirectUrlSuggestion }}
                    </code>
                  </div>
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.google.redirectUrlHint') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- GitHub OAuth 登录 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.github.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.github.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">
                  {{ t('admin.settings.github.enable') }}
                </label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.github.enableHint') }}
                </p>
              </div>
              <Toggle v-model="form.github_oauth_enabled" />
            </div>

            <div
              v-if="form.github_oauth_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div class="grid grid-cols-1 gap-6">
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.github.clientId') }}
                  </label>
                  <input
                    v-model="form.github_oauth_client_id"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.github.clientIdPlaceholder')"
                  />
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.github.clientSecret') }}
                  </label>
                  <input
                    v-model="form.github_oauth_client_secret"
                    type="password"
                    class="input font-mono text-sm"
                    :placeholder="
                      form.github_oauth_client_secret_configured
                        ? t('admin.settings.github.clientSecretConfiguredPlaceholder')
                        : t('admin.settings.github.clientSecretPlaceholder')
                    "
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.github_oauth_client_secret_configured
                        ? t('admin.settings.github.clientSecretConfiguredHint')
                        : t('admin.settings.github.clientSecretHint')
                    }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.github.redirectUrl') }}
                  </label>
                  <input
                    v-model="form.github_oauth_redirect_url"
                    type="url"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.github.redirectUrlPlaceholder')"
                  />
                  <div class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm w-fit"
                      @click="setAndCopyGithubRedirectUrl"
                    >
                      {{ t('admin.settings.github.quickSetCopy') }}
                    </button>
                    <code
                      v-if="githubRedirectUrlSuggestion"
                      class="select-all break-all rounded bg-gray-50 px-2 py-1 font-mono text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                    >
                      {{ githubRedirectUrlSuggestion }}
                    </code>
                  </div>
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.github.redirectUrlHint') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
        </div><!-- /Tab: Security — Registration, Turnstile, LinuxDo -->

        <!-- Tab: Users -->
        <div v-show="activeTab === 'users'" class="space-y-6">
        <!-- Default Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.defaults.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.defaults.description') }}
            </p>
          </div>
          <div class="space-y-6 p-6">
            <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.defaults.defaultBalance') }}
                </label>
                <input
                  v-model.number="form.default_balance"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input"
                  placeholder="0.00"
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.defaults.defaultBalanceHint') }}
                </p>
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.defaults.defaultConcurrency') }}
                </label>
                <input
                  v-model.number="form.default_concurrency"
                  type="number"
                  min="1"
                  class="input"
                  placeholder="1"
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.defaults.defaultConcurrencyHint') }}
                </p>
              </div>
            </div>

            <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
              <div class="mb-3 flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">
                    {{ t('admin.settings.defaults.defaultSubscriptions') }}
                  </label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.defaults.defaultSubscriptionsHint') }}
                  </p>
                </div>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="addDefaultSubscription"
                  :disabled="subscriptionGroups.length === 0"
                >
                  {{ t('admin.settings.defaults.addDefaultSubscription') }}
                </button>
              </div>

              <div
                v-if="form.default_subscriptions.length === 0"
                class="rounded border border-dashed border-gray-300 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
              >
                {{ t('admin.settings.defaults.defaultSubscriptionsEmpty') }}
              </div>

              <div v-else class="space-y-3">
                <div
                  v-for="(item, index) in form.default_subscriptions"
                  :key="`default-sub-${index}`"
                  class="grid grid-cols-1 gap-3 rounded border border-gray-200 p-3 md:grid-cols-[1fr_160px_auto] dark:border-dark-600"
                >
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.settings.defaults.subscriptionGroup') }}
                    </label>
                    <Select
                      v-model="item.group_id"
                      class="default-sub-group-select"
                      :options="defaultSubscriptionGroupOptions"
                      :placeholder="t('admin.settings.defaults.subscriptionGroup')"
                    >
                      <template #selected="{ option }">
                        <GroupBadge
                          v-if="option"
                          :name="(option as unknown as DefaultSubscriptionGroupOption).label"
                          :platform="(option as unknown as DefaultSubscriptionGroupOption).platform"
                          :subscription-type="(option as unknown as DefaultSubscriptionGroupOption).subscriptionType"
                          :rate-multiplier="(option as unknown as DefaultSubscriptionGroupOption).rate"
                        />
                        <span v-else class="text-gray-400">
                          {{ t('admin.settings.defaults.subscriptionGroup') }}
                        </span>
                      </template>
                      <template #option="{ option, selected }">
                        <GroupOptionItem
                          :name="(option as unknown as DefaultSubscriptionGroupOption).label"
                          :platform="(option as unknown as DefaultSubscriptionGroupOption).platform"
                          :subscription-type="(option as unknown as DefaultSubscriptionGroupOption).subscriptionType"
                          :rate-multiplier="(option as unknown as DefaultSubscriptionGroupOption).rate"
                          :description="(option as unknown as DefaultSubscriptionGroupOption).description"
                          :selected="selected"
                        />
                      </template>
                    </Select>
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                      {{ t('admin.settings.defaults.subscriptionValidityDays') }}
                    </label>
                    <input
                      v-model.number="item.validity_days"
                      type="number"
                      min="1"
                      max="36500"
                      class="input h-[42px]"
                    />
                  </div>
                  <div class="flex items-end">
                    <button
                      type="button"
                      class="btn btn-secondary default-sub-delete-btn w-full text-red-600 hover:text-red-700 dark:text-red-400"
                      @click="removeDefaultSubscription(index)"
                    >
                      {{ t('common.delete') }}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.balanceAlert.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.balanceAlert.description') }}
            </p>
          </div>
          <div class="space-y-6 p-6">
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">
                  {{ t('admin.settings.balanceAlert.enabled') }}
                </label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.balanceAlert.enabledHint') }}
                </p>
              </div>
              <Toggle v-model="form.balance_alert_enabled" />
            </div>
            <div v-if="form.balance_alert_enabled">
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.balanceAlert.defaultThreshold') }}
              </label>
              <input
                v-model.number="form.balance_alert_default_threshold"
                type="number"
                step="0.01"
                min="0.10"
                class="input w-48"
                placeholder="5.00"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.balanceAlert.defaultThresholdHint') }}
              </p>
            </div>
          </div>
        </div>
        </div><!-- /Tab: Users -->

        <!-- Tab: Gateway — Claude Code, Scheduling -->
        <div v-show="activeTab === 'gateway'" class="space-y-6">
        <!-- Claude Code Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.claudeCode.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.claudeCode.description') }}
            </p>
          </div>
          <div class="p-6">
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.claudeCode.minVersion') }}
              </label>
              <input
                v-model="form.min_claude_code_version"
                type="text"
                class="input max-w-xs font-mono text-sm"
                :placeholder="t('admin.settings.claudeCode.minVersionPlaceholder')"
                pattern="\d+\.\d+\.\d+"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.claudeCode.minVersionHint') }}
              </p>
            </div>
          </div>
        </div>

        <!-- Gateway Scheduling Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.scheduling.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.scheduling.description') }}
            </p>
          </div>
          <div class="p-6">
            <div class="flex items-center justify-between">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.scheduling.allowUngroupedKey') }}
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.scheduling.allowUngroupedKeyHint') }}
                </p>
              </div>
              <label class="toggle">
                <input v-model="form.allow_ungrouped_key_scheduling" type="checkbox" />
                <span class="toggle-slider"></span>
              </label>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">网关请求处理行为</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              控制 OAuth 请求处理时的头部、metadata 和 Anthropic 缓存 TTL 处理。
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div class="flex items-center justify-between gap-4">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  统一 OAuth 指纹头
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  对 OAuth 账号使用稳定的浏览器指纹，降低上游会话异常概率。
                </p>
              </div>
              <label class="toggle">
                <input v-model="form.enable_fingerprint_unification" type="checkbox" />
                <span class="toggle-slider"></span>
              </label>
            </div>

            <div class="flex items-center justify-between gap-4">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  透传客户端 metadata
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  开启后保留客户端原始 metadata；默认关闭以使用网关生成的稳定标识。
                </p>
              </div>
              <label class="toggle">
                <input v-model="form.enable_metadata_passthrough" type="checkbox" />
                <span class="toggle-slider"></span>
              </label>
            </div>

            <div class="flex items-center justify-between gap-4">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  签名 billing header CCH
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  对 billing header 中的 cch 值添加签名，避免请求链路中被篡改。
                </p>
              </div>
              <label class="toggle">
                <input v-model="form.enable_cch_signing" type="checkbox" />
                <span class="toggle-slider"></span>
              </label>
            </div>

            <div class="flex items-center justify-between gap-4">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Anthropic 缓存 TTL 注入 1h
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  仅对 Anthropic OAuth/SetupToken 生效，把已有 ephemeral cache_control 的 ttl 改为 1h。
                </p>
              </div>
              <label class="toggle">
                <input v-model="form.enable_anthropic_cache_ttl_1h_injection" type="checkbox" />
                <span class="toggle-slider"></span>
              </label>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Web Search 模拟</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              为不原生支持搜索的 Anthropic API Key 账号提供 web search 模拟能力。
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div v-if="webSearchLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <template v-else>
              <div class="flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">启用 Web Search 模拟</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    关闭后渠道和账号上的 Web Search 模拟配置都不会生效。
                  </p>
                </div>
                <Toggle v-model="webSearchConfig.enabled" />
              </div>

              <div
                v-if="webSearchConfig.enabled"
                class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <div
                  v-for="(provider, index) in webSearchConfig.providers"
                  :key="`websearch-provider-${index}`"
                  class="rounded-lg border border-gray-200 p-4 dark:border-dark-600"
                >
                  <div class="mb-3 flex items-center justify-between">
                    <span class="text-sm font-medium text-gray-900 dark:text-white">
                      Provider {{ index + 1 }}
                    </span>
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                      @click="removeWebSearchProvider(index)"
                    >
                      删除
                    </button>
                  </div>

                  <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                    <div class="md:col-span-2">
                      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                        API Key
                      </label>
                      <input
                        v-model="provider.api_key"
                        type="password"
                        class="input font-mono text-sm"
                        :placeholder="provider.api_key_configured ? '••••••••' : '请输入 Provider API Key'"
                      />
                    </div>

                    <div>
                      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                        月额度上限
                      </label>
                      <input
                        v-model.number="provider.quota_limit"
                        type="number"
                        min="0"
                        class="input"
                        placeholder="0 表示不限"
                      />
                    </div>

                    <div>
                      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                        订阅开始时间
                      </label>
                      <input
                        v-model="provider.subscribed_at"
                        type="date"
                        class="input"
                      />
                    </div>

                    <div>
                      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                        代理
                      </label>
                      <select v-model="provider.proxy_id" class="input">
                        <option :value="null">不使用代理</option>
                        <option v-for="proxy in webSearchProxies" :key="proxy.id" :value="proxy.id">
                          {{ proxy.name }}
                        </option>
                      </select>
                    </div>

                    <div class="flex items-end text-xs text-gray-500 dark:text-gray-400">
                      已使用次数：{{ provider.quota_used || 0 }}
                    </div>
                  </div>
                </div>

                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="addWebSearchProvider"
                >
                  添加 Provider
                </button>

                <div class="grid grid-cols-1 gap-4 md:grid-cols-[1fr_auto]">
                  <div>
                    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      测试查询
                    </label>
                    <input
                      v-model="webSearchTestQuery"
                      type="text"
                      class="input"
                      placeholder="OpenAI latest release"
                    />
                  </div>
                  <div class="flex items-end">
                    <button
                      type="button"
                      class="btn btn-secondary"
                      :disabled="webSearchTestLoading"
                      @click="testWebSearchProvider"
                    >
                      {{ webSearchTestLoading ? '测试中' : '测试 Provider' }}
                    </button>
                  </div>
                </div>

                <div
                  v-if="webSearchTestResult"
                  class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm dark:border-dark-600 dark:bg-dark-800"
                >
                  Provider: {{ webSearchTestResult.provider }}，结果数：{{ webSearchTestResult.results.length }}
                </div>
              </div>

              <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="webSearchSaving"
                  @click="saveWebSearchConfig"
                >
                  {{ webSearchSaving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </template>
          </div>
        </div>
        </div><!-- /Tab: Gateway — Claude Code, Scheduling -->

        <!-- Tab: General -->
        <div v-show="activeTab === 'general'" class="space-y-6">
        <!-- Site Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.site.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.site.description') }}
            </p>
          </div>
          <div class="space-y-6 p-6">
            <!-- Backend Mode -->
            <div
              class="flex items-center justify-between rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20"
            >
              <div>
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('admin.settings.site.backendMode') }}
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.site.backendModeDescription') }}
                </p>
              </div>
              <Toggle v-model="form.backend_mode_enabled" />
            </div>

            <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.site.siteName') }}
                </label>
                <input
                  v-model="form.site_name"
                  type="text"
                  class="input"
                  :placeholder="t('admin.settings.site.siteNamePlaceholder')"
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.site.siteNameHint') }}
                </p>
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.site.siteSubtitle') }}
                </label>
                <input
                  v-model="form.site_subtitle"
                  type="text"
                  class="input"
                  :placeholder="t('admin.settings.site.siteSubtitlePlaceholder')"
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.site.siteSubtitleHint') }}
                </p>
              </div>
            </div>

            <!-- API Base URL -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.site.apiBaseUrl') }}
              </label>
              <input
                v-model="form.api_base_url"
                type="text"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.site.apiBaseUrlPlaceholder')"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.apiBaseUrlHint') }}
              </p>
            </div>

            <!-- Contact Info -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.site.contactInfo') }}
              </label>
              <input
                v-model="form.contact_info"
                type="text"
                class="input"
                :placeholder="t('admin.settings.site.contactInfoPlaceholder')"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.contactInfoHint') }}
              </p>
            </div>

            <!-- 技术客服二维码（负责安装及环境配置问题） -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.site.techSupportQRCode') }}
              </label>
              <input
                v-model="form.tech_support_qrcode"
                type="url"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.site.techSupportQRCodePlaceholder')"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.techSupportQRCodeHint') }}
              </p>
            </div>

            <!-- 售后客服二维码（负责账户充值、推广等售后事宜） -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.site.afterSalesQRCode') }}
              </label>
              <input
                v-model="form.after_sales_qrcode"
                type="url"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.site.afterSalesQRCodePlaceholder')"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.afterSalesQRCodeHint') }}
              </p>
            </div>

            <!-- Doc URL -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.site.docUrl') }}
              </label>
              <input
                v-model="form.doc_url"
                type="url"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.site.docUrlPlaceholder')"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.docUrlHint') }}
              </p>
            </div>

            <!-- Chatbot URL -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                Chatbot 地址
              </label>
              <input
                v-model="form.chatbot_url"
                type="url"
                class="input font-mono text-sm"
                placeholder="https://chat.example.com"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                用户菜单「打开 Chatbot」会先生成一次性 SSO ticket，然后跳转到该地址的 /sso?ticket=...
              </p>
            </div>

            <!-- Site Logo Upload -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.site.siteLogo') }}
              </label>
              <ImageUpload
                v-model="form.site_logo"
                mode="image"
                :upload-label="t('admin.settings.site.uploadImage')"
                :remove-label="t('admin.settings.site.remove')"
                :hint="t('admin.settings.site.logoHint')"
                :max-size="300 * 1024"
              />
            </div>

            <!-- Home Content -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.site.homeContent') }}
              </label>
              <textarea
                v-model="form.home_content"
                rows="6"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.site.homeContentPlaceholder')"
              ></textarea>
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.homeContentHint') }}
              </p>
              <!-- iframe CSP Warning -->
              <p class="mt-2 text-xs text-amber-600 dark:text-amber-400">
                {{ t('admin.settings.site.homeContentIframeWarning') }}
              </p>
            </div>

            <!-- Landing Model Pricing Display -->
            <div class="rounded-lg border border-gray-100 p-4 dark:border-dark-700">
              <div class="mb-4">
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                  官网模型价格展示
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  仅影响 landing page 展示，不影响真实分组倍率、扣费和调度。
                </p>
              </div>

              <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
                <div>
                  <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Pro 展示倍率
                  </label>
                  <input
                    v-model.number="form.landing_pricing_pro_multiplier"
                    type="number"
                    min="0.01"
                    step="0.01"
                    class="input"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    首页按 ¥{{ formatCompactNumber(positiveNumberOrDefault(form.landing_pricing_pro_multiplier, 1.2)) }} = $1 展示，约 {{ landingPricingPreview.proDiscount }}。
                  </p>
                </div>

                <div>
                  <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Max 展示倍率
                  </label>
                  <input
                    v-model.number="form.landing_pricing_max_multiplier"
                    type="number"
                    min="0.01"
                    step="0.01"
                    class="input"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    首页按 ¥{{ formatCompactNumber(positiveNumberOrDefault(form.landing_pricing_max_multiplier, 4)) }} = $1 展示，约 {{ landingPricingPreview.maxDiscount }}。
                  </p>
                </div>

                <div>
                  <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    折扣参考汇率
                  </label>
                  <input
                    v-model.number="form.landing_pricing_exchange_rate"
                    type="number"
                    min="0.01"
                    step="0.01"
                    class="input"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    用于计算“几折”，例如 7 表示按 1 USD = ¥7 估算。
                  </p>
                </div>
              </div>
            </div>

            <!-- Hide CCS Import Button -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.site.hideCcsImportButton')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.site.hideCcsImportButtonHint') }}
                </p>
              </div>
              <Toggle v-model="form.hide_ccs_import_button" />
            </div>

          </div>
        </div>

        <!-- Purchase Subscription Page -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.purchase.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.purchase.description') }}
            </p>
          </div>
          <div class="space-y-6 p-6">
            <!-- Enable Toggle -->
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.purchase.enabled')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.purchase.enabledHint') }}
                </p>
              </div>
              <Toggle v-model="form.purchase_subscription_enabled" />
            </div>

            <!-- URL -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.purchase.url') }}
              </label>
              <input
                v-model="form.purchase_subscription_url"
                type="url"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.purchase.urlPlaceholder')"
              />
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.purchase.urlHint') }}
              </p>
              <p class="mt-2 text-xs text-amber-600 dark:text-amber-400">
                {{ t('admin.settings.purchase.iframeWarning') }}
              </p>
            </div>

            <!-- Integration Docs -->
            <div class="flex items-center gap-2 text-sm">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <a
                :href="paymentIntegrationDocUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-600 hover:underline dark:text-blue-400"
              >
                {{ t('admin.settings.purchase.integrationDoc') }}
              </a>
              <span class="text-gray-400 dark:text-gray-500">—</span>
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.purchase.integrationDocHint') }}
              </span>
            </div>

            <div class="border-t border-gray-100 pt-6 dark:border-dark-700">
              <div class="flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">链动小铺卡密商城</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    启用后，用户充值页会显示固定金额商品，并跳转到链动后台「自营商品」复制出来的真实商品独立链接。
                  </p>
                </div>
                <Toggle v-model="form.card_shop_enabled" />
              </div>

              <div v-if="form.card_shop_enabled" class="mt-5 space-y-4">
                <div
                  v-for="(product, index) in form.card_shop_products"
                  :key="product.id || index"
                  class="rounded-lg border border-gray-100 p-4 dark:border-dark-700"
                >
                  <div class="mb-4 flex items-center justify-between gap-3">
                    <div>
                      <p class="text-sm font-medium text-gray-900 dark:text-white">
                        商品 {{ index + 1 }}
                      </p>
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        建议和链动小铺自营商品面额一一对应。
                      </p>
                    </div>
                    <div class="flex items-center gap-3">
                      <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
                        <input v-model="product.enabled" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                        启用
                      </label>
                      <button
                        type="button"
                        class="btn btn-secondary btn-sm text-red-600 hover:text-red-700"
                        @click="removeCardShopProduct(index)"
                      >
                        删除
                      </button>
                    </div>
                  </div>
                  <div class="grid grid-cols-1 gap-4 md:grid-cols-[1fr_140px]">
                    <div>
                      <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">商品名称</label>
                      <input v-model="product.label" type="text" class="input" placeholder="¥20 余额卡" />
                    </div>
                    <div>
                      <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">金额（元）</label>
                      <input v-model.number="product.amount_cny" type="number" min="1" step="1" class="input" />
                    </div>
                  </div>
                  <div class="mt-4">
                    <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">链动商品链接</label>
                    <input v-model="product.url" type="url" class="input font-mono text-sm" placeholder="https://www.ldxp.cn/..." />
                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                      从链动小铺商家后台的「自营商品」列表复制该商品的独立链接，不能填写测试页或店铺首页。
                    </p>
                  </div>
                </div>

                <button type="button" class="btn btn-secondary btn-sm" @click="addCardShopProduct">
                  添加商品
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Stripe Payment Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.stripe.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.stripe.description') }}
            </p>
          </div>
          <div class="space-y-6 p-6">
            <!-- Enable Toggle -->
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.stripe.enabled')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.stripe.enabledHint') }}
                </p>
              </div>
              <Toggle v-model="form.stripe_enabled" />
            </div>

            <div
              v-if="form.stripe_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div class="grid grid-cols-1 gap-6">
                <!-- Secret Key -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.stripe.secretKey') }}
                  </label>
                  <input
                    v-model="form.stripe_secret_key"
                    type="password"
                    class="input font-mono text-sm"
                    placeholder="sk_live_..."
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.stripe_secret_key_configured
                        ? t('admin.settings.stripe.secretKeyConfiguredHint')
                        : t('admin.settings.stripe.secretKeyHint')
                    }}
                  </p>
                </div>
                <!-- Webhook Secret -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.stripe.webhookSecret') }}
                  </label>
                  <input
                    v-model="form.stripe_webhook_secret"
                    type="password"
                    class="input font-mono text-sm"
                    placeholder="whsec_..."
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.stripe_webhook_secret_configured
                        ? t('admin.settings.stripe.webhookSecretConfiguredHint')
                        : t('admin.settings.stripe.webhookSecretHint')
                    }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 虎皮椒聚合支付（余额充值） -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">虎皮椒充值</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              配置虎皮椒聚合支付，支持支付宝和微信余额充值（人民币 1:1 换算为美元余额）
            </p>
          </div>
          <div class="space-y-6 p-6">
            <!-- Notify URL (read-only) -->
            <div>
              <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">回调地址（复制到虎皮椒后台）</label>
              <div class="flex items-center gap-2">
                <input readonly :value="xunhuNotifyURLDisplay" class="input font-mono text-sm flex-1 bg-gray-50 dark:bg-dark-800 cursor-text select-all" />
                <button type="button" @click="copyNotifyURL" class="btn btn-secondary btn-sm whitespace-nowrap">{{ notifyURLCopied ? '已复制' : '复制' }}</button>
              </div>
            </div>

            <!-- Alipay section -->
            <div class="rounded-lg border border-gray-100 dark:border-dark-700 p-4 space-y-4">
              <div class="flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">支付宝渠道</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">虎皮椒支付宝收款</p>
                </div>
                <Toggle v-model="form.xunhu_alipay_enabled" />
              </div>
              <div v-if="form.xunhu_alipay_enabled" class="grid grid-cols-1 gap-4 border-t border-gray-100 dark:border-dark-700 pt-4">
                <div>
                  <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">AppID</label>
                  <input v-model="form.xunhu_alipay_appid" type="text" class="input font-mono text-sm" placeholder="20211119704" />
                </div>
                <div>
                  <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">密钥</label>
                  <input v-model="form.xunhu_alipay_key" type="password" class="input font-mono text-sm" placeholder="留空则保持不变" />
                  <p class="mt-1 text-xs text-gray-400">{{ form.xunhu_alipay_key_configured ? '密钥已配置，留空则保持原密钥' : '尚未配置密钥' }}</p>
                </div>
              </div>
            </div>

            <!-- WeChat section -->
            <div class="rounded-lg border border-gray-100 dark:border-dark-700 p-4 space-y-4">
              <div class="flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">微信支付渠道</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">虎皮椒微信收款</p>
                </div>
                <Toggle v-model="form.xunhu_wechat_enabled" />
              </div>
              <div v-if="form.xunhu_wechat_enabled" class="grid grid-cols-1 gap-4 border-t border-gray-100 dark:border-dark-700 pt-4">
                <div>
                  <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">AppID</label>
                  <input v-model="form.xunhu_wechat_appid" type="text" class="input font-mono text-sm" placeholder="20211119681" />
                </div>
                <div>
                  <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">密钥</label>
                  <input v-model="form.xunhu_wechat_key" type="password" class="input font-mono text-sm" placeholder="留空则保持不变" />
                  <p class="mt-1 text-xs text-gray-400">{{ form.xunhu_wechat_key_configured ? '密钥已配置，留空则保持原密钥' : '尚未配置密钥' }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Custom Menu Items -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.customMenu.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.customMenu.description') }}
            </p>
          </div>
          <div class="space-y-4 p-6">
            <!-- Existing menu items -->
            <div
              v-for="(item, index) in form.custom_menu_items"
              :key="item.id || index"
              class="rounded-lg border border-gray-200 p-4 dark:border-dark-600"
            >
              <div class="mb-3 flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.customMenu.itemLabel', { n: index + 1 }) }}
                </span>
                <div class="flex items-center gap-2">
                  <!-- Move up -->
                  <button
                    v-if="index > 0"
                    type="button"
                    class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                    :title="t('admin.settings.customMenu.moveUp')"
                    @click="moveMenuItem(index, -1)"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" /></svg>
                  </button>
                  <!-- Move down -->
                  <button
                    v-if="index < form.custom_menu_items.length - 1"
                    type="button"
                    class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                    :title="t('admin.settings.customMenu.moveDown')"
                    @click="moveMenuItem(index, 1)"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" /></svg>
                  </button>
                  <!-- Delete -->
                  <button
                    type="button"
                    class="rounded p-1 text-red-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                    :title="t('admin.settings.customMenu.remove')"
                    @click="removeMenuItem(index)"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                  </button>
                </div>
              </div>

              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <!-- Label -->
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.customMenu.name') }}
                  </label>
                  <input
                    v-model="item.label"
                    type="text"
                    class="input text-sm"
                    :placeholder="t('admin.settings.customMenu.namePlaceholder')"
                  />
                </div>

                <!-- Visibility -->
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.customMenu.visibility') }}
                  </label>
                  <select v-model="item.visibility" class="input text-sm">
                    <option value="user">{{ t('admin.settings.customMenu.visibilityUser') }}</option>
                    <option value="admin">{{ t('admin.settings.customMenu.visibilityAdmin') }}</option>
                  </select>
                </div>

                <!-- URL (full width) -->
                <div class="sm:col-span-2">
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.customMenu.url') }}
                  </label>
                  <input
                    v-model="item.url"
                    type="url"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.customMenu.urlPlaceholder')"
                  />
                </div>

                <!-- SVG Icon (full width) -->
                <div class="sm:col-span-2">
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.customMenu.iconSvg') }}
                  </label>
                  <ImageUpload
                    :model-value="item.icon_svg"
                    mode="svg"
                    size="sm"
                    :upload-label="t('admin.settings.customMenu.uploadSvg')"
                    :remove-label="t('admin.settings.customMenu.removeSvg')"
                    @update:model-value="(v: string) => item.icon_svg = v"
                  />
                </div>
              </div>
            </div>

            <!-- Add button -->
            <button
              type="button"
              class="flex w-full items-center justify-center gap-2 rounded-lg border-2 border-dashed border-gray-300 py-3 text-sm text-gray-500 transition-colors hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:text-gray-400 dark:hover:border-primary-500 dark:hover:text-primary-400"
              @click="addMenuItem"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" /></svg>
              {{ t('admin.settings.customMenu.add') }}
            </button>
          </div>
        </div>

        </div><!-- /Tab: General -->

        <!-- Tab: Email -->
        <div v-show="activeTab === 'email'" class="space-y-6">
        <!-- Email disabled hint - show when email_verify_enabled is off -->
        <div v-if="!form.email_verify_enabled" class="card">
          <div class="p-6">
            <div class="flex items-start gap-3">
              <Icon name="mail" size="md" class="mt-0.5 flex-shrink-0 text-gray-400 dark:text-gray-500" />
              <div>
                <h3 class="font-medium text-gray-900 dark:text-white">
                  {{ t('admin.settings.emailTabDisabledTitle') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.emailTabDisabledHint') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- SMTP Settings - Only show when email verification is enabled -->
        <div v-if="form.email_verify_enabled" class="card">
          <div
            class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700"
          >
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.settings.smtp.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.smtp.description') }}
              </p>
            </div>
            <button
              type="button"
              @click="testSmtpConnection"
              :disabled="testingSmtp"
              class="btn btn-secondary btn-sm"
            >
              <svg v-if="testingSmtp" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              {{
                testingSmtp
                  ? t('admin.settings.smtp.testing')
                  : t('admin.settings.smtp.testConnection')
              }}
            </button>
          </div>
          <div class="space-y-6 p-6">
            <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.smtp.host') }}
                </label>
                <input
                  v-model="form.smtp_host"
                  type="text"
                  class="input"
                  :placeholder="t('admin.settings.smtp.hostPlaceholder')"
                />
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.smtp.port') }}
                </label>
                <input
                  v-model.number="form.smtp_port"
                  type="number"
                  min="1"
                  max="65535"
                  class="input"
                  :placeholder="t('admin.settings.smtp.portPlaceholder')"
                />
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.smtp.username') }}
                </label>
                <input
                  v-model="form.smtp_username"
                  type="text"
                  class="input"
                  :placeholder="t('admin.settings.smtp.usernamePlaceholder')"
                />
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.smtp.password') }}
                </label>
                <input
                  v-model="form.smtp_password"
                  type="password"
                  class="input"
                  :placeholder="
                    form.smtp_password_configured
                      ? t('admin.settings.smtp.passwordConfiguredPlaceholder')
                      : t('admin.settings.smtp.passwordPlaceholder')
                  "
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{
                    form.smtp_password_configured
                      ? t('admin.settings.smtp.passwordConfiguredHint')
                      : t('admin.settings.smtp.passwordHint')
                  }}
                </p>
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.smtp.fromEmail') }}
                </label>
                <input
                  v-model="form.smtp_from_email"
                  type="email"
                  class="input"
                  :placeholder="t('admin.settings.smtp.fromEmailPlaceholder')"
                />
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.smtp.fromName') }}
                </label>
                <input
                  v-model="form.smtp_from_name"
                  type="text"
                  class="input"
                  :placeholder="t('admin.settings.smtp.fromNamePlaceholder')"
                />
              </div>
              <div class="md:col-span-2">
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('feedback.admin.notifyEmail') }}
                </label>
                <input
                  v-model="form.feedback_notify_email"
                  type="email"
                  class="input"
                  :placeholder="t('feedback.admin.notifyEmailPlaceholder')"
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('feedback.admin.notifyEmailHint') }}
                </p>
              </div>
            </div>

            <!-- Use TLS Toggle -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.smtp.useTls')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.smtp.useTlsHint') }}
                </p>
              </div>
              <Toggle v-model="form.smtp_use_tls" />
            </div>

          </div>
        </div>

        <!-- Send Test Email - Only show when email verification is enabled -->
        <div v-if="form.email_verify_enabled" class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.testEmail.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.testEmail.description') }}
            </p>
          </div>
          <div class="p-6">
            <div class="flex items-end gap-4">
              <div class="flex-1">
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.testEmail.recipientEmail') }}
                </label>
                <input
                  v-model="testEmailAddress"
                  type="email"
                  class="input"
                  :placeholder="t('admin.settings.testEmail.recipientEmailPlaceholder')"
                />
              </div>
              <button
                type="button"
                @click="sendTestEmail"
                :disabled="sendingTestEmail || !testEmailAddress"
                class="btn btn-secondary"
              >
                <svg
                  v-if="sendingTestEmail"
                  class="h-4 w-4 animate-spin"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                {{
                  sendingTestEmail
                    ? t('admin.settings.testEmail.sending')
                    : t('admin.settings.testEmail.sendTestEmail')
                }}
              </button>
            </div>
          </div>
        </div>
        </div><!-- /Tab: Email -->

        <!-- Tab: Backup -->
        <div v-show="activeTab === 'backup'">
          <BackupSettings />
        </div>

        <!-- Save Button -->
        <div v-show="activeTab !== 'backup'" class="flex justify-end">
          <button type="submit" :disabled="saving" class="btn btn-primary">
            <svg v-if="saving" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ saving ? t('admin.settings.saving') : t('admin.settings.saveSettings') }}
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type {
  SystemSettings,
  UpdateSettingsRequest,
  DefaultSubscriptionSetting,
  WebSearchProviderConfig,
  WebSearchTestResult,
  OpenAIFastPolicyRule
} from '@/api/admin/settings'
import type { AdminGroup, CardShopProduct, Proxy } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import Toggle from '@/components/common/Toggle.vue'
import ImageUpload from '@/components/common/ImageUpload.vue'
import BackupSettings from '@/views/admin/BackupView.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import {
  isRegistrationEmailSuffixDomainValid,
  normalizeRegistrationEmailSuffixDomain,
  normalizeRegistrationEmailSuffixDomains,
  parseRegistrationEmailSuffixWhitelistInput
} from '@/utils/registrationEmailPolicy'

const { t } = useI18n()
const appStore = useAppStore()
const adminSettingsStore = useAdminSettingsStore()

type SettingsTab = 'general' | 'security' | 'users' | 'gateway' | 'email' | 'backup'
const activeTab = ref<SettingsTab>('general')
const settingsTabs = [
  { key: 'general'  as SettingsTab, icon: 'home'   as const },
  { key: 'security' as SettingsTab, icon: 'shield' as const },
  { key: 'users'    as SettingsTab, icon: 'user'   as const },
  { key: 'gateway'  as SettingsTab, icon: 'server' as const },
  { key: 'email'    as SettingsTab, icon: 'mail'   as const },
  { key: 'backup'   as SettingsTab, icon: 'database' as const },
]
const { copyToClipboard } = useClipboard()
const paymentIntegrationDocUrl = computed(() => appStore.docUrl || '/docs')

function positiveNumberOrDefault(value: number | null | undefined, fallback: number): number {
  return Number.isFinite(value) && Number(value) > 0 ? Number(value) : fallback
}

function formatCompactNumber(value: number, maximumFractionDigits = 2): string {
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits }).format(value)
}

function formatLandingDiscount(multiplier: number, exchangeRate: number): string {
  return `${formatCompactNumber((multiplier / exchangeRate) * 10, 1)}折`
}

const loading = ref(true)
const saving = ref(false)
const testingSmtp = ref(false)
const sendingTestEmail = ref(false)
const testEmailAddress = ref('')
const registrationEmailSuffixWhitelistTags = ref<string[]>([])
const registrationEmailSuffixWhitelistDraft = ref('')

// Admin API Key 状态
const adminApiKeyLoading = ref(true)
const adminApiKeyExists = ref(false)
const adminApiKeyMasked = ref('')
const adminApiKeyOperating = ref(false)
const newAdminApiKey = ref('')
const subscriptionGroups = ref<AdminGroup[]>([])

// 虎皮椒回调地址
const legacyXunhuNotifyURL = 'https://your-domain.example/api/v1/topup/notify'
function getBrowserOrigin(): string {
  if (typeof window === 'undefined' || !window.location?.origin) {
    return ''
  }
  return window.location.origin
}
function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, '')
}
function buildDefaultXunhuNotifyURL(frontendURL: string): string {
  const baseURL = trimTrailingSlash(frontendURL || getBrowserOrigin())
  return baseURL ? `${baseURL}/api/v1/topup/notify` : ''
}
function normalizeXunhuNotifyURL(value: string, frontendURL: string): string {
  const trimmed = value.trim()
  const generated = buildDefaultXunhuNotifyURL(frontendURL)
  if (!trimmed || trimmed === legacyXunhuNotifyURL) {
    return generated
  }
  return trimmed
}
const defaultXunhuNotifyURL = computed(() => buildDefaultXunhuNotifyURL(form.frontend_url))
const xunhuNotifyURLDisplay = computed(() =>
  normalizeXunhuNotifyURL(form.xunhu_notify_url, form.frontend_url) || defaultXunhuNotifyURL.value
)
const notifyURLCopied = ref(false)
async function copyNotifyURL() {
  const success = await copyToClipboard(xunhuNotifyURLDisplay.value)
  if (success) {
    notifyURLCopied.value = true
    setTimeout(() => { notifyURLCopied.value = false }, 2000)
  }
}

// Stream Timeout 状态
const streamTimeoutLoading = ref(true)
const streamTimeoutSaving = ref(false)
const streamTimeoutForm = reactive({
  enabled: true,
  action: 'temp_unsched' as 'temp_unsched' | 'error' | 'none',
  temp_unsched_minutes: 5,
  threshold_count: 3,
  threshold_window_minutes: 10
})

// Rectifier 状态
const rectifierLoading = ref(true)
const rectifierSaving = ref(false)
const rectifierForm = reactive({
  enabled: true,
  thinking_signature_enabled: true,
  thinking_budget_enabled: true
})

// Beta Policy 状态
const betaPolicyLoading = ref(true)
const betaPolicySaving = ref(false)
const betaPolicyForm = reactive({
  rules: [] as Array<{
    beta_token: string
    action: 'pass' | 'filter' | 'block'
    scope: 'all' | 'oauth' | 'apikey' | 'bedrock'
    error_message?: string
  }>
})
type OpenAIFastPolicyRuleForm = OpenAIFastPolicyRule & {
  model_whitelist_text: string
}

const openAIFastPolicyLoading = ref(true)
const openAIFastPolicySaving = ref(false)
const openAIFastPolicyForm = reactive({
  rules: [] as OpenAIFastPolicyRuleForm[]
})
const webSearchLoading = ref(true)
const webSearchSaving = ref(false)
const webSearchTestLoading = ref(false)
const webSearchTestQuery = ref('')
const webSearchTestResult = ref<WebSearchTestResult | null>(null)
const webSearchProxies = ref<Proxy[]>([])
const webSearchConfig = reactive<{
  enabled: boolean
  providers: WebSearchProviderConfig[]
}>({
  enabled: false,
  providers: []
})

interface DefaultSubscriptionGroupOption {
  value: number
  label: string
  description: string | null
  platform: AdminGroup['platform']
  subscriptionType: AdminGroup['subscription_type']
  rate: number
  [key: string]: unknown
}

type SettingsForm = SystemSettings & {
  smtp_password: string
  turnstile_secret_key: string
  linuxdo_connect_client_secret: string
  oidc_connect_client_secret: string
  github_oauth_client_secret: string
  stripe_secret_key: string
  stripe_webhook_secret: string
  xunhu_alipay_key: string
  xunhu_wechat_key: string
}

const form = reactive<SettingsForm>({
  registration_enabled: true,
  email_verify_enabled: false,
  registration_email_suffix_whitelist: [],
  promo_code_enabled: true,
  invitation_code_enabled: false,
  password_reset_enabled: false,
  totp_enabled: false,
  totp_encryption_key_configured: false,
  default_balance: 0,
  default_concurrency: 1,
  default_subscriptions: [],
  site_name: '老实人AI',
  site_logo: '',
  site_subtitle: 'Subscription to API Conversion Platform',
  api_base_url: '',
  contact_info: '',
  tech_support_qrcode: '',
  after_sales_qrcode: '',
  doc_url: '',
  chatbot_url: '',
  home_content: '',
  landing_reports_enabled: true,
  landing_pricing_pro_multiplier: 1.2,
  landing_pricing_max_multiplier: 4,
  landing_pricing_exchange_rate: 7,
  backend_mode_enabled: false,
  hide_ccs_import_button: false,
  purchase_subscription_enabled: false,
  purchase_subscription_url: '',
  card_shop_enabled: false,
  card_shop_products: [],
  invoice_management_enabled: false,
  feedback_management_enabled: true,
  group_cache_hit_rate_enabled: false,
  sora_client_enabled: false,
  balance_alert_enabled: true,
  balance_alert_default_threshold: 5,
  custom_menu_items: [] as Array<{id: string; label: string; icon_svg: string; url: string; visibility: 'user' | 'admin'; sort_order: number}>,
  frontend_url: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  smtp_password_configured: false,
  smtp_from_email: '',
  smtp_from_name: '',
  smtp_use_tls: true,
  feedback_notify_email: '',
  // Cloudflare Turnstile
  turnstile_enabled: false,
  turnstile_site_key: '',
  turnstile_secret_key: '',
  turnstile_secret_key_configured: false,
  // LinuxDo Connect OAuth 登录
  linuxdo_connect_enabled: false,
  linuxdo_connect_client_id: '',
  linuxdo_connect_client_secret: '',
  linuxdo_connect_client_secret_configured: false,
  linuxdo_connect_redirect_url: '',
  // Google / OIDC OAuth 登录
  oidc_connect_enabled: false,
  oidc_connect_provider_name: 'Google',
  oidc_connect_client_id: '',
  oidc_connect_client_secret: '',
  oidc_connect_client_secret_configured: false,
  oidc_connect_issuer_url: 'https://accounts.google.com',
  oidc_connect_discovery_url: 'https://accounts.google.com/.well-known/openid-configuration',
  oidc_connect_authorize_url: 'https://accounts.google.com/o/oauth2/v2/auth',
  oidc_connect_token_url: 'https://oauth2.googleapis.com/token',
  oidc_connect_userinfo_url: 'https://openidconnect.googleapis.com/v1/userinfo',
  oidc_connect_jwks_url: 'https://www.googleapis.com/oauth2/v3/certs',
  oidc_connect_scopes: 'openid profile email',
  oidc_connect_redirect_url: '',
  oidc_connect_frontend_redirect_url: '/auth/google/callback',
  oidc_connect_token_auth_method: 'client_secret_post',
  oidc_connect_use_pkce: false,
  oidc_connect_validate_id_token: false,
  oidc_connect_allowed_signing_algs: 'RS256',
  oidc_connect_clock_skew_seconds: 120,
  oidc_connect_require_email_verified: true,
  oidc_connect_userinfo_email_path: 'email',
  oidc_connect_userinfo_id_path: 'sub',
  oidc_connect_userinfo_username_path: 'name',
  // GitHub OAuth 登录
  github_oauth_enabled: false,
  github_oauth_client_id: '',
  github_oauth_client_secret: '',
  github_oauth_client_secret_configured: false,
  github_oauth_redirect_url: '',
  github_oauth_frontend_redirect_url: '/auth/github/callback',
  // Model fallback
  enable_model_fallback: false,
  fallback_model_anthropic: 'claude-3-5-sonnet-20241022',
  fallback_model_openai: 'gpt-4o',
  fallback_model_gemini: 'gemini-2.5-pro',
  fallback_model_antigravity: 'gemini-2.5-pro',
  // Identity patch (Claude -> Gemini)
  enable_identity_patch: true,
  identity_patch_prompt: '',
  // Ops monitoring (vNext)
  ops_monitoring_enabled: true,
  ops_realtime_monitoring_enabled: true,
  ops_query_mode_default: 'auto',
  ops_metrics_interval_seconds: 60,
  // Claude Code version check
  min_claude_code_version: '',
  // 分组隔离
  allow_ungrouped_key_scheduling: false,
  // Gateway forwarding behavior
  enable_fingerprint_unification: true,
  enable_metadata_passthrough: false,
  enable_cch_signing: false,
  enable_anthropic_cache_ttl_1h_injection: false,
  // Stripe 支付
  stripe_enabled: false,
  stripe_secret_key: '',
  stripe_secret_key_configured: false,
  stripe_webhook_secret: '',
  stripe_webhook_secret_configured: false,
  // 支付宝支付
  alipay_enabled: false,
  // 虎皮椒充值
  xunhu_alipay_enabled: false,
  xunhu_alipay_appid: '',
  xunhu_alipay_key: '',
  xunhu_alipay_key_configured: false,
  xunhu_wechat_enabled: false,
  xunhu_wechat_appid: '',
  xunhu_wechat_key: '',
  xunhu_wechat_key_configured: false,
  xunhu_notify_url: ''
})

const landingPricingPreview = computed(() => {
  const exchangeRate = positiveNumberOrDefault(form.landing_pricing_exchange_rate, 7)
  const proMultiplier = positiveNumberOrDefault(form.landing_pricing_pro_multiplier, 1.2)
  const maxMultiplier = positiveNumberOrDefault(form.landing_pricing_max_multiplier, 4)

  return {
    proDiscount: formatLandingDiscount(proMultiplier, exchangeRate),
    maxDiscount: formatLandingDiscount(maxMultiplier, exchangeRate)
  }
})

const defaultSubscriptionGroupOptions = computed<DefaultSubscriptionGroupOption[]>(() =>
  subscriptionGroups.value.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    platform: group.platform,
    subscriptionType: group.subscription_type,
    rate: group.rate_multiplier
  }))
)

const registrationEmailSuffixWhitelistSeparatorKeys = new Set([' ', ',', '，', 'Enter', 'Tab'])

function removeRegistrationEmailSuffixWhitelistTag(suffix: string) {
  registrationEmailSuffixWhitelistTags.value = registrationEmailSuffixWhitelistTags.value.filter(
    (item) => item !== suffix
  )
}

function addRegistrationEmailSuffixWhitelistTag(raw: string) {
  const suffix = normalizeRegistrationEmailSuffixDomain(raw)
  if (
    !isRegistrationEmailSuffixDomainValid(suffix) ||
    registrationEmailSuffixWhitelistTags.value.includes(suffix)
  ) {
    return
  }
  registrationEmailSuffixWhitelistTags.value = [
    ...registrationEmailSuffixWhitelistTags.value,
    suffix
  ]
}

function commitRegistrationEmailSuffixWhitelistDraft() {
  if (!registrationEmailSuffixWhitelistDraft.value) {
    return
  }
  addRegistrationEmailSuffixWhitelistTag(registrationEmailSuffixWhitelistDraft.value)
  registrationEmailSuffixWhitelistDraft.value = ''
}

function handleRegistrationEmailSuffixWhitelistDraftInput() {
  registrationEmailSuffixWhitelistDraft.value = normalizeRegistrationEmailSuffixDomain(
    registrationEmailSuffixWhitelistDraft.value
  )
}

function handleRegistrationEmailSuffixWhitelistDraftKeydown(event: KeyboardEvent) {
  if (event.isComposing) {
    return
  }

  if (registrationEmailSuffixWhitelistSeparatorKeys.has(event.key)) {
    event.preventDefault()
    commitRegistrationEmailSuffixWhitelistDraft()
    return
  }

  if (
    event.key === 'Backspace' &&
    !registrationEmailSuffixWhitelistDraft.value &&
    registrationEmailSuffixWhitelistTags.value.length > 0
  ) {
    registrationEmailSuffixWhitelistTags.value.pop()
  }
}

function handleRegistrationEmailSuffixWhitelistPaste(event: ClipboardEvent) {
  const text = event.clipboardData?.getData('text') || ''
  if (!text.trim()) {
    return
  }
  event.preventDefault()
  const tokens = parseRegistrationEmailSuffixWhitelistInput(text)
  for (const token of tokens) {
    addRegistrationEmailSuffixWhitelistTag(token)
  }
}

// LinuxDo OAuth redirect URL suggestion
const linuxdoRedirectUrlSuggestion = computed(() => {
  if (typeof window === 'undefined') return ''
  const origin =
    window.location.origin || `${window.location.protocol}//${window.location.host}`
  return `${origin}/api/v1/auth/oauth/linuxdo/callback`
})

const googleRedirectUrlSuggestion = computed(() => {
  if (typeof window === 'undefined') return ''
  const origin =
    window.location.origin || `${window.location.protocol}//${window.location.host}`
  return `${origin}/api/v1/auth/oauth/google/callback`
})

const githubRedirectUrlSuggestion = computed(() => {
  if (typeof window === 'undefined') return ''
  const origin =
    window.location.origin || `${window.location.protocol}//${window.location.host}`
  return `${origin}/api/v1/auth/oauth/github/callback`
})

async function setAndCopyLinuxdoRedirectUrl() {
  const url = linuxdoRedirectUrlSuggestion.value
  if (!url) return

  form.linuxdo_connect_redirect_url = url
  await copyToClipboard(url, t('admin.settings.linuxdo.redirectUrlSetAndCopied'))
}

async function setAndCopyGoogleRedirectUrl() {
  const url = googleRedirectUrlSuggestion.value
  if (!url) return
  form.oidc_connect_redirect_url = url
  form.oidc_connect_frontend_redirect_url = '/auth/google/callback'
  await copyToClipboard(url, t('admin.settings.google.redirectUrlSetAndCopied'))
}

async function setAndCopyGithubRedirectUrl() {
  const url = githubRedirectUrlSuggestion.value
  if (!url) return
  form.github_oauth_redirect_url = url
  form.github_oauth_frontend_redirect_url = '/auth/github/callback'
  await copyToClipboard(url, t('admin.settings.github.redirectUrlSetAndCopied'))
}

// Custom menu item management
function addMenuItem() {
  form.custom_menu_items.push({
    id: '',
    label: '',
    icon_svg: '',
    url: '',
    visibility: 'user',
    sort_order: form.custom_menu_items.length,
  })
}

function removeMenuItem(index: number) {
  form.custom_menu_items.splice(index, 1)
  // Re-index sort_order
  form.custom_menu_items.forEach((item, i) => {
    item.sort_order = i
  })
}

function moveMenuItem(index: number, direction: -1 | 1) {
  const targetIndex = index + direction
  if (targetIndex < 0 || targetIndex >= form.custom_menu_items.length) return
  const items = form.custom_menu_items
  const temp = items[index]
  items[index] = items[targetIndex]
  items[targetIndex] = temp
  // Re-index sort_order
  items.forEach((item, i) => {
    item.sort_order = i
  })
}

function createCardShopProduct(): CardShopProduct {
  const nextIndex = form.card_shop_products.length
  const amount = [20, 50, 100, 200, 1000, 2000][nextIndex] ?? 20
  return {
    id: `card-shop-${Date.now()}-${nextIndex}`,
    label: `¥${amount} 余额卡`,
    amount_cny: amount,
    url: '',
    enabled: false,
    sort_order: nextIndex
  }
}

function addCardShopProduct() {
  form.card_shop_products.push(createCardShopProduct())
}

function removeCardShopProduct(index: number) {
  form.card_shop_products.splice(index, 1)
  form.card_shop_products.forEach((item, i) => {
    item.sort_order = i
  })
}

async function loadSettings() {
  loading.value = true
  try {
    const settings = await adminAPI.settings.getSettings()
    Object.assign(form, settings)
    form.card_shop_products = Array.isArray(settings.card_shop_products)
      ? settings.card_shop_products.map((item, index) => ({
          ...item,
          sort_order: item.sort_order ?? index
        }))
      : []
    form.backend_mode_enabled = settings.backend_mode_enabled
    form.default_subscriptions = Array.isArray(settings.default_subscriptions)
      ? settings.default_subscriptions
          .filter((item) => item.group_id > 0 && item.validity_days > 0)
          .map((item) => ({
            group_id: item.group_id,
            validity_days: item.validity_days
          }))
      : []
    registrationEmailSuffixWhitelistTags.value = normalizeRegistrationEmailSuffixDomains(
      settings.registration_email_suffix_whitelist
    )
    registrationEmailSuffixWhitelistDraft.value = ''
    form.smtp_password = ''
    form.feedback_notify_email = settings.feedback_notify_email || ''
    form.turnstile_secret_key = ''
    form.linuxdo_connect_client_secret = ''
    form.oidc_connect_client_secret = ''
    form.github_oauth_client_secret = ''
    form.stripe_secret_key = ''
    form.stripe_webhook_secret = ''
    form.xunhu_alipay_key = ''
    form.xunhu_wechat_key = ''
    form.xunhu_notify_url = normalizeXunhuNotifyURL(
      settings.xunhu_notify_url || '',
      settings.frontend_url || ''
    )
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToLoad') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    loading.value = false
  }
}

async function loadSubscriptionGroups() {
  try {
    const groups = await adminAPI.groups.getAll()
    subscriptionGroups.value = groups.filter(
      (group) =>
        (group.subscription_type === 'subscription' || group.subscription_type === 'credit') &&
        group.status === 'active'
    )
  } catch (error) {
    console.error('Failed to load subscription groups:', error)
    subscriptionGroups.value = []
  }
}

function addDefaultSubscription() {
  if (subscriptionGroups.value.length === 0) return
  const existing = new Set(form.default_subscriptions.map((item) => item.group_id))
  const candidate = subscriptionGroups.value.find((group) => !existing.has(group.id))
  if (!candidate) return
  form.default_subscriptions.push({
    group_id: candidate.id,
    validity_days: 30
  })
}

function removeDefaultSubscription(index: number) {
  form.default_subscriptions.splice(index, 1)
}

async function saveSettings() {
  saving.value = true
  try {
    const normalizedDefaultSubscriptions = form.default_subscriptions
      .filter((item) => item.group_id > 0 && item.validity_days > 0)
      .map((item: DefaultSubscriptionSetting) => ({
        group_id: item.group_id,
        validity_days: Math.min(36500, Math.max(1, Math.floor(item.validity_days)))
      }))

    const seenGroupIDs = new Set<number>()
    const duplicateDefaultSubscription = normalizedDefaultSubscriptions.find((item) => {
      if (seenGroupIDs.has(item.group_id)) {
        return true
      }
      seenGroupIDs.add(item.group_id)
      return false
    })
    if (duplicateDefaultSubscription) {
      appStore.showError(
        t('admin.settings.defaults.defaultSubscriptionsDuplicate', {
          groupId: duplicateDefaultSubscription.group_id
        })
      )
      return
    }

    if (form.chatbot_url) {
      const chatbotURL = form.chatbot_url.trim()
      let validChatbotURL = false
      try {
        const parsed = new URL(chatbotURL)
        validChatbotURL = parsed.protocol === 'https:'
          || (parsed.protocol === 'http:' && ['localhost', '127.0.0.1'].includes(parsed.hostname))
      } catch {
        validChatbotURL = false
      }
      if (!validChatbotURL) {
        appStore.showError('Chatbot 地址必须使用 HTTPS；本地开发允许 http://localhost 或 http://127.0.0.1')
        return
      }
    }

    const payload: UpdateSettingsRequest = {
      registration_enabled: form.registration_enabled,
      email_verify_enabled: form.email_verify_enabled,
      registration_email_suffix_whitelist: registrationEmailSuffixWhitelistTags.value.map(
        (suffix) => `@${suffix}`
      ),
      promo_code_enabled: form.promo_code_enabled,
      invitation_code_enabled: form.invitation_code_enabled,
      password_reset_enabled: form.password_reset_enabled,
      totp_enabled: form.totp_enabled,
      default_balance: form.default_balance,
      default_concurrency: form.default_concurrency,
      default_subscriptions: normalizedDefaultSubscriptions,
      site_name: form.site_name,
      site_logo: form.site_logo,
      site_subtitle: form.site_subtitle,
      api_base_url: form.api_base_url,
      contact_info: form.contact_info,
      tech_support_qrcode: form.tech_support_qrcode,
      after_sales_qrcode: form.after_sales_qrcode,
      doc_url: form.doc_url,
      chatbot_url: form.chatbot_url,
      home_content: form.home_content,
      landing_reports_enabled: form.landing_reports_enabled,
      landing_pricing_pro_multiplier: positiveNumberOrDefault(form.landing_pricing_pro_multiplier, 1.2),
      landing_pricing_max_multiplier: positiveNumberOrDefault(form.landing_pricing_max_multiplier, 4),
      landing_pricing_exchange_rate: positiveNumberOrDefault(form.landing_pricing_exchange_rate, 7),
      backend_mode_enabled: form.backend_mode_enabled,
      hide_ccs_import_button: form.hide_ccs_import_button,
      purchase_subscription_enabled: form.purchase_subscription_enabled,
      purchase_subscription_url: form.purchase_subscription_enabled ? form.purchase_subscription_url : '',
      card_shop_enabled: form.card_shop_enabled,
      card_shop_products: form.card_shop_products.map((item, index) => ({
        ...item,
        label: item.label.trim(),
        url: item.url.trim(),
        sort_order: index
      })),
      invoice_management_enabled: form.invoice_management_enabled,
      feedback_management_enabled: form.feedback_management_enabled,
      group_cache_hit_rate_enabled: form.group_cache_hit_rate_enabled,
      sora_client_enabled: form.sora_client_enabled,
      balance_alert_enabled: form.balance_alert_enabled,
      balance_alert_default_threshold: form.balance_alert_default_threshold,
      custom_menu_items: form.custom_menu_items,
      frontend_url: form.frontend_url,
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: form.smtp_password || undefined,
      smtp_from_email: form.smtp_from_email,
      smtp_from_name: form.smtp_from_name,
      smtp_use_tls: form.smtp_use_tls,
      feedback_notify_email: form.feedback_notify_email,
      turnstile_enabled: form.turnstile_enabled,
      turnstile_site_key: form.turnstile_site_key,
      turnstile_secret_key: form.turnstile_secret_key || undefined,
      linuxdo_connect_enabled: form.linuxdo_connect_enabled,
      linuxdo_connect_client_id: form.linuxdo_connect_client_id,
      linuxdo_connect_client_secret: form.linuxdo_connect_client_secret || undefined,
      linuxdo_connect_redirect_url: form.linuxdo_connect_redirect_url,
      oidc_connect_enabled: form.oidc_connect_enabled,
      oidc_connect_provider_name: form.oidc_connect_provider_name || 'Google',
      oidc_connect_client_id: form.oidc_connect_client_id,
      oidc_connect_client_secret: form.oidc_connect_client_secret || undefined,
      oidc_connect_issuer_url: form.oidc_connect_issuer_url || 'https://accounts.google.com',
      oidc_connect_discovery_url:
        form.oidc_connect_discovery_url ||
        'https://accounts.google.com/.well-known/openid-configuration',
      oidc_connect_authorize_url:
        form.oidc_connect_authorize_url || 'https://accounts.google.com/o/oauth2/v2/auth',
      oidc_connect_token_url: form.oidc_connect_token_url || 'https://oauth2.googleapis.com/token',
      oidc_connect_userinfo_url:
        form.oidc_connect_userinfo_url || 'https://openidconnect.googleapis.com/v1/userinfo',
      oidc_connect_jwks_url: form.oidc_connect_jwks_url || 'https://www.googleapis.com/oauth2/v3/certs',
      oidc_connect_scopes: form.oidc_connect_scopes || 'openid profile email',
      oidc_connect_redirect_url: form.oidc_connect_redirect_url,
      oidc_connect_frontend_redirect_url:
        form.oidc_connect_frontend_redirect_url || '/auth/google/callback',
      oidc_connect_token_auth_method: form.oidc_connect_token_auth_method || 'client_secret_post',
      oidc_connect_use_pkce: form.oidc_connect_use_pkce,
      oidc_connect_validate_id_token: form.oidc_connect_validate_id_token,
      oidc_connect_allowed_signing_algs: form.oidc_connect_allowed_signing_algs || 'RS256',
      oidc_connect_clock_skew_seconds: form.oidc_connect_clock_skew_seconds || 120,
      oidc_connect_require_email_verified: form.oidc_connect_require_email_verified,
      oidc_connect_userinfo_email_path: form.oidc_connect_userinfo_email_path || 'email',
      oidc_connect_userinfo_id_path: form.oidc_connect_userinfo_id_path || 'sub',
      oidc_connect_userinfo_username_path: form.oidc_connect_userinfo_username_path || 'name',
      github_oauth_enabled: form.github_oauth_enabled,
      github_oauth_client_id: form.github_oauth_client_id,
      github_oauth_client_secret: form.github_oauth_client_secret || undefined,
      github_oauth_redirect_url: form.github_oauth_redirect_url,
      github_oauth_frontend_redirect_url:
        form.github_oauth_frontend_redirect_url || '/auth/github/callback',
      enable_model_fallback: form.enable_model_fallback,
      fallback_model_anthropic: form.fallback_model_anthropic,
      fallback_model_openai: form.fallback_model_openai,
      fallback_model_gemini: form.fallback_model_gemini,
      fallback_model_antigravity: form.fallback_model_antigravity,
      enable_identity_patch: form.enable_identity_patch,
      identity_patch_prompt: form.identity_patch_prompt,
      min_claude_code_version: form.min_claude_code_version,
      allow_ungrouped_key_scheduling: form.allow_ungrouped_key_scheduling,
      enable_fingerprint_unification: form.enable_fingerprint_unification,
      enable_metadata_passthrough: form.enable_metadata_passthrough,
      enable_cch_signing: form.enable_cch_signing,
      enable_anthropic_cache_ttl_1h_injection: form.enable_anthropic_cache_ttl_1h_injection,
      stripe_enabled: form.stripe_enabled,
      stripe_secret_key: form.stripe_secret_key || undefined,
      stripe_webhook_secret: form.stripe_webhook_secret || undefined,
      alipay_enabled: form.alipay_enabled,
      xunhu_alipay_enabled: form.xunhu_alipay_enabled,
      xunhu_alipay_appid: form.xunhu_alipay_appid,
      xunhu_alipay_key: form.xunhu_alipay_key || undefined,
      xunhu_wechat_enabled: form.xunhu_wechat_enabled,
      xunhu_wechat_appid: form.xunhu_wechat_appid,
      xunhu_wechat_key: form.xunhu_wechat_key || undefined,
      xunhu_notify_url: normalizeXunhuNotifyURL(form.xunhu_notify_url, form.frontend_url)
    }
    const updated = await adminAPI.settings.updateSettings(payload)
    Object.assign(form, updated)
    form.card_shop_products = Array.isArray(updated.card_shop_products)
      ? updated.card_shop_products.map((item, index) => ({
          ...item,
          sort_order: item.sort_order ?? index
        }))
      : []
    registrationEmailSuffixWhitelistTags.value = normalizeRegistrationEmailSuffixDomains(
      updated.registration_email_suffix_whitelist
    )
    registrationEmailSuffixWhitelistDraft.value = ''
    form.smtp_password = ''
    form.turnstile_secret_key = ''
    form.linuxdo_connect_client_secret = ''
    form.oidc_connect_client_secret = ''
    form.github_oauth_client_secret = ''
    form.stripe_secret_key = ''
    form.stripe_webhook_secret = ''
    form.xunhu_alipay_key = ''
    form.xunhu_wechat_key = ''
    form.xunhu_notify_url = normalizeXunhuNotifyURL(
      updated.xunhu_notify_url || '',
      updated.frontend_url || ''
    )
    // Refresh cached settings so sidebar/header update immediately
    await appStore.fetchPublicSettings(true)
    await adminSettingsStore.fetch(true)
    appStore.showSuccess(t('admin.settings.settingsSaved'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToSave') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    saving.value = false
  }
}

async function testSmtpConnection() {
  testingSmtp.value = true
  try {
    const result = await adminAPI.settings.testSmtpConnection({
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: form.smtp_password,
      smtp_use_tls: form.smtp_use_tls
    })
    // API returns { message: "..." } on success, errors are thrown as exceptions
    appStore.showSuccess(result.message || t('admin.settings.smtpConnectionSuccess'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToTestSmtp') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    testingSmtp.value = false
  }
}

async function sendTestEmail() {
  if (!testEmailAddress.value) {
    appStore.showError(t('admin.settings.testEmail.enterRecipientHint'))
    return
  }

  sendingTestEmail.value = true
  try {
    const result = await adminAPI.settings.sendTestEmail({
      email: testEmailAddress.value,
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: form.smtp_password,
      smtp_from_email: form.smtp_from_email,
      smtp_from_name: form.smtp_from_name,
      smtp_use_tls: form.smtp_use_tls
    })
    // API returns { message: "..." } on success, errors are thrown as exceptions
    appStore.showSuccess(result.message || t('admin.settings.testEmailSent'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToSendTestEmail') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    sendingTestEmail.value = false
  }
}

// Admin API Key 方法
async function loadAdminApiKey() {
  adminApiKeyLoading.value = true
  try {
    const status = await adminAPI.settings.getAdminApiKey()
    adminApiKeyExists.value = status.exists
    adminApiKeyMasked.value = status.masked_key
  } catch (error: any) {
    console.error('Failed to load admin API key status:', error)
  } finally {
    adminApiKeyLoading.value = false
  }
}

async function createAdminApiKey() {
  adminApiKeyOperating.value = true
  try {
    const result = await adminAPI.settings.regenerateAdminApiKey()
    newAdminApiKey.value = result.key
    adminApiKeyExists.value = true
    adminApiKeyMasked.value = result.key.substring(0, 10) + '...' + result.key.slice(-4)
    appStore.showSuccess(t('admin.settings.adminApiKey.keyGenerated'))
  } catch (error: any) {
    appStore.showError(error.message || t('common.error'))
  } finally {
    adminApiKeyOperating.value = false
  }
}

async function regenerateAdminApiKey() {
  if (!confirm(t('admin.settings.adminApiKey.regenerateConfirm'))) return
  await createAdminApiKey()
}

async function deleteAdminApiKey() {
  if (!confirm(t('admin.settings.adminApiKey.deleteConfirm'))) return
  adminApiKeyOperating.value = true
  try {
    await adminAPI.settings.deleteAdminApiKey()
    adminApiKeyExists.value = false
    adminApiKeyMasked.value = ''
    newAdminApiKey.value = ''
    appStore.showSuccess(t('admin.settings.adminApiKey.keyDeleted'))
  } catch (error: any) {
    appStore.showError(error.message || t('common.error'))
  } finally {
    adminApiKeyOperating.value = false
  }
}

function copyNewKey() {
  navigator.clipboard
    .writeText(newAdminApiKey.value)
    .then(() => {
      appStore.showSuccess(t('admin.settings.adminApiKey.keyCopied'))
    })
    .catch(() => {
      appStore.showError(t('common.copyFailed'))
    })
}

// Stream Timeout 方法
async function loadStreamTimeoutSettings() {
  streamTimeoutLoading.value = true
  try {
    const settings = await adminAPI.settings.getStreamTimeoutSettings()
    Object.assign(streamTimeoutForm, settings)
  } catch (error: any) {
    console.error('Failed to load stream timeout settings:', error)
  } finally {
    streamTimeoutLoading.value = false
  }
}

async function saveStreamTimeoutSettings() {
  streamTimeoutSaving.value = true
  try {
    const updated = await adminAPI.settings.updateStreamTimeoutSettings({
      enabled: streamTimeoutForm.enabled,
      action: streamTimeoutForm.action,
      temp_unsched_minutes: streamTimeoutForm.temp_unsched_minutes,
      threshold_count: streamTimeoutForm.threshold_count,
      threshold_window_minutes: streamTimeoutForm.threshold_window_minutes
    })
    Object.assign(streamTimeoutForm, updated)
    appStore.showSuccess(t('admin.settings.streamTimeout.saved'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.streamTimeout.saveFailed') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    streamTimeoutSaving.value = false
  }
}

// Rectifier 方法
async function loadRectifierSettings() {
  rectifierLoading.value = true
  try {
    const settings = await adminAPI.settings.getRectifierSettings()
    Object.assign(rectifierForm, settings)
  } catch (error: any) {
    console.error('Failed to load rectifier settings:', error)
  } finally {
    rectifierLoading.value = false
  }
}

async function saveRectifierSettings() {
  rectifierSaving.value = true
  try {
    const updated = await adminAPI.settings.updateRectifierSettings({
      enabled: rectifierForm.enabled,
      thinking_signature_enabled: rectifierForm.thinking_signature_enabled,
      thinking_budget_enabled: rectifierForm.thinking_budget_enabled
    })
    Object.assign(rectifierForm, updated)
    appStore.showSuccess(t('admin.settings.rectifier.saved'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.rectifier.saveFailed') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    rectifierSaving.value = false
  }
}

const betaPolicyActionOptions = computed(() => [
  { value: 'pass', label: t('admin.settings.betaPolicy.actionPass') },
  { value: 'filter', label: t('admin.settings.betaPolicy.actionFilter') },
  { value: 'block', label: t('admin.settings.betaPolicy.actionBlock') }
])

const betaPolicyScopeOptions = computed(() => [
  { value: 'all', label: t('admin.settings.betaPolicy.scopeAll') },
  { value: 'oauth', label: t('admin.settings.betaPolicy.scopeOAuth') },
  { value: 'apikey', label: t('admin.settings.betaPolicy.scopeAPIKey') },
  { value: 'bedrock', label: t('admin.settings.betaPolicy.scopeBedrock') }
])

const openAIFastPolicyTierOptions = computed(() => [
  { value: 'all', label: 'all' },
  { value: 'priority', label: 'priority / fast' },
  { value: 'flex', label: 'flex' }
])

// Beta Policy 方法
const betaDisplayNames: Record<string, string> = {
  'fast-mode-2026-02-01': 'Fast Mode',
  'context-1m-2025-08-07': 'Context 1M'
}

function getBetaDisplayName(token: string): string {
  return betaDisplayNames[token] || token
}

async function loadBetaPolicySettings() {
  betaPolicyLoading.value = true
  try {
    const settings = await adminAPI.settings.getBetaPolicySettings()
    betaPolicyForm.rules = settings.rules
  } catch (error: any) {
    console.error('Failed to load beta policy settings:', error)
  } finally {
    betaPolicyLoading.value = false
  }
}

async function saveBetaPolicySettings() {
  betaPolicySaving.value = true
  try {
    const updated = await adminAPI.settings.updateBetaPolicySettings({
      rules: betaPolicyForm.rules
    })
    betaPolicyForm.rules = updated.rules
    appStore.showSuccess(t('admin.settings.betaPolicy.saved'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.betaPolicy.saveFailed') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    betaPolicySaving.value = false
  }
}

function toOpenAIFastPolicyRuleForm(rule: OpenAIFastPolicyRule): OpenAIFastPolicyRuleForm {
  return {
    ...rule,
    service_tier: rule.service_tier || 'all',
    action: rule.action || 'pass',
    scope: rule.scope || 'all',
    model_whitelist_text: Array.isArray(rule.model_whitelist) ? rule.model_whitelist.join(',') : ''
  }
}

function fromOpenAIFastPolicyRuleForm(rule: OpenAIFastPolicyRuleForm): OpenAIFastPolicyRule {
  const modelWhitelist = rule.model_whitelist_text
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  return {
    service_tier: rule.service_tier,
    action: rule.action,
    scope: rule.scope,
    error_message: rule.error_message || undefined,
    model_whitelist: modelWhitelist,
    fallback_action: rule.fallback_action || undefined,
    fallback_error_message: rule.fallback_error_message || undefined
  }
}

function addOpenAIFastPolicyRule() {
  openAIFastPolicyForm.rules.push({
    service_tier: 'priority',
    action: 'filter',
    scope: 'all',
    model_whitelist: [],
    model_whitelist_text: '',
    fallback_action: 'pass'
  })
}

function removeOpenAIFastPolicyRule(index: number) {
  openAIFastPolicyForm.rules.splice(index, 1)
}

async function loadOpenAIFastPolicySettings() {
  openAIFastPolicyLoading.value = true
  try {
    const settings = await adminAPI.settings.getOpenAIFastPolicySettings()
    openAIFastPolicyForm.rules = settings.rules.map(toOpenAIFastPolicyRuleForm)
  } catch (error) {
    console.error('Failed to load OpenAI fast policy settings:', error)
  } finally {
    openAIFastPolicyLoading.value = false
  }
}

async function saveOpenAIFastPolicySettings() {
  openAIFastPolicySaving.value = true
  try {
    const updated = await adminAPI.settings.updateOpenAIFastPolicySettings({
      rules: openAIFastPolicyForm.rules.map(fromOpenAIFastPolicyRuleForm)
    })
    openAIFastPolicyForm.rules = updated.rules.map(toOpenAIFastPolicyRuleForm)
    appStore.showSuccess('OpenAI Fast/Flex Policy 已保存')
  } catch (error: any) {
    appStore.showError(
      'OpenAI Fast/Flex Policy 保存失败: ' + (error.message || t('common.unknownError'))
    )
  } finally {
    openAIFastPolicySaving.value = false
  }
}

function addWebSearchProvider() {
  webSearchConfig.providers.push({
    api_key: '',
    quota_limit: null,
    subscribed_at: '',
    proxy_id: null
  })
}

function removeWebSearchProvider(index: number) {
  webSearchConfig.providers.splice(index, 1)
}

async function loadWebSearchConfig() {
  webSearchLoading.value = true
  try {
    const [config, proxies] = await Promise.all([
      adminAPI.settings.getWebSearchEmulationConfig().catch(() => ({
        enabled: false,
        providers: [] as WebSearchProviderConfig[]
      })),
      adminAPI.proxies.getAll().catch(() => [] as Proxy[])
    ])
    webSearchConfig.enabled = config.enabled === true
    webSearchConfig.providers = config.providers || []
    webSearchProxies.value = proxies
  } finally {
    webSearchLoading.value = false
  }
}

async function saveWebSearchConfig() {
  webSearchSaving.value = true
  try {
    const providers = webSearchConfig.providers.map((provider) => ({
      ...provider,
      quota_limit:
        provider.quota_limit != null && provider.quota_limit > 0 ? provider.quota_limit : null,
      proxy_id: provider.proxy_id || null
    }))
    const updated = await adminAPI.settings.updateWebSearchEmulationConfig({
      enabled: webSearchConfig.enabled,
      providers
    })
    webSearchConfig.enabled = updated.enabled
    webSearchConfig.providers = updated.providers || []
    appStore.showSuccess('Web Search 模拟配置已保存')
  } catch (error: any) {
    appStore.showError(error.message || '保存 Web Search 模拟配置失败')
  } finally {
    webSearchSaving.value = false
  }
}

async function testWebSearchProvider() {
  webSearchTestLoading.value = true
  try {
    webSearchTestResult.value = await adminAPI.settings.testWebSearchEmulation(
      webSearchTestQuery.value.trim() || 'OpenAI latest release'
    )
  } catch (error: any) {
    appStore.showError(error.message || '测试 Web Search Provider 失败')
  } finally {
    webSearchTestLoading.value = false
  }
}

onMounted(() => {
  loadSettings()
  loadSubscriptionGroups()
  loadAdminApiKey()
  loadStreamTimeoutSettings()
  loadRectifierSettings()
  loadBetaPolicySettings()
  loadOpenAIFastPolicySettings()
  loadWebSearchConfig()
})
</script>

<style scoped>
.default-sub-group-select :deep(.select-trigger) {
  @apply h-[42px];
}

.default-sub-delete-btn {
  @apply h-[42px];
}

/* ============ Settings Tab Navigation ============ */

/* Scroll container: thin scrollbar on PC, auto-hide on mobile */
.settings-tabs-scroll {
  scrollbar-width: thin;
  scrollbar-color: transparent transparent;
}
.settings-tabs-scroll:hover {
  scrollbar-color: rgb(0 0 0 / 0.15) transparent;
}
:root.dark .settings-tabs-scroll:hover {
  scrollbar-color: rgb(255 255 255 / 0.2) transparent;
}
.settings-tabs-scroll::-webkit-scrollbar {
  height: 3px;
}
.settings-tabs-scroll::-webkit-scrollbar-track {
  background: transparent;
}
.settings-tabs-scroll::-webkit-scrollbar-thumb {
  background: transparent;
  border-radius: 3px;
}
.settings-tabs-scroll:hover::-webkit-scrollbar-thumb {
  background: rgb(0 0 0 / 0.15);
}
:root.dark .settings-tabs-scroll:hover::-webkit-scrollbar-thumb {
  background: rgb(255 255 255 / 0.2);
}

.settings-tabs {
  @apply inline-flex min-w-full gap-0.5 rounded-2xl
         border border-gray-100 bg-white/80 p-1 backdrop-blur-sm
         dark:border-dark-700/50 dark:bg-dark-800/80;
  box-shadow: 0 1px 3px rgb(0 0 0 / 0.04), 0 1px 2px rgb(0 0 0 / 0.02);
}

@media (min-width: 640px) {
  .settings-tabs {
    @apply flex;
  }
}

.settings-tab {
  @apply relative flex flex-1 items-center justify-center gap-1.5
         whitespace-nowrap rounded-xl px-2.5 py-2
         text-sm font-medium
         text-gray-500 dark:text-dark-400
         transition-all duration-200 ease-out;
}

.settings-tab:hover:not(.settings-tab-active) {
  @apply text-gray-700 dark:text-gray-300;
  background: rgb(0 0 0 / 0.03);
}

:root.dark .settings-tab:hover:not(.settings-tab-active) {
  background: rgb(255 255 255 / 0.04);
}

.settings-tab-active {
  @apply text-primary-600 dark:text-primary-400;
  background: linear-gradient(135deg, rgba(20, 184, 166, 0.08), rgba(20, 184, 166, 0.03));
  box-shadow: 0 1px 2px rgba(20, 184, 166, 0.1);
}

:root.dark .settings-tab-active {
  background: linear-gradient(135deg, rgba(45, 212, 191, 0.12), rgba(45, 212, 191, 0.05));
  box-shadow: 0 1px 3px rgb(0 0 0 / 0.25);
}

.settings-tab-icon {
  @apply flex h-6 w-6 items-center justify-center rounded-lg
         transition-all duration-200;
}

.settings-tab-active .settings-tab-icon {
  @apply bg-primary-500/15 text-primary-600
         dark:bg-primary-400/15 dark:text-primary-400;
}
</style>
