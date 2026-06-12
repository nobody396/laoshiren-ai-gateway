<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-4">
          <section class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ t('admin.suppliers.monitorTitle', '供应商探针') }}
                  </h2>
                  <span :class="probeBadgeClass(snapshotOverallStatus)">
                    {{ probeStatusLabel(snapshotOverallStatus) }}
                  </span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">
                    {{ probeSnapshot ? formatDateTime(probeSnapshot.generated_at) : t('common.noData', '暂无数据') }}
                  </span>
                </div>
              </div>

              <div class="flex flex-wrap items-center gap-2">
                <button class="btn btn-secondary" :disabled="snapshotLoading" @click="loadProbeSnapshot">
                  <Icon name="refresh" size="md" :class="snapshotLoading ? 'animate-spin' : ''" />
                </button>
                <button class="btn btn-secondary" :disabled="syncingAccounts" @click="syncAccountsToSuppliers">
                  <Icon name="download" size="md" class="mr-2" />
                  {{ t('admin.suppliers.syncAccounts', '同步账号') }}
                </button>
                <button class="btn btn-secondary" :disabled="bulkSaving" @click="bulkToggleProbes(true)">
                  {{ t('admin.suppliers.enableAllProbes', '开启全部探针') }}
                </button>
                <button class="btn btn-secondary" :disabled="bulkSaving" @click="bulkToggleProbes(false)">
                  {{ t('admin.suppliers.disableAllProbes', '关闭全部探针') }}
                </button>
                <button class="btn btn-primary" :disabled="batchProbing" @click="runAllEnabledProbes">
                  <Icon name="refresh" size="md" :class="batchProbing ? 'animate-spin mr-2' : 'mr-2'" />
                  {{ t('admin.suppliers.probeAll', '检测已启用') }}
                </button>
              </div>
            </div>

            <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
              <div
                v-for="card in probeMetricCards"
                :key="card.label"
                class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</div>
                <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ card.hint }}</div>
              </div>
            </div>

            <div class="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(320px,0.7fr)]">
              <div>
                <div class="mb-2 flex items-center justify-between">
                  <div class="text-sm font-medium text-gray-800 dark:text-gray-200">
                    {{ t('admin.suppliers.hourlyStability', '每日小时稳定性') }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.suppliers.hourlyWindow', { days: probeSnapshot?.days || 7 }, `最近 ${probeSnapshot?.days || 7} 天`) }}
                  </div>
                </div>
                <div class="grid grid-cols-6 gap-1 sm:grid-cols-12">
                  <div
                    v-for="bucket in hourlyBuckets"
                    :key="bucket.hour"
                    class="h-12 rounded border px-1 py-1 text-center text-[10px] leading-tight"
                    :class="hourBucketClass(bucket)"
                    :title="hourBucketTitle(bucket)"
                  >
                    <div>{{ `${String(bucket.hour).padStart(2, '0')}` }}</div>
                    <div class="mt-1 font-semibold">{{ bucket.total > 0 ? formatPercent(bucket.success_rate) : '-' }}</div>
                  </div>
                </div>
              </div>

              <div>
                <div class="mb-2 text-sm font-medium text-gray-800 dark:text-gray-200">
                  {{ t('admin.suppliers.riskHours', '更容易出事的时段') }}
                </div>
                <div class="space-y-2">
                  <div
                    v-for="bucket in riskyHours"
                    :key="bucket.hour"
                    class="flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900"
                  >
                    <span class="font-medium text-gray-800 dark:text-gray-200">{{ bucket.label }}</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ bucket.failed + bucket.degraded }}/{{ bucket.total }} · {{ formatPercent(bucket.success_rate) }}
                    </span>
                  </div>
                  <div v-if="riskyHours.length === 0" class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
                    {{ t('admin.suppliers.noRiskHours', '还没有足够探针数据') }}
                  </div>
                </div>
              </div>
            </div>

            <div class="mt-4 overflow-x-auto rounded-lg border border-gray-100 dark:border-dark-700">
              <div class="grid min-w-[920px] grid-cols-[minmax(180px,1fr)_minmax(160px,0.8fr)_minmax(220px,1.4fr)_90px_90px_110px] bg-gray-50 px-3 py-2 text-xs font-medium text-gray-500 dark:bg-dark-900 dark:text-gray-400">
                <span>{{ t('admin.suppliers.monitorSupplier', '供应商') }}</span>
                <span>{{ t('admin.suppliers.monitorGroups', '对应分组') }}</span>
                <span>{{ t('admin.suppliers.monitorSlots', '状态区块') }}</span>
                <span>{{ t('admin.suppliers.monitorWindowRate', '窗口成功率') }}</span>
                <span>{{ t('admin.suppliers.monitorLatency', '延迟') }}</span>
                <span>{{ t('admin.suppliers.monitorLast', '最近探针') }}</span>
              </div>
              <button
                v-for="item in monitoredSupplierRows"
                :key="item.id"
                type="button"
                class="grid min-w-[920px] w-full grid-cols-[minmax(180px,1fr)_minmax(160px,0.8fr)_minmax(220px,1.4fr)_90px_90px_110px] items-center border-t border-gray-100 px-3 py-2 text-left text-sm transition hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-900"
                @click="openProbeDetail(item)"
              >
                <div class="min-w-0">
                  <div class="truncate font-medium text-gray-800 dark:text-gray-200">{{ item.name }}</div>
                  <div class="truncate text-xs text-gray-500 dark:text-gray-400">{{ item.probe_model || '-' }}</div>
                </div>
                <div class="flex min-w-0 flex-wrap gap-1">
                  <GroupBadge
                    v-for="group in itemTargetGroups(item)"
                    :key="group.id"
                    :name="group.name"
                    :platform="group.platform"
                    :subscription-type="group.subscription_type"
                    :rate-multiplier="group.rate_multiplier"
                    class="max-w-28"
                  />
                  <span v-if="itemTargetGroups(item).length === 0" class="text-xs text-gray-400">-</span>
                </div>
                <div class="grid grid-cols-12 gap-1">
                  <span
                    v-for="slot in supplierTimelineSlots(item)"
                    :key="slot.key"
                    class="h-3 min-w-0 rounded-full"
                    :class="probeSlotClass(slot.status)"
                    :title="probeSlotTitle(slot)"
                  />
                </div>
                <span :class="probeBadgeClass(item.last_probe_status)">
                  {{ item.window_total > 0 ? formatPercent(item.window_success_rate) : probeStatusLabel(item.last_probe_status) }}
                </span>
                <span class="text-gray-600 dark:text-gray-300">
                  {{ item.window_average_latency_ms > 0 ? `${item.window_average_latency_ms}ms` : '-' }}
                </span>
                <span class="text-gray-500 dark:text-gray-400">{{ formatShortDateTime(item.last_probe_at) }}</span>
              </button>
              <div v-if="monitoredSupplierRows.length === 0" class="px-3 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.suppliers.noProbeRows', '暂无供应商探针数据') }}
              </div>
            </div>
          </section>

          <div class="flex flex-col justify-between gap-4 lg:flex-row lg:items-start">
            <div class="flex flex-1 flex-wrap items-center gap-3">
              <div class="relative w-full sm:w-72">
                <Icon
                  name="search"
                  size="md"
                  class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
                />
                <input
                  v-model="searchQuery"
                  type="text"
                  :placeholder="t('admin.suppliers.search', '搜索供应商、官网、Base URL...')"
                  class="input pl-10"
                  @input="handleSearch"
                />
              </div>

              <Select
                v-model="filters.status"
                :options="statusFilterOptions"
                :placeholder="t('admin.suppliers.allStatus', '全部状态')"
                class="w-40"
                @change="loadSuppliers"
              />
              <Select
                v-model="filters.probe_status"
                :options="probeStatusFilterOptions"
                :placeholder="t('admin.suppliers.allProbeStatus', '全部探针')"
                class="w-40"
                @change="loadSuppliers"
              />
            </div>

            <div class="flex w-full flex-shrink-0 flex-wrap items-center justify-end gap-3 lg:w-auto">
              <button
                class="btn btn-secondary"
                :disabled="loading"
                :title="t('common.refresh', '刷新')"
                @click="refreshSupplierPage"
              >
                <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
              </button>
              <button class="btn btn-primary" @click="openCreateDialog">
                <Icon name="plus" size="md" class="mr-2" />
                {{ t('admin.suppliers.create', '新增供应商') }}
              </button>
            </div>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="suppliers"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="updated_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #cell-name="{ row }">
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
              <a
                v-if="row.website_url"
                :href="withHttp(row.website_url)"
                target="_blank"
                rel="noopener noreferrer"
                class="block truncate text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400"
              >
                {{ row.website_url }}
              </a>
              <div v-if="row.base_url" class="truncate text-xs text-gray-500 dark:text-gray-400">
                {{ row.base_url }}
              </div>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span :class="statusBadgeClass(row.status)">{{ statusLabel(row.status) }}</span>
          </template>

          <template #cell-upstream_group="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ row.upstream_group || '-' }}</span>
          </template>

          <template #cell-target_groups="{ row }">
            <div class="flex min-w-36 flex-wrap gap-1">
              <span
                v-for="group in targetGroups(row)"
                :key="group.id"
                class="inline-flex max-w-32 items-center truncate rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300"
                :title="group.name"
              >
                {{ group.name }}
              </span>
              <span v-if="targetGroups(row).length === 0" class="text-sm text-gray-400">-</span>
            </div>
          </template>

          <template #cell-cost="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ formatCost(row.cost_rmb_per_usd) }}
            </span>
          </template>

          <template #cell-probe="{ row }">
            <div class="min-w-36 space-y-1">
              <div class="flex items-center gap-2">
                <span :class="probeBadgeClass(row.last_probe_status)">
                  {{ probeStatusLabel(row.last_probe_status) }}
                </span>
                <span v-if="!row.probe_enabled" class="text-xs text-gray-400">
                  {{ t('admin.suppliers.probeDisabled', '未启用') }}
                </span>
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ formatProbeSummary(row) }}
              </div>
              <div
                v-if="formatProbeDetail(row)"
                class="truncate text-xs text-gray-400 dark:text-gray-500"
                :title="row.last_probe_error || formatProbeDetail(row)"
              >
                {{ formatProbeDetail(row) }}
              </div>
            </div>
          </template>

          <template #cell-contact="{ row }">
            <div class="min-w-28 text-sm text-gray-700 dark:text-gray-300">
              <div>{{ contactPlatformLabel(row.contact_platform) }}</div>
              <div class="truncate text-xs text-gray-500 dark:text-gray-400">{{ row.contact_value || '-' }}</div>
            </div>
          </template>

          <template #cell-updated_at="{ value }">
            <span class="text-sm text-gray-600 dark:text-gray-400">{{ formatDate(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-emerald-600 disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-emerald-400"
                :disabled="probingSupplierId === row.id"
                @click="runProbe(row)"
              >
                <Icon name="refresh" size="sm" :class="probingSupplierId === row.id ? 'animate-spin' : ''" />
                <span class="text-xs">{{ t('admin.suppliers.probeNow', '探针') }}</span>
              </button>
              <button
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                @click="openEditDialog(row)"
              >
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('common.edit', '编辑') }}</span>
              </button>
              <button
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                @click="openDeleteDialog(row)"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete', '删除') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.suppliers.emptyTitle', '暂无供应商')"
              :description="t('admin.suppliers.emptyDescription', '先记录一个待考察上游')"
              :action-text="t('admin.suppliers.create', '新增供应商')"
              @action="openCreateDialog"
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

    <BaseDialog
      :show="showDialog"
      :title="editingSupplier ? t('admin.suppliers.edit', '编辑供应商') : t('admin.suppliers.create', '新增供应商')"
      width="extra-wide"
      @close="closeDialog"
    >
      <form id="supplier-form" class="space-y-6" @submit.prevent="handleSubmit">
        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">
              {{ t('admin.suppliers.form.name', '供应商名称') }}
              <span class="text-red-500">*</span>
            </label>
            <input v-model="form.name" required type="text" maxlength="100" class="input" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.suppliers.form.status', '考察状态') }}</label>
            <Select v-model="form.status" :options="statusEditOptions" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.suppliers.form.website', '官网') }}</label>
            <input v-model="form.website_url" type="url" maxlength="500" class="input" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.suppliers.form.upstreamGroup', '上游分组') }}</label>
            <input v-model="form.upstream_group" type="text" maxlength="100" class="input" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.suppliers.form.baseUrl', 'Base URL') }}</label>
            <input v-model="form.base_url" type="url" maxlength="500" class="input" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.suppliers.form.apiKey', 'Key') }}</label>
            <input v-model="form.api_key" type="password" maxlength="4000" class="input" autocomplete="off" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.suppliers.form.costRmbPerUsd', '进货成本（人民币 / USD）') }}</label>
            <input v-model="form.cost_rmb_per_usd" type="number" min="0" step="0.00000001" class="input" />
          </div>

          <div class="grid grid-cols-[minmax(0,0.9fr)_minmax(0,1.4fr)] gap-3">
            <div>
              <label class="input-label">{{ t('admin.suppliers.form.contactPlatform', '联系平台') }}</label>
              <Select v-model="form.contact_platform" :options="contactPlatformOptions" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.suppliers.form.contactValue', '联系方式') }}</label>
              <input v-model="form.contact_value" type="text" maxlength="200" class="input" />
            </div>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-[minmax(0,0.7fr)_minmax(0,1fr)_minmax(0,0.7fr)]">
          <label class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
            <input
              v-model="form.probe_enabled"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
            />
            <span class="text-sm text-gray-800 dark:text-gray-200">{{ t('admin.suppliers.form.probeEnabled', '启用探针') }}</span>
          </label>

          <div>
            <label class="input-label">{{ t('admin.suppliers.form.probeModel', '探针模型') }}</label>
            <input v-model="form.probe_model" type="text" maxlength="100" class="input" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.suppliers.form.probeInterval', '轮询间隔（分钟）') }}</label>
            <input v-model="form.probe_interval_minutes" type="number" min="1" max="1440" step="1" class="input" />
          </div>
        </div>

        <div>
          <label class="input-label">
            {{ t('admin.suppliers.form.targetGroups', '准备用到的本地分组') }}
            <span class="font-normal text-gray-400">
              {{ t('common.selectedCount', { count: form.target_group_ids.length }) }}
            </span>
          </label>
          <div class="grid max-h-40 grid-cols-1 gap-1 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-2 dark:border-dark-600 dark:bg-dark-800 sm:grid-cols-2 lg:grid-cols-3">
            <label
              v-for="group in allGroups"
              :key="group.id"
              class="flex min-w-0 cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm transition-colors hover:bg-white dark:hover:bg-dark-700"
            >
              <input
                type="checkbox"
                :checked="form.target_group_ids.includes(group.id)"
                class="h-3.5 w-3.5 shrink-0 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
                @change="toggleTargetGroup(group.id, ($event.target as HTMLInputElement).checked)"
              />
              <GroupBadge
                :name="group.name"
                :platform="group.platform"
                :subscription-type="group.subscription_type"
                :rate-multiplier="group.rate_multiplier"
                class="min-w-0 flex-1"
              />
            </label>
            <div
              v-if="allGroups.length === 0"
              class="col-span-full py-2 text-center text-sm text-gray-500 dark:text-gray-400"
            >
              {{ t('common.noGroupsAvailable') }}
            </div>
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.suppliers.form.notes', '考察备注') }}</label>
          <textarea v-model="form.notes" rows="3" class="input"></textarea>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeDialog">
            {{ t('common.cancel', '取消') }}
          </button>
          <button type="submit" form="supplier-form" class="btn btn-primary" :disabled="submitting">
            {{
              submitting
                ? t('common.submitting', '提交中...')
                : editingSupplier
                  ? t('common.update', '更新')
                  : t('common.create', '创建')
            }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showProbeDetailDialog"
      :title="selectedProbeItem ? `${selectedProbeItem.name} · 探针详情` : t('admin.suppliers.probeDetail', '探针详情')"
      width="extra-wide"
      @close="closeProbeDetail"
    >
      <div v-if="selectedProbeItem" class="space-y-5">
        <div class="grid gap-3 md:grid-cols-4">
          <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
            <div class="text-xs text-gray-500 dark:text-gray-400">当前状态</div>
            <div class="mt-2">
              <span :class="probeBadgeClass(selectedProbeItem.last_probe_status)">
                {{ probeStatusLabel(selectedProbeItem.last_probe_status) }}
              </span>
            </div>
          </div>
          <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
            <div class="text-xs text-gray-500 dark:text-gray-400">窗口成功率</div>
            <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
              {{ selectedProbeItem.window_total > 0 ? formatPercent(selectedProbeItem.window_success_rate) : '-' }}
            </div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ selectedProbeItem.window_success }}/{{ selectedProbeItem.window_total }}
            </div>
          </div>
          <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
            <div class="text-xs text-gray-500 dark:text-gray-400">平均延迟</div>
            <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
              {{ selectedProbeItem.window_average_latency_ms > 0 ? `${selectedProbeItem.window_average_latency_ms}ms` : '-' }}
            </div>
          </div>
          <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
            <div class="text-xs text-gray-500 dark:text-gray-400">下次探针</div>
            <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
              {{ formatDateTime(selectedProbeItem.next_probe_at) }}
            </div>
          </div>
        </div>

        <div>
          <div class="mb-2 text-sm font-medium text-gray-800 dark:text-gray-200">对应分组</div>
          <div class="flex flex-wrap gap-1.5">
            <GroupBadge
              v-for="group in itemTargetGroups(selectedProbeItem)"
              :key="group.id"
              :name="group.name"
              :platform="group.platform"
              :subscription-type="group.subscription_type"
              :rate-multiplier="group.rate_multiplier"
            />
            <span v-if="itemTargetGroups(selectedProbeItem).length === 0" class="text-sm text-gray-400">-</span>
          </div>
        </div>

        <div>
          <div class="mb-2 flex items-center justify-between">
            <div class="text-sm font-medium text-gray-800 dark:text-gray-200">最近状态区块</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              每 {{ selectedProbeItem.probe_interval_minutes || 30 }} 分钟一个 slot
            </div>
          </div>
          <div class="grid grid-cols-12 gap-1">
            <span
              v-for="slot in supplierTimelineSlots(selectedProbeItem)"
              :key="slot.key"
              class="h-5 min-w-0 rounded"
              :class="probeSlotClass(slot.status)"
              :title="probeSlotTitle(slot)"
            />
          </div>
        </div>

        <div class="overflow-hidden rounded-lg border border-gray-100 dark:border-dark-700">
          <div class="grid grid-cols-[130px_90px_100px_90px_minmax(180px,1fr)] bg-gray-50 px-3 py-2 text-xs font-medium text-gray-500 dark:bg-dark-900 dark:text-gray-400">
            <span>时间</span>
            <span>状态</span>
            <span>HTTP</span>
            <span>延迟</span>
            <span>备注</span>
          </div>
          <div v-if="probeHistoryLoading" class="px-3 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
            加载中...
          </div>
          <template v-else>
            <div
              v-for="result in probeHistory?.results || []"
              :key="result.id"
              class="grid grid-cols-[130px_90px_100px_90px_minmax(180px,1fr)] items-center border-t border-gray-100 px-3 py-2 text-sm dark:border-dark-700"
            >
              <span class="text-gray-500 dark:text-gray-400">{{ formatShortDateTime(result.checked_at) }}</span>
              <span :class="probeBadgeClass(result.status)">{{ probeStatusLabel(result.status) }}</span>
              <span class="text-gray-600 dark:text-gray-300">{{ result.http_code || '-' }}</span>
              <span class="text-gray-600 dark:text-gray-300">{{ result.latency_ms ? `${result.latency_ms}ms` : '-' }}</span>
              <span class="truncate text-gray-500 dark:text-gray-400" :title="result.error_message || result.response_text">
                {{ probeHistorySummary(result) }}
              </span>
            </div>
          </template>
          <div
            v-if="!probeHistoryLoading && (probeHistory?.results || []).length === 0"
            class="px-3 py-6 text-center text-sm text-gray-500 dark:text-gray-400"
          >
            暂无历史探针记录
          </div>
        </div>
      </div>
    </BaseDialog>

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.suppliers.delete', '删除供应商')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete', '删除')"
      :cancel-text="t('common.cancel', '取消')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  Supplier,
  SupplierContactPlatform,
  SupplierHourlyStability,
  SupplierProbeHistory,
  SupplierProbeResult,
  SupplierProbeSnapshot,
  SupplierProbeSnapshotItem,
  SupplierProbeStatus,
  SupplierProbeTimelinePoint,
  SupplierStatus,
  SupplierTargetGroup
} from '@/api/admin/suppliers'
import type { AdminGroup } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const defaultProbeModel = 'claude-haiku-4-5-20251001'
const supplierTimelineSlotCount = 12

