<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-6 pb-12">
      <header class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.2em] text-primary-700 dark:text-primary-300">合伙人计划 V3</p>
          <h1 class="mt-1 text-3xl font-bold tracking-tight text-gray-950 dark:text-white">联盟运营台</h1>
          <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">计划开关、合伙人总览、收款审核、人工打款和社群引导的单一操作入口。</p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="loadAll">刷新</button>
      </header>

      <div v-if="error" class="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
        {{ error }}
      </div>

      <section v-if="loading" class="grid gap-4 md:grid-cols-3">
        <div v-for="item in 3" :key="item" class="card h-44 animate-pulse bg-gray-100 dark:bg-dark-800" />
      </section>

      <template v-else>
        <section v-if="operationsSummary && (operationsSummary.actionable_total > 0 || operationsSummary.qualified_followup > 0)" class="card flex flex-wrap items-center gap-3 p-4" aria-label="联盟待处理事项">
          <button v-if="operationsSummary.actionable_total > 0" type="button" class="inline-flex items-center gap-2 rounded-xl bg-primary-600 px-4 py-2 text-sm font-semibold text-white" @click="openFirstActionableQueue">
            待处理
            <span class="rounded-full bg-white/20 px-2 py-0.5 text-xs">{{ operationsSummary.actionable_total }}</span>
          </button>
          <button
            v-for="item in operationsQueueChips"
            :key="item.tab"
            type="button"
            class="inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-sm font-medium"
            :class="item.tone === 'danger'
              ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300'
              : item.tone === 'info'
                ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950 dark:text-blue-300'
                : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300'"
            @click="activeTab = item.tab"
          >
            {{ item.label }} <span class="font-bold">{{ item.count }}</span>
          </button>
        </section>

        <nav class="card flex flex-wrap gap-2 p-2" aria-label="联盟运营台子页面">
          <button
            v-for="tab in affiliateTabs"
            :key="tab.id"
            type="button"
            class="inline-flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition"
            :class="activeTab === tab.id
              ? 'bg-primary-600 text-white shadow-sm'
              : 'text-gray-600 hover:bg-gray-100 hover:text-gray-950 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white'"
            @click="activeTab = tab.id"
          >
            <span>{{ tab.label }}</span>
            <span
              v-if="tab.count !== null && tab.count > 0"
              class="rounded-full px-2 py-0.5 text-xs"
              :class="activeTab === tab.id ? 'bg-white/20 text-white' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'"
            >
              {{ tab.count }}
            </span>
          </button>
        </nav>

        <section v-if="activeTab === 'rules' && program" class="card overflow-hidden">
          <div class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <div>
              <h2 class="text-xl font-semibold text-gray-950 dark:text-white">计划模式与核心规则</h2>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">必须先经过观察模式验证，才能进入正式模式；金额按固定精度保存。</p>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-xs text-gray-600 dark:text-dark-300">版本 {{ program.revision }}</span>
              <span class="rounded-full px-3 py-1 text-xs font-semibold" :class="programModeClass">{{ formatProgramMode(program.mode) }}</span>
            </div>
          </div>

          <form class="p-6" @submit.prevent="saveProgram">
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <label class="block">
                <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300">运行模式</span>
                <select v-model="programForm.mode" class="input">
                  <option value="off">关闭 · 不记录不入账</option>
                  <option value="shadow">观察 · 只观察不入账</option>
                  <option value="live" :disabled="program.mode === 'off'">正式 · 正式入账</option>
                </select>
              </label>
              <NumberField v-model="programForm.ordinaryReferralRate" label="邀请人首付奖励（固定）" suffix="%" :disabled="true" />
              <NumberField v-model="programForm.ordinaryInviteeRate" label="被邀请人首付奖励（固定）" suffix="%" :disabled="true" />
              <NumberField v-model="programForm.directUserCount" label="路线 A 有效用户" suffix="人" :min="1" :step="1" />
              <NumberField v-model="programForm.perUserConsumption" label="单个有效用户消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.directTeamConsumption" label="路线 A 团队消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.selfConsumption" label="路线 B 本人消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.maxCampaignLinks" label="最多活动链接" suffix="条" :min="0" :max="100" :step="1" />
              <NumberField v-model="programForm.conversionMultiplier" label="现金转额度倍率（固定）" suffix="×" :disabled="true" />
              <NumberField v-model="programForm.withdrawalMinimum" label="最低提现金额" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.withdrawalSLAHours" label="处理时限" suffix="小时" :min="1" :max="168" :step="1" />
              <NumberField v-model="programForm.marginFloor" label="压力毛利率底线" suffix="%" :min="35" :max="100" :step="1" />
              <NumberField v-model="programForm.stressCostPerRawCredit" label="每1个原始额度的保守成本" prefix="¥" :min="0.01" :step="0.01" />
              <NumberField v-model="programForm.operationalReserve" label="额外风险预留（按售价）" suffix="%" :min="2" :max="30" :step="0.5" />
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900 sm:col-span-2 lg:col-span-3">
                <p class="text-xs text-gray-600 dark:text-dark-300">合伙人奖励池</p>
                <p class="mt-1 text-xl font-bold text-gray-950 dark:text-white">{{ program.agent_pool_rate_bps / 100 }}%</p>
                <p class="mt-1 text-xs text-gray-600 dark:text-dark-300">该值由后端锁死，不可扩大；客户返利 + 合伙人现金佣金固定共 10%。</p>
              </div>
            </div>
            <div class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-5 dark:border-dark-800">
              <p class="text-xs leading-5 text-gray-600 dark:text-dark-300">
                {{ program.started_at ? `首次正式启用：${formatBeijingTime(program.started_at)}` : '尚未进入正式模式；首次启用时间将由服务器保存，并按北京时间展示。' }}
                月卡 1 个原始额度等于用户看到的 10⚡；保存时会立即校验全部在售套餐的 35% 毛利底线。
              </p>
              <button class="btn btn-primary" :disabled="programSaving">{{ programSaving ? '保存中…' : '保存计划设置' }}</button>
            </div>
          </form>
        </section>

        <section v-if="activeTab === 'rules' && commercialPolicy" class="card overflow-hidden">
          <div class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <div>
              <h2 class="text-xl font-semibold text-gray-950 dark:text-white">35% 压力毛利门禁</h2>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">
                已扣除链动小铺 3% 手续费、佣金转额度后的 12% 最坏联盟负担、{{ (commercialPolicy.operational_reserve_bps / 100).toFixed(1) }}% 运营储备，并按每单位原始额度 ¥{{ commercialPolicy.stress_cost_per_credit.toFixed(2) }} 测算。
              </p>
            </div>
            <span class="rounded-full px-3 py-1 text-xs font-semibold" :class="commercialPolicy.passes_configured_margin_gate ? 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300' : 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300'">
              {{ commercialPolicy.passes_configured_margin_gate ? `通过 · 最低 ${commercialPolicy.minimum_stress_margin_percent.toFixed(2)}%` : '未通过 · 禁止保存' }}
            </span>
          </div>
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-800">
              <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900 dark:text-dark-400">
                <tr>
                  <th class="px-5 py-3 text-left font-medium">产品</th>
                  <th class="px-5 py-3 text-right font-medium">售价</th>
                  <th class="px-5 py-3 text-right font-medium">平台额度</th>
                  <th class="px-5 py-3 text-right font-medium">压力成本</th>
                  <th class="px-5 py-3 text-right font-medium">小铺毛利率</th>
                  <th class="px-5 py-3 text-right font-medium">直付毛利率</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                <tr v-for="item in commercialPolicy.packages" :key="item.id">
                  <td class="px-5 py-3 font-medium text-gray-900 dark:text-white">{{ item.name }}</td>
                  <td class="px-5 py-3 text-right text-gray-700 dark:text-dark-200">
                    ¥{{ item.shop_price_cny }} / ¥{{ item.direct_price_cny }}
                  </td>
                  <td class="px-5 py-3 text-right text-gray-700 dark:text-dark-200">⚡{{ item.platform_credits.toLocaleString() }}</td>
                  <td class="px-5 py-3 text-right text-gray-700 dark:text-dark-200">¥{{ item.stress_cost_cny.toFixed(2) }}</td>
                  <td class="px-5 py-3 text-right font-semibold" :class="item.passes_configured_margin_gate ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                    {{ item.shop_stress_margin_percent.toFixed(2) }}%
                  </td>
                  <td class="px-5 py-3 text-right font-semibold" :class="item.passes_configured_margin_gate ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                    {{ item.direct_stress_margin_percent.toFixed(2) }}%
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="grid gap-3 border-t border-gray-100 p-5 dark:border-dark-800 sm:grid-cols-2 lg:grid-cols-4">
            <div v-for="target in commercialPolicy.group_targets" :key="target.id" class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
              <p class="text-xs text-gray-600 dark:text-dark-300">{{ target.name }}</p>
              <p class="mt-1 text-lg font-bold text-gray-950 dark:text-white">{{ target.rate_multiplier.toFixed(2) }}×</p>
            </div>
          </div>
          <p class="border-t border-gray-100 px-5 py-4 text-xs text-gray-500 dark:border-dark-800 dark:text-dark-400">
            GPT 成本按便宜账号 30%（0.15）+ 贵账号 70%（0.20）计算，混合账号倍率 {{ commercialPolicy.gpt_cost_mix.blended_account_multiplier.toFixed(3) }}。
          </p>
        </section>

        <section v-if="activeTab === 'qualified'" class="card overflow-hidden">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <div>
              <h2 class="text-xl font-semibold text-gray-950 dark:text-white">已达标待申请</h2>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">这些用户已满足当前消费门槛，但尚未主动提交申请。这里只用于运营跟进，不会自动开通合伙人或产生佣金。</p>
            </div>
            <span class="rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-700 dark:bg-blue-950 dark:text-blue-300">{{ qualifiedCandidates.length }} 人待申请</span>
          </div>
          <div class="max-h-[38rem] overflow-auto">
            <table class="min-w-[1100px] table-fixed divide-y divide-gray-100 text-sm dark:divide-dark-800">
              <thead class="sticky top-0 z-10 bg-gray-50 text-xs text-gray-600 dark:bg-dark-900 dark:text-dark-300">
                <tr>
                  <th class="px-5 py-3 text-left font-medium">用户</th>
                  <th class="px-5 py-3 text-left font-medium">达标路线</th>
                  <th class="px-5 py-3 text-right font-medium">有效直属</th>
                  <th class="px-5 py-3 text-right font-medium">本人确认消费</th>
                  <th class="px-5 py-3 text-right font-medium">直属确认消费</th>
                  <th class="px-5 py-3 text-right font-medium">合计确认消费</th>
                  <th class="px-5 py-3 text-left font-medium">下一步</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                <tr v-for="item in qualifiedCandidates" :key="item.user_id" class="align-top">
                  <td class="px-5 py-4">
                    <p class="font-semibold text-gray-900 dark:text-white">#{{ item.user_id }} · {{ item.username || item.email }}</p>
                    <p v-if="item.username" class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.email }}</p>
                  </td>
                  <td class="px-5 py-4">{{ formatQualificationRoute(item.qualification_route) }}</td>
                  <td class="px-5 py-4 text-right">{{ item.valid_direct_user_count }} 人</td>
                  <td class="px-5 py-4 text-right">{{ formatMicros(item.self_consumption_micros, '¥') }}</td>
                  <td class="px-5 py-4 text-right">{{ formatMicros(item.direct_team_consumption_micros, '¥') }}</td>
                  <td class="px-5 py-4 text-right font-semibold text-gray-900 dark:text-white">{{ formatMicros(item.combined_consumption_micros, '¥') }}</td>
                  <td class="px-5 py-4 text-gray-600 dark:text-dark-300">等待用户在联盟计划页面提交申请</td>
                </tr>
                <tr v-if="!qualifiedCandidates.length"><td colspan="7" class="px-5 py-12 text-center text-gray-600 dark:text-dark-300">当前没有已达标但尚未申请的用户</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section v-if="activeTab === 'applications'" class="card overflow-hidden">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <div>
              <h2 class="text-xl font-semibold text-gray-950 dark:text-white">合伙人申请</h2>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">达标后系统自动开通合伙人并生成默认链接；此处保留全部申请与开通记录备查，遗留待审记录仍可人工处理。</p>
            </div>
            <span class="rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-700 dark:bg-amber-950 dark:text-amber-300">{{ pendingApplications.length }} 待审核</span>
          </div>
          <div class="max-h-[38rem] overflow-auto">
            <table class="min-w-[1350px] table-fixed divide-y divide-gray-100 text-sm dark:divide-dark-800">
              <thead class="sticky top-0 z-10 bg-gray-50 text-xs text-gray-600 dark:bg-dark-900 dark:text-dark-300">
                <tr>
                  <th class="px-5 py-3 text-left font-medium">申请人</th>
                  <th class="px-5 py-3 text-left font-medium">达标路线</th>
                  <th class="px-5 py-3 text-right font-medium">有效直属</th>
                  <th class="px-5 py-3 text-right font-medium">本人确认消费</th>
                  <th class="px-5 py-3 text-right font-medium">直属确认消费</th>
                  <th class="px-5 py-3 text-right font-medium">合并确认消费</th>
                  <th class="px-5 py-3 text-left font-medium">申请说明</th>
                  <th class="px-5 py-3 text-left font-medium">审核说明</th>
                  <th class="sticky right-0 bg-gray-50 px-5 py-3 text-right font-medium dark:bg-dark-900">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                <tr v-for="item in applications" :key="item.id" class="align-top">
                  <td class="px-5 py-4">
                    <p class="font-semibold text-gray-900 dark:text-white">#{{ item.user_id }} · {{ item.username || item.email }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatOptionalBeijingTime(item.submitted_at) }}</p>
                  </td>
                  <td class="px-5 py-4">{{ formatQualificationRoute(item.qualifying_route) }}</td>
                  <td class="px-5 py-4 text-right">{{ item.valid_direct_user_count }} 人</td>
                  <td class="px-5 py-4 text-right">{{ formatMicros(item.self_consumption_micros, '¥') }}</td>
                  <td class="px-5 py-4 text-right">{{ formatMicros(item.direct_team_consumption_micros, '¥') }}</td>
                  <td class="px-5 py-4 text-right">{{ formatMicros(item.combined_consumption_micros, '¥') }}</td>
                  <td class="px-5 py-4 text-gray-600 dark:text-dark-300">{{ item.application_note || '—' }}</td>
                  <td class="px-5 py-4">
                    <input v-if="item.status === 'pending_review'" v-model.trim="applicationNotes[item.id]" maxlength="500" class="input min-w-48" placeholder="审核说明">
                    <span v-else class="text-gray-600 dark:text-dark-300">{{ formatApplicationDecision(item) }}</span>
                  </td>
                  <td class="sticky right-0 bg-white px-5 py-4 text-right dark:bg-dark-900">
                    <div v-if="item.status === 'pending_review'" class="flex justify-end gap-2">
                      <button class="btn btn-secondary btn-sm" :disabled="applicationReviewingId === item.id" @click="reviewApplication(item, false)">不通过</button>
                      <button class="btn btn-primary btn-sm" :disabled="applicationReviewingId === item.id" @click="reviewApplication(item, true)">通过并开通</button>
                    </div>
                    <span v-else class="rounded-full px-2 py-1 text-xs font-medium" :class="applicationStatusClass(item.status)">
                      {{ formatApplicationStatus(item.status) }}
                    </span>
                  </td>
                </tr>
                <tr v-if="!applications.length"><td colspan="9" class="px-5 py-12 text-center text-gray-600 dark:text-dark-300">暂无申请记录</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section v-if="activeTab === 'partners' || activeTab === 'risk'" class="space-y-6">
          <article v-if="activeTab === 'partners'" class="card overflow-hidden">
            <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
              <div>
                <h2 class="text-xl font-semibold text-gray-950 dark:text-white">合伙人管理</h2>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">查看当前全部合伙人；发现异常时可先暂停邀请、提现和佣金转额度，确认没问题后再恢复。</p>
                <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">
                  本人消费返佣仅适用于单独开启后新购买并实际使用的付费权益。关闭只影响关闭后新购买的权益；如需立即停止结算，请将合伙人状态改为“待审核”或“已暂停”。
                </p>
              </div>
              <div class="flex flex-wrap justify-end gap-2">
                <span class="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                  共 {{ agentOverviewStats.total }} 位合伙人
                </span>
                <span class="rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-700 dark:bg-amber-950 dark:text-amber-300">
                  {{ agentOverviewStats.abnormal }} 个异常
                </span>
              </div>
            </div>
            <div class="border-b border-gray-100 dark:border-dark-800">
              <div class="flex flex-wrap items-center justify-between gap-3 px-6 py-4">
                <div>
                  <h3 class="font-semibold text-gray-950 dark:text-white">合伙人业绩</h3>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">默认统计成为合伙人以来；充值仅计真实付费权益，消费仅计联盟确认消费并扣除冲销。</p>
                </div>
                <span class="text-xs text-gray-500 dark:text-dark-400">点击“查看明细”可按时间筛选直属用户、佣金和提现记录</span>
              </div>
              <div class="max-h-[30rem] overflow-auto">
                <table class="min-w-[1880px] table-fixed divide-y divide-gray-100 text-sm dark:divide-dark-800">
                  <thead class="sticky top-0 z-10 bg-gray-50 text-xs text-gray-600 dark:bg-dark-900 dark:text-dark-300">
                    <tr>
                      <th class="px-5 py-3 text-left font-medium">合伙人</th>
                      <th class="px-5 py-3 text-right font-medium">直属 / 付费</th>
                      <th class="px-5 py-3 text-right font-medium">本人充值</th>
                      <th class="px-5 py-3 text-right font-medium">本人消费</th>
                      <th class="px-5 py-3 text-right font-medium">团队充值</th>
                      <th class="px-5 py-3 text-right font-medium">团队消费</th>
                      <th class="px-5 py-3 text-right font-medium">近30天消费</th>
                      <th class="px-5 py-3 text-right font-medium">累计佣金</th>
                      <th class="px-5 py-3 text-right font-medium">可提现</th>
                      <th class="px-5 py-3 text-right font-medium">提现中</th>
                      <th class="px-5 py-3 text-right font-medium">已返佣</th>
                      <th class="sticky right-0 bg-gray-50 px-5 py-3 text-right font-medium dark:bg-dark-900">操作</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                    <tr v-for="item in partnerPerformance" :key="item.agent_id">
                      <td class="px-5 py-4"><p class="font-semibold text-gray-900 dark:text-white">#{{ item.agent_id }} · {{ item.username || item.email }}</p><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">开通 {{ formatBeijingDate(item.activated_at) }}</p></td>
                      <td class="px-5 py-4 text-right">{{ item.direct_user_count }} / {{ item.paid_direct_user_count }}</td>
                      <td class="px-5 py-4 text-right">{{ formatMicros(item.self_recharge_micros, '¥') }}</td>
                      <td class="px-5 py-4 text-right">{{ formatMicros(item.self_consumption_micros, '¥') }}</td>
                      <td class="px-5 py-4 text-right">{{ formatMicros(item.direct_team_recharge_micros, '¥') }}</td>
                      <td class="px-5 py-4 text-right font-semibold">{{ formatMicros(item.direct_team_consumption_micros, '¥') }}</td>
                      <td class="px-5 py-4 text-right">{{ formatMicros(item.recent_30d_consumption_micros, '¥') }}</td>
                      <td class="px-5 py-4 text-right">{{ formatMicros(item.lifetime_earned_micros, '¥') }}</td>
                      <td class="px-5 py-4 text-right font-semibold text-green-600 dark:text-green-400">{{ formatMicros(item.available_commission_micros, '¥') }}</td>
                      <td class="px-5 py-4 text-right">{{ formatMicros(item.processing_withdrawal_micros, '¥') }}</td>
                      <td class="px-5 py-4 text-right">{{ formatMicros(item.paid_commission_micros, '¥') }}</td>
                      <td class="sticky right-0 bg-white px-5 py-4 text-right dark:bg-dark-900"><button class="btn btn-secondary btn-sm whitespace-nowrap" @click="openPerformanceDetail(item)">查看明细</button></td>
                    </tr>
                    <tr v-if="!partnerPerformance.length"><td colspan="12" class="px-5 py-10 text-center text-gray-500 dark:text-dark-400">暂无合伙人业绩</td></tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div class="grid gap-3 border-b border-gray-100 p-5 dark:border-dark-800 sm:grid-cols-2 lg:grid-cols-5">
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <p class="text-xs text-gray-600 dark:text-dark-300">合伙人总数</p>
                <p class="mt-1 text-2xl font-bold text-gray-950 dark:text-white">{{ agentOverviewStats.total }}</p>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <p class="text-xs text-gray-600 dark:text-dark-300">正常合伙人</p>
                <p class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">{{ agentOverviewStats.clear }}</p>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <p class="text-xs text-gray-600 dark:text-dark-300">需处理</p>
                <p class="mt-1 text-2xl font-bold text-amber-600 dark:text-amber-400">{{ agentOverviewStats.abnormal }}</p>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <p class="text-xs text-gray-600 dark:text-dark-300">待审收款码</p>
                <p class="mt-1 text-2xl font-bold text-gray-950 dark:text-white">{{ pendingProfiles.length }}</p>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <p class="text-xs text-gray-600 dark:text-dark-300">提现处理中</p>
                <p class="mt-1 text-2xl font-bold text-gray-950 dark:text-white">{{ withdrawals.length }}</p>
              </div>
            </div>
            <div class="max-h-[34rem] overflow-auto">
              <table class="min-w-[1540px] table-fixed divide-y divide-gray-100 text-sm dark:divide-dark-800">
                <colgroup>
                  <col class="w-[250px]">
                  <col class="w-[130px]">
                  <col class="w-[260px]">
                  <col class="w-[110px]">
                  <col class="w-[110px]">
                  <col class="w-[220px]">
                  <col class="w-[170px]">
                  <col class="w-[220px]">
                  <col class="w-[120px]">
                </colgroup>
                <thead class="sticky top-0 z-10 bg-gray-50 text-xs uppercase tracking-wider text-gray-600 dark:bg-dark-900 dark:text-dark-300">
                  <tr>
                    <th class="px-5 py-3 text-left font-medium">合伙人</th>
                    <th class="px-5 py-3 text-left font-medium">状态</th>
                    <th class="px-5 py-3 text-left font-medium">本人消费返佣</th>
                    <th class="px-5 py-3 text-right font-medium">暂缓额度</th>
                    <th class="px-5 py-3 text-right font-medium">暂缓现金</th>
                    <th class="px-5 py-3 text-left font-medium">最近原因</th>
                    <th class="px-5 py-3 text-left font-medium">处理状态</th>
                    <th class="px-5 py-3 text-left font-medium">处理说明</th>
                    <th class="sticky right-0 border-l border-gray-100 bg-gray-50 px-5 py-3 text-right font-medium dark:border-dark-800 dark:bg-dark-900">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                  <tr v-for="item in riskPrincipals" :key="item.agent_id" class="align-top hover:bg-gray-50/70 dark:hover:bg-dark-800/60">
                    <td class="whitespace-nowrap px-5 py-4">
                      <p class="font-semibold text-gray-900 dark:text-white">#{{ item.agent_id }} · {{ item.username || item.email }}</p>
                      <p class="mt-1 text-xs text-gray-600 dark:text-dark-300">{{ item.email }}</p>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4">
                      <div class="flex flex-col items-start gap-2">
                        <span class="rounded-full px-3 py-1 text-xs font-semibold" :class="agentStatusClass(item.agent_status)">
                          {{ formatAgentStatus(item.agent_status) }}
                        </span>
                        <span class="rounded-full px-3 py-1 text-xs font-semibold" :class="riskStatusClass(item.risk_status)">
                          {{ formatRiskStatus(item.risk_status) }}
                        </span>
                      </div>
                    </td>
                    <td class="px-5 py-4">
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <span
                            class="inline-flex rounded-full px-3 py-1 text-xs font-semibold"
                            :class="selfCommissionStatusClass(selfCommissionPresentation(item).tone)"
                          >
                            {{ selfCommissionPresentation(item).label }}
                          </span>
                          <p class="mt-2 text-xs leading-5 text-gray-600 dark:text-dark-300">
                            {{ selfCommissionPresentation(item).detail }}
                          </p>
                          <p v-if="item.self_commission_effective_at" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                            生效时间：{{ formatBeijingTime(item.self_commission_effective_at) }}
                          </p>
                          <div v-if="item.self_commission_revision > 0" class="mt-2 border-t border-gray-100 pt-2 text-xs leading-5 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                            <p>最近原因：{{ item.self_commission_reason || '—' }}</p>
                            <p v-if="item.self_commission_updated_at">
                              {{ formatBeijingTime(item.self_commission_updated_at) }} · {{ selfCommissionOperatorLabel(item) }}
                            </p>
                          </div>
                        </div>
                        <button
                          v-if="item.self_commission_enabled || selfCommissionPresentation(item).canEnable"
                          class="btn btn-secondary btn-sm shrink-0 whitespace-nowrap"
                          :disabled="selfCommissionUpdatingId === item.agent_id"
                          @click="openSelfCommissionDialog(item, !item.self_commission_enabled)"
                        >
                          {{ item.self_commission_enabled ? '关闭' : '开启' }}
                        </button>
                      </div>
                    </td>
                    <td class="px-5 py-4 text-right text-gray-700 dark:text-dark-200">
                      <p class="font-semibold">{{ formatMicros(item.held_reward_micros, '⚡') }}</p>
                      <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.held_reward_count }} 笔</p>
                    </td>
                    <td class="px-5 py-4 text-right text-gray-700 dark:text-dark-200">
                      <p class="font-semibold">{{ formatMicros(item.held_cash_micros, '¥') }}</p>
                      <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.held_cash_count }} 笔</p>
                    </td>
                    <td class="px-5 py-4 text-xs leading-5 text-gray-600 dark:text-dark-300">
                      {{ item.risk_note || '—' }}
                    </td>
                    <td class="whitespace-nowrap px-5 py-4">
                      <select v-model="riskTargets[item.agent_id]" class="input min-w-36">
                        <option value="clear">正常开放</option>
                        <option value="review">待审核（暂时停用）</option>
                        <option value="blocked">已暂停</option>
                      </select>
                    </td>
                    <td class="px-5 py-4">
                      <input v-model.trim="riskReasons[item.agent_id]" maxlength="500" class="input min-w-44" placeholder="处理原因">
                    </td>
                    <td class="sticky right-0 border-l border-gray-100 bg-white px-5 py-4 text-right dark:border-dark-800 dark:bg-dark-900">
                      <button class="btn btn-primary btn-sm whitespace-nowrap" :disabled="riskUpdatingId === item.agent_id" @click="applyRiskStatus(item)">
                        {{ riskUpdatingId === item.agent_id ? '保存中…' : '保存状态' }}
                      </button>
                    </td>
                  </tr>
                  <tr v-if="!riskPrincipals.length">
                    <td colspan="9" class="px-5 py-12 text-center text-sm text-gray-600 dark:text-dark-300">尚无合伙人</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </article>

          <article v-if="activeTab === 'risk'" class="card p-6">
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-red-600 dark:text-red-400">订单修正</p>
            <h2 class="mt-2 text-xl font-semibold text-gray-950 dark:text-white">撤回一笔确认消费</h2>
            <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">
              用于订单退款或误记账。填写原消费记录 ID 后，系统会撤回这笔消费带来的资格进度、客户返利和合伙人佣金；同一笔不会重复撤回。
            </p>
            <form class="mt-5 space-y-4" @submit.prevent="submitReversal">
              <label class="block">
                <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300">原消费记录 ID</span>
                <input v-model.number="reversalForm.eventId" type="number" min="1" required class="input">
              </label>
              <label class="block">
                <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300">处理说明</span>
                <textarea v-model.trim="reversalForm.reason" required maxlength="500" class="input min-h-28" placeholder="例如：订单退款、异常账号确认" />
              </label>
              <button class="btn w-full bg-red-600 text-white hover:bg-red-700" :disabled="reversalProcessing">
                {{ reversalProcessing ? '处理中…' : '确认撤回' }}
              </button>
            </form>
            <div v-if="lastReversal" class="mt-5 rounded-xl bg-green-50 p-4 text-sm text-green-800 dark:bg-green-950 dark:text-green-200">
              <p class="font-semibold">已撤回记录 #{{ lastReversal.id }}</p>
              <p class="mt-1">消费 {{ formatMicros(lastReversal.amount_micros, '¥') }} · 客户额度 {{ formatMicros(lastReversal.reversed_reward_micros, '⚡') }} · 现金 {{ formatMicros(lastReversal.reversed_cash_micros, '¥') }}</p>
            </div>
          </article>
        </section>

        <section v-if="activeTab === 'profiles'" class="space-y-6">
          <article class="card overflow-hidden">
            <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
              <div>
                <h2 class="text-xl font-semibold text-gray-950 dark:text-white">收款资料审核</h2>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">只有实名、账号和收款码通过审核后才允许提现。</p>
              </div>
              <span class="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-300">{{ pendingProfiles.length }} 待审</span>
            </div>
            <div class="max-h-[30rem] overflow-auto">
              <table class="min-w-[1100px] table-fixed divide-y divide-gray-100 text-sm dark:divide-dark-800">
                <colgroup>
                  <col class="w-[190px]">
                  <col class="w-[230px]">
                  <col class="w-[190px]">
                  <col class="w-[240px]">
                  <col class="w-[250px]">
                </colgroup>
                <thead class="sticky top-0 z-10 bg-gray-50 text-xs uppercase tracking-wider text-gray-600 dark:bg-dark-900 dark:text-dark-300">
                  <tr>
                    <th class="px-5 py-3 text-left font-medium">合伙人</th>
                    <th class="px-5 py-3 text-left font-medium">支付宝资料</th>
                    <th class="px-5 py-3 text-left font-medium">资料备注</th>
                    <th class="px-5 py-3 text-left font-medium">审核备注</th>
                    <th class="sticky right-0 border-l border-gray-100 bg-gray-50 px-5 py-3 text-right font-medium dark:border-dark-800 dark:bg-dark-900">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                  <tr v-for="profile in pendingProfiles" :key="profile.agent_id" class="align-top hover:bg-gray-50/70 dark:hover:bg-dark-800/60">
                    <td class="whitespace-nowrap px-5 py-4">
                      <p class="font-semibold text-gray-900 dark:text-white">#{{ profile.agent_id }} · {{ profile.alipay_real_name }}</p>
                      <p class="mt-1 text-xs text-gray-600 dark:text-dark-300">{{ profile.contact_phone }}</p>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-gray-700 dark:text-dark-200">{{ profile.alipay_account }}</td>
                    <td class="px-5 py-4 text-xs leading-5 text-gray-600 dark:text-dark-300">
                      {{ profile.payment_note || '—' }}
                    </td>
                    <td class="px-5 py-4">
                      <input v-model="reviewNotes[profile.agent_id]" maxlength="500" class="input min-w-52" placeholder="拒绝时必填">
                    </td>
                    <td class="sticky right-0 border-l border-gray-100 bg-white px-5 py-4 dark:border-dark-800 dark:bg-dark-900">
                      <div class="flex justify-end gap-2">
                        <button class="btn btn-secondary btn-sm whitespace-nowrap" @click="previewPaymentProfile(profile)">查看收款码</button>
                        <button class="btn btn-secondary btn-sm whitespace-nowrap" :disabled="reviewingId === profile.agent_id" @click="rejectPaymentProfile(profile)">拒绝并退回</button>
                        <button class="btn btn-primary btn-sm whitespace-nowrap" :disabled="reviewingId === profile.agent_id" @click="verifyPaymentProfile(profile)">验证通过</button>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="!pendingProfiles.length">
                    <td colspan="5" class="px-5 py-12 text-center text-sm text-gray-600 dark:text-dark-300">当前没有待审核资料</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </article>

        </section>

        <section v-if="activeTab === 'payouts'" class="space-y-6">
          <article class="card overflow-hidden">
            <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
              <div>
                <h2 class="text-xl font-semibold text-gray-950 dark:text-white">提现打款队列</h2>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">提交即“处理中”；人工扫码后只需标记“已到账”。</p>
              </div>
              <span class="rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-700 dark:bg-amber-950 dark:text-amber-300">{{ withdrawals.length }} 处理中</span>
            </div>
            <div class="max-h-[30rem] overflow-auto">
              <table class="min-w-[1120px] table-fixed divide-y divide-gray-100 text-sm dark:divide-dark-800">
                <colgroup>
                  <col class="w-[100px]">
                  <col class="w-[200px]">
                  <col class="w-[230px]">
                  <col class="w-[150px]">
                  <col class="w-[220px]">
                  <col class="w-[220px]">
                </colgroup>
                <thead class="sticky top-0 z-10 bg-gray-50 text-xs uppercase tracking-wider text-gray-600 dark:bg-dark-900 dark:text-dark-300">
                  <tr>
                    <th class="px-5 py-3 text-right font-medium">金额</th>
                    <th class="px-5 py-3 text-left font-medium">合伙人</th>
                    <th class="px-5 py-3 text-left font-medium">支付宝账号</th>
                    <th class="px-5 py-3 text-left font-medium">截止时间</th>
                    <th class="px-5 py-3 text-left font-medium">流水/失败原因</th>
                    <th class="sticky right-0 border-l border-gray-100 bg-gray-50 px-5 py-3 text-right font-medium dark:border-dark-800 dark:bg-dark-900">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                  <tr v-for="withdrawal in withdrawals" :key="withdrawal.id" class="align-top hover:bg-gray-50/70 dark:hover:bg-dark-800/60">
                    <td class="whitespace-nowrap px-5 py-4 text-right text-base font-bold text-gray-950 dark:text-white">{{ formatMicros(withdrawal.amount_micros, '¥') }}</td>
                    <td class="whitespace-nowrap px-5 py-4">
                      <p class="font-semibold text-gray-900 dark:text-white">#{{ withdrawal.agent_id }} · {{ withdrawal.payment_alipay_real_name }}</p>
                      <p v-if="withdrawal.agent_risk_status !== 'clear'" class="mt-1 text-xs font-semibold text-red-600 dark:text-red-400">当前已暂停，暂时不能确认打款</p>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-gray-700 dark:text-dark-200">{{ withdrawal.payment_alipay_account }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-xs text-gray-600 dark:text-dark-300">{{ formatBeijingTime(withdrawal.due_at) }}</td>
                    <td class="px-5 py-4">
                      <input v-model="paymentReferences[withdrawal.id]" maxlength="200" class="input min-w-52" placeholder="支付宝流水号（必填）">
                    </td>
                    <td class="sticky right-0 border-l border-gray-100 bg-white px-5 py-4 dark:border-dark-800 dark:bg-dark-900">
                      <div class="flex justify-end gap-2">
                        <button class="btn btn-secondary btn-sm whitespace-nowrap" @click="previewWithdrawalQR(withdrawal)">扫码打款</button>
                        <button class="btn btn-secondary btn-sm whitespace-nowrap" :disabled="processingWithdrawalId === withdrawal.id" @click="failWithdrawal(withdrawal)">打款失败</button>
                        <button class="btn btn-primary btn-sm whitespace-nowrap" :disabled="processingWithdrawalId === withdrawal.id || withdrawal.agent_risk_status !== 'clear' || !(paymentReferences[withdrawal.id] || '').trim()" @click="completeWithdrawal(withdrawal)">标记已到账</button>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="!withdrawals.length">
                    <td colspan="6" class="px-5 py-12 text-center text-sm text-gray-600 dark:text-dark-300">当前没有待打款申请</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </article>
        </section>

        <section v-if="activeTab === 'archive'" class="space-y-6">
          <article class="card overflow-hidden">
            <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
              <div>
                <h2 class="text-xl font-semibold text-gray-950 dark:text-white">已到账归档</h2>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">人工打款完成后的历史记录，方便和支付宝流水对账。</p>
              </div>
              <span class="rounded-full bg-green-100 px-3 py-1 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-300">{{ paidWithdrawals.length }} 已到账</span>
            </div>
            <div class="max-h-[30rem] overflow-auto">
              <table class="min-w-[1120px] table-fixed divide-y divide-gray-100 text-sm dark:divide-dark-800">
                <colgroup>
                  <col class="w-[100px]">
                  <col class="w-[200px]">
                  <col class="w-[230px]">
                  <col class="w-[160px]">
                  <col class="w-[160px]">
                  <col class="w-[220px]">
                  <col class="w-[120px]">
                </colgroup>
                <thead class="sticky top-0 z-10 bg-gray-50 text-xs uppercase tracking-wider text-gray-600 dark:bg-dark-900 dark:text-dark-300">
                  <tr>
                    <th class="px-5 py-3 text-right font-medium">金额</th>
                    <th class="px-5 py-3 text-left font-medium">合伙人</th>
                    <th class="px-5 py-3 text-left font-medium">支付宝账号</th>
                    <th class="px-5 py-3 text-left font-medium">申请时间</th>
                    <th class="px-5 py-3 text-left font-medium">到账时间</th>
                    <th class="px-5 py-3 text-left font-medium">流水号</th>
                    <th class="px-5 py-3 text-left font-medium">处理人</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                  <tr v-for="withdrawal in paidWithdrawals" :key="withdrawal.id" class="align-top hover:bg-gray-50/70 dark:hover:bg-dark-800/60">
                    <td class="whitespace-nowrap px-5 py-4 text-right text-base font-bold text-gray-950 dark:text-white">{{ formatMicros(withdrawal.amount_micros, '¥') }}</td>
                    <td class="whitespace-nowrap px-5 py-4">
                      <p class="font-semibold text-gray-900 dark:text-white">#{{ withdrawal.agent_id }} · {{ withdrawal.payment_alipay_real_name }}</p>
                      <p class="mt-1 text-xs text-gray-600 dark:text-dark-300">提现 #{{ withdrawal.id }}</p>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-gray-700 dark:text-dark-200">{{ withdrawal.payment_alipay_account }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-xs text-gray-600 dark:text-dark-300">{{ formatBeijingTime(withdrawal.requested_at) }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-xs text-gray-600 dark:text-dark-300">{{ formatOptionalBeijingTime(withdrawal.paid_at) }}</td>
                    <td class="px-5 py-4 text-gray-700 dark:text-dark-200">{{ withdrawal.payment_reference || '—' }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-gray-700 dark:text-dark-200">{{ withdrawal.handled_by ? `#${withdrawal.handled_by}` : '—' }}</td>
                  </tr>
                  <tr v-if="!paidWithdrawals.length">
                    <td colspan="7" class="px-5 py-12 text-center text-sm text-gray-600 dark:text-dark-300">当前没有已到账记录</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </article>
        </section>

        <section v-if="activeTab === 'community' && community" class="card overflow-hidden">
          <div class="border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <h2 class="text-xl font-semibold text-gray-950 dark:text-white">合伙人社群引导</h2>
            <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">启用后，仅已成为合伙人的用户能看到文字与二维码；二维码接口禁止公开缓存。</p>
          </div>
          <form class="grid gap-6 p-6 lg:grid-cols-[1fr_20rem]" @submit.prevent="saveCommunity">
            <div class="space-y-4">
              <label class="flex items-center gap-3">
                <input v-model="communityForm.enabled" type="checkbox" class="h-4 w-4 accent-primary-600">
                <span class="text-sm font-medium text-gray-800 dark:text-dark-200">启用合伙人社群引导</span>
              </label>
              <label class="block">
                <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300">标题</span>
                <input v-model.trim="communityForm.title" required maxlength="120" class="input">
              </label>
              <label class="block">
                <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300">加群说明</span>
                <textarea v-model.trim="communityForm.message" maxlength="2000" class="input min-h-36" />
              </label>
              <button class="btn btn-primary" :disabled="communitySaving">保存社群设置</button>
            </div>
            <div>
              <label class="block rounded-2xl border border-dashed border-gray-300 p-4 text-center text-sm text-gray-600 hover:border-primary-400 dark:border-dark-600 dark:text-dark-300">
                <input type="file" accept="image/png,image/jpeg,image/webp" class="sr-only" @change="uploadCommunityQR">
                {{ community.has_qr_code ? '更换社群二维码' : '上传社群二维码' }}
              </label>
              <img v-if="communityQRPreview" :src="communityQRPreview" alt="合伙人社群二维码预览" class="mx-auto mt-4 max-h-64 rounded-xl border border-gray-200 p-2 dark:border-dark-700">
              <p v-else class="mt-4 rounded-xl bg-gray-50 p-6 text-center text-sm text-gray-500 dark:bg-dark-900 dark:text-dark-400">尚未上传二维码</p>
            </div>
          </form>
        </section>
      </template>

      <Teleport to="body">
        <div
          v-if="performanceDetailAgent"
          data-testid="affiliate-performance-detail"
          class="fixed inset-0 z-[60] flex justify-end bg-black/60"
          role="dialog"
          aria-modal="true"
          aria-labelledby="performance-detail-title"
          @click.self="closePerformanceDetail"
        >
          <section class="h-[100dvh] w-full max-w-5xl overscroll-contain overflow-y-auto bg-white shadow-xl dark:bg-dark-900">
          <header class="sticky top-0 z-20 flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 bg-white px-6 py-5 dark:border-dark-800 dark:bg-dark-900">
            <div class="min-w-0 flex-1">
              <p class="text-xs font-semibold text-primary-700 dark:text-primary-300">合伙人业绩明细</p>
              <h2 id="performance-detail-title" class="mt-1 break-all text-xl font-bold text-gray-950 dark:text-white">#{{ performanceDetailAgent.agent_id }} · {{ performanceDetailAgent.username || performanceDetailAgent.email }}</h2>
            </div>
            <button class="btn btn-secondary btn-sm shrink-0" @click="closePerformanceDetail">关闭</button>
          </header>
          <div class="space-y-6 p-6">
            <div class="flex flex-wrap items-end gap-3">
              <label class="block"><span class="mb-1 block text-xs text-gray-500 dark:text-dark-400">统计区间</span><select v-model="performancePeriod" class="input min-w-44" @change="loadPerformanceDetail"><option value="since_activation">成为合伙人以来</option><option value="30d">最近30天</option><option value="month">本月</option><option value="custom">自定义</option></select></label>
              <template v-if="performancePeriod === 'custom'">
                <label class="block"><span class="mb-1 block text-xs text-gray-500 dark:text-dark-400">开始日期</span><input v-model="performanceCustomStart" type="date" class="input"></label>
                <label class="block"><span class="mb-1 block text-xs text-gray-500 dark:text-dark-400">结束日期</span><input v-model="performanceCustomEnd" type="date" class="input"></label>
                <button class="btn btn-primary" @click="loadPerformanceDetail">查询</button>
              </template>
            </div>
            <div v-if="performanceDetailLoading" class="card h-36 animate-pulse bg-gray-100 dark:bg-dark-800" />
            <template v-else-if="performanceDetail">
              <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <div v-for="metric in performanceDetailMetrics" :key="metric.label" class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><p class="text-xs text-gray-500 dark:text-dark-400">{{ metric.label }}</p><p class="mt-1 text-xl font-bold text-gray-950 dark:text-white">{{ metric.value }}</p></div>
              </div>
              <section class="card overflow-hidden">
                <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-800"><h3 class="font-semibold text-gray-950 dark:text-white">直属用户业绩</h3><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">新人体验按实付金额与已消耗额度展示，但不产生佣金。</p></div>
                <div class="max-h-80 overflow-auto"><table class="min-w-[900px] divide-y divide-gray-100 text-sm dark:divide-dark-800"><thead class="sticky top-0 bg-gray-50 text-xs text-gray-500 dark:bg-dark-900 dark:text-dark-400"><tr><th class="px-4 py-3 text-left">用户</th><th class="px-4 py-3 text-left">加入时间</th><th class="px-4 py-3 text-right">充值</th><th class="px-4 py-3 text-right">消费</th><th class="px-4 py-3 text-right">产生佣金</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-800"><tr v-for="user in performanceDetail.direct_users" :key="user.user_id"><td class="px-4 py-3"><p class="font-medium">#{{ user.user_id }} · {{ user.username || user.email }}</p><p class="text-xs text-gray-500">{{ user.email }}</p></td><td class="px-4 py-3">{{ formatBeijingDate(user.joined_at) }}</td><td class="px-4 py-3 text-right">{{ formatMicros(user.recharge_micros, '¥') }}</td><td class="px-4 py-3 text-right">{{ formatMicros(user.consumption_micros, '¥') }}</td><td class="px-4 py-3 text-right">{{ formatMicros(user.generated_commission_micros, '¥') }}</td></tr><tr v-if="!performanceDetail.direct_users.length"><td colspan="5" class="px-4 py-8 text-center text-gray-500">该区间暂无直属用户业绩</td></tr></tbody></table></div>
              </section>
              <div class="grid gap-6 xl:grid-cols-2">
                <section class="card overflow-hidden">
                  <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-800">
                    <h3 class="font-semibold">佣金流水（最近100笔）</h3>
                    <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">逐笔佣金按微单位精确记录；不足 0.01 元时展示到 6 位小数，不是 0 元。</p>
                  </div>
                  <div class="max-h-80 overflow-auto">
                    <div v-for="entry in performanceDetail.commission_ledger" :key="entry.id" class="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-4 border-b border-gray-100 px-5 py-3 text-sm dark:border-dark-800">
                      <div class="min-w-0">
                        <p>#{{ entry.id }} · {{ entry.consumer_user_id ? `用户 #${entry.consumer_user_id}` : '系统流水' }}</p>
                        <p class="text-xs text-gray-500 dark:text-dark-400">{{ formatCommissionEntryType(entry.entry_type) }} · {{ formatBeijingTime(entry.occurred_at) }}</p>
                        <p v-if="entry.source_amount_micros" class="mt-1 break-words text-xs text-gray-500 dark:text-dark-400">
                          对应消费 {{ formatPreciseMicros(entry.source_amount_micros, '¥') }}<template v-if="entry.customer_rebate_rate_bps"> · 用户返利 {{ formatRateBPS(entry.customer_rebate_rate_bps) }}</template><template v-if="entry.agent_commission_rate_bps"> · 合伙人 {{ formatRateBPS(entry.agent_commission_rate_bps) }}</template>
                        </p>
                      </div>
                      <strong class="whitespace-nowrap tabular-nums" :class="entry.amount_micros < 0 ? 'text-red-600' : 'text-green-600'">{{ formatPreciseMicros(entry.amount_micros, '¥') }}</strong>
                    </div>
                    <p v-if="!performanceDetail.commission_ledger.length" class="p-6 text-center text-sm text-gray-500">暂无佣金流水</p>
                  </div>
                </section>
                <section class="card overflow-hidden"><div class="border-b border-gray-100 px-5 py-4 dark:border-dark-800"><h3 class="font-semibold">提现记录（最近100笔）</h3></div><div class="max-h-80 overflow-auto"><div v-for="withdrawal in performanceDetail.withdrawals" :key="withdrawal.id" class="flex items-center justify-between gap-4 border-b border-gray-100 px-5 py-3 text-sm dark:border-dark-800"><div><p>#{{ withdrawal.id }} · {{ formatWithdrawalStatus(withdrawal.status) }}</p><p class="text-xs text-gray-500">{{ formatBeijingTime(withdrawal.requested_at) }} · {{ withdrawal.payment_reference || withdrawal.failure_reason || '—' }}</p></div><strong>{{ formatMicros(withdrawal.amount_micros, '¥') }}</strong></div><p v-if="!performanceDetail.withdrawals.length" class="p-6 text-center text-sm text-gray-500">暂无提现记录</p></div></section>
              </div>
            </template>
          </div>
          </section>
        </div>
      </Teleport>

      <div
        v-if="selfCommissionDialogItem"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
        role="dialog"
        aria-modal="true"
        aria-labelledby="self-commission-dialog-title"
        @click.self="closeSelfCommissionDialog"
      >
        <form class="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-900" @submit.prevent="submitSelfCommissionPolicy">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 id="self-commission-dialog-title" class="text-lg font-semibold text-gray-950 dark:text-white">
                {{ selfCommissionNextEnabled ? '开启本人消费返佣' : '关闭本人消费返佣' }}
              </h2>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">
                合伙人 #{{ selfCommissionDialogItem.agent_id }} · {{ selfCommissionDialogItem.username || selfCommissionDialogItem.email }}
              </p>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="selfCommissionUpdatingId !== null" @click="closeSelfCommissionDialog">关闭</button>
          </div>

          <div class="mt-5 rounded-xl bg-amber-50 p-4 text-sm leading-6 text-amber-900 dark:bg-amber-950 dark:text-amber-200">
            <template v-if="selfCommissionNextEnabled">
              开启后，仅新购买并实际使用的付费余额卡和月卡参与本人消费返佣，历史余额、赠送额度和此前购买的权益不会补算；返佣比例固定为 10%。
            </template>
            <template v-else>
              关闭只影响关闭后新购买的付费权益，已经按购买时规则获得资格的权益仍会继续结算。如需立即停止，请先将该合伙人状态改为“待审核”或“已暂停”。
            </template>
          </div>

          <label class="mt-5 block">
            <span class="mb-1 block text-sm font-medium text-gray-800 dark:text-dark-200">操作原因</span>
            <textarea
              v-model.trim="selfCommissionReason"
              required
              maxlength="500"
              class="input min-h-28"
              placeholder="请填写本次开启或关闭的原因，便于后续核对"
            />
            <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ selfCommissionReason.length }}/500</span>
          </label>

          <div class="mt-6 flex justify-end gap-3">
            <button type="button" class="btn btn-secondary" :disabled="selfCommissionUpdatingId !== null" @click="closeSelfCommissionDialog">取消</button>
            <button class="btn btn-primary" :disabled="selfCommissionUpdatingId !== null || !selfCommissionReason.trim()">
              {{ selfCommissionUpdatingId !== null ? '保存中…' : (selfCommissionNextEnabled ? '确认开启' : '确认关闭') }}
            </button>
          </div>
        </form>
      </div>

      <div v-if="qrPreviewURL" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true" @click.self="closeQRPreview">
        <div class="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl dark:bg-dark-900">
          <div class="flex items-center justify-between gap-3">
            <h2 class="font-semibold text-gray-950 dark:text-white">{{ qrPreviewTitle }}</h2>
            <button class="btn btn-secondary btn-sm" @click="closeQRPreview">关闭</button>
          </div>
          <img :src="qrPreviewURL" :alt="qrPreviewTitle" class="mx-auto mt-4 max-h-[65vh] rounded-xl border border-gray-200 p-2 dark:border-dark-700">
        </div>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import {
  completeAffiliateWithdrawal,
  failAffiliateWithdrawal,
  getAffiliateCommunity,
  getAffiliateCommunityQRCode,
  getAffiliateCommercialPolicy,
  getAffiliateOperationsSummary,
  getAffiliatePartnerPerformance,
  getAffiliateProgram,
  getAffiliateWithdrawalQRCode,
  getPaymentQRCode,
  listAffiliateRiskPrincipals,
  listAffiliateApplications,
  listAffiliateQualifiedCandidates,
  listAffiliatePartnerPerformance,
  listAffiliateWithdrawals,
  listPendingPaymentProfiles,
  reverseAffiliatePerformance,
  reviewPaymentProfile,
  reviewAffiliateApplication,
  updateAffiliateRisk,
  updateAffiliateSelfCommissionPolicy,
  updateAffiliateCommunity,
  updateAffiliateProgram,
  uploadAffiliateCommunityQRCode,
  type AdminAffiliateWithdrawal,
  type AffiliateAgentApplication,
  type AffiliateQualifiedCandidate,
  type AffiliateCommunitySettings,
  type AffiliateCommercialPolicy,
  type AffiliatePerformanceReversal,
  type AffiliateOperationsSummary,
  type AffiliatePartnerPerformance,
  type AffiliatePartnerPerformanceDetail,
  type AffiliateProgramSettings,
  type AffiliateRiskPrincipal,
  type AffiliateRiskStatus,
  type AgentPaymentProfile
} from '@/api/admin/agents'
import { useAppStore } from '@/stores/app'
import {
  getSelfCommissionOperatorLabel,
  getSelfCommissionPolicyErrorMessage,
  getSelfCommissionPresentation,
  type SelfCommissionTone
} from '@/features/affiliate/selfCommission'
import { buildAuthErrorMessage } from '@/utils/authError'

