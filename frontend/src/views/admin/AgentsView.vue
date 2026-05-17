<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-5">
      <div class="card p-4">
        <div class="flex flex-wrap items-end gap-3">
          <div class="min-w-64 flex-1">
            <label class="input-label">{{ t('admin.agents.search') }}</label>
            <input v-model="search" class="input" :placeholder="t('admin.agents.searchPlaceholder')" @keyup.enter="reloadAgents" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.agents.sortBy') }}</label>
            <Select v-model="sortBy" :options="sortByOptions" class="min-w-44" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.agents.sortOrder') }}</label>
            <Select v-model="sortOrder" :options="sortOrderOptions" class="min-w-28" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.agents.settlementFilter') }}</label>
            <Select v-model="settlementStatus" :options="settlementStatusOptions" class="min-w-36" />
          </div>
          <div>
            <label class="input-label">{{ t('agent.dateRange') }}</label>
            <div class="flex items-center gap-2">
              <input v-model="startDate" type="date" class="input w-auto text-sm" :max="endDate || undefined" />
              <span class="text-gray-400">-</span>
              <input v-model="endDate" type="date" class="input w-auto text-sm" :min="startDate || undefined" />
            </div>
          </div>
          <button class="btn btn-primary" @click="reloadAgents">{{ t('common.refresh') }}</button>
        </div>
      </div>

      <div class="card p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.agents.levelRules') }}</h3>
            <p class="mt-1 text-xs text-gray-500">{{ t('admin.agents.levelRulesHint') }}</p>
          </div>
          <div class="flex gap-2">
            <button class="btn btn-secondary btn-sm" :disabled="levelEvaluationRunning" @click="runAllLevelEvaluations">
              {{ levelEvaluationRunning ? t('common.processing') : t('admin.agents.runLevelEvaluation') }}
            </button>
            <button class="btn btn-secondary btn-sm" :disabled="levelRulesSaving" @click="saveLevelRules">
              {{ levelRulesSaving ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </div>
        <div class="mt-4 overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-800">
            <thead class="bg-gray-50 dark:bg-dark-800/50">
              <tr>
                <th class="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.level') }}</th>
                <th class="px-3 py-2 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.rate') }}</th>
                <th class="px-3 py-2 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.monthlyThreshold') }}</th>
                <th class="px-3 py-2 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.cumulativeThreshold') }}</th>
                <th class="px-3 py-2 text-center text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.enabled') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-dark-800">
              <tr v-for="rule in levelRules" :key="rule.level_key">
                <td class="px-3 py-2 text-sm font-medium text-gray-900 dark:text-white">{{ displayAgentLevelName(rule.level_name, rule.level_key) }}</td>
                <td class="px-3 py-2 text-right">
                  <input v-model.number="rule.ratePercent" type="number" min="0" max="20" step="0.01" class="input ml-auto w-24 text-right text-sm" />
                </td>
                <td class="px-3 py-2 text-right">
                  <input v-model.number="rule.monthly_consumption_threshold" type="number" min="0" step="1" class="input ml-auto w-28 text-right text-sm" :placeholder="t('common.none')" />
                </td>
                <td class="px-3 py-2 text-right">
                  <input v-model.number="rule.cumulative_consumption_threshold" type="number" min="0" step="1" class="input ml-auto w-28 text-right text-sm" :placeholder="t('common.none')" />
                </td>
                <td class="px-3 py-2 text-center">
                  <input v-model="rule.enabled" type="checkbox" class="rounded border-gray-300" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card p-4">
        <div class="flex flex-wrap items-end gap-3">
          <div>
            <label class="input-label">{{ t('admin.agents.consumptionRate') }}</label>
            <div class="flex items-center gap-2">
              <input v-model.number="globalRateForm.consumption" type="number" min="0" max="100" step="0.01" class="input w-28" />
              <span class="text-sm text-gray-500">%</span>
            </div>
          </div>
          <div>
            <label class="input-label">{{ t('admin.agents.inviteeRate') }}</label>
            <div class="flex items-center gap-2">
              <input v-model.number="globalRateForm.invitee" type="number" min="0" max="100" step="0.01" class="input w-28" />
              <span class="text-sm text-gray-500">%</span>
            </div>
          </div>
          <div>
            <label class="input-label">{{ t('admin.agents.referralRate') }}</label>
            <div class="flex items-center gap-2">
              <input v-model.number="globalRateForm.referral" type="number" min="0" max="100" step="0.01" class="input w-28" />
              <span class="text-sm text-gray-500">%</span>
            </div>
          </div>
          <button class="btn btn-secondary" :disabled="ratesSaving" @click="saveGlobalRates">
            {{ ratesSaving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </div>

      <div class="card p-4">
        <div class="flex flex-wrap items-end gap-3">
          <div>
            <label class="input-label">{{ t('admin.agents.settlementMinimum') }}</label>
            <div class="flex items-center gap-2">
              <input v-model.number="settlementSettingsForm.minimum" type="number" min="0" step="0.01" class="input w-32" />
              <span class="text-sm text-gray-500">$</span>
            </div>
          </div>
          <button class="btn btn-secondary" :disabled="settlementSettingsSaving" @click="saveSettlementSettings">
            {{ settlementSettingsSaving ? t('common.saving') : t('common.save') }}
          </button>
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.agents.settlementMinimumHint') }}</p>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-5">
        <div class="card p-4">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.agents.agentCount') }}</p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ pagination.total }}</p>
        </div>
        <div class="card p-4">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.agents.totalCommission') }}</p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ money(listTotals.totalCommission) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.agents.settledCommission') }}</p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ money(listTotals.settled) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.agents.unsettledCommission') }}</p>
          <p class="mt-1 text-2xl font-semibold text-green-600 dark:text-green-400">{{ money(listTotals.unsettled) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.agents.eligibleSettlementCount') }}</p>
          <p class="mt-1 text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ listTotals.eligible }}</p>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-800">
            <thead class="bg-gray-50 dark:bg-dark-800/50">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.agent') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.invitedUserCount') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.totalConsumption') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.totalCommission') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.settledCommission') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.unsettledCommission') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.settlementStatus') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.currentLevel') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.lastMonthConsumption') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.consumptionRate') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-dark-800">
              <tr v-if="loading">
                <td colspan="11" class="px-4 py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</td>
              </tr>
              <tr
                v-for="agent in agents"
                v-else
                :key="agent.agent_id"
                class="cursor-pointer transition-colors"
                :class="selectedAgent?.agent_id === agent.agent_id ? 'bg-primary-50/80 dark:bg-primary-950/30' : 'hover:bg-gray-50 dark:hover:bg-dark-800/30'"
                @click="selectAgent(agent)"
              >
                <td class="relative px-4 py-3">
                  <span v-if="selectedAgent?.agent_id === agent.agent_id" class="absolute inset-y-2 left-0 w-1 rounded-r bg-primary-500" />
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="text-sm font-medium text-gray-900 dark:text-white">{{ displayAgent(agent) }}</span>
                    <span v-if="selectedAgent?.agent_id === agent.agent_id" class="rounded-full bg-primary-100 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/50 dark:text-primary-200">
                      {{ t('admin.agents.selected') }}
                    </span>
                  </div>
                  <div class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-500">
                    <span>#{{ agent.agent_id }}</span>
                    <button v-if="agent.invite_code" class="text-primary-600 hover:underline" @click.stop="copyInvite(agent)">
                      {{ agent.invite_code }}
                    </button>
                  </div>
                </td>
                <td class="px-4 py-3 text-right text-sm">{{ agent.invited_user_count }}</td>
                <td class="px-4 py-3 text-right text-sm">{{ money(agent.total_consumption) }}</td>
                <td class="px-4 py-3 text-right text-sm font-medium">{{ money(agent.total_commission) }}</td>
                <td class="px-4 py-3 text-right text-sm">{{ money(agent.settled_commission) }}</td>
                <td class="px-4 py-3 text-right text-sm font-medium text-green-600 dark:text-green-400">{{ money(agent.unsettled_commission) }}</td>
                <td class="px-4 py-3 text-sm">
                  <span class="rounded px-2 py-0.5 text-xs font-medium" :class="settlementStatusClass(agent)">
                    {{ settlementStatusLabel(agent) }}
                  </span>
                </td>
                <td class="px-4 py-3 text-sm">
                  <span class="rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-dark-800 dark:text-dark-200">{{ formatLevel(agent.current_level) }}</span>
                </td>
                <td class="px-4 py-3 text-right text-sm">{{ money(agent.last_month_consumption || 0) }}</td>
                <td class="px-4 py-3 text-right text-sm">
                  {{ percent(agent.consumption_rate) }}
                  <span v-if="agent.rate_source === 'agent_override' || agent.rate_source === 'agent_manual_base'" class="ml-1 rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700">{{ t('admin.agents.override') }}</span>
                </td>
                <td class="px-4 py-3 text-right">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-secondary btn-sm" @click.stop="openAgentDetails(agent)">
                      {{ t('admin.agents.viewDetails') }}
                    </button>
                    <button class="btn btn-secondary btn-sm" :disabled="!agent.settlement_eligible" @click.stop="openSettlement(agent)">
                      {{ t('admin.agents.settle') }}
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-if="!loading && agents.length === 0">
                <td colspan="11" class="px-4 py-10 text-center text-sm text-gray-500">{{ t('admin.agents.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="pagination.pages > 1" class="flex items-center justify-between border-t border-gray-100 px-4 py-3 dark:border-dark-800">
          <p class="text-sm text-gray-500">{{ pagination.page }} / {{ pagination.pages }}</p>
          <div class="flex gap-2">
            <button class="btn btn-secondary btn-sm" :disabled="pagination.page <= 1" @click="changeAgentPage(pagination.page - 1)">{{ t('common.prev') }}</button>
            <button class="btn btn-secondary btn-sm" :disabled="pagination.page >= pagination.pages" @click="changeAgentPage(pagination.page + 1)">{{ t('common.next') }}</button>
          </div>
        </div>
      </div>

      <div v-if="selectedAgent" class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-800">
          <div>
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.agents.currentViewing', { agent: displayAgent(selectedAgent) }) }}</h3>
            <p class="text-sm text-gray-500">{{ t('admin.agents.detailSummary', { unsettled: money(selectedAgent.unsettled_commission), settled: money(selectedAgent.settled_commission) }) }}</p>
            <div class="mt-2 flex flex-wrap gap-2 text-xs text-gray-500">
              <span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-800">#{{ selectedAgent.agent_id }}</span>
              <span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ t('admin.agents.invitedUserCount') }}: {{ selectedAgent.invited_user_count }}</span>
              <span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ t('admin.agents.currentLevel') }}: {{ formatLevel(selectedAgent.current_level) }}</span>
              <span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ t('admin.agents.nextLevelGap') }}: {{ money(selectedAgent.next_level_gap || 0) }}</span>
              <span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ t('admin.agents.consumptionRate') }}: {{ percent(selectedAgent.consumption_rate) }}</span>
              <span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ t('admin.agents.settlementStatus') }}: {{ settlementStatusLabel(selectedAgent) }}</span>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <button class="btn btn-secondary btn-sm" @click="openPaymentProfile(selectedAgent)">
              {{ t('admin.agents.paymentProfile') }}
            </button>
            <button class="btn btn-secondary btn-sm" :disabled="levelEvaluationRunning" @click="runSelectedLevelEvaluation">
              {{ t('admin.agents.runThisAgentLevelEvaluation') }}
            </button>
            <button class="btn btn-primary btn-sm" @click="openBindUser">
              {{ t('admin.agents.bindUser') }}
            </button>
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-300">
              <input v-model="agentRateForm.enabled" type="checkbox" class="rounded border-gray-300" />
              {{ t('admin.agents.agentOverride') }}
            </label>
            <div class="flex items-center gap-2">
              <input v-model.number="agentRateForm.consumption" type="number" min="0" max="100" step="0.01" class="input w-24" :disabled="!agentRateForm.enabled" />
              <span class="text-sm text-gray-500">%</span>
            </div>
            <button class="btn btn-secondary btn-sm" :disabled="agentRateSaving" @click="saveAgentRate">{{ t('common.save') }}</button>
          </div>
        </div>

        <div class="flex gap-2 border-b border-gray-100 px-5 py-3 dark:border-dark-800">
          <button v-for="tab in detailTabs" :key="tab.key" class="btn btn-sm" :class="detailTab === tab.key ? 'btn-primary' : 'btn-secondary'" @click="detailTab = tab.key">
            {{ tab.label }}
          </button>
        </div>

        <div class="overflow-x-auto">
          <table v-if="detailTab === 'users'" class="min-w-full divide-y divide-gray-100 dark:divide-dark-800">
            <thead class="bg-gray-50 dark:bg-dark-800/50">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('agent.userLabel') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('agent.joinedAt') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.totalRecharged') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.totalConsumption') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.userContribution') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-dark-800">
              <tr v-for="user in detailUsers" :key="user.user_id">
                <td class="px-4 py-3 text-sm">
                  <div class="font-medium text-gray-900 dark:text-white">{{ user.username || user.email || `#${user.user_id}` }}</div>
                  <div class="text-xs text-gray-500">{{ user.email }}</div>
                </td>
                <td class="px-4 py-3 text-sm text-gray-500">{{ date(user.joined_at) }}</td>
                <td class="px-4 py-3 text-right text-sm">{{ money(user.total_recharged) }}</td>
                <td class="px-4 py-3 text-right text-sm">{{ money(user.total_consumption) }}</td>
                <td class="px-4 py-3 text-right text-sm font-medium text-green-600">{{ money(user.total_commission) }}</td>
              </tr>
              <tr v-if="detailUsers.length === 0">
                <td colspan="5" class="px-4 py-8 text-center text-sm text-gray-500">{{ t('agent.noInvitedUsers') }}</td>
              </tr>
            </tbody>
          </table>

          <table v-else-if="detailTab === 'commissions'" class="min-w-full divide-y divide-gray-100 dark:divide-dark-800">
            <thead class="bg-gray-50 dark:bg-dark-800/50">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('agent.commissionTime') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('agent.triggeredBy') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('agent.commissionType') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('agent.sourceAmount') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.rate') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('agent.commissionAmount') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-dark-800">
              <tr v-for="record in detailCommissions" :key="record.id">
                <td class="px-4 py-3 text-sm text-gray-500">{{ datetime(record.created_at) }}</td>
                <td class="px-4 py-3 text-sm">{{ record.user_email || `#${record.user_id}` }}</td>
                <td class="px-4 py-3 text-sm">{{ commissionType(record.type) }}</td>
                <td class="px-4 py-3 text-right text-sm">{{ money(record.source_amount) }}</td>
                <td class="px-4 py-3 text-right text-sm">{{ record.rate ? percent(record.rate) : '-' }}</td>
                <td class="px-4 py-3 text-right text-sm font-medium text-green-600">{{ money(record.amount) }}</td>
              </tr>
              <tr v-if="detailCommissions.length === 0">
                <td colspan="6" class="px-4 py-8 text-center text-sm text-gray-500">{{ t('agent.noCommissions') }}</td>
              </tr>
            </tbody>
          </table>

          <table v-else class="min-w-full divide-y divide-gray-100 dark:divide-dark-800">
            <thead class="bg-gray-50 dark:bg-dark-800/50">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('agent.commissionTime') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.settlementAmount') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.operator') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.alipayAccount') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.paymentReference') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.agents.note') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-dark-800">
              <tr v-for="item in detailSettlements" :key="item.id">
                <td class="px-4 py-3 text-sm text-gray-500">{{ datetime(item.created_at) }}</td>
                <td class="px-4 py-3 text-right text-sm font-medium">{{ money(item.amount) }}</td>
                <td class="px-4 py-3 text-sm">#{{ item.operator_id }}</td>
                <td class="px-4 py-3 text-sm">{{ item.payment_alipay_account || '-' }}</td>
                <td class="px-4 py-3 text-sm">{{ item.payment_reference || '-' }}</td>
                <td class="px-4 py-3 text-sm text-gray-500">{{ item.note || '-' }}</td>
              </tr>
              <tr v-if="detailSettlements.length === 0">
                <td colspan="6" class="px-4 py-8 text-center text-sm text-gray-500">{{ t('admin.agents.noSettlements') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="bindUserDialog.show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
        <div class="w-full max-w-md rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.agents.bindUser') }}</h3>
          <p class="mt-1 text-sm text-gray-500">{{ selectedAgent ? displayAgent(selectedAgent) : '' }}</p>
          <div class="mt-4 space-y-3">
            <div>
              <label class="input-label">{{ t('admin.agents.userEmail') }}</label>
              <input v-model.trim="bindUserDialog.email" type="email" class="input" :placeholder="t('admin.agents.userEmailPlaceholder')" @keyup.enter="submitBindUser" />
            </div>
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-dark-300">
              <input v-model="bindUserDialog.overwriteAgent" type="checkbox" class="mt-1 rounded border-gray-300" />
              <span>{{ t('admin.agents.overwriteAgentBinding') }}</span>
            </label>
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button class="btn btn-secondary" @click="closeBindUser">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="bindUserSaving || !bindUserDialog.email" @click="submitBindUser">
              {{ bindUserSaving ? t('common.saving') : t('admin.agents.bindUser') }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="paymentProfileDialog.show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
        <div class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.agents.paymentProfile') }}</h3>
          <p class="mt-1 text-sm text-gray-500">{{ paymentProfileDialog.agent ? displayAgent(paymentProfileDialog.agent) : '' }}</p>
          <div v-if="paymentProfileDialog.loading" class="mt-6 py-10 text-center text-sm text-gray-500">
            {{ t('common.loading') }}
          </div>
          <div v-else class="mt-4 grid grid-cols-1 gap-5 lg:grid-cols-[1fr_260px]">
            <div class="space-y-4">
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <p class="text-xs text-gray-500">{{ t('admin.agents.alipayRealName') }}</p>
                  <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ paymentProfileDialog.paymentProfile?.alipay_real_name || '-' }}</p>
                </div>
                <div>
                  <p class="text-xs text-gray-500">{{ t('admin.agents.alipayAccount') }}</p>
                  <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ paymentProfileDialog.paymentProfile?.alipay_account || '-' }}</p>
                </div>
                <div>
                  <p class="text-xs text-gray-500">{{ t('admin.agents.contactPhone') }}</p>
                  <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ paymentProfileDialog.paymentProfile?.contact_phone || '-' }}</p>
                </div>
                <div>
                  <p class="text-xs text-gray-500">{{ t('admin.agents.paymentProfileStatus') }}</p>
                  <p class="mt-1 text-sm font-medium" :class="paymentProfileDialog.paymentProfile?.complete ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'">
                    {{ paymentProfileDialog.paymentProfile?.complete ? t('admin.agents.paymentProfileComplete') : t('admin.agents.paymentProfileIncomplete') }}
                  </p>
                </div>
              </div>
              <div>
                <p class="text-xs text-gray-500">{{ t('admin.agents.note') }}</p>
                <p class="mt-1 whitespace-pre-wrap text-sm text-gray-700 dark:text-dark-200">{{ paymentProfileDialog.paymentProfile?.payment_note || '-' }}</p>
              </div>
            </div>
            <div class="flex min-h-72 items-center justify-center rounded-lg border border-dashed border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900">
              <img v-if="paymentProfileDialog.qrPreviewUrl" :src="paymentProfileDialog.qrPreviewUrl" class="max-h-64 max-w-full rounded-md object-contain" :alt="t('admin.agents.alipayQRCode')" />
              <div v-else class="text-center text-sm text-gray-500">{{ t('admin.agents.noAlipayQRCode') }}</div>
            </div>
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button class="btn btn-secondary" @click="closePaymentProfile">{{ t('common.close') }}</button>
          </div>
        </div>
      </div>

      <div v-if="settlementDialog.show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
        <div class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.agents.settle') }}</h3>
          <p class="mt-1 text-sm text-gray-500">{{ settlementDialog.agent ? displayAgent(settlementDialog.agent) : '' }}</p>
          <div class="mt-4 grid grid-cols-1 gap-5 lg:grid-cols-[1fr_240px]">
            <div class="space-y-3">
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <p class="text-xs text-gray-500">{{ t('admin.agents.alipayRealName') }}</p>
                  <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ settlementDialog.paymentProfile?.alipay_real_name || '-' }}</p>
                </div>
                <div>
                  <p class="text-xs text-gray-500">{{ t('admin.agents.alipayAccount') }}</p>
                  <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ settlementDialog.paymentProfile?.alipay_account || '-' }}</p>
                </div>
                <div>
                  <p class="text-xs text-gray-500">{{ t('admin.agents.contactPhone') }}</p>
                  <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ settlementDialog.paymentProfile?.contact_phone || '-' }}</p>
                </div>
                <div>
                  <p class="text-xs text-gray-500">{{ t('admin.agents.settlementStatus') }}</p>
                  <p class="mt-1 text-sm font-medium text-green-600 dark:text-green-400">{{ settlementDialog.agent ? settlementStatusLabel(settlementDialog.agent) : '-' }}</p>
                </div>
              </div>
              <div>
                <label class="input-label">{{ t('admin.agents.settlementAmount') }}</label>
                <input v-model.number="settlementDialog.amount" type="number" min="0" step="0.01" class="input" />
                <p class="mt-1 text-xs text-gray-500">{{ t('admin.agents.availableToSettle', { amount: money(settlementDialog.agent?.unsettled_commission || 0) }) }}</p>
              </div>
              <div>
                <label class="input-label">{{ t('admin.agents.paymentReference') }}</label>
                <input v-model.trim="settlementDialog.paymentReference" class="input" :placeholder="t('admin.agents.paymentReferencePlaceholder')" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.agents.note') }}</label>
                <textarea v-model="settlementDialog.note" class="input min-h-20" />
              </div>
            </div>
            <div class="flex min-h-60 items-center justify-center rounded-lg border border-dashed border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900">
              <img v-if="settlementDialog.qrPreviewUrl" :src="settlementDialog.qrPreviewUrl" class="max-h-56 max-w-full rounded-md object-contain" :alt="t('admin.agents.alipayQRCode')" />
              <div v-else class="text-center text-sm text-gray-500">{{ t('admin.agents.noAlipayQRCode') }}</div>
            </div>
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button class="btn btn-secondary" @click="closeSettlement">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="settlementSaving || !settlementDialog.paymentProfile?.complete" @click="submitSettlement">{{ settlementSaving ? t('common.saving') : t('admin.agents.completeSettlement') }}</button>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type {
  AdminAgentSummary,
  AdminAgentUserStat,
  AdminAgentCommissionRecord,
  AgentSettlement,
  AgentLevelRule,
  AgentPaymentProfile
} from '@/api/admin/agents'
import { useAppStore } from '@/stores/app'
import { buildAuthErrorMessage } from '@/utils/authError'
import { imageBlobToDataURL } from '@/utils/imagePreview'
import { useClipboard } from '@/composables/useClipboard'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const ratesSaving = ref(false)
const agentRateSaving = ref(false)
const settlementSaving = ref(false)
const settlementSettingsSaving = ref(false)
const bindUserSaving = ref(false)
const levelRulesSaving = ref(false)
const levelEvaluationRunning = ref(false)
const agents = ref<AdminAgentSummary[]>([])
const selectedAgent = ref<AdminAgentSummary | null>(null)
const detailUsers = ref<AdminAgentUserStat[]>([])
const detailCommissions = ref<AdminAgentCommissionRecord[]>([])
const detailSettlements = ref<AgentSettlement[]>([])
type EditableAgentLevelRule = AgentLevelRule & { ratePercent: number }
const levelRules = ref<EditableAgentLevelRule[]>([])
const detailTab = ref<'users' | 'commissions' | 'settlements'>('users')
const search = ref('')
const sortBy = ref('total_commission')
const sortOrder = ref<'asc' | 'desc'>('desc')
const settlementStatus = ref('')
const currentPage = ref(1)
const pagination = reactive({ total: 0, page: 1, page_size: 20, pages: 1 })
const globalRateForm = reactive({ consumption: 6, invitee: 10, referral: 5 })
const agentRateForm = reactive({ enabled: false, consumption: 6 })
const settlementSettingsForm = reactive({ minimum: 50 })
const settlementDialog = reactive<{
  show: boolean
  agent: AdminAgentSummary | null
  amount: number
  note: string
  paymentReference: string
  paymentProfile: AgentPaymentProfile | null
  qrPreviewUrl: string
}>({
  show: false,
  agent: null,
  amount: 0,
  note: '',
  paymentReference: '',
  paymentProfile: null,
  qrPreviewUrl: ''
})
const paymentProfileDialog = reactive<{
  show: boolean
  loading: boolean
  agent: AdminAgentSummary | null
  paymentProfile: AgentPaymentProfile | null
  qrPreviewUrl: string
}>({
  show: false,
  loading: false,
  agent: null,
  paymentProfile: null,
  qrPreviewUrl: ''
})
const bindUserDialog = reactive<{ show: boolean; email: string; overwriteAgent: boolean }>({
  show: false,
  email: '',
  overwriteAgent: false
})

