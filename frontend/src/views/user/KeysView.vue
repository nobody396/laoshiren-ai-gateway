<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700" :aria-label="t('team.scopeSwitch')">
            <button type="button" :class="['rounded-md px-3 py-1.5 text-sm', activeScope === 'personal' ? 'bg-white font-medium text-primary-600 shadow-sm dark:bg-dark-800' : 'text-gray-500']" @click="setScope('personal')">{{ t('team.personalKeys') }}</button>
            <button type="button" :class="['rounded-md px-3 py-1.5 text-sm', activeScope === 'team' ? 'bg-white font-medium text-primary-600 shadow-sm dark:bg-dark-800' : 'text-gray-500']" @click="setScope('team')">{{ t('team.teamKeys') }}</button>
          </div>
          <SearchInput
            v-model="filterSearch"
            :placeholder="t('keys.searchPlaceholder')"
            class="w-full sm:w-64"
            @search="onFilterChange"
          />
          <div
            class="flex w-full min-w-0 items-center rounded-xl border border-gray-200 bg-white shadow-sm transition-all duration-200 focus-within:border-primary-500 focus-within:ring-2 focus-within:ring-primary-500/30 dark:border-dark-600 dark:bg-dark-800 sm:w-[24rem] xl:w-[26rem]"
          >
            <span class="shrink-0 px-3 text-xs font-medium text-gray-500 dark:text-dark-300">
              {{ t('keys.baseUrl') }}
            </span>
            <input
              :value="displayApiBaseUrl"
              type="url"
              readonly
              class="min-w-0 flex-1 bg-transparent py-2.5 pr-2 font-mono text-sm text-gray-900 outline-none dark:text-gray-100"
              @focus="selectBaseUrl"
            />
            <button
              type="button"
              @click="copyApiBaseUrl"
              class="self-stretch border-l border-gray-200 px-3 text-gray-400 transition-colors hover:bg-gray-50 hover:text-primary-600 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-primary-500/40 dark:border-dark-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
              :class="copiedBaseUrl ? 'text-green-500 dark:text-green-400' : ''"
              :title="copiedBaseUrl ? t('keys.copied') : t('keys.copyBaseUrl')"
              :aria-label="t('keys.copyBaseUrl')"
            >
              <Icon
                v-if="copiedBaseUrl"
                name="check"
                size="sm"
                :stroke-width="2"
              />
              <Icon v-else name="copy" size="sm" />
            </button>
          </div>
          <Select
            :model-value="filterGroupId"
            class="w-40"
            :options="groupFilterOptions"
            @update:model-value="onGroupFilterChange"
          />
          <Select
            :model-value="filterStatus"
            class="w-40"
            :options="statusFilterOptions"
            @update:model-value="onStatusFilterChange"
          />
        </div>
      </template>

      <template #actions>
        <div class="flex justify-end gap-3">
        <button
          @click="loadApiKeys"
          :disabled="loading"
          class="btn btn-secondary"
          :title="t('common.refresh')"
        >
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
        <button @click="showCreateModal = true" class="btn btn-primary" data-tour="keys-create-btn">
          <Icon name="plus" size="md" class="mr-2" />
          {{ t('keys.createKey') }}
        </button>
      </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="apiKeys" :loading="loading">
          <template #cell-key="{ value, row }">
            <div class="flex items-center gap-2">
              <code class="code text-xs">
                {{ maskKey(value) }}
              </code>
              <button
                @click="copyToClipboard(value, row.id)"
                class="rounded-lg p-1 transition-colors hover:bg-gray-100 dark:hover:bg-dark-700"
                :class="
                  copiedKeyId === row.id
                    ? 'text-green-500'
                    : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'
                "
                :title="copiedKeyId === row.id ? t('keys.copied') : t('keys.copyToClipboard')"
              >
                <Icon
                  v-if="copiedKeyId === row.id"
                  name="check"
                  size="sm"
                  :stroke-width="2"
                />
                <Icon v-else name="clipboard" size="sm" />
              </button>
            </div>
          </template>

          <template #cell-name="{ value, row }">
            <div class="flex items-center gap-1.5">
              <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
              <Icon
                v-if="row.ip_whitelist?.length > 0 || row.ip_blacklist?.length > 0"
                name="shield"
                size="sm"
                class="text-blue-500"
                :title="t('keys.ipRestrictionEnabled')"
              />
            </div>
          </template>

          <template #cell-group="{ row }">
            <button v-if="row.group_ids?.length" type="button" class="btn btn-secondary btn-sm" @click="editKey(row)">已授权 {{ row.group_ids.length }} 个分组 · 管理</button>
            <KeyGroupSelector v-else
              :model-value="row.group_id"
              :options="baseGroupOptions"
              :fallback-option="getKeyGroupFallbackOption(row)"
              :include-monthly="subscriptionStore.hasActiveSubscriptions"
              variant="inline"
              :placeholder="t('keys.selectGroup')"
              :search-placeholder="t('keys.searchGroup')"
              :title="t('keys.clickToChangeGroup')"
              @update:model-value="changeGroup(row, $event)"
            />
          </template>

          <template #cell-usage="{ row }">
            <div class="text-sm">
              <div class="flex items-center gap-1.5">
                <span class="text-gray-500 dark:text-gray-400">{{ t('keys.today') }}:</span>
                <span class="font-medium text-gray-900 dark:text-white">
                  ⚡{{ (usageStats[row.id]?.today_actual_cost ?? 0).toFixed(4) }}
                </span>
              </div>
              <div class="mt-0.5 flex items-center gap-1.5">
                <span class="text-gray-500 dark:text-gray-400">{{ t('keys.total') }}:</span>
                <span class="font-medium text-gray-900 dark:text-white">
                  ⚡{{ (usageStats[row.id]?.total_actual_cost ?? 0).toFixed(4) }}
                </span>
              </div>
              <!-- Quota progress (if quota is set) -->
              <div v-if="row.quota > 0" class="mt-1.5">
                <div class="flex items-center gap-1.5">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('keys.quota') }}:</span>
                  <span :class="[
                    'font-medium',
                    row.quota_used >= row.quota ? 'text-red-500' :
                    row.quota_used >= row.quota * 0.8 ? 'text-yellow-500' :
                    'text-gray-900 dark:text-white'
                  ]">
                    ⚡{{ row.quota_used?.toFixed(2) || '0.00' }} / ⚡{{ row.quota?.toFixed(2) }}
                  </span>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.quota_used >= row.quota ? 'bg-red-500' :
                      row.quota_used >= row.quota * 0.8 ? 'bg-yellow-500' :
                      'bg-primary-500'
                    ]"
                    :style="{ width: Math.min((row.quota_used / row.quota) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>
          </template>

          <template #cell-rate_limit="{ row }">
            <div v-if="row.rate_limit_5h > 0 || row.rate_limit_1d > 0 || row.rate_limit_7d > 0" class="space-y-1.5 min-w-[140px]">
              <!-- 5h window -->
              <div v-if="row.rate_limit_5h > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">5h</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_5h >= row.rate_limit_5h ? 'text-red-500' :
                    row.usage_5h >= row.rate_limit_5h * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ⚡{{ row.usage_5h?.toFixed(2) || '0.00' }}/⚡{{ row.rate_limit_5h?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_5h >= row.rate_limit_5h ? 'bg-red-500' :
                      row.usage_5h >= row.rate_limit_5h * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_5h / row.rate_limit_5h) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_5h_at && formatResetTime(row.reset_5h_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_5h_at) }}
                </div>
              </div>
              <!-- 1d window -->
              <div v-if="row.rate_limit_1d > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">1d</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_1d >= row.rate_limit_1d ? 'text-red-500' :
                    row.usage_1d >= row.rate_limit_1d * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ⚡{{ row.usage_1d?.toFixed(2) || '0.00' }}/⚡{{ row.rate_limit_1d?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_1d >= row.rate_limit_1d ? 'bg-red-500' :
                      row.usage_1d >= row.rate_limit_1d * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_1d / row.rate_limit_1d) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_1d_at && formatResetTime(row.reset_1d_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_1d_at) }}
                </div>
              </div>
              <!-- 7d window -->
              <div v-if="row.rate_limit_7d > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">7d</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_7d >= row.rate_limit_7d ? 'text-red-500' :
                    row.usage_7d >= row.rate_limit_7d * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ⚡{{ row.usage_7d?.toFixed(2) || '0.00' }}/⚡{{ row.rate_limit_7d?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_7d >= row.rate_limit_7d ? 'bg-red-500' :
                      row.usage_7d >= row.rate_limit_7d * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_7d / row.rate_limit_7d) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_7d_at && formatResetTime(row.reset_7d_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_7d_at) }}
                </div>
              </div>
              <!-- Reset button -->
              <button
                v-if="row.usage_5h > 0 || row.usage_1d > 0 || row.usage_7d > 0"
                @click.stop="confirmResetRateLimitFromTable(row)"
                class="mt-0.5 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="t('keys.resetRateLimitUsage')"
              >
                <Icon name="refresh" size="xs" />
                {{ t('keys.resetUsage') }}
              </button>
            </div>
            <span v-else class="text-sm text-gray-400 dark:text-dark-400">-</span>
          </template>

          <template #cell-expires_at="{ value }">
            <span v-if="value" :class="[
              'text-sm',
              new Date(value) < new Date() ? 'text-red-500 dark:text-red-400' : 'text-gray-500 dark:text-dark-400'
            ]">
              {{ formatDateTime(value) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-400">{{ t('keys.noExpiration') }}</span>
          </template>

          <template #cell-status="{ value }">
            <span :class="[
              'badge',
              value === 'active' ? 'badge-success' :
              value === 'quota_exhausted' ? 'badge-warning' :
              value === 'expired' ? 'badge-danger' :
              'badge-gray'
            ]">
              {{ t('keys.status.' + value) }}
            </span>
          </template>

          <template #cell-last_used_at="{ value }">
            <span v-if="value" class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(value) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-400">-</span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <!-- Use Key Button -->
              <button
                @click="openUseKeyModal(row)"
                data-tour="keys-use-key"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400"
              >
                <Icon name="terminal" size="sm" />
                <span class="text-xs">{{ t('keys.useKey') }}</span>
              </button>
              <div
                v-if="canOpenCodexSetup(row) || (!publicSettings?.hide_ccs_import_button && canImportToCcs(row))"
                class="flex items-center gap-1 rounded-lg"
                data-tour="keys-setup-options"
              >
                <!-- Client Auto Config Button -->
                <button
                  v-if="canOpenCodexSetup(row)"
                  @click="openClientSetup(row)"
                  :title="t('keys.configureClientHint', { client: 'Codex' })"
                  class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
                >
                  <Icon name="terminal" size="sm" />
                  <span class="text-xs">{{ t('keys.configureClient') }}</span>
                </button>
                <!-- Import to CC Switch Button -->
                <button
                  v-if="!publicSettings?.hide_ccs_import_button && canImportToCcs(row)"
                  @click="importToCcswitch(row)"
                  :title="t('keys.importToCcSwitchHint')"
                  data-tour="keys-import-ccs"
                  class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
                >
                  <Icon name="upload" size="sm" />
                  <span class="text-xs">{{ t('keys.importToCcSwitch') }}</span>
                </button>
              </div>
              <!-- Chat with this API Key -->
              <button
                v-if="publicSettings?.chatbot_url && row.group?.chatbot_enabled"
                @click="openChatbotWithKey(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-purple-50 hover:text-purple-600 dark:hover:bg-purple-900/20 dark:hover:text-purple-400"
              >
                <Icon name="chat" size="sm" />
                <span class="text-xs">聊天</span>
              </button>
              <!-- Toggle Status Button -->
              <button
                @click="toggleKeyStatus(row)"
                :class="[
                  'flex flex-col items-center gap-0.5 rounded-lg p-1.5 transition-colors',
                  row.status === 'active'
                    ? 'text-gray-500 hover:bg-yellow-50 hover:text-yellow-600 dark:hover:bg-yellow-900/20 dark:hover:text-yellow-400'
                    : 'text-gray-500 hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400'
                ]"
              >
                <Icon v-if="row.status === 'active'" name="ban" size="sm" />
                <Icon v-else name="checkCircle" size="sm" />
                <span class="text-xs">{{ row.status === 'active' ? t('keys.disable') : t('keys.enable') }}</span>
              </button>
              <!-- Edit Button -->
              <button
                @click="editKey(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
              >
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('common.edit') }}</span>
              </button>
              <!-- Delete Button -->
              <button
                @click="confirmDelete(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('keys.noKeysYet')"
              :description="t('keys.createFirstKey')"
              :action-text="t('keys.createKey')"
              @action="showCreateModal = true"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Create/Edit Modal -->
    <BaseDialog
      :show="showCreateModal || showEditModal"
      :title="showEditModal ? t('keys.editKey') : t('keys.createKey')"
      width="normal"
      @close="closeModals"
    >
      <form id="key-form" @submit.prevent="handleSubmit" class="space-y-5">
        <div>
          <label class="input-label">{{ t('keys.nameLabel') }}</label>
          <input
            v-model="formData.name"
            type="text"
            required
            class="input"
            :placeholder="t('keys.namePlaceholder')"
            data-tour="key-form-name"
          />
        </div>

        <div>
          <label class="input-label">授权分组</label>
          <div class="mb-3 flex gap-4 text-sm">
            <label><input v-model="keyGroupMode" type="radio" value="multi" /> 多分组 Key</label>
            <label><input v-model="keyGroupMode" type="radio" value="single" /> 单分组 Key（兼容模式）</label>
          </div>
          <div v-if="keyGroupMode === 'multi' && !groupsLoading && groupsLoadedScope !== activeScope" class="mb-3 text-sm" role="alert">分组加载失败，暂不能创建此 Key。<button type="button" class="btn btn-secondary btn-sm" @click="loadGroups">重新加载</button></div>
          <KeyGroupMultiSelect v-if="keyGroupMode === 'multi'" v-model="selectedGroupIds" :groups="multiGroupOptions" :rates="activeScope === 'personal' ? userGroupRates : {}" :disabled="groupsLoading" />
          <KeyGroupSelector v-else
            v-model="formData.group_id"
            :options="baseGroupOptions"
            :placeholder="t('keys.selectGroup')"
            :search-placeholder="t('keys.searchGroup')"
            :include-monthly="subscriptionStore.hasActiveSubscriptions"
            data-tour="key-form-group"
          />
        </div>

        <!-- Custom Key Section (only for create) -->
        <div v-if="!showEditModal" class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.customKeyLabel') }}</label>
            <button
              type="button"
              @click="formData.use_custom_key = !formData.use_custom_key"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.use_custom_key ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.use_custom_key ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          <div v-if="formData.use_custom_key">
            <input
              v-model="formData.custom_key"
              type="text"
              class="input font-mono"
              :placeholder="t('keys.customKeyPlaceholder')"
              :class="{ 'border-red-500 dark:border-red-500': customKeyError }"
            />
            <p v-if="customKeyError" class="mt-1 text-sm text-red-500">{{ customKeyError }}</p>
            <p v-else class="input-hint">{{ t('keys.customKeyHint') }}</p>
          </div>
        </div>

        <div v-if="showEditModal">
          <label class="input-label">{{ t('keys.statusLabel') }}</label>
          <Select
            v-model="formData.status"
            :options="statusOptions"
            :placeholder="t('keys.selectStatus')"
          />
        </div>

        <!-- IP Restriction Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.ipRestriction') }}</label>
            <button
              type="button"
              @click="formData.enable_ip_restriction = !formData.enable_ip_restriction"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_ip_restriction ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_ip_restriction ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_ip_restriction" class="space-y-4 pt-2">
            <div>
              <label class="input-label">{{ t('keys.ipWhitelist') }}</label>
              <textarea
                v-model="formData.ip_whitelist"
                rows="3"
                class="input font-mono text-sm"
                :placeholder="t('keys.ipWhitelistPlaceholder')"
              />
              <p class="input-hint">{{ t('keys.ipWhitelistHint') }}</p>
            </div>

            <div>
              <label class="input-label">{{ t('keys.ipBlacklist') }}</label>
              <textarea
                v-model="formData.ip_blacklist"
                rows="3"
                class="input font-mono text-sm"
                :placeholder="t('keys.ipBlacklistPlaceholder')"
              />
              <p class="input-hint">{{ t('keys.ipBlacklistHint') }}</p>
            </div>
          </div>
        </div>

        <!-- Quota Limit Section -->
        <div class="space-y-3">
          <label class="input-label">{{ t('keys.quotaLimit') }}</label>
          <!-- Switch commented out - always show input, 0 = unlimited
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.quotaLimit') }}</label>
            <button
              type="button"
              @click="formData.enable_quota = !formData.enable_quota"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_quota ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_quota ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          -->

          <div class="space-y-4">
            <div>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">⚡</span>
                <input
                  v-model.number="formData.quota"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="t('keys.quotaAmountPlaceholder')"
                />
              </div>
              <p class="input-hint">{{ t('keys.quotaAmountHint') }}</p>
            </div>

            <!-- Quota used display (only in edit mode) -->
            <div v-if="showEditModal && selectedKey && selectedKey.quota > 0">
              <label class="input-label">{{ t('keys.quotaUsed') }}</label>
              <div class="flex items-center gap-2">
                <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700">
                  <span class="font-medium text-gray-900 dark:text-white">
                    ⚡{{ selectedKey.quota_used?.toFixed(4) || '0.0000' }}
                  </span>
                  <span class="mx-2 text-gray-400">/</span>
                  <span class="text-gray-500 dark:text-gray-400">
                    ⚡{{ selectedKey.quota?.toFixed(2) || '0.00' }}
                  </span>
                </div>
                <button
                  type="button"
                  @click="confirmResetQuota"
                  class="btn btn-secondary text-sm"
                  :title="t('keys.resetQuotaUsed')"
                >
                  {{ t('keys.reset') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Rate Limit Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.rateLimitSection') }}</label>
            <button
              type="button"
              @click="formData.enable_rate_limit = !formData.enable_rate_limit"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_rate_limit ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_rate_limit ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_rate_limit" class="space-y-4 pt-2">
            <p class="input-hint -mt-2">{{ t('keys.rateLimitHint') }}</p>
            <!-- 5-Hour Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit5h') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">⚡</span>
                <input
                  v-model.number="formData.rate_limit_5h"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_5h > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h ? 'text-red-500' :
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ⚡{{ selectedKey.usage_5h?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ⚡{{ selectedKey.rate_limit_5h?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h ? 'bg-red-500' :
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_5h / selectedKey.rate_limit_5h) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- Daily Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit1d') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">⚡</span>
                <input
                  v-model.number="formData.rate_limit_1d"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_1d > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d ? 'text-red-500' :
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ⚡{{ selectedKey.usage_1d?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ⚡{{ selectedKey.rate_limit_1d?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d ? 'bg-red-500' :
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_1d / selectedKey.rate_limit_1d) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- 7-Day Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit7d') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">⚡</span>
                <input
                  v-model.number="formData.rate_limit_7d"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_7d > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d ? 'text-red-500' :
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ⚡{{ selectedKey.usage_7d?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ⚡{{ selectedKey.rate_limit_7d?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d ? 'bg-red-500' :
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_7d / selectedKey.rate_limit_7d) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- Reset Rate Limit button (edit mode only) -->
            <div v-if="showEditModal && selectedKey && (selectedKey.rate_limit_5h > 0 || selectedKey.rate_limit_1d > 0 || selectedKey.rate_limit_7d > 0)">
              <button
                type="button"
                @click="confirmResetRateLimit"
                class="btn btn-secondary text-sm"
              >
                {{ t('keys.resetRateLimitUsage') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Expiration Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.expiration') }}</label>
            <button
              type="button"
              @click="formData.enable_expiration = !formData.enable_expiration"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_expiration ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_expiration ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_expiration" class="space-y-4 pt-2">
            <!-- Quick select buttons (for both create and edit mode) -->
            <div class="flex flex-wrap gap-2">
              <button
                v-for="days in ['7', '30', '90']"
                :key="days"
                type="button"
                @click="setExpirationDays(parseInt(days))"
                :class="[
                  'rounded-lg px-3 py-1.5 text-sm transition-colors',
                  formData.expiration_preset === days
                    ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
                ]"
              >
                {{ showEditModal ? t('keys.extendDays', { days }) : t('keys.expiresInDays', { days }) }}
              </button>
              <button
                type="button"
                @click="formData.expiration_preset = 'custom'"
                :class="[
                  'rounded-lg px-3 py-1.5 text-sm transition-colors',
                  formData.expiration_preset === 'custom'
                    ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
                ]"
              >
                {{ t('keys.customDate') }}
              </button>
            </div>

            <!-- Date picker (always show for precise adjustment) -->
            <div>
              <label class="input-label">{{ t('keys.expirationDate') }}</label>
              <input
                v-model="formData.expiration_date"
                type="datetime-local"
                class="input"
              />
              <p class="input-hint">{{ t('keys.expirationDateHint') }}</p>
            </div>

            <!-- Current expiration display (only in edit mode) -->
            <div v-if="showEditModal && selectedKey?.expires_at" class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('keys.currentExpiration') }}: </span>
              <span class="font-medium text-gray-900 dark:text-white">
                {{ formatDateTime(selectedKey.expires_at) }}
              </span>
            </div>
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeModals" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            form="key-form"
            type="submit"
            :disabled="submitting"
            class="btn btn-primary"
            data-tour="key-form-submit"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
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
              submitting
                ? t('keys.saving')
                : showEditModal
                  ? t('common.update')
                  : t('common.create')
            }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('keys.deleteKey')"
      :message="t('keys.deleteConfirmMessage', { name: selectedKey?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Reset Quota Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetQuotaDialog"
      :title="t('keys.resetQuotaTitle')"
      :message="t('keys.resetQuotaConfirmMessage', { name: selectedKey?.name, used: selectedKey?.quota_used?.toFixed(4) })"
      :confirm-text="t('keys.reset')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="resetQuotaUsed"
      @cancel="showResetQuotaDialog = false"
    />

    <!-- Reset Rate Limit Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetRateLimitDialog"
      :title="t('keys.resetRateLimitTitle')"
      :message="t('keys.resetRateLimitConfirmMessage', { name: selectedKey?.name })"
      :confirm-text="t('keys.reset')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="resetRateLimitUsage"
      @cancel="showResetRateLimitDialog = false"
    />

    <!-- Use Key Modal -->
    <BaseDialog v-if="selectedKey?.group_ids?.length" :show="showUseKeyModal" title="使用多分组 Key" @close="closeUseKeyModal">
      <div class="space-y-4 text-sm">
        <p>同一把 Key 可用于各客户端，服务域名不变。客户端请求使用什么协议，网关就按该协议和模型名匹配已授权分组。</p>
        <p class="break-all font-mono">{{ displayApiBaseUrl }}</p>
        <p>Claude Code 使用根地址；OpenAI 兼容客户端通常使用带 /v1 的地址；Gemini 使用原生接口。请按客户端要求填写，不代表所有协议可以互换。</p>
        <p>当前支持 Messages、Responses、Chat Completions 和 Gemini 文本 HTTP。图片、视频、实时连接仍使用单分组 Key。一键安装配置暂不适用于此类 Key。</p>
        <button type="button" class="btn btn-primary" @click="clipboardCopy(selectedKey.key, 'Key 已复制')">复制这把 Key</button>
      </div>
    </BaseDialog>
    <UseKeyModal v-if="!selectedKey?.group_ids?.length"
      :show="showUseKeyModal"
      :api-key="selectedKey?.key || ''"
      :base-url="publicSettings?.api_base_url || ''"
      :platform="selectedKey?.group?.platform || null"
      :allow-messages-dispatch="selectedKey?.group?.allow_messages_dispatch || false"
      :default-mapped-model="selectedKey?.group?.default_mapped_model || ''"
      @close="closeUseKeyModal"
    />

    <ClientSetupModal
      :show="showClientSetup"
      :api-key-id="clientSetupRow?.id || 0"
      :key-name="clientSetupRow?.name || ''"
      :group-name="clientSetupRow?.group?.name || ''"
      @close="closeClientSetup"
    />

    <!-- CCS Client Selection Dialog -->
    <BaseDialog
      :show="showCcsClientSelect"
      :title="t('keys.ccsClientSelect.title')"
      width="narrow"
      @close="closeCcsClientSelect"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-400">
          {{ t('keys.ccsClientSelect.description') }}
        </p>
        <div :class="['grid gap-3', ccsClientOptions.length >= 3 ? 'grid-cols-1 sm:grid-cols-2' : 'grid-cols-2']">
          <button
            v-for="option in ccsClientOptions"
            :key="`${option.value}-${option.codexContextProfile || 'default'}`"
            @click="handleCcsClientSelect(option.value, option.codexContextProfile)"
            class="group flex flex-col items-center gap-2.5 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-700 dark:focus-visible:ring-offset-dark-900"
          >
            <CcsClientIcon
              :client="option.value"
              class="h-14 w-14 transition-transform duration-200 group-hover:scale-[1.04]"
            />
            <span class="font-medium text-gray-900 dark:text-white">{{ option.label }}</span>
            <span class="text-center text-xs text-gray-500 dark:text-gray-400">{{ option.description }}</span>
          </button>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('keys.ccsClientSelect.compatibilityNote') }}
        </p>
        <p
          v-if="ccsHasClaudeCodeTarget"
          class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
        >
          {{ t('keys.ccsClientSelect.claudeDesktopManualNotice') }}
        </p>
      </div>
      <template #footer>
        <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          <button
            type="button"
            class="btn btn-secondary"
            data-testid="ccs-open-diagnostics"
            @click="openCcsDiagnostics(false)"
          >
            <Icon name="questionCircle" size="sm" class="mr-2" />
            {{ t('keys.ccsDiagnostics.helpButton') }}
          </button>
          <button @click="closeCcsClientSelect" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- CC Switch beginner-friendly diagnostics -->
    <BaseDialog
      :show="showCcsDiagnostics"
      :title="t('keys.ccsDiagnostics.title')"
      width="normal"
      @close="closeCcsDiagnostics"
    >
      <div class="space-y-5">
        <div
          v-if="ccsDiagnosticsAutoPrompt"
          class="flex gap-3 rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-200"
        >
          <Icon name="exclamationCircle" size="md" class="mt-0.5 shrink-0" />
          <p>{{ t('keys.ccsDiagnostics.autoPrompt') }}</p>
        </div>

        <p class="text-sm leading-6 text-gray-600 dark:text-gray-300">
          {{ t('keys.ccsDiagnostics.description') }}
        </p>

        <div class="grid grid-cols-2 rounded-xl bg-gray-100 p-1 dark:bg-dark-700" role="tablist">
          <button
            type="button"
            role="tab"
            :aria-selected="ccsDiagnosticPlatform === 'windows'"
            data-testid="ccs-platform-windows"
            :class="[
              'rounded-lg px-3 py-2 text-sm font-medium transition-colors',
              ccsDiagnosticPlatform === 'windows'
                ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-800 dark:text-primary-400'
                : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
            ]"
            @click="selectCcsDiagnosticPlatform('windows')"
          >
            Windows
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="ccsDiagnosticPlatform === 'macos'"
            data-testid="ccs-platform-macos"
            :class="[
              'rounded-lg px-3 py-2 text-sm font-medium transition-colors',
              ccsDiagnosticPlatform === 'macos'
                ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-800 dark:text-primary-400'
                : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
            ]"
            @click="selectCcsDiagnosticPlatform('macos')"
          >
            Mac
          </button>
        </div>

        <ol class="space-y-3">
          <li class="flex gap-3">
            <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary-100 text-sm font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">1</span>
            <div>
              <p class="font-medium text-gray-900 dark:text-white">
                {{ t(`keys.ccsDiagnostics.${ccsDiagnosticPlatform}.openTitle`) }}
              </p>
              <p class="mt-0.5 text-sm leading-5 text-gray-500 dark:text-gray-400">
                {{ t(`keys.ccsDiagnostics.${ccsDiagnosticPlatform}.openDescription`) }}
              </p>
            </div>
          </li>
          <li class="flex gap-3">
            <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary-100 text-sm font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">2</span>
            <div>
              <p class="font-medium text-gray-900 dark:text-white">
                {{ t('keys.ccsDiagnostics.copyTitle') }}
              </p>
              <p class="mt-0.5 text-sm leading-5 text-gray-500 dark:text-gray-400">
                {{ t('keys.ccsDiagnostics.copyDescription') }}
              </p>
            </div>
          </li>
          <li class="flex gap-3">
            <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary-100 text-sm font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">3</span>
            <div>
              <p class="font-medium text-gray-900 dark:text-white">
                {{ t('keys.ccsDiagnostics.runTitle') }}
              </p>
              <p class="mt-0.5 text-sm leading-5 text-gray-500 dark:text-gray-400">
                {{ t(`keys.ccsDiagnostics.${ccsDiagnosticPlatform}.runDescription`) }}
              </p>
            </div>
          </li>
        </ol>

        <div class="overflow-hidden rounded-xl border border-gray-200 bg-gray-950 dark:border-dark-600">
          <div class="flex items-center justify-between border-b border-white/10 px-3 py-2">
            <span class="text-xs font-medium text-gray-300">
              {{ t('keys.ccsDiagnostics.commandLabel') }}
            </span>
            <button
              type="button"
              data-testid="ccs-copy-diagnostic-command"
              class="inline-flex items-center rounded-lg bg-white/10 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-white/20"
              @click="copyCcsDiagnosticCommand"
            >
              <Icon :name="ccsDiagnosticCopied ? 'check' : 'copy'" size="sm" class="mr-1.5" />
              {{ ccsDiagnosticCopied ? t('keys.ccsDiagnostics.copied') : t('keys.ccsDiagnostics.copyCommand') }}
            </button>
          </div>
          <pre
            data-testid="ccs-diagnostic-command"
            class="overflow-x-auto whitespace-pre-wrap break-all p-3 font-mono text-xs leading-5 text-emerald-300"
          ><code>{{ ccsDiagnosticCommand }}</code></pre>
        </div>

        <div class="rounded-xl bg-emerald-50 p-3 text-sm leading-6 text-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300">
          <p class="font-medium">{{ t('keys.ccsDiagnostics.automaticTitle') }}</p>
          <p>{{ t('keys.ccsDiagnostics.automaticDescription') }}</p>
        </div>

        <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
          {{ t('keys.ccsDiagnostics.privacyNote') }}
        </p>
      </div>
      <template #footer>
        <div class="flex w-full justify-end">
          <button type="button" class="btn btn-secondary" @click="closeCcsDiagnostics">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

  </AppLayout>
</template>

<script setup lang="ts">
	import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
	import { useRoute, useRouter } from 'vue-router'
	import { useI18n } from 'vue-i18n'
	import { useAppStore } from '@/stores/app'
	import { useOnboardingStore } from '@/stores/onboarding'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { publicGroupDisplayName } from '@/utils/groupDisplayName'
	import { useClipboard } from '@/composables/useClipboard'

const { t } = useI18n()
import { keysAPI, authAPI, usageAPI, userGroupsAPI } from '@/api'
import { getGatewayModels } from '@/api/gatewayModels'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
	import DataTable from '@/components/common/DataTable.vue'
	import Pagination from '@/components/common/Pagination.vue'
	import BaseDialog from '@/components/common/BaseDialog.vue'
	import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
	import EmptyState from '@/components/common/EmptyState.vue'
	import Select from '@/components/common/Select.vue'
	import SearchInput from '@/components/common/SearchInput.vue'
	import Icon from '@/components/icons/Icon.vue'
import UseKeyModal from '@/components/keys/UseKeyModal.vue'
import ClientSetupModal from '@/components/keys/ClientSetupModal.vue'
	import CcsClientIcon from '@/components/keys/CcsClientIcon.vue'
	import KeyGroupMultiSelect from '@/components/keys/KeyGroupMultiSelect.vue'
import { defaultKeyGroupIds } from '@/utils/keyGroupSelection'
import KeyGroupSelector from '@/components/keys/KeyGroupSelector.vue'
	import type { ApiKey, Group, PublicSettings, SubscriptionType, GroupPlatform } from '@/types'
import type { GroupCacheStats } from '@/api/groups'
import type { Column } from '@/components/common/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'
import { formatDateTime } from '@/utils/format'
import {
  buildCcsImportDeeplink,
  getCompatibleCcsTargets,
  type CcsImportTarget,
  type CodexContextProfile
} from '@/utils/ccSwitchImport'
import {
  buildCcsDiagnosticCommand,
  detectCcsDiagnosticPlatform,
  type CcsDiagnosticPlatform
} from '@/utils/ccSwitchDiagnostics'
import {
  classifyGroupOptionFamily,
  type GroupOptionFamilyId,
  type GroupOptionSectionId
} from '@/utils/groupOptionSections'
import {
  resolveGroupDisplayProtocol,
  type GroupDisplayProtocol
} from '@/utils/groupDisplayProtocol'
import { resolveGroupDisplayModel } from '@/utils/groupDisplayModel'

// Helper to format date for datetime-local input
const formatDateTimeLocal = (isoDate: string): string => {
  const date = new Date(isoDate)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

interface GroupOption {
  [key: string]: unknown
  value: number
  label: string
  description: string | null
  rate: number
  userRate: number | null
  subscriptionType: SubscriptionType
  platform: GroupPlatform
  displayProtocol: GroupDisplayProtocol
  displayModel: string
  cacheHitRatePct: number | null
  cacheWindowDays: number
  groupKey: GroupOptionSectionId
  familyKey: GroupOptionFamilyId
}

type CcsClientOption = {
  value: CcsImportTarget
  label: string
  description: string
  codexContextProfile?: CodexContextProfile
}

const appStore = useAppStore()
const route = useRoute()
const router = useRouter()
const onboardingStore = useOnboardingStore()
const subscriptionStore = useSubscriptionStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('common.name'), sortable: true },
  { key: 'key', label: t('keys.apiKey'), sortable: false },
  { key: 'group', label: t('keys.group'), sortable: false },
  { key: 'usage', label: t('keys.usage'), sortable: false },
  { key: 'rate_limit', label: t('keys.rateLimitColumn'), sortable: false },
  { key: 'expires_at', label: t('keys.expiresAt'), sortable: true },
  { key: 'status', label: t('common.status'), sortable: true },
  { key: 'last_used_at', label: t('keys.lastUsedAt'), sortable: true },
  { key: 'created_at', label: t('keys.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), sortable: false }
])

const apiKeys = ref<ApiKey[]>([])
const groups = ref<Group[]>([])
const groupsLoading = ref(true)
const groupsLoadedScope = ref<string | null>(null)
const keyGroupMode = ref<'single' | 'multi'>('multi')
const selectedGroupIds = ref<number[]>([])
const multiGroupOptions = computed(() => groups.value.filter(group => group.platform !== 'universal'))
const loading = ref(false)
const submitting = ref(false)
const now = ref(new Date())
let resetTimer: ReturnType<typeof setInterval> | null = null
const usageStats = ref<Record<string, BatchApiKeyUsageStats>>({})
const userGroupRates = ref<Record<number, number>>({})
const groupCacheStats = ref<Record<number, GroupCacheStats>>({})
const groupCacheWindowDays = ref(7)

const pagination = ref({
  page: 1,
  page_size: 10,
  total: 0,
  pages: 0
})

// Filter state
const filterSearch = ref('')
const filterStatus = ref('')
const filterGroupId = ref<string | number>('')
const activeScope = ref<'personal' | 'team'>(route.query.scope === 'team' ? 'team' : 'personal')

const showCreateModal = ref(false)
watch(showCreateModal, (show) => {
  if (show) { keyGroupMode.value = 'multi'; selectedGroupIds.value = defaultKeyGroupIds(groups.value) }
})
const showEditModal = ref(false)
const showDeleteDialog = ref(false)
const showResetQuotaDialog = ref(false)
const showResetRateLimitDialog = ref(false)
const showUseKeyModal = ref(false)
const showClientSetup = ref(false)
const clientSetupRow = ref<ApiKey | null>(null)
const showCcsClientSelect = ref(false)
const showCcsDiagnostics = ref(false)
const pendingCcsRow = ref<ApiKey | null>(null)
const ccsDiagnosticPlatform = ref<CcsDiagnosticPlatform>('windows')
const ccsDiagnosticCopied = ref(false)
const ccsDiagnosticsAutoPrompt = ref(false)
const selectedKey = ref<ApiKey | null>(null)
const copiedKeyId = ref<number | null>(null)
const copiedBaseUrl = ref(false)
const publicSettings = ref<PublicSettings | null>(null)
let abortController: AbortController | null = null
let ccsLaunchFallbackTimer: ReturnType<typeof setTimeout> | null = null
let ccsLaunchObserved = false

const groupCacheHitRateEnabled = computed(() => publicSettings.value?.group_cache_hit_rate_enabled === true)
const displayApiBaseUrl = computed(() => {
  const configuredBaseUrl = publicSettings.value?.api_base_url?.trim()
  const fallbackBaseUrl = typeof window !== 'undefined' ? window.location.origin : ''
  return (configuredBaseUrl || fallbackBaseUrl).replace(/\/+$/, '')
})

const ccsClientOptions = computed<CcsClientOption[]>(() => {
  const platform = pendingCcsRow.value?.group?.platform || 'anthropic'
  const allowMessagesDispatch = pendingCcsRow.value?.group?.allow_messages_dispatch === true
  const options: CcsClientOption[] = getCompatibleCcsTargets(platform, allowMessagesDispatch).map((target) => {
    switch (target) {
      case 'claude':
        return {
          value: target,
          label: t('keys.ccsClientSelect.claudeCodeCli'),
          description: t('keys.ccsClientSelect.claudeCodeCliDesc')
        }
      case 'codex':
        return {
          value: target,
          label: t('keys.ccsClientSelect.codex'),
          description: t('keys.ccsClientSelect.codexDesc')
        }
      case 'grokbuild':
        return {
          value: target,
          label: t('keys.ccsClientSelect.grokBuild'),
          description: t('keys.ccsClientSelect.grokBuildDesc')
        }
      case 'opencode':
        return {
          value: target,
          label: t('keys.ccsClientSelect.opencode'),
          description: t('keys.ccsClientSelect.opencodeDesc')
        }
      case 'openclaw':
        return {
          value: target,
          label: t('keys.ccsClientSelect.openclaw'),
          description: t('keys.ccsClientSelect.openclawDesc')
        }
      case 'hermes':
        return {
          value: target,
          label: t('keys.ccsClientSelect.hermes'),
          description: t('keys.ccsClientSelect.hermesDesc')
        }
      case 'gemini':
        return {
          value: target,
          label: t('keys.ccsClientSelect.geminiCli'),
          description: t('keys.ccsClientSelect.geminiCliDesc')
      }
    }
  })
  if (platform !== 'openai') return options

  return options.flatMap<CcsClientOption>((option) => option.value === 'codex'
    ? [
        {
          ...option,
          label: t('keys.ccsClientSelect.codexStandard'),
          description: t('keys.ccsClientSelect.codexStandardDesc'),
          codexContextProfile: 'standard' as const
        },
        {
          ...option,
          label: t('keys.ccsClientSelect.codexLong'),
          description: t('keys.ccsClientSelect.codexLongDesc'),
          codexContextProfile: 'long' as const
        }
      ]
    : [option]
  )
})
const ccsHasClaudeCodeTarget = computed(() =>
  ccsClientOptions.value.some((option) => option.value === 'claude')
)
const ccsDiagnosticCommand = computed(() =>
  buildCcsDiagnosticCommand(ccsDiagnosticPlatform.value, window.location.origin)
)

const formData = ref({
  name: '',
  group_id: null as number | null,
  status: 'active' as 'active' | 'inactive',
  use_custom_key: false,
  custom_key: '',
  enable_ip_restriction: false,
  ip_whitelist: '',
  ip_blacklist: '',
  // Quota settings (empty = unlimited)
  enable_quota: false,
  quota: null as number | null,
  // Rate limit settings
  enable_rate_limit: false,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null,
  enable_expiration: false,
  expiration_preset: '30' as '7' | '30' | '90' | 'custom',
  expiration_date: ''
})

// 自定义Key验证
const customKeyError = computed(() => {
  if (!formData.value.use_custom_key || !formData.value.custom_key) {
    return ''
  }
  const key = formData.value.custom_key
  if (key.length < 16) {
    return t('keys.customKeyTooShort')
  }
  // 检查字符：只允许字母、数字、下划线、连字符
  if (!/^[a-zA-Z0-9_-]+$/.test(key)) {
    return t('keys.customKeyInvalidChars')
  }
  return ''
})

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

// Filter dropdown options
const groupFilterOptions = computed(() => [
  { value: '', label: t('keys.allGroups') },
  { value: 0, label: t('keys.noGroup') },
  ...groups.value.map((g) => ({ value: g.id, label: g.name }))
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('keys.allStatus') },
  { value: 'active', label: t('keys.status.active') },
  { value: 'inactive', label: t('keys.status.inactive') },
  { value: 'quota_exhausted', label: t('keys.status.quota_exhausted') },
  { value: 'expired', label: t('keys.status.expired') }
])

const onFilterChange = () => {
  pagination.value.page = 1
  loadApiKeys()
}

const onGroupFilterChange = (value: string | number | boolean | null) => {
  filterGroupId.value = value as string | number
  onFilterChange()
}

const onStatusFilterChange = (value: string | number | boolean | null) => {
  filterStatus.value = value as string
  onFilterChange()
}

// Convert groups to selector options, then organize them by billing mode.
const baseGroupOptions = computed<GroupOption[]>(() =>
  groups.value.map((group) => {
    const cacheStats = groupCacheStats.value[group.id]
    const subscriptionType = group.subscription_type
    const label = publicGroupDisplayName(group.name)
    return {
      value: group.id,
      label,
      description: group.description,
      rate: group.rate_multiplier,
      userRate: userGroupRates.value[group.id] ?? null,
      subscriptionType,
      platform: group.platform,
      displayProtocol: resolveGroupDisplayProtocol({
        name: label,
        platform: group.platform,
        defaultMappedModel: group.default_mapped_model,
        allowMessagesDispatch: group.allow_messages_dispatch
      }),
      displayModel: resolveGroupDisplayModel({
        name: label,
        platform: group.platform,
        defaultMappedModel: group.default_mapped_model
      }),
      cacheHitRatePct: groupCacheHitRateEnabled.value && cacheStats?.has_data ? cacheStats.hit_rate_pct : null,
      cacheWindowDays: groupCacheWindowDays.value,
      groupKey: subscriptionType === 'subscription' || subscriptionType === 'credit'
        ? 'monthly'
        : 'payg',
      familyKey: classifyGroupOptionFamily({ label, platform: group.platform })
    }
  })
)

const getKeyGroupFallbackOption = (key: ApiKey): GroupOption | null => {
  const group = key.group
  if (!group) return null
  const existing = baseGroupOptions.value.find((option) => option.value === group.id)
  if (existing) return existing
  const subscriptionType = group.subscription_type
  const label = publicGroupDisplayName(group.name)
  return {
    value: group.id,
    label,
    description: group.description,
    rate: group.rate_multiplier,
    userRate: userGroupRates.value[group.id] ?? null,
    subscriptionType,
    platform: group.platform,
    displayProtocol: resolveGroupDisplayProtocol({
      name: label,
      platform: group.platform,
      defaultMappedModel: group.default_mapped_model,
      allowMessagesDispatch: group.allow_messages_dispatch
    }),
    displayModel: resolveGroupDisplayModel({
      name: label,
      platform: group.platform,
      defaultMappedModel: group.default_mapped_model
    }),
    cacheHitRatePct: null,
    cacheWindowDays: groupCacheWindowDays.value,
    groupKey: subscriptionType === 'subscription' || subscriptionType === 'credit' ? 'monthly' : 'payg',
    familyKey: classifyGroupOptionFamily({ label, platform: group.platform })
  }
}

const maskKey = (key: string): string => {
  if (key.length <= 12) return key
  return `${key.slice(0, 8)}...${key.slice(-4)}`
}

const copyToClipboard = async (text: string, keyId: number) => {
  const success = await clipboardCopy(text, t('keys.copied'))
  if (success) {
    copiedKeyId.value = keyId
    setTimeout(() => {
      copiedKeyId.value = null
    }, 800)
  }
}

const selectBaseUrl = (event: FocusEvent) => {
  const target = event.target as HTMLInputElement
  target.select()
}

const copyApiBaseUrl = async () => {
  const success = await clipboardCopy(displayApiBaseUrl.value, t('keys.baseUrlCopied'))
  if (success) {
    copiedBaseUrl.value = true
    setTimeout(() => {
      copiedBaseUrl.value = false
    }, 1200)
  }
}

const codexSetupGroupIds = new Set([6, 58, 59])
const canOpenCodexSetup = (row: ApiKey): boolean => (
  row.status === 'active' &&
  !row.group_ids?.length &&
  Boolean(row.group && codexSetupGroupIds.has(row.group.id))
)

const openClientSetup = (row: ApiKey) => {
  clientSetupRow.value = row
  showClientSetup.value = true
}

const closeClientSetup = () => {
  showClientSetup.value = false
  clientSetupRow.value = null
}

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const { name, code } = error as { name?: string; code?: string }
  return name === 'AbortError' || code === 'ERR_CANCELED'
}

const loadApiKeys = async () => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  const { signal } = controller
  loading.value = true
  try {
    // Build filters
    const filters: { search?: string; status?: string; group_id?: number | string; scope?: 'personal' | 'team' } = { scope: activeScope.value }
    if (filterSearch.value) filters.search = filterSearch.value
    if (filterStatus.value) filters.status = filterStatus.value
    if (filterGroupId.value !== '') filters.group_id = filterGroupId.value

    const response = await keysAPI.list(pagination.value.page, pagination.value.page_size, filters, {
      signal
    })
    if (signal.aborted) return
    apiKeys.value = response.items
    pagination.value.total = response.total
    pagination.value.pages = response.pages

    // Load usage stats for all API keys in the list
    if (response.items.length > 0) {
      const keyIds = response.items.map((k) => k.id)
      try {
        const usageResponse = await usageAPI.getDashboardApiKeysUsage(keyIds, { signal })
        if (signal.aborted) return
        usageStats.value = usageResponse.stats
      } catch (e) {
        if (!isAbortError(e)) {
          console.error('Failed to load usage stats:', e)
        }
      }
    }
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    appStore.showError(t('keys.failedToLoad'))
  } finally {
    if (abortController === controller) {
      loading.value = false
    }
  }
}

const loadGroups = async () => {
  groupsLoading.value = true
  const requestedScope = activeScope.value
  try {
    const loaded = await userGroupsAPI.getAvailable(requestedScope)
    if (requestedScope !== activeScope.value) return
    groups.value = loaded
    groupsLoadedScope.value = requestedScope
    if (showCreateModal.value) selectedGroupIds.value = defaultKeyGroupIds(groups.value)
  } catch (error) {
    if (requestedScope === activeScope.value) {
      groups.value = []
      selectedGroupIds.value = []
      groupsLoadedScope.value = null
    }
    console.error('Failed to load groups:', error)
  } finally {
    if (requestedScope === activeScope.value) groupsLoading.value = false
  }
}

const loadUserGroupRates = async () => {
  try {
    userGroupRates.value = await userGroupsAPI.getUserGroupRates()
  } catch (error) {
    console.error('Failed to load user group rates:', error)
  }
}

const loadGroupCacheStats = async () => {
  try {
    const response = await userGroupsAPI.getCacheStats(groupCacheWindowDays.value)
    groupCacheWindowDays.value = response.window_days
    groupCacheStats.value = response.groups || {}
  } catch (error) {
    groupCacheStats.value = {}
    console.error('Failed to load group cache stats:', error)
  }
}

const loadPublicSettings = async () => {
  try {
    publicSettings.value = await authAPI.getPublicSettings()
    if (publicSettings.value.group_cache_hit_rate_enabled) {
      await loadGroupCacheStats()
    } else {
      groupCacheStats.value = {}
    }
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
}

const openUseKeyModal = (key: ApiKey) => {
  selectedKey.value = key
  showUseKeyModal.value = true
}

const closeUseKeyModal = () => {
  showUseKeyModal.value = false
  selectedKey.value = null
}

const handlePageChange = (page: number) => {
  pagination.value.page = page
  loadApiKeys()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  loadApiKeys()
}

const editKey = (key: ApiKey) => {
  keyGroupMode.value = key.group_ids?.length ? 'multi' : 'single'
  selectedGroupIds.value = [...(key.group_ids || [])]
  selectedKey.value = key
  const hasIPRestriction = (key.ip_whitelist?.length > 0) || (key.ip_blacklist?.length > 0)
  const hasExpiration = !!key.expires_at
  formData.value = {
    name: key.name,
    group_id: key.group_id,
    status: key.status === 'quota_exhausted' || key.status === 'expired' ? 'inactive' : key.status,
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: hasIPRestriction,
    ip_whitelist: (key.ip_whitelist || []).join('\n'),
    ip_blacklist: (key.ip_blacklist || []).join('\n'),
    enable_quota: key.quota > 0,
    quota: key.quota > 0 ? key.quota : null,
    enable_rate_limit: (key.rate_limit_5h > 0) || (key.rate_limit_1d > 0) || (key.rate_limit_7d > 0),
    rate_limit_5h: key.rate_limit_5h || null,
    rate_limit_1d: key.rate_limit_1d || null,
    rate_limit_7d: key.rate_limit_7d || null,
    enable_expiration: hasExpiration,
    expiration_preset: 'custom',
    expiration_date: key.expires_at ? formatDateTimeLocal(key.expires_at) : ''
  }
  showEditModal.value = true
}

const toggleKeyStatus = async (key: ApiKey) => {
  const newStatus = key.status === 'active' ? 'inactive' : 'active'
  try {
    await keysAPI.toggleStatus(key.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('keys.keyEnabledSuccess') : t('keys.keyDisabledSuccess')
    )
    loadApiKeys()
  } catch (error) {
    appStore.showError(t('keys.failedToUpdateStatus'))
  }
}

const changeGroup = async (key: ApiKey, newGroupId: number | null) => {
  if (key.group_id === newGroupId) return

  try {
    await keysAPI.update(key.id, { group_id: newGroupId })
    appStore.showSuccess(t('keys.groupChangedSuccess'))
    loadApiKeys()
  } catch (error) {
    appStore.showError(t('keys.failedToChangeGroup'))
  }
}

const confirmDelete = (key: ApiKey) => {
  selectedKey.value = key
  showDeleteDialog.value = true
}

const handleSubmit = async () => {
  // Explicit empty selection must never mean unrestricted access.
  if (keyGroupMode.value === 'multi' && (groupsLoading.value || groupsLoadedScope.value !== activeScope.value || selectedGroupIds.value.length === 0)) {
    appStore.showError('请至少选择一个授权分组')
    return
  }
  if (keyGroupMode.value === 'single' && formData.value.group_id === null) {
    appStore.showError(t('keys.groupRequired'))
    return
  }

  // Validate custom key if enabled
  if (!showEditModal.value && formData.value.use_custom_key) {
    if (!formData.value.custom_key) {
      appStore.showError(t('keys.customKeyRequired'))
      return
    }
    if (customKeyError.value) {
      appStore.showError(customKeyError.value)
      return
    }
  }

  // Parse IP lists only if IP restriction is enabled
  const parseIPList = (text: string): string[] =>
    text.split('\n').map(ip => ip.trim()).filter(ip => ip.length > 0)
  const ipWhitelist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_whitelist) : []
  const ipBlacklist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_blacklist) : []

  // Calculate quota value (null/empty/0 = unlimited, stored as 0)
  const quota = formData.value.quota && formData.value.quota > 0 ? formData.value.quota : 0

  // Calculate expiration
  let expiresInDays: number | undefined
  let expiresAt: string | null | undefined
  if (formData.value.enable_expiration && formData.value.expiration_date) {
    if (!showEditModal.value) {
      // Create mode: calculate days from date
      const expDate = new Date(formData.value.expiration_date)
      const now = new Date()
      const diffDays = Math.ceil((expDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
      expiresInDays = diffDays > 0 ? diffDays : 1
    } else {
      // Edit mode: use custom date directly
      expiresAt = new Date(formData.value.expiration_date).toISOString()
    }
  } else if (showEditModal.value) {
    // Edit mode: if expiration disabled or date cleared, send empty string to clear
    expiresAt = ''
  }

  // Calculate rate limit values (send 0 when toggle is off)
  const rateLimitData = formData.value.enable_rate_limit ? {
    rate_limit_5h: formData.value.rate_limit_5h && formData.value.rate_limit_5h > 0 ? formData.value.rate_limit_5h : 0,
    rate_limit_1d: formData.value.rate_limit_1d && formData.value.rate_limit_1d > 0 ? formData.value.rate_limit_1d : 0,
    rate_limit_7d: formData.value.rate_limit_7d && formData.value.rate_limit_7d > 0 ? formData.value.rate_limit_7d : 0,
  } : { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 }

  submitting.value = true
  let shouldAdvanceKeyCreationTour = false
  try {
    if (showEditModal.value && selectedKey.value) {
      await keysAPI.update(selectedKey.value.id, {
        name: formData.value.name,
        ...(keyGroupMode.value === 'multi' ? { group_ids: selectedGroupIds.value } : { group_id: formData.value.group_id }),
        status: formData.value.status,
        ip_whitelist: ipWhitelist,
        ip_blacklist: ipBlacklist,
        quota: quota,
        expires_at: expiresAt,
        rate_limit_5h: rateLimitData.rate_limit_5h,
        rate_limit_1d: rateLimitData.rate_limit_1d,
        rate_limit_7d: rateLimitData.rate_limit_7d,
      })
      appStore.showSuccess(t('keys.keyUpdatedSuccess'))
    } else {
      const customKey = formData.value.use_custom_key ? formData.value.custom_key : undefined
      await keysAPI.create(
        formData.value.name,
        formData.value.group_id,
        customKey,
        ipWhitelist,
        ipBlacklist,
        quota,
        expiresInDays,
        rateLimitData,
        activeScope.value,
        keyGroupMode.value === 'multi' ? selectedGroupIds.value : undefined
      )
      appStore.showSuccess(t('keys.keyCreatedSuccess'))
      // Only advance tour if active, on submit step, and creation succeeded
      shouldAdvanceKeyCreationTour = onboardingStore.isCurrentStep('[data-tour="key-form-submit"]')
      if (shouldAdvanceKeyCreationTour) {
        filterSearch.value = ''
        filterStatus.value = ''
        filterGroupId.value = ''
        pagination.value.page = 1
      }
    }
    closeModals()
    await loadApiKeys()
    if (shouldAdvanceKeyCreationTour) {
      onboardingStore.nextStep(500)
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToSave')
    appStore.showError(errorMsg)
    // Don't advance tour on error
  } finally {
    submitting.value = false
  }
}

/**
 * 处理删除 API Key 的操作
 * 优化：错误处理改进，优先显示后端返回的具体错误消息（如权限不足等），
 * 若后端未返回消息则显示默认的国际化文本
 */
const handleDelete = async () => {
  if (!selectedKey.value) return

  try {
    await keysAPI.delete(selectedKey.value.id)
    appStore.showSuccess(t('keys.keyDeletedSuccess'))
    showDeleteDialog.value = false
    loadApiKeys()
  } catch (error: any) {
    // 优先使用后端返回的错误消息，提供更具体的错误信息给用户
    const errorMsg = error?.message || t('keys.failedToDelete')
    appStore.showError(errorMsg)
  }
}

const closeModals = () => {
  showCreateModal.value = false
  showEditModal.value = false
  selectedKey.value = null
  formData.value = {
    name: '',
    group_id: null,
    status: 'active',
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: false,
    ip_whitelist: '',
    ip_blacklist: '',
    enable_quota: false,
    quota: null,
    enable_rate_limit: false,
    rate_limit_5h: null,
    rate_limit_1d: null,
    rate_limit_7d: null,
    enable_expiration: false,
    expiration_preset: '30',
    expiration_date: ''
  }
}

// Show reset quota confirmation dialog
const confirmResetQuota = () => {
  showResetQuotaDialog.value = true
}

// Set expiration date based on quick select days
const setExpirationDays = (days: number) => {
  formData.value.expiration_preset = days.toString() as '7' | '30' | '90'
  const expDate = new Date()
  expDate.setDate(expDate.getDate() + days)
  formData.value.expiration_date = formatDateTimeLocal(expDate.toISOString())
}

// Reset quota used for an API key
const resetQuotaUsed = async () => {
  if (!selectedKey.value) return
  showResetQuotaDialog.value = false
  try {
    await keysAPI.update(selectedKey.value.id, { reset_quota: true })
    appStore.showSuccess(t('keys.quotaResetSuccess'))
    // Update local state
    if (selectedKey.value) {
      selectedKey.value.quota_used = 0
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetQuota')
    appStore.showError(errorMsg)
  }
}

// Show reset rate limit confirmation dialog (from edit modal)
const confirmResetRateLimit = () => {
  showResetRateLimitDialog.value = true
}

// Show reset rate limit confirmation dialog (from table row)
const confirmResetRateLimitFromTable = (row: ApiKey) => {
  selectedKey.value = row
  showResetRateLimitDialog.value = true
}

// Reset rate limit usage for an API key
const resetRateLimitUsage = async () => {
  if (!selectedKey.value) return
  showResetRateLimitDialog.value = false
  try {
    await keysAPI.update(selectedKey.value.id, { reset_rate_limit_usage: true })
    appStore.showSuccess(t('keys.rateLimitResetSuccess'))
    // Refresh key data
    await loadApiKeys()
    // Update the editing key with fresh data
    const refreshedKey = apiKeys.value.find(k => k.id === selectedKey.value!.id)
    if (refreshedKey) {
      selectedKey.value = refreshedKey
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetRateLimit')
    appStore.showError(errorMsg)
  }
}

const getCcsTargetsForKey = (row: ApiKey): CcsImportTarget[] => {
  if (row.group_ids?.length) return [] // Old one-click profiles assume exactly one group.
  const platform = row.group?.platform || 'anthropic'
  return getCompatibleCcsTargets(platform, row.group?.allow_messages_dispatch === true)
}

const canImportToCcs = (row: ApiKey): boolean => getCcsTargetsForKey(row).length > 0

const selectCcsDiagnosticPlatform = (platform: CcsDiagnosticPlatform) => {
  ccsDiagnosticPlatform.value = platform
  ccsDiagnosticCopied.value = false
}

const openCcsDiagnostics = (autoPrompt = false) => {
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
  ccsDiagnosticPlatform.value = detectCcsDiagnosticPlatform() || 'windows'
  ccsDiagnosticCopied.value = false
  ccsDiagnosticsAutoPrompt.value = autoPrompt
  showCcsDiagnostics.value = true
}

const closeCcsDiagnostics = () => {
  showCcsDiagnostics.value = false
  ccsDiagnosticsAutoPrompt.value = false
}

const copyCcsDiagnosticCommand = async () => {
  const success = await clipboardCopy(
    ccsDiagnosticCommand.value,
    t('keys.ccsDiagnostics.commandCopied')
  )
  if (success) {
    ccsDiagnosticCopied.value = true
  }
}

const cleanupCcsLaunchWatch = () => {
  if (ccsLaunchFallbackTimer) {
    clearTimeout(ccsLaunchFallbackTimer)
    ccsLaunchFallbackTimer = null
  }
  window.removeEventListener('blur', markCcsLaunchObserved)
  document.removeEventListener('visibilitychange', markCcsLaunchObserved)
}

function markCcsLaunchObserved() {
  if (!document.hasFocus() || document.hidden) {
    ccsLaunchObserved = true
    cleanupCcsLaunchWatch()
  }
}

const watchCcsLaunch = () => {
  cleanupCcsLaunchWatch()
  ccsLaunchObserved = false
  window.addEventListener('blur', markCcsLaunchObserved)
  document.addEventListener('visibilitychange', markCcsLaunchObserved)
  ccsLaunchFallbackTimer = setTimeout(() => {
    cleanupCcsLaunchWatch()
    if (!ccsLaunchObserved) {
      openCcsDiagnostics(true)
    }
  }, 4500)
}

const importToCcswitch = (row: ApiKey) => {
  if (!canImportToCcs(row)) {
    appStore.showError(t('keys.ccsClientSelect.noCompatibleTargets'))
    return
  }
  pendingCcsRow.value = row
  showCcsClientSelect.value = true
}

const openChatbotWithKey = async (row: ApiKey) => {
  const chatbotUrl = publicSettings.value?.chatbot_url?.trim()
  if (!chatbotUrl) {
    appStore.showError('Chatbot 地址未配置')
    return
  }
  if (row.status !== 'active') {
    appStore.showError('请先启用该 APIKey')
    return
  }

  try {
    const { ticket } = await authAPI.issueSSOTicket(row.id)
    const base = chatbotUrl.replace(/\/+$/, '')
    window.open(`${base}/sso?ticket=${encodeURIComponent(ticket)}`, '_blank', 'noopener,noreferrer')
  } catch (error: any) {
    appStore.showError(error?.message || '打开 Chatbot 失败')
  }
}

const executeCcsImport = async (
  row: ApiKey,
  clientType: CcsImportTarget,
  codexContextProfile: CodexContextProfile = 'standard'
) => {
  const baseUrl = publicSettings.value?.api_base_url || window.location.origin

  try {
    const needsOpenAIModels = row.group?.platform === 'openai' && (
      clientType === 'codex' ||
      clientType === 'opencode' ||
      clientType === 'openclaw' ||
      clientType === 'hermes'
    )
    let availableModels: readonly string[] | undefined
    if (needsOpenAIModels) {
      availableModels = await getGatewayModels(baseUrl, row.key)
      if (!availableModels?.length) {
        throw new Error('CC Switch import could not load any active models for this OpenAI group')
      }
    }

    const deeplink = buildCcsImportDeeplink({
      key: row,
      target: clientType,
      apiBaseUrl: baseUrl,
      siteName: publicSettings.value?.site_name,
      availableModels,
      codexContextProfile
    })
    watchCcsLaunch()
    window.open(deeplink, '_self')
  } catch (error) {
    cleanupCcsLaunchWatch()
    console.error('Failed to build CC Switch import link', error)
    appStore.showError(t('keys.ccsClientSelect.importFailed'))
  }
}

const handleCcsClientSelect = async (
  clientType: CcsImportTarget,
  codexContextProfile: CodexContextProfile = 'standard'
) => {
  const row = pendingCcsRow.value
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
  if (!row) return

  // CC Switch 3.19.2 collapses Grok Build deeplinks to one model. The one-time
  // compatibility flow writes the dual-model config, atomically upserts one
  // neutral Grok Provider in CC Switch's native store, then reopens the app.
  if (clientType === 'grokbuild') {
    appStore.showWarning(t('keys.configureClientPausedHint'), 6000)
    return
  }
  await executeCcsImport(row, clientType, codexContextProfile)
}

const closeCcsClientSelect = () => {
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

function formatResetTime(resetAt: string | null): string {
  if (!resetAt) return ''
  const diff = new Date(resetAt).getTime() - now.value.getTime()
  if (diff <= 0) return t('keys.resetNow')
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const mins = Math.floor((diff % 3600000) / 60000)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

onMounted(() => {
  loadApiKeys()
  loadGroups()
  loadUserGroupRates()
  loadPublicSettings()
  resetTimer = setInterval(() => { now.value = new Date() }, 60000)
})

const setScope = async (scope: 'personal' | 'team') => {
  if (activeScope.value === scope) return
  closeModals()
  groups.value = []
  selectedGroupIds.value = []
  groupsLoadedScope.value = null
  activeScope.value = scope
  pagination.value.page = 1
  filterGroupId.value = ''
  await router.replace({ query: { ...route.query, scope } })
  await Promise.all([loadGroups(), loadApiKeys()])
}

onUnmounted(() => {
  cleanupCcsLaunchWatch()
  if (resetTimer) clearInterval(resetTimer)
})
</script>