const NumberField = defineComponent({
  props: {
    modelValue: { type: Number, required: true },
    label: { type: String, required: true },
    prefix: { type: String, default: '' },
    suffix: { type: String, default: '' },
    min: { type: Number, default: undefined },
    max: { type: Number, default: undefined },
    step: { type: Number, default: 1 },
    disabled: { type: Boolean, default: false }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () => h('label', { class: 'block' }, [
      h('span', { class: 'mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300' }, props.label),
      h('div', { class: 'relative' }, [
        props.prefix ? h('span', { class: 'pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-gray-600 dark:text-dark-300' }, props.prefix) : null,
        h('input', {
          value: props.modelValue,
          type: 'number',
          min: props.min,
          max: props.max,
          step: props.step,
          disabled: props.disabled,
          class: ['input', props.prefix ? 'pl-8' : '', props.suffix ? 'pr-14' : '', props.disabled ? 'cursor-not-allowed opacity-60' : ''],
          onInput: (event: Event) => emit('update:modelValue', Number((event.target as HTMLInputElement).value))
        }),
        props.suffix ? h('span', { class: 'pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-gray-600 dark:text-dark-300' }, props.suffix) : null
      ])
    ])
  }
})

const appStore = useAppStore()
const loading = ref(true)
const error = ref('')
const programSaving = ref(false)
const communitySaving = ref(false)
const reviewingId = ref<number | null>(null)
const applicationReviewingId = ref<number | null>(null)
const processingWithdrawalId = ref<number | null>(null)
const riskUpdatingId = ref<number | null>(null)
const selfCommissionUpdatingId = ref<number | null>(null)
const selfCommissionDialogItem = ref<AffiliateRiskPrincipal | null>(null)
const selfCommissionNextEnabled = ref(false)
const selfCommissionReason = ref('')
const reversalProcessing = ref(false)
const program = ref<AffiliateProgramSettings | null>(null)
const commercialPolicy = ref<AffiliateCommercialPolicy | null>(null)
const community = ref<AffiliateCommunitySettings | null>(null)
const pendingProfiles = ref<AgentPaymentProfile[]>([])
const withdrawals = ref<AdminAffiliateWithdrawal[]>([])
const paidWithdrawals = ref<AdminAffiliateWithdrawal[]>([])
const riskPrincipals = ref<AffiliateRiskPrincipal[]>([])
const applications = ref<AffiliateAgentApplication[]>([])
const pendingApplications = computed(() => applications.value.filter(item => item.status === 'pending_review'))
const qualifiedCandidates = ref<AffiliateQualifiedCandidate[]>([])
const operationsSummary = ref<AffiliateOperationsSummary | null>(null)
const partnerPerformance = ref<AffiliatePartnerPerformance[]>([])
const performanceDetailAgent = ref<AffiliatePartnerPerformance | null>(null)
const performanceDetail = ref<AffiliatePartnerPerformanceDetail | null>(null)
const performanceDetailLoading = ref(false)
const performancePeriod = ref<'since_activation' | '30d' | 'month' | 'custom'>('since_activation')
const performanceCustomStart = ref('')
const performanceCustomEnd = ref('')
let summaryRefreshTimer: ReturnType<typeof setInterval> | undefined
const applicationNotes = reactive<Record<number, string>>({})
const reviewNotes = reactive<Record<number, string>>({})
const paymentReferences = reactive<Record<number, string>>({})
const riskReasons = reactive<Record<number, string>>({})
const riskTargets = reactive<Record<number, AffiliateRiskStatus>>({})
const reversalForm = reactive({ eventId: 0, reason: '' })
const lastReversal = ref<AffiliatePerformanceReversal | null>(null)
const communityQRPreview = ref('')
const qrPreviewURL = ref('')
const qrPreviewTitle = ref('')
const programForm = reactive({
  mode: 'off' as AffiliateProgramSettings['mode'],
  ordinaryReferralRate: 5,
  ordinaryInviteeRate: 5,
  directUserCount: 5,
  perUserConsumption: 20,
  directTeamConsumption: 1000,
  selfConsumption: 500,
  maxCampaignLinks: 5,
  conversionMultiplier: 1.2,
  withdrawalMinimum: 100,
  withdrawalSLAHours: 24,
  marginFloor: 35,
  stressCostPerRawCredit: 0.37,
  operationalReserve: 2
})
const communityForm = reactive({ enabled: false, title: '', message: '' })