function defaultDates() {
  const now = new Date()
  const end = now.toISOString().slice(0, 10)
  const startDate = new Date(now)
  startDate.setDate(startDate.getDate() - 30)
  return { start: startDate.toISOString().slice(0, 10), end }
}

const defaults = defaultDates()
const startDate = ref(defaults.start)
const endDate = ref(defaults.end)

const detailTabs = computed(() => [
  { key: 'users', label: t('admin.agents.invitedUsers') },
  { key: 'commissions', label: t('admin.agents.commissionRecords') },
  { key: 'settlements', label: t('admin.agents.settlementRecords') }
] as const)

const sortByOptions = computed<SelectOption[]>(() => [
  { value: 'total_commission', label: t('admin.agents.totalCommission') },
  { value: 'unsettled_commission', label: t('admin.agents.unsettledCommission') },
  { value: 'settlement_gap', label: t('admin.agents.settlementGap') },
  { value: 'invited_user_count', label: t('admin.agents.invitedUserCount') },
  { value: 'total_consumption', label: t('admin.agents.totalConsumption') },
  { value: 'email', label: t('admin.agents.email') }
])

const sortOrderOptions = computed<SelectOption[]>(() => [
  { value: 'desc', label: t('admin.agents.desc') },
  { value: 'asc', label: t('admin.agents.asc') }
])

const settlementStatusOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.agents.settlementAll') },
  { value: 'eligible', label: t('admin.agents.settlementEligible') },
  { value: 'below_threshold', label: t('admin.agents.settlementBelowThreshold') },
  { value: 'missing_profile', label: t('admin.agents.settlementMissingProfile') }
])

const listTotals = computed(() => agents.value.reduce((acc, item) => {
  acc.totalCommission += item.total_commission || 0
  acc.settled += item.settled_commission || 0
  acc.unsettled += item.unsettled_commission || 0
  if (item.settlement_eligible) acc.eligible += 1
  return acc
}, { totalCommission: 0, settled: 0, unsettled: 0, eligible: 0 }))

async function loadRates() {
  try {
    const rates = await adminAPI.agents.getRates()
    globalRateForm.consumption = toPercentNumber(rates.consumption_rate)
    globalRateForm.invitee = toPercentNumber(rates.first_recharge_invitee_rate)
    globalRateForm.referral = toPercentNumber(rates.first_recharge_referral_rate)
  } catch (error: any) {
    appStore.showError(buildAuthErrorMessage(error, { fallback: t('admin.agents.failedToLoad') }))
  }
}

async function loadSettlementSettings() {
  try {
    const settings = await adminAPI.agents.getSettlementSettings()
    settlementSettingsForm.minimum = Number(settings.minimum_amount || 50)
  } catch (error: any) {
    appStore.showError(buildAuthErrorMessage(error, { fallback: t('admin.agents.failedToLoad') }))
  }
}