type SupplierTimelineSlot = {
  key: string
  at: Date
  status: SupplierProbeStatus | 'missing'
  point?: SupplierProbeTimelinePoint
}

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.suppliers.columns.name', '供应商'), sortable: true },
  { key: 'status', label: t('admin.suppliers.columns.status', '状态'), sortable: true },
  { key: 'upstream_group', label: t('admin.suppliers.columns.upstreamGroup', '上游分组'), sortable: false },
  { key: 'target_groups', label: t('admin.suppliers.columns.targetGroups', '本地分组'), sortable: false },
  { key: 'cost', label: t('admin.suppliers.columns.cost', '成本'), sortable: false },
  { key: 'probe', label: t('admin.suppliers.columns.probe', '探针稳定性'), sortable: false },
  { key: 'contact', label: t('admin.suppliers.columns.contact', '联系方式'), sortable: false },
  { key: 'updated_at', label: t('admin.suppliers.columns.updatedAt', '更新'), sortable: true },
  { key: 'actions', label: t('admin.suppliers.columns.actions', '操作'), sortable: false }
])

const statusLabels: Record<SupplierStatus, string> = {
  evaluating: '考察中',
  active: '启用中'
}

const probeStatusLabels: Record<SupplierProbeStatus, string> = {
  unknown: '暂无数据',
  success: '正常',
  degraded: '需观察',
  failed: '异常'
}