type AffiliateOperationsTab = 'rules' | 'qualified' | 'applications' | 'partners' | 'profiles' | 'payouts' | 'archive' | 'risk' | 'community'
const activeTab = ref<AffiliateOperationsTab>('rules')

const affiliateTabs = computed<Array<{ id: AffiliateOperationsTab; label: string; count: number | null }>>(() => [
  { id: 'rules', label: '计划规则', count: null },
  { id: 'qualified', label: '已达标待申请', count: operationsSummary.value?.qualified_followup ?? 0 },
  { id: 'applications', label: '合伙人申请', count: operationsSummary.value?.pending_applications ?? 0 },
  { id: 'partners', label: '合伙人管理', count: null },
  { id: 'profiles', label: '资料审核', count: operationsSummary.value?.pending_payment_profiles ?? 0 },
  { id: 'payouts', label: '提现打款', count: operationsSummary.value?.processing_withdrawals ?? 0 },
  { id: 'archive', label: '已到账归档', count: null },
  { id: 'risk', label: '异常与冲销', count: operationsSummary.value?.abnormal_partners ?? 0 },
  { id: 'community', label: '社群引导', count: null }
])

const operationsQueueChips = computed(() => {
  const summary = operationsSummary.value
  if (!summary) return []
  return [
    { tab: 'qualified' as const, label: '已达标待申请', count: summary.qualified_followup, tone: 'info' },
    { tab: 'applications' as const, label: '合伙人申请', count: summary.pending_applications, tone: 'warning' },
    { tab: 'profiles' as const, label: '资料审核', count: summary.pending_payment_profiles, tone: 'warning' },
    { tab: 'payouts' as const, label: summary.overdue_withdrawals > 0 ? `提现打款（逾期 ${summary.overdue_withdrawals}）` : '提现打款', count: summary.processing_withdrawals, tone: summary.overdue_withdrawals > 0 ? 'danger' : 'warning' },
    { tab: 'risk' as const, label: '异常与冲销', count: summary.abnormal_partners, tone: 'danger' }
  ].filter(item => item.count > 0)
})