async function saveSettlementSettings() {
  settlementSettingsSaving.value = true
  try {
    await adminAPI.agents.updateSettlementSettings({ minimum_amount: Number(settlementSettingsForm.minimum) })
    appStore.showSuccess(t('admin.agents.settlementSettingsUpdated'))
    await reloadAgents()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.agents.failedToSave'))
  } finally {
    settlementSettingsSaving.value = false
  }
}

async function loadLevelRules() {
  try {
    const rules = await adminAPI.agents.getLevelRules()
    levelRules.value = (rules || []).map((rule) => ({
      ...rule,
      monthly_consumption_threshold: rule.monthly_consumption_threshold ?? null,
      cumulative_consumption_threshold: rule.cumulative_consumption_threshold ?? null,
      ratePercent: toPercentNumber(rule.rate)
    }))
  } catch (error: any) {
    appStore.showError(buildAuthErrorMessage(error, { fallback: t('admin.agents.failedToLoad') }))
  }
}

async function saveGlobalRates() {
  ratesSaving.value = true
  try {
    await adminAPI.agents.updateRates({
      consumption_rate: fromPercent(globalRateForm.consumption),
      first_recharge_invitee_rate: fromPercent(globalRateForm.invitee),
      first_recharge_referral_rate: fromPercent(globalRateForm.referral)
    })
    appStore.showSuccess(t('admin.agents.ratesUpdated'))
    await reloadAgents()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.agents.failedToSave'))
  } finally {
    ratesSaving.value = false
  }
}