const probeSubStatusLabels: Record<string, string> = {
  slow_latency: '慢响应',
  rate_limit: '限流',
  server_error: '服务器错误',
  client_error: '客户端错误',
  auth_error: '认证失败',
  invalid_request: '请求无效',
  network_error: '网络错误',
  response_timeout: '响应超时',
  content_mismatch: '内容不匹配'
}

const contactPlatformLabels: Record<SupplierContactPlatform, string> = {
  '': '未选择',
  wechat: '微信',
  telegram: 'Telegram',
  qq: 'QQ',
  email: 'Email',
  phone: '电话',
  other: '其他'
}

const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.suppliers.allStatus', '全部状态') },
  ...Object.entries(statusLabels).map(([value, label]) => ({ value, label }))
])
const statusEditOptions = computed(() =>
  Object.entries(statusLabels).map(([value, label]) => ({ value, label }))
)
const probeStatusFilterOptions = computed(() => [
  { value: '', label: t('admin.suppliers.allProbeStatus', '全部探针') },
  ...Object.entries(probeStatusLabels).map(([value, label]) => ({ value, label }))
])
const contactPlatformOptions = computed(() =>
  Object.entries(contactPlatformLabels).map(([value, label]) => ({ value, label }))
)

const suppliers = ref<Supplier[]>([])
const allGroups = ref<AdminGroup[]>([])
const probeSnapshot = ref<SupplierProbeSnapshot | null>(null)
const loading = ref(false)
const snapshotLoading = ref(false)
const submitting = ref(false)
const probingSupplierId = ref<number | null>(null)
const syncingAccounts = ref(false)
const bulkSaving = ref(false)
const batchProbing = ref(false)
const searchQuery = ref('')
const filters = reactive({
  status: '',
  probe_status: ''
})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0
})
const sortState = reactive({
  sort_by: 'updated_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const showDialog = ref(false)
const editingSupplier = ref<Supplier | null>(null)
const showDeleteDialog = ref(false)
const deletingSupplier = ref<Supplier | null>(null)
const showProbeDetailDialog = ref(false)
const selectedProbeItem = ref<SupplierProbeSnapshotItem | null>(null)
const probeHistory = ref<SupplierProbeHistory | null>(null)
const probeHistoryLoading = ref(false)

const form = reactive({
  name: '',
  website_url: '',
  base_url: '',
  api_key: '',
  upstream_group: '',
  contact_platform: '' as SupplierContactPlatform,
  contact_value: '',
  status: 'evaluating' as SupplierStatus,
  cost_rmb_per_usd: null as number | string | null,
  notes: '',
  probe_enabled: true,
  probe_model: defaultProbeModel,
  probe_interval_minutes: 30 as number | string,
  target_group_ids: [] as number[]
})

let abortController: AbortController | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null

const deleteConfirmMessage = computed(() =>
  deletingSupplier.value
    ? t('admin.suppliers.deleteConfirm', { name: deletingSupplier.value.name }, `确认删除供应商「${deletingSupplier.value.name}」？`)
    : ''
)

const snapshotOverallStatus = computed<SupplierProbeStatus>(() => {
  const snapshot = probeSnapshot.value
  if (!snapshot || snapshot.window_total === 0) return 'unknown'
  if (snapshot.window_success_rate >= 95) return 'success'
  if (snapshot.window_success_rate >= 80) return 'degraded'
  return 'failed'
})

const probeMetricCards = computed(() => {
  const snapshot = probeSnapshot.value
  return [
	    {
	      label: t('admin.suppliers.metricCoverage', '覆盖账号'),
	      value: snapshot ? `${snapshot.enabled_suppliers}/${snapshot.total_suppliers}` : '-',
	      hint: t('admin.suppliers.metricCoverageHint', '启用探针 / 启用账号')
	    },
    {
      label: t('admin.suppliers.metricWindowRate', '窗口成功率'),
      value: snapshot && snapshot.window_total > 0 ? formatPercent(snapshot.window_success_rate) : '-',
      hint: snapshot ? `${snapshot.window_success}/${snapshot.window_total}` : '-'
    },
    {
      label: t('admin.suppliers.metricHealthy', '当前正常'),
      value: snapshot ? String(snapshot.healthy_suppliers) : '-',
      hint: t('admin.suppliers.metricHealthyHint', '最近一次探针正常')
    },
    {
      label: t('admin.suppliers.metricProblem', '异常/观察'),
      value: snapshot ? String(snapshot.failed_suppliers + snapshot.degraded_suppliers) : '-',
      hint: t('admin.suppliers.metricProblemHint', '失败或慢响应')
    },
    {
      label: t('admin.suppliers.metricLatency', '平均延迟'),
      value: snapshot && snapshot.average_latency_ms > 0 ? `${snapshot.average_latency_ms}ms` : '-',
      hint: t('admin.suppliers.metricLatencyHint', '当前窗口')
    }
  ]
})

const hourlyBuckets = computed<SupplierHourlyStability[]>(() => probeSnapshot.value?.hourly || [])

const riskyHours = computed<SupplierHourlyStability[]>(() =>
  hourlyBuckets.value
    .filter((bucket) => bucket.total > 0 && bucket.success_rate < 95)
    .sort((a, b) => {
      const aRisk = a.failed + a.degraded
      const bRisk = b.failed + b.degraded
      if (aRisk !== bRisk) return bRisk - aRisk
      return a.success_rate - b.success_rate
    })
    .slice(0, 3)
)

const monitoredSupplierRows = computed<SupplierProbeSnapshotItem[]>(() =>
  [...(probeSnapshot.value?.suppliers || [])]
    .sort((a, b) => {
      const rank: Record<SupplierProbeStatus, number> = {
        failed: 0,
        degraded: 1,
        unknown: 2,
        success: 3
      }
      const aRank = rank[a.last_probe_status] ?? 2
      const bRank = rank[b.last_probe_status] ?? 2
      if (aRank !== bRank) return aRank - bRank
      return (b.window_total || 0) - (a.window_total || 0)
    })
)

function statusLabel(value: SupplierStatus): string {
  return statusLabels[value] || value
}

function probeStatusLabel(value: SupplierProbeStatus): string {
  return probeStatusLabels[value] || value
}

function contactPlatformLabel(value: SupplierContactPlatform): string {
  return contactPlatformLabels[value] || '-'
}

function statusBadgeClass(value: SupplierStatus): string {
  const base = 'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium'
  const map: Record<string, string> = {
    evaluating: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300',
    active: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  }
  return `${base} ${map[value] || map.evaluating}`
}

function probeBadgeClass(value: SupplierProbeStatus): string {
  const base = 'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium'
  const map: Record<string, string> = {
    unknown: 'bg-gray-100 text-gray-700 dark:bg-dark-600 dark:text-gray-300',
    success: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
    degraded: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
    failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  }
  return `${base} ${map[value] || map.unknown}`
}

function withHttp(value: string): string {
  if (!value) return '#'
  return /^https?:\/\//i.test(value) ? value : `https://${value}`
}

function formatDate(value: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleDateString()
}

function formatDateTime(value: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function formatShortDateTime(value: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${Number(value).toFixed(0)}%`
}

function formatCost(value: number | null): string {
  if (value === null || value === undefined) return '-'
  return `1 USD = ¥${Number(value).toLocaleString(undefined, { maximumFractionDigits: 8 })}`
}

function formatProbeSummary(row: Supplier): string {
  if (row.probe_total_count <= 0) {
    return row.probe_model || '-'
  }
  const rate = Number(row.probe_success_rate || 0).toFixed(0)
  const latency = row.last_probe_latency_ms ? `${row.last_probe_latency_ms}ms` : '-'
  return `${rate}% · ${row.probe_success_count}/${row.probe_total_count} · ${latency}`
}

function formatProbeDetail(row: Supplier): string {
  const parts: string[] = []
  if (row.last_probe_sub_status) {
    parts.push(probeSubStatusLabels[row.last_probe_sub_status] || row.last_probe_sub_status)
  }
  if (row.last_probe_http_code !== null && row.last_probe_http_code !== undefined && row.last_probe_http_code > 0) {
    parts.push(`HTTP ${row.last_probe_http_code}`)
  }
  return parts.join(' · ')
}

function hourBucketClass(bucket: SupplierHourlyStability): string {
  if (bucket.total <= 0) {
    return 'border-gray-100 bg-gray-50 text-gray-400 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-500'
  }
  if (bucket.status === 'success') {
    return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-900/20 dark:text-emerald-300'
  }
  if (bucket.status === 'degraded') {
    return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-300'
  }
  return 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300'
}

function hourBucketTitle(bucket: SupplierHourlyStability): string {
  if (bucket.total <= 0) return `${bucket.label} · 暂无数据`
  return `${bucket.label} · ${formatPercent(bucket.success_rate)} · ${bucket.success}/${bucket.total} · ${bucket.average_latency_ms || '-'}ms`
}

function targetGroups(row: Supplier): SupplierTargetGroup[] {
  if (row.target_groups?.length) {
    return row.target_groups
  }
  const ids = new Set(row.target_group_ids || [])
  return allGroups.value
    .filter((group) => ids.has(group.id))
    .map(groupToSupplierTargetGroup)
}

function itemTargetGroups(item: SupplierProbeSnapshotItem): SupplierTargetGroup[] {
  if (item.target_groups?.length) {
    return item.target_groups
  }
  const ids = new Set(item.target_group_ids || [])
  return allGroups.value
    .filter((group) => ids.has(group.id))
    .map(groupToSupplierTargetGroup)
}

function groupToSupplierTargetGroup(group: AdminGroup): SupplierTargetGroup {
  return {
    id: group.id,
    name: group.name,
    platform: group.platform,
    status: group.status,
    subscription_type: group.subscription_type,
    rate_multiplier: group.rate_multiplier
  }
}

function supplierTimelineSlots(item: SupplierProbeSnapshotItem): SupplierTimelineSlot[] {
  const now = new Date(probeSnapshot.value?.generated_at || Date.now())
  now.setSeconds(0, 0)
  const intervalMinutes = Math.max(1, Number(item.probe_interval_minutes || 30))
  const intervalMs = intervalMinutes * 60_000
  const pointsBySlotDistance = new Map<number, SupplierProbeTimelinePoint>()

  for (const point of item.points || []) {
    const date = new Date(point.checked_at)
    if (Number.isNaN(date.getTime())) continue
    const distanceFromNow = Math.round((now.getTime() - date.getTime()) / intervalMs)
    if (distanceFromNow < 0 || distanceFromNow >= supplierTimelineSlotCount) continue
    const existing = pointsBySlotDistance.get(distanceFromNow)
    if (!existing || new Date(point.checked_at).getTime() > new Date(existing.checked_at).getTime()) {
      pointsBySlotDistance.set(distanceFromNow, point)
    }
  }

  const slots: SupplierTimelineSlot[] = []
  for (let idx = supplierTimelineSlotCount - 1; idx >= 0; idx--) {
    const at = new Date(now.getTime() - idx * intervalMs)
    const point = pointsBySlotDistance.get(idx)
    slots.push({
      key: at.toISOString(),
      at,
      status: point?.status || 'missing',
      point
    })
  }
  return slots
}

function probeSlotClass(status: SupplierProbeStatus | 'missing'): string {
  const map: Record<string, string> = {
    success: 'bg-emerald-400',
    degraded: 'bg-amber-400',
    failed: 'bg-red-400',
    unknown: 'bg-gray-300 dark:bg-dark-600',
    missing: 'bg-gray-200 dark:bg-dark-600'
  }
  return map[status] || map.missing
}

function probeSlotTitle(slot: SupplierTimelineSlot): string {
  const time = slot.at.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  if (!slot.point) return `${time} 无数据`
  const subStatus = slot.point.sub_status ? ` ${probeSubStatusLabels[slot.point.sub_status] || slot.point.sub_status}` : ''
  const http = slot.point.http_code > 0 ? ` HTTP ${slot.point.http_code}` : ''
  const latency = slot.point.latency_ms > 0 ? ` ${slot.point.latency_ms}ms` : ''
  return `${time} ${probeStatusLabel(slot.point.status)}${subStatus}${http}${latency}`
}

function probeHistorySummary(result: SupplierProbeResult): string {
  const parts: string[] = []
  if (result.sub_status) {
    parts.push(probeSubStatusLabels[result.sub_status] || result.sub_status)
  }
  if (result.error_message) {
    parts.push(result.error_message)
  } else if (result.response_text) {
    parts.push(result.response_text)
  }
  return parts.join(' · ') || '-'
}

function normalizeNumber(value: number | string | null): number | null {
  if (value === null || value === '') return null
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : null
}

function normalizeInterval(value: number | string): number {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric <= 0) return 30
  return Math.min(1440, Math.floor(numeric))
}

async function loadProbeSnapshot(): Promise<void> {
  snapshotLoading.value = true
  try {
    probeSnapshot.value = await adminAPI.suppliers.getProbeSnapshot({
      window_minutes: 60,
      days: 7
    })
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.snapshotError', '加载探针快照失败')))
  } finally {
    snapshotLoading.value = false
  }
}

async function loadSuppliers(): Promise<void> {
  abortController?.abort()
  abortController = new AbortController()
  loading.value = true
  try {
    const result = await adminAPI.suppliers.list(
      pagination.page,
      pagination.page_size,
      {
        status: filters.status,
        probe_status: filters.probe_status,
        search: searchQuery.value.trim(),
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      { signal: abortController.signal }
    )
    suppliers.value = result.items || []
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
  } catch (error: unknown) {
    if ((error as { name?: string })?.name !== 'CanceledError') {
      appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.loadError', '加载供应商失败')))
    }
  } finally {
    loading.value = false
  }
}

async function refreshSupplierPage(): Promise<void> {
  await Promise.all([loadSuppliers(), loadProbeSnapshot()])
}

async function loadFormOptions(): Promise<void> {
  try {
    allGroups.value = await adminAPI.groups.getAll()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.optionsError', '加载分组失败')))
  }
}

function handleSearch(): void {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    pagination.page = 1
    loadSuppliers()
  }, 300)
}

function handleSort(key: string, order: 'asc' | 'desc'): void {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadSuppliers()
}

function handlePageChange(page: number): void {
  pagination.page = page
  loadSuppliers()
}

function handlePageSizeChange(pageSize: number): void {
  pagination.page_size = pageSize
  pagination.page = 1
  loadSuppliers()
}

function resetForm(): void {
  Object.assign(form, {
    name: '',
    website_url: '',
    base_url: '',
    api_key: '',
    upstream_group: '',
    contact_platform: '',
    contact_value: '',
    status: 'evaluating',
    cost_rmb_per_usd: null,
    notes: '',
    probe_enabled: true,
    probe_model: defaultProbeModel,
    probe_interval_minutes: 30,
    target_group_ids: []
  })
}

function openCreateDialog(): void {
  editingSupplier.value = null
  resetForm()
  showDialog.value = true
  if (allGroups.value.length === 0) {
    loadFormOptions()
  }
}

function openEditDialog(supplier: Supplier): void {
  editingSupplier.value = supplier
  Object.assign(form, {
    name: supplier.name,
    website_url: supplier.website_url || '',
    base_url: supplier.base_url || '',
    api_key: supplier.api_key || '',
    upstream_group: supplier.upstream_group || '',
    contact_platform: supplier.contact_platform || '',
    contact_value: supplier.contact_value || '',
    status: supplier.status,
    cost_rmb_per_usd: supplier.cost_rmb_per_usd,
    notes: supplier.notes || '',
    probe_enabled: supplier.probe_enabled,
    probe_model: supplier.probe_model || defaultProbeModel,
    probe_interval_minutes: supplier.probe_interval_minutes || 30,
    target_group_ids: [...(supplier.target_group_ids || [])]
  })
  showDialog.value = true
  if (allGroups.value.length === 0) {
    loadFormOptions()
  }
}

function closeDialog(): void {
  showDialog.value = false
  editingSupplier.value = null
}

function toggleTargetGroup(groupId: number, checked: boolean): void {
  form.target_group_ids = checked
    ? Array.from(new Set([...form.target_group_ids, groupId]))
    : form.target_group_ids.filter((id) => id !== groupId)
}

async function handleSubmit(): Promise<void> {
  if (!form.name.trim()) {
    appStore.showError(t('admin.suppliers.nameRequired', '请输入供应商名称'))
    return
  }

  submitting.value = true
  const payload = {
    name: form.name.trim(),
    website_url: form.website_url.trim(),
    base_url: form.base_url.trim(),
    api_key: form.api_key.trim(),
    upstream_group: form.upstream_group.trim(),
    contact_platform: form.contact_platform,
    contact_value: form.contact_value.trim(),
    status: form.status,
    cost_rmb_per_usd: normalizeNumber(form.cost_rmb_per_usd),
    notes: form.notes.trim(),
    probe_enabled: form.probe_enabled,
    probe_model: (form.probe_model || defaultProbeModel).trim(),
    probe_interval_minutes: normalizeInterval(form.probe_interval_minutes),
    target_group_ids: [...form.target_group_ids]
  }

  try {
    if (editingSupplier.value) {
      await adminAPI.suppliers.update(editingSupplier.value.id, payload)
      appStore.showSuccess(t('admin.suppliers.updateSuccess', '供应商已更新'))
    } else {
      await adminAPI.suppliers.create(payload)
      appStore.showSuccess(t('admin.suppliers.createSuccess', '供应商已创建'))
    }
    closeDialog()
    await refreshSupplierPage()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.saveError', '保存供应商失败')))
  } finally {
    submitting.value = false
  }
}

async function runProbe(supplier: Supplier): Promise<void> {
  probingSupplierId.value = supplier.id
  try {
    const result = await adminAPI.suppliers.probe(supplier.id)
    const index = suppliers.value.findIndex((item) => item.id === supplier.id)
    if (index >= 0) {
      suppliers.value.splice(index, 1, result.supplier)
    }
    if (result.result.status === 'success') {
      appStore.showSuccess(t('admin.suppliers.probeSuccess', '探针成功'))
    } else if (result.result.status === 'degraded') {
      appStore.showWarning(result.result.error_message || t('admin.suppliers.probeDegraded', '探针需观察'))
    } else {
      appStore.showError(result.result.error_message || t('admin.suppliers.probeFailed', '探针失败'))
    }
    await loadProbeSnapshot()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.probeFailed', '探针失败')))
  } finally {
    probingSupplierId.value = null
  }
}

async function openProbeDetail(item: SupplierProbeSnapshotItem): Promise<void> {
  selectedProbeItem.value = item
  showProbeDetailDialog.value = true
  probeHistory.value = null
  probeHistoryLoading.value = true
  try {
    probeHistory.value = await adminAPI.suppliers.getProbeHistory(item.id, {
      days: 7,
      limit: 200
    })
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.probeHistoryError', '加载探针历史失败')))
  } finally {
    probeHistoryLoading.value = false
  }
}

function closeProbeDetail(): void {
  showProbeDetailDialog.value = false
  selectedProbeItem.value = null
  probeHistory.value = null
}

async function syncAccountsToSuppliers(): Promise<void> {
  syncingAccounts.value = true
  try {
    const result = await adminAPI.suppliers.syncAccounts()
    appStore.showSuccess(
      t(
        'admin.suppliers.syncAccountsSuccess',
        { created: result.created, updated: result.updated, removed: result.removed, skipped: result.skipped },
        `账号已同步：新增 ${result.created}，更新 ${result.updated}，移除 ${result.removed}，跳过 ${result.skipped}`
      )
    )
    await refreshSupplierPage()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.syncAccountsError', '同步账号失败')))
  } finally {
    syncingAccounts.value = false
  }
}

async function bulkToggleProbes(enabled: boolean): Promise<void> {
  bulkSaving.value = true
  try {
    const result = await adminAPI.suppliers.bulkSetProbeEnabled([], enabled)
    appStore.showSuccess(
      enabled
        ? t('admin.suppliers.enableAllSuccess', { count: result.affected }, `已开启 ${result.affected} 个探针`)
        : t('admin.suppliers.disableAllSuccess', { count: result.affected }, `已关闭 ${result.affected} 个探针`)
    )
    await refreshSupplierPage()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.bulkToggleError', '批量更新探针失败')))
  } finally {
    bulkSaving.value = false
  }
}

async function runAllEnabledProbes(): Promise<void> {
  batchProbing.value = true
  try {
    const result = await adminAPI.suppliers.probeAll([])
    appStore.showSuccess(
      t(
        'admin.suppliers.probeAllDone',
        { success: result.success, degraded: result.degraded, failed: result.failed, skipped: result.skipped },
        `检测完成：正常 ${result.success}，观察 ${result.degraded}，失败 ${result.failed}，跳过 ${result.skipped}`
      )
    )
    await refreshSupplierPage()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.probeAllError', '批量探针失败')))
  } finally {
    batchProbing.value = false
  }
}

function openDeleteDialog(supplier: Supplier): void {
  deletingSupplier.value = supplier
  showDeleteDialog.value = true
}

async function confirmDelete(): Promise<void> {
  if (!deletingSupplier.value) return
  try {
    await adminAPI.suppliers.remove(deletingSupplier.value.id)
    appStore.showSuccess(t('admin.suppliers.deleteSuccess', '供应商已删除'))
    showDeleteDialog.value = false
    deletingSupplier.value = null
    await refreshSupplierPage()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.suppliers.deleteError', '删除供应商失败')))
  }
}

onMounted(() => {
  refreshSupplierPage()
  loadFormOptions()
})

onUnmounted(() => {
  abortController?.abort()
  if (searchTimer) clearTimeout(searchTimer)
})
</script>