const performanceDetailMetrics = computed(() => {
  const item = performanceDetail.value?.summary
  if (!item) return []
  return [
    { label: '直属用户 / 付费用户', value: `${item.direct_user_count} / ${item.paid_direct_user_count}` },
    { label: '本人充值', value: formatMicros(item.self_recharge_micros, '¥') },
    { label: '本人确认消费', value: formatMicros(item.self_consumption_micros, '¥') },
    { label: '直属团队充值', value: formatMicros(item.direct_team_recharge_micros, '¥') },
    { label: '直属团队确认消费', value: formatMicros(item.direct_team_consumption_micros, '¥') },
    { label: '期间累计佣金', value: formatMicros(item.lifetime_earned_micros, '¥') },
    { label: '当前可提现', value: formatMicros(item.available_commission_micros, '¥') },
    { label: '提现中 / 已返佣', value: `${formatMicros(item.processing_withdrawal_micros, '¥')} / ${formatMicros(item.paid_commission_micros, '¥')}` }
  ]
})

const programModeClass = computed(() => (
  program.value?.mode === 'live'
    ? 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300'
    : program.value?.mode === 'shadow'
      ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300'
      : 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
))

const agentOverviewStats = computed(() => {
  const items = riskPrincipals.value
  return {
    total: items.length,
    clear: items.filter(item => item.risk_status === 'clear').length,
    abnormal: items.filter(item => item.risk_status !== 'clear').length
  }
})