async function saveLevelRules() {
  levelRulesSaving.value = true
  try {
    const payload: AgentLevelRule[] = levelRules.value.map((rule) => ({
      level_key: rule.level_key,
      level_name: rule.level_name,
      rate: fromPercent(rule.ratePercent),
      monthly_consumption_threshold: normalizeOptionalNumber(rule.monthly_consumption_threshold),
      cumulative_consumption_threshold: normalizeOptionalNumber(rule.cumulative_consumption_threshold),
      sort_order: rule.sort_order,
      enabled: rule.enabled
    }))
    await adminAPI.agents.updateLevelRules(payload)
    appStore.showSuccess(t('admin.agents.levelRulesUpdated'))
    await loadLevelRules()
    await reloadAgents()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.agents.failedToSave'))
  } finally {
    levelRulesSaving.value = false
  }
}

async function runAllLevelEvaluations() {
  levelEvaluationRunning.value = true
  try {
    const result = await adminAPI.agents.runLevelEvaluations()
    appStore.showSuccess(t('admin.agents.levelEvaluationCompleted', { count: result.evaluated_count, failed: result.failed_count }))
    await reloadAgents()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.agents.failedToRunLevelEvaluation'))
  } finally {
    levelEvaluationRunning.value = false
  }
}

async function runSelectedLevelEvaluation() {
  if (!selectedAgent.value) return
  levelEvaluationRunning.value = true
  try {
    await adminAPI.agents.runAgentLevelEvaluation(selectedAgent.value.agent_id)
    appStore.showSuccess(t('admin.agents.agentLevelEvaluationCompleted'))
    await reloadAgents()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.agents.failedToRunLevelEvaluation'))
  } finally {
    levelEvaluationRunning.value = false
  }
}

