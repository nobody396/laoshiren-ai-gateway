<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-6 pb-12">
      <header class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.2em] text-primary-700 dark:text-primary-300">合伙人计划 V2.1</p>
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
        <section v-if="program" class="card overflow-hidden">
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
              <NumberField v-model="programForm.ordinaryReferralRate" label="普通邀请奖励" suffix="%" :min="0" :max="10" :step="1" />
              <NumberField v-model="programForm.firstPaidThreshold" label="首笔奖励门槛" prefix="¥" :min="0" :step="1" />
              <NumberField v-model="programForm.firstPaidBonus" label="被邀请人固定奖励" prefix="⚡" :min="0" :step="1" />
              <NumberField v-model="programForm.directUserCount" label="路线 A 有效用户" suffix="人" :min="1" :step="1" />
              <NumberField v-model="programForm.perUserConsumption" label="单个有效用户消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.directTeamConsumption" label="路线 A 团队消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.combinedConsumption" label="路线 B 合并消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.maxCampaignLinks" label="最多活动链接" suffix="条" :min="0" :max="100" :step="1" />
              <NumberField v-model="programForm.conversionMultiplier" label="现金转额度倍率" suffix="×" :min="1" :step="0.1" />
              <NumberField v-model="programForm.withdrawalMinimum" label="最低提现金额" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.withdrawalSLAHours" label="处理时限" suffix="小时" :min="1" :max="168" :step="1" />
              <NumberField v-model="programForm.marginFloor" label="压力毛利率底线" suffix="%" :min="35" :max="100" :step="1" />
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900 sm:col-span-2 lg:col-span-3">
                <p class="text-xs text-gray-600 dark:text-dark-300">合伙人奖励池</p>
                <p class="mt-1 text-xl font-bold text-gray-950 dark:text-white">{{ program.agent_pool_rate_bps / 100 }}%</p>
                <p class="mt-1 text-xs text-gray-600 dark:text-dark-300">该值由后端锁死，不可扩大；客户返利 + 合伙人现金佣金固定共 10%。</p>
              </div>
            </div>
            <div class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-5 dark:border-dark-800">
              <p class="text-xs leading-5 text-gray-600 dark:text-dark-300">
                {{ program.started_at ? `首次正式启用：${formatBeijingTime(program.started_at)}` : '尚未进入正式模式；首次启用时间将由服务器保存，并按北京时间展示。' }}
              </p>
              <button class="btn btn-primary" :disabled="programSaving">{{ programSaving ? '保存中…' : '保存计划设置' }}</button>
            </div>
          </form>
        </section>

        <section v-if="commercialPolicy" class="card overflow-hidden">
          <div class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <div>
              <h2 class="text-xl font-semibold text-gray-950 dark:text-white">35% 压力毛利门禁</h2>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">
                已扣除链动小铺 3% 手续费、完整 10% 联盟奖励池，并按每单位原始额度 ¥0.50 的保守成本测算。
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

        <section class="space-y-6">
          <article class="card overflow-hidden">
            <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
              <div>
                <h2 class="text-xl font-semibold text-gray-950 dark:text-white">合伙人管理</h2>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">查看当前全部合伙人；发现异常时可先暂停邀请、提现和佣金转额度，确认没问题后再恢复。</p>
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
              <table class="min-w-[1280px] table-fixed divide-y divide-gray-100 text-sm dark:divide-dark-800">
                <colgroup>
                  <col class="w-[250px]">
                  <col class="w-[130px]">
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
                        <option value="review">先暂停，待确认</option>
                        <option value="blocked">暂停合作</option>
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
                    <td colspan="8" class="px-5 py-12 text-center text-sm text-gray-600 dark:text-dark-300">尚无合伙人</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </article>

          <article class="card p-6">
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

        <section class="space-y-6">
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
                      <input v-model="paymentReferences[withdrawal.id]" maxlength="200" class="input min-w-52" placeholder="支付宝流水号（可选）">
                    </td>
                    <td class="sticky right-0 border-l border-gray-100 bg-white px-5 py-4 dark:border-dark-800 dark:bg-dark-900">
                      <div class="flex justify-end gap-2">
                        <button class="btn btn-secondary btn-sm whitespace-nowrap" @click="previewWithdrawalQR(withdrawal)">扫码打款</button>
                        <button class="btn btn-secondary btn-sm whitespace-nowrap" :disabled="processingWithdrawalId === withdrawal.id" @click="failWithdrawal(withdrawal)">打款失败</button>
                        <button class="btn btn-primary btn-sm whitespace-nowrap" :disabled="processingWithdrawalId === withdrawal.id || withdrawal.agent_risk_status !== 'clear'" @click="completeWithdrawal(withdrawal)">标记已到账</button>
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

        <section v-if="community" class="card overflow-hidden">
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
  getAffiliateProgram,
  getAffiliateWithdrawalQRCode,
  getPaymentQRCode,
  listAffiliateRiskPrincipals,
  listAffiliateWithdrawals,
  listPendingPaymentProfiles,
  reverseAffiliatePerformance,
  reviewPaymentProfile,
  updateAffiliateRisk,
  updateAffiliateCommunity,
  updateAffiliateProgram,
  uploadAffiliateCommunityQRCode,
  type AdminAffiliateWithdrawal,
  type AffiliateCommunitySettings,
  type AffiliateCommercialPolicy,
  type AffiliatePerformanceReversal,
  type AffiliateProgramSettings,
  type AffiliateRiskPrincipal,
  type AffiliateRiskStatus,
  type AgentPaymentProfile
} from '@/api/admin/agents'
import { useAppStore } from '@/stores/app'
import { buildAuthErrorMessage } from '@/utils/authError'