watch(program, value => {
  if (!value) return
  programForm.mode = value.mode
  programForm.ordinaryReferralRate = value.ordinary_referral_rate_bps / 100
  programForm.ordinaryInviteeRate = value.ordinary_invitee_rate_bps / 100
  programForm.directUserCount = value.qualification_direct_user_count
  programForm.perUserConsumption = microsToUnits(value.qualification_min_user_consumption_micros)
  programForm.directTeamConsumption = microsToUnits(value.qualification_direct_team_consumption_micros)
  programForm.selfConsumption = microsToUnits(value.qualification_self_consumption_micros)
  programForm.maxCampaignLinks = value.max_campaign_links
  programForm.conversionMultiplier = value.commission_conversion_multiplier_millis / 1000
  programForm.withdrawalMinimum = microsToUnits(value.withdrawal_min_micros)
  programForm.withdrawalSLAHours = value.withdrawal_sla_hours
  programForm.marginFloor = value.margin_floor_bps / 100
  programForm.stressCostPerRawCredit = value.stress_cost_per_raw_credit_micros / 1_000_000
  programForm.operationalReserve = value.operational_reserve_bps / 100
}, { immediate: true })

watch(community, value => {
  if (!value) return
  communityForm.enabled = value.enabled
  communityForm.title = value.title
  communityForm.message = value.message
}, { immediate: true })