async function reloadAgents() {
  loading.value = true
  try {
    const res = await adminAPI.agents.listSettlementCandidates({
      page: currentPage.value,
      page_size: pagination.page_size,
      search: search.value || undefined,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
      settlement_status: settlementStatus.value as any,
      start: startDate.value || undefined,
      end: endDate.value || undefined
    })
    agents.value = res.items || []
    pagination.total = res.total || 0
    pagination.page = res.page || 1
    pagination.page_size = res.page_size || 20
    pagination.pages = res.pages || 1
    await reconcileSelectedAgent()
  } catch (error: any) {
    appStore.showError(buildAuthErrorMessage(error, { fallback: t('admin.agents.failedToLoad') }))
  } finally {
    loading.value = false
  }
}

async function selectAgent(agent: AdminAgentSummary) {
  syncSelectedAgent(agent)
  await loadDetail()
}

async function openAgentDetails(agent: AdminAgentSummary) {
  await selectAgent(agent)
  await openPaymentProfile(agent)
}

async function reconcileSelectedAgent() {
  if (agents.value.length === 0) {
    clearSelectedAgent()
    return
  }

  const selectedId = selectedAgent.value?.agent_id
  const nextAgent = selectedId
    ? agents.value.find((item) => item.agent_id === selectedId) || agents.value[0]
    : agents.value[0]
  await selectAgent(nextAgent)
}