const NumberField = defineComponent({
  props: {
    modelValue: { type: Number, required: true },
    label: { type: String, required: true },
    prefix: { type: String, default: '' },
    suffix: { type: String, default: '' },
    min: { type: Number, default: undefined },
    max: { type: Number, default: undefined },
    step: { type: Number, default: 1 }
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
          class: ['input', props.prefix ? 'pl-8' : '', props.suffix ? 'pr-14' : ''],
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
const processingWithdrawalId = ref<number | null>(null)
const riskUpdatingId = ref<number | null>(null)
const reversalProcessing = ref(false)
const program = ref<AffiliateProgramSettings | null>(null)
const commercialPolicy = ref<AffiliateCommercialPolicy | null>(null)
const community = ref<AffiliateCommunitySettings | null>(null)
const pendingProfiles = ref<AgentPaymentProfile[]>([])
const withdrawals = ref<AdminAffiliateWithdrawal[]>([])
const riskPrincipals = ref<AffiliateRiskPrincipal[]>([])
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
  firstPaidThreshold: 50,
  firstPaidBonus: 5,
  directUserCount: 10,
  perUserConsumption: 20,
  directTeamConsumption: 1000,
  combinedConsumption: 2000,
  maxCampaignLinks: 5,
  conversionMultiplier: 1.2,
  withdrawalMinimum: 100,
  withdrawalSLAHours: 24,
  marginFloor: 35
})
const communityForm = reactive({ enabled: false, title: '', message: '' })

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
  programForm.firstPaidThreshold = microsToUnits(value.first_paid_bonus_threshold_micros)
  programForm.firstPaidBonus = microsToUnits(value.first_paid_bonus_micros)
  programForm.directUserCount = value.qualification_direct_user_count
  programForm.perUserConsumption = microsToUnits(value.qualification_min_user_consumption_micros)
  programForm.directTeamConsumption = microsToUnits(value.qualification_direct_team_consumption_micros)
  programForm.combinedConsumption = microsToUnits(value.qualification_combined_consumption_micros)
  programForm.maxCampaignLinks = value.max_campaign_links
  programForm.conversionMultiplier = value.commission_conversion_multiplier_millis / 1000
  programForm.withdrawalMinimum = microsToUnits(value.withdrawal_min_micros)
  programForm.withdrawalSLAHours = value.withdrawal_sla_hours
  programForm.marginFloor = value.margin_floor_bps / 100
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

function formatProgramMode(mode: AffiliateProgramSettings['mode']) {
  if (mode === 'live') return '正式'
  if (mode === 'shadow') return '观察'
  return '关闭'
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

function formatRiskStatus(status: AffiliateRiskStatus) {
  if (status === 'clear') return '正常'
  if (status === 'review') return '先暂停，待确认'
  if (status === 'blocked') return '暂停合作'
  return status
}

function setObjectURL(target: typeof communityQRPreview, blob: Blob) {
  if (target.value.startsWith('blob:')) URL.revokeObjectURL(target.value)
  target.value = URL.createObjectURL(blob)
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [settings, policy, communitySettings, profiles, payoutQueue, principals] = await Promise.all([
      getAffiliateProgram(),
      getAffiliateCommercialPolicy(),
      getAffiliateCommunity(),
      listPendingPaymentProfiles(),
      listAffiliateWithdrawals(),
      listAffiliateRiskPrincipals(500)
    ])
    program.value = settings
    commercialPolicy.value = policy
    community.value = communitySettings
    pendingProfiles.value = profiles
    withdrawals.value = payoutQueue
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
      first_paid_bonus_threshold_micros: unitsToMicros(programForm.firstPaidThreshold),
      first_paid_bonus_micros: unitsToMicros(programForm.firstPaidBonus),
      qualification_direct_user_count: Math.round(programForm.directUserCount),
      qualification_min_user_consumption_micros: unitsToMicros(programForm.perUserConsumption),
      qualification_direct_team_consumption_micros: unitsToMicros(programForm.directTeamConsumption),
      qualification_combined_consumption_micros: unitsToMicros(programForm.combinedConsumption),
      max_campaign_links: Math.round(programForm.maxCampaignLinks),
      commission_conversion_multiplier_millis: Math.round(programForm.conversionMultiplier * 1000),
      withdrawal_min_micros: unitsToMicros(programForm.withdrawalMinimum),
      withdrawal_sla_hours: Math.round(programForm.withdrawalSLAHours),
      margin_floor_bps: Math.round(programForm.marginFloor * 100)
    })
    commercialPolicy.value = await getAffiliateCommercialPolicy()
    appStore.showSuccess('联盟计划设置已保存')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '计划设置保存失败' }))
  } finally {
    programSaving.value = false
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
    if (action.next_risk_status === 'clear') {
      item.held_reward_count = 0
      item.held_reward_micros = 0
      item.held_cash_count = 0
      item.held_cash_micros = 0
    }
    riskReasons[item.agent_id] = ''
    withdrawals.value = await listAffiliateWithdrawals()
    appStore.showSuccess(`合伙人 #${item.agent_id} 状态已更新为「${formatRiskStatus(action.next_risk_status)}」`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '合伙人状态更新失败' }))
  } finally {
    riskUpdatingId.value = null
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

onMounted(loadAll)
onBeforeUnmount(() => {
  closeQRPreview()
  if (communityQRPreview.value.startsWith('blob:')) URL.revokeObjectURL(communityQRPreview.value)
})
</script>