function microsToUnits(value: number) {
  return value / 1_000_000
}

function unitsToMicros(value: number) {
  return Math.round(value * 1_000_000)
}

function formatMicros(value: number, symbol: string) {
  return `${symbol}${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(microsToUnits(value))}`
}

function formatPreciseMicros(value: number, symbol: string) {
  return `${symbol}${new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 6
  }).format(microsToUnits(value))}`
}

function formatRateBPS(value: number) {
  return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(value / 100)}%`
}

function formatProgramMode(mode: AffiliateProgramSettings['mode']) {
  if (mode === 'live') return '正式'
  if (mode === 'shadow') return '观察'
  return '关闭'
}

function formatQualificationRoute(route: AffiliateAgentApplication['qualifying_route']) {
  if (route === 'direct_team') return '路线 A · 直属团队'
  if (route === 'self_consumption') return '路线 B · 本人消费'
  return '历史路线 B · 本人 + 直属'
}

function formatBeijingTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  }).format(new Date(value))
}

function formatOptionalBeijingTime(value?: string) {
  return value ? formatBeijingTime(value) : '—'
}

function formatBeijingDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit'
  }).format(new Date(value))
}

function formatCommissionEntryType(type: string) {
  const labels: Record<string, string> = {
    earned: '佣金入账', risk_release: '风险释放', reversal: '冲销',
    withdrawal_hold: '提现冻结', withdrawal_release: '提现退回', conversion: '转额度'
  }
  return labels[type] || type
}

function formatWithdrawalStatus(status: string) {
  if (status === 'processing') return '处理中'
  if (status === 'paid') return '已到账'
  if (status === 'failed') return '已退回'
  return status
}

function openFirstActionableQueue() {
  const summary = operationsSummary.value
  if (!summary) return
  if (summary.pending_applications > 0) activeTab.value = 'applications'
  else if (summary.pending_payment_profiles > 0) activeTab.value = 'profiles'
  else if (summary.processing_withdrawals > 0) activeTab.value = 'payouts'
  else if (summary.abnormal_partners > 0) activeTab.value = 'risk'
  else if (summary.qualified_followup > 0) activeTab.value = 'qualified'
}

function performanceDateParams() {
  const now = new Date()
  if (performancePeriod.value === '30d') {
    const start = new Date(now)
    start.setDate(start.getDate() - 30)
    return { start: start.toISOString().slice(0, 10), end: now.toISOString().slice(0, 10) }
  }
  if (performancePeriod.value === 'month') {
    const start = new Date(now.getFullYear(), now.getMonth(), 1)
    return { start: start.toISOString().slice(0, 10), end: now.toISOString().slice(0, 10) }
  }
  if (performancePeriod.value === 'custom' && performanceCustomStart.value && performanceCustomEnd.value) {
    return { start: performanceCustomStart.value, end: performanceCustomEnd.value }
  }
  return undefined
}

async function openPerformanceDetail(item: AffiliatePartnerPerformance) {
  performanceDetailAgent.value = item
  performancePeriod.value = 'since_activation'
  performanceCustomStart.value = ''
  performanceCustomEnd.value = ''
  await loadPerformanceDetail()
}

async function loadPerformanceDetail() {
  const item = performanceDetailAgent.value
  if (!item || (performancePeriod.value === 'custom' && (!performanceCustomStart.value || !performanceCustomEnd.value))) return
  performanceDetailLoading.value = true
  try {
    performanceDetail.value = await getAffiliatePartnerPerformance(item.agent_id, performanceDateParams())
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '合伙人业绩明细加载失败' }))
  } finally {
    performanceDetailLoading.value = false
  }
}

function closePerformanceDetail() {
  performanceDetailAgent.value = null
  performanceDetail.value = null
}

async function refreshOperationsSummary() {
  if (document.visibilityState !== 'visible') return
  try {
    operationsSummary.value = await getAffiliateOperationsSummary()
  } catch { /* 主数据刷新会展示错误，后台轮询保持安静 */ }
}

function riskStatusClass(status: AffiliateRiskStatus) {
  if (status === 'blocked') return 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300'
  if (status === 'review') return 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300'
  return 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300'
}

function agentStatusClass(status: string) {
  if (status === 'active') return 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300'
  if (status === 'suspended' || status === 'disabled') return 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
}

function formatAgentStatus(status: string) {
  if (status === 'active') return '正常'
  if (status === 'suspended') return '已暂停'
  if (status === 'disabled') return '已停用'
  if (status === 'candidate') return '待升级'
  if (status === 'pending_review') return '待审核'
  if (status === 'rejected') return '未通过'
  return status
}

function formatApplicationStatus(status: string) {
  if (status === 'approved') return '已开通'
  if (status === 'rejected') return '未通过'
  if (status === 'cancelled') return '已取消'
  return '待审核'
}

function applicationStatusClass(status: string) {
  if (status === 'approved') return 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300'
  if (status === 'rejected') return 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
}

function formatApplicationDecision(item: AffiliateAgentApplication) {
  if (item.status === 'approved' && item.decision_note === 'auto') return '达标自动开通'
  return item.decision_note || '—'
}

function formatRiskStatus(status: AffiliateRiskStatus) {
  if (status === 'clear') return '正常'
  if (status === 'review') return '待审核'
  if (status === 'blocked') return '已暂停'
  return status
}

function selfCommissionPresentation(item: AffiliateRiskPrincipal) {
  return getSelfCommissionPresentation(item)
}

function selfCommissionOperatorLabel(item: AffiliateRiskPrincipal) {
  return getSelfCommissionOperatorLabel(item)
}

function selfCommissionStatusClass(tone: SelfCommissionTone) {
  if (tone === 'enabled') return 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300'
  if (tone === 'blocked') return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300'
}

function setObjectURL(target: typeof communityQRPreview, blob: Blob) {
  if (target.value.startsWith('blob:')) URL.revokeObjectURL(target.value)
  target.value = URL.createObjectURL(blob)
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [settings, policy, communitySettings, summary, performance, qualifiedQueue, applicationQueue, profiles, payoutQueue, paidQueue, principals] = await Promise.all([
      getAffiliateProgram(),
      getAffiliateCommercialPolicy(),
      getAffiliateCommunity(),
      getAffiliateOperationsSummary(),
      listAffiliatePartnerPerformance(500),
      listAffiliateQualifiedCandidates(500),
      listAffiliateApplications('all', 500),
      listPendingPaymentProfiles(),
      listAffiliateWithdrawals('processing'),
      listAffiliateWithdrawals('paid'),
      listAffiliateRiskPrincipals(500)
    ])
    program.value = settings
    commercialPolicy.value = policy
    community.value = communitySettings
    operationsSummary.value = summary
    partnerPerformance.value = performance
    qualifiedCandidates.value = qualifiedQueue
    applications.value = applicationQueue
    pendingProfiles.value = profiles
    withdrawals.value = payoutQueue
    paidWithdrawals.value = paidQueue
    riskPrincipals.value = principals
    for (const item of principals) {
      riskTargets[item.agent_id] = item.risk_status
    }
    if (communitySettings.has_qr_code) {
      try { setObjectURL(communityQRPreview, await getAffiliateCommunityQRCode()) } catch { /* optional preview */ }
    }
  } catch (cause: unknown) {
    error.value = buildAuthErrorMessage(cause, { fallback: '联盟运营数据加载失败' })
  } finally {
    loading.value = false
  }
}

async function saveProgram() {
  if (!program.value) return
  programSaving.value = true
  try {
    program.value = await updateAffiliateProgram({
      ...program.value,
      mode: programForm.mode,
      ordinary_referral_rate_bps: Math.round(programForm.ordinaryReferralRate * 100),
      ordinary_invitee_rate_bps: Math.round(programForm.ordinaryInviteeRate * 100),
      first_paid_bonus_threshold_micros: 0,
      first_paid_bonus_micros: 0,
      qualification_direct_user_count: Math.round(programForm.directUserCount),
      qualification_min_user_consumption_micros: unitsToMicros(programForm.perUserConsumption),
      qualification_direct_team_consumption_micros: unitsToMicros(programForm.directTeamConsumption),
      qualification_self_consumption_micros: unitsToMicros(programForm.selfConsumption),
      max_campaign_links: Math.round(programForm.maxCampaignLinks),
      commission_conversion_multiplier_millis: Math.round(programForm.conversionMultiplier * 1000),
      withdrawal_min_micros: unitsToMicros(programForm.withdrawalMinimum),
      withdrawal_sla_hours: Math.round(programForm.withdrawalSLAHours),
      margin_floor_bps: Math.round(programForm.marginFloor * 100),
      operational_reserve_bps: Math.round(programForm.operationalReserve * 100),
      stress_cost_per_raw_credit_micros: unitsToMicros(programForm.stressCostPerRawCredit)
    })
    commercialPolicy.value = await getAffiliateCommercialPolicy()
    appStore.showSuccess('联盟计划设置已保存')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '计划设置保存失败' }))
  } finally {
    programSaving.value = false
  }
}

async function reviewApplication(item: AffiliateAgentApplication, approve: boolean) {
  applicationReviewingId.value = item.id
  try {
    const { application } = await reviewAffiliateApplication(item.id, {
      approve,
      note: (applicationNotes[item.id] || '').trim()
    })
    applications.value = applications.value.map(entry => entry.id === item.id ? application : entry)
    riskPrincipals.value = await listAffiliateRiskPrincipals(500)
    partnerPerformance.value = await listAffiliatePartnerPerformance(500)
    void refreshOperationsSummary()
    appStore.showSuccess(approve ? '申请已通过，合伙人已开通' : '申请已标记为未通过')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '申请处理失败' }))
  } finally {
    applicationReviewingId.value = null
  }
}

async function applyRiskStatus(item: AffiliateRiskPrincipal) {
  const reason = (riskReasons[item.agent_id] || '').trim()
  if (!reason) {
    appStore.showError('请先填写处理原因')
    return
  }
  riskUpdatingId.value = item.agent_id
  try {
    const action = await updateAffiliateRisk(item.agent_id, {
      status: riskTargets[item.agent_id] || item.risk_status,
      reason
    })
    item.risk_status = action.next_risk_status
    item.risk_note = action.reason
    item.self_commission_eligible = !item.has_upstream &&
      item.agent_status === 'active' &&
      action.next_risk_status === 'clear'
    if (action.next_risk_status === 'clear') {
      item.held_reward_count = 0
      item.held_reward_micros = 0
      item.held_cash_count = 0
      item.held_cash_micros = 0
    }
    riskReasons[item.agent_id] = ''
    withdrawals.value = await listAffiliateWithdrawals('processing')
    void refreshOperationsSummary()
    appStore.showSuccess(`合伙人 #${item.agent_id} 状态已更新为「${formatRiskStatus(action.next_risk_status)}」`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '合伙人状态更新失败' }))
  } finally {
    riskUpdatingId.value = null
  }
}