function syncSelectedAgent(agent: AdminAgentSummary) {
  selectedAgent.value = agent
  agentRateForm.enabled = agent.override_enabled
  agentRateForm.consumption = toPercentNumber(agent.override_consumption_rate ?? agent.consumption_rate)
}

function clearSelectedAgent() {
  selectedAgent.value = null
  detailUsers.value = []
  detailCommissions.value = []
  detailSettlements.value = []
}

async function loadDetail() {
  if (!selectedAgent.value) return
  const id = selectedAgent.value.agent_id
  try {
    const [users, commissions, settlements] = await Promise.all([
      adminAPI.agents.listUsers(id, { page: 1, page_size: 10, start: startDate.value, end: endDate.value }),
      adminAPI.agents.listCommissions(id, { page: 1, page_size: 10, start: startDate.value, end: endDate.value }),
      adminAPI.agents.listSettlements(id, { page: 1, page_size: 10 })
    ])
    detailUsers.value = users.items || []
    detailCommissions.value = commissions.items || []
    detailSettlements.value = settlements.items || []
  } catch (error: any) {
    appStore.showError(buildAuthErrorMessage(error, { fallback: t('admin.agents.failedToLoad') }))
  }
}

async function saveAgentRate() {
  if (!selectedAgent.value) return
  agentRateSaving.value = true
  try {
    await adminAPI.agents.updateAgentRate(selectedAgent.value.agent_id, {
      enabled: agentRateForm.enabled,
      consumption_rate: fromPercent(agentRateForm.consumption)
    })
    appStore.showSuccess(t('admin.agents.agentRateUpdated'))
    await reloadAgents()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.agents.failedToSave'))
  } finally {
    agentRateSaving.value = false
  }
}

async function openSettlement(agent: AdminAgentSummary) {
  if (selectedAgent.value?.agent_id !== agent.agent_id) {
    await selectAgent(agent)
  }
  setSettlementQRPreview('')
  settlementDialog.show = true
  settlementDialog.agent = agent
  settlementDialog.amount = floorMoney(agent.unsettled_commission || 0)
  settlementDialog.note = ''
  settlementDialog.paymentReference = ''
  settlementDialog.paymentProfile = null
  try {
    const profile = await adminAPI.agents.getPaymentProfile(agent.agent_id)
    settlementDialog.paymentProfile = profile
    if (profile.has_alipay_qr) {
      const blob = await adminAPI.agents.getPaymentQRCode(agent.agent_id)
      setSettlementQRPreview(await imageBlobToDataURL(blob))
    }
  } catch (error: any) {
    appStore.showError(buildAuthErrorMessage(error, { fallback: t('admin.agents.failedToLoad') }))
  }
}

function closeSettlement() {
  settlementDialog.show = false
  settlementDialog.agent = null
  settlementDialog.paymentProfile = null
  setSettlementQRPreview('')
}

async function openPaymentProfile(agent: AdminAgentSummary | null) {
  if (!agent) return
  if (selectedAgent.value?.agent_id !== agent.agent_id) {
    await selectAgent(agent)
  }
  setPaymentProfileQRPreview('')
  paymentProfileDialog.show = true
  paymentProfileDialog.loading = true
  paymentProfileDialog.agent = agent
  paymentProfileDialog.paymentProfile = null
  try {
    const profile = await adminAPI.agents.getPaymentProfile(agent.agent_id)
    paymentProfileDialog.paymentProfile = profile
    if (profile.has_alipay_qr) {
      const blob = await adminAPI.agents.getPaymentQRCode(agent.agent_id)
      setPaymentProfileQRPreview(await imageBlobToDataURL(blob))
    }
  } catch (error: any) {
    appStore.showError(buildAuthErrorMessage(error, { fallback: t('admin.agents.failedToLoad') }))
  } finally {
    paymentProfileDialog.loading = false
  }
}

function closePaymentProfile() {
  paymentProfileDialog.show = false
  paymentProfileDialog.agent = null
  paymentProfileDialog.paymentProfile = null
  paymentProfileDialog.loading = false
  setPaymentProfileQRPreview('')
}