function openSelfCommissionDialog(item: AffiliateRiskPrincipal, enabled: boolean) {
  if (enabled && !getSelfCommissionPresentation(item).canEnable) {
    appStore.showError(getSelfCommissionPresentation(item).detail)
    return
  }
  selfCommissionDialogItem.value = item
  selfCommissionNextEnabled.value = enabled
  selfCommissionReason.value = ''
}

function closeSelfCommissionDialog() {
  if (selfCommissionUpdatingId.value !== null) return
  selfCommissionDialogItem.value = null
  selfCommissionReason.value = ''
}

async function submitSelfCommissionPolicy() {
  const item = selfCommissionDialogItem.value
  const reason = selfCommissionReason.value.trim()
  if (!item || !reason) {
    appStore.showError('请填写操作原因')
    return
  }
  selfCommissionUpdatingId.value = item.agent_id
  try {
    const policy = await updateAffiliateSelfCommissionPolicy(item.agent_id, {
      enabled: selfCommissionNextEnabled.value,
      expected_revision: item.self_commission_revision,
      reason
    })
    item.self_commission_enabled = policy.enabled
    item.self_commission_rate_bps = policy.rate_bps
    item.self_commission_effective_at = policy.effective_at
    item.self_commission_revision = policy.revision
    item.self_commission_reason = policy.reason
    item.self_commission_updated_by = policy.updated_by
    item.self_commission_updated_by_email = ''
    item.self_commission_updated_by_username = ''
    item.self_commission_updated_at = policy.updated_at
    item.has_upstream = policy.has_upstream
    item.self_commission_eligible = policy.eligible
    item.self_commission_block_reason = policy.block_reason_code
    try {
      riskPrincipals.value = await listAffiliateRiskPrincipals(500)
      for (const principal of riskPrincipals.value) {
        riskTargets[principal.agent_id] = principal.risk_status
      }
    } catch {
      // The policy is already saved; keep the local response if the audit refresh fails.
    }
    appStore.showSuccess(policy.enabled ? '本人消费返佣已开启' : '本人消费返佣已关闭')
    selfCommissionDialogItem.value = null
    selfCommissionReason.value = ''
  } catch (cause: unknown) {
    let refreshed = false
    try {
      riskPrincipals.value = await listAffiliateRiskPrincipals(500)
      for (const principal of riskPrincipals.value) {
        riskTargets[principal.agent_id] = principal.risk_status
      }
      refreshed = true
    } catch {
      // Keep the original error visible; the regular refresh path can recover later.
    }
    const message = getSelfCommissionPolicyErrorMessage(cause)
    appStore.showError(refreshed ? message : `${message} 当前数据刷新失败，请手动刷新页面。`)
    selfCommissionDialogItem.value = null
    selfCommissionReason.value = ''
  } finally {
    selfCommissionUpdatingId.value = null
  }
}