function openBindUser() {
  bindUserDialog.show = true
  bindUserDialog.email = ''
  bindUserDialog.overwriteAgent = false
}

function closeBindUser() {
  bindUserDialog.show = false
  bindUserDialog.email = ''
  bindUserDialog.overwriteAgent = false
}

async function submitBindUser() {
  if (!selectedAgent.value || !bindUserDialog.email) return
  bindUserSaving.value = true
  try {
    const result = await adminAPI.agents.bindUser(selectedAgent.value.agent_id, {
      email: bindUserDialog.email,
      overwrite_agent: bindUserDialog.overwriteAgent
    })
    const messageKey = result.already_bound
      ? 'admin.agents.userAlreadyBound'
      : result.overwritten
        ? 'admin.agents.userRebound'
        : 'admin.agents.userBound'
    appStore.showSuccess(t(messageKey, { email: result.email }))
    closeBindUser()
    await reloadAgents()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.agents.failedToBindUser'))
  } finally {
    bindUserSaving.value = false
  }
}

async function submitSettlement() {
  if (!settlementDialog.agent) return
  settlementSaving.value = true
  try {
    await adminAPI.agents.createSettlement(settlementDialog.agent.agent_id, settlementDialog.amount, settlementDialog.note, settlementDialog.paymentReference)
    appStore.showSuccess(t('admin.agents.settlementCreated'))
    closeSettlement()
    await reloadAgents()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.agents.failedToSettle'))
  } finally {
    settlementSaving.value = false
  }
}

function changeAgentPage(page: number) {
  currentPage.value = page
  reloadAgents()
}

function displayAgent(agent: AdminAgentSummary): string {
  return agent.username || agent.email || `#${agent.agent_id}`
}

function settlementStatusLabel(agent: AdminAgentSummary): string {
  if (!agent.payment_profile_complete) return t('admin.agents.settlementMissingProfile')
  if (agent.settlement_eligible) return t('admin.agents.settlementEligible')
  return t('admin.agents.settlementGapLabel', { amount: money(agent.settlement_gap || agent.settlement_minimum_amount || 50) })
}

function settlementStatusClass(agent: AdminAgentSummary): string {
  if (!agent.payment_profile_complete) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  if (agent.settlement_eligible) return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

function setSettlementQRPreview(url: string) {
  if (settlementDialog.qrPreviewUrl.startsWith('blob:')) {
    URL.revokeObjectURL(settlementDialog.qrPreviewUrl)
  }
  settlementDialog.qrPreviewUrl = url
}

function setPaymentProfileQRPreview(url: string) {
  if (paymentProfileDialog.qrPreviewUrl.startsWith('blob:')) {
    URL.revokeObjectURL(paymentProfileDialog.qrPreviewUrl)
  }
  paymentProfileDialog.qrPreviewUrl = url
}

function money(value: number): string {
  return `$${Number(value || 0).toFixed(2)}`
}

function floorMoney(value: number): number {
  return Math.floor(Number(value || 0) * 100) / 100
}

function percent(value: number): string {
  return `${toPercentNumber(value).toFixed(2)}%`
}

function formatLevel(level?: string): string {
  const key = (level || '').trim()
  if (!key) return '-'
  const found = levelRules.value.find((rule) => rule.level_key === key)
  if (found) return displayAgentLevelName(found.level_name, found.level_key)
  const localeKey = `admin.agents.level_${key}`
  const translated = t(localeKey)
  if (translated !== localeKey) return translated
  return displayAgentLevelName(key)
}

function displayAgentLevelName(name?: string | null, fallbackKey?: string): string {
  const normalized = (name || '').trim()
  switch (normalized) {
    case '轻代理':
      return t('admin.agents.level_light')
    case '标准代理':
      return t('admin.agents.level_standard')
    case '核心代理':
      return t('admin.agents.level_core')
    case '超级代理':
      return t('admin.agents.level_super')
    default: {
      if (normalized) return normalized
      if (!fallbackKey) return '-'
      const localeKey = `admin.agents.level_${fallbackKey}`
      const translated = t(localeKey)
      return translated === localeKey ? fallbackKey : translated
    }
  }
}

function toPercentNumber(value: number): number {
  return Number(((value || 0) * 100).toFixed(4))
}

function fromPercent(value: number): number {
  return Number(((value || 0) / 100).toFixed(6))
}

function normalizeOptionalNumber(value: number | null | undefined): number | null {
  if (value === null || value === undefined || (value as any) === '') return null
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : null
}

function date(value: string): string {
  return value ? new Date(value).toLocaleDateString() : '-'
}

function datetime(value: string): string {
  return value ? new Date(value).toLocaleString() : '-'
}

function commissionType(type: string): string {
  const key = `agent.type_${type}`
  const translated = t(key)
  return translated === key ? type : translated
}

async function copyInvite(agent: AdminAgentSummary) {
  if (!agent.invite_code) return
  await copyToClipboard(`${window.location.origin}/register?ref=${agent.invite_code}`, t('common.copiedToClipboard'))
}

watch([sortBy, sortOrder, settlementStatus], () => {
  currentPage.value = 1
  reloadAgents()
})

onMounted(async () => {
  await loadRates()
  await loadSettlementSettings()
  await loadLevelRules()
  await reloadAgents()
})

onUnmounted(() => {
  setSettlementQRPreview('')
  setPaymentProfileQRPreview('')
})
</script>