async function submitReversal() {
  const eventId = Math.trunc(reversalForm.eventId)
  const reason = reversalForm.reason.trim()
  if (eventId <= 0 || !reason) {
    appStore.showError('请输入有效的消费记录 ID 和处理说明')
    return
  }
  if (!window.confirm(`确认撤回消费记录 #${eventId}？该操作不可撤销。`)) return
  reversalProcessing.value = true
  try {
    lastReversal.value = await reverseAffiliatePerformance(eventId, reason)
    reversalForm.eventId = 0
    reversalForm.reason = ''
    riskPrincipals.value = await listAffiliateRiskPrincipals(500)
    partnerPerformance.value = await listAffiliatePartnerPerformance(500)
    void refreshOperationsSummary()
    for (const item of riskPrincipals.value) {
      riskTargets[item.agent_id] = item.risk_status
    }
    appStore.showSuccess(`消费记录 #${eventId} 已撤回`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '消费记录撤回失败' }))
  } finally {
    reversalProcessing.value = false
  }
}

async function verifyPaymentProfile(profile: AgentPaymentProfile) {
  reviewingId.value = profile.agent_id
  try {
    await reviewPaymentProfile(profile.agent_id, { status: 'verified', note: reviewNotes[profile.agent_id] || '' })
    pendingProfiles.value = pendingProfiles.value.filter(item => item.agent_id !== profile.agent_id)
    void refreshOperationsSummary()
    appStore.showSuccess(`合伙人 #${profile.agent_id} 收款资料已验证`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '验证失败' }))
  } finally {
    reviewingId.value = null
  }
}

async function rejectPaymentProfile(profile: AgentPaymentProfile) {
  const note = (reviewNotes[profile.agent_id] || '').trim()
  if (!note) {
    appStore.showError('拒绝时请填写修改原因')
    return
  }
  reviewingId.value = profile.agent_id
  try {
    await reviewPaymentProfile(profile.agent_id, { status: 'rejected', note })
    pendingProfiles.value = pendingProfiles.value.filter(item => item.agent_id !== profile.agent_id)
    void refreshOperationsSummary()
    appStore.showSuccess(`合伙人 #${profile.agent_id} 资料已退回`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '退回失败' }))
  } finally {
    reviewingId.value = null
  }
}

async function previewPaymentProfile(profile: AgentPaymentProfile) {
  try {
    setObjectURL(qrPreviewURL, await getPaymentQRCode(profile.agent_id))
    qrPreviewTitle.value = `合伙人 #${profile.agent_id} · ${profile.alipay_real_name}`
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '收款码加载失败' }))
  }
}

async function previewWithdrawalQR(withdrawal: AdminAffiliateWithdrawal) {
  try {
    setObjectURL(qrPreviewURL, await getAffiliateWithdrawalQRCode(withdrawal.id))
    qrPreviewTitle.value = `提现 #${withdrawal.id} · ${formatMicros(withdrawal.amount_micros, '¥')}`
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '收款码加载失败' }))
  }
}

function closeQRPreview() {
  if (qrPreviewURL.value.startsWith('blob:')) URL.revokeObjectURL(qrPreviewURL.value)
  qrPreviewURL.value = ''
  qrPreviewTitle.value = ''
}

async function completeWithdrawal(withdrawal: AdminAffiliateWithdrawal) {
  processingWithdrawalId.value = withdrawal.id
  try {
    await completeAffiliateWithdrawal(withdrawal.id, paymentReferences[withdrawal.id] || '')
    withdrawals.value = withdrawals.value.filter(item => item.id !== withdrawal.id)
    paidWithdrawals.value = await listAffiliateWithdrawals('paid')
    partnerPerformance.value = await listAffiliatePartnerPerformance(500)
    void refreshOperationsSummary()
    appStore.showSuccess(`提现 #${withdrawal.id} 已标记到账`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '到账确认失败' }))
  } finally {
    processingWithdrawalId.value = null
  }
}

async function failWithdrawal(withdrawal: AdminAffiliateWithdrawal) {
  const reason = (paymentReferences[withdrawal.id] || '').trim()
  if (!reason) {
    appStore.showError('请在流水号输入框填写失败原因')
    return
  }
  processingWithdrawalId.value = withdrawal.id
  try {
    await failAffiliateWithdrawal(withdrawal.id, reason)
    withdrawals.value = withdrawals.value.filter(item => item.id !== withdrawal.id)
    partnerPerformance.value = await listAffiliatePartnerPerformance(500)
    void refreshOperationsSummary()
    appStore.showSuccess(`提现 #${withdrawal.id} 已退回余额`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '提现退回失败' }))
  } finally {
    processingWithdrawalId.value = null
  }
}

async function saveCommunity() {
  if (!community.value) return
  communitySaving.value = true
  try {
    community.value = await updateAffiliateCommunity({
      enabled: communityForm.enabled,
      title: communityForm.title,
      message: communityForm.message,
      revision: community.value.revision
    })
    appStore.showSuccess('社群卡片已保存')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '社群设置保存失败' }))
  } finally {
    communitySaving.value = false
  }
}

async function uploadCommunityQR(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  communitySaving.value = true
  try {
    community.value = await uploadAffiliateCommunityQRCode(file)
    setObjectURL(communityQRPreview, file)
    appStore.showSuccess('社群二维码已上传')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '二维码上传失败' }))
  } finally {
    communitySaving.value = false
    input.value = ''
  }
}

onMounted(() => {
  void loadAll()
  summaryRefreshTimer = setInterval(() => { void refreshOperationsSummary() }, 60_000)
})
onBeforeUnmount(() => {
  if (summaryRefreshTimer) clearInterval(summaryRefreshTimer)
  closeQRPreview()
  if (communityQRPreview.value.startsWith('blob:')) URL.revokeObjectURL(communityQRPreview.value)
})
</script>
