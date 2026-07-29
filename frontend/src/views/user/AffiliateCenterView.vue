<template>
  <AppLayout>
    <main class="mx-auto max-w-6xl space-y-6 pb-12">
      <header v-if="programIsLive" class="relative overflow-hidden rounded-3xl border border-primary-200 bg-primary-50 p-6 dark:border-primary-900 dark:bg-primary-950 sm:p-8">
        <div class="relative grid gap-6 lg:grid-cols-[1.4fr_1fr] lg:items-end">
          <div>
            <p class="mb-3 text-xs font-semibold uppercase tracking-[0.22em] text-primary-700 dark:text-primary-300">
              合伙人计划
            </p>
            <h1 class="max-w-2xl text-3xl font-bold tracking-tight text-gray-950 dark:text-white sm:text-4xl">
              一条真实邀请，<br class="hidden sm:block">一份长期回报。
            </h1>
            <p class="mt-4 max-w-2xl text-sm leading-7 text-gray-600 dark:text-dark-300">
              普通邀请的首笔真实付费，邀请人与被邀请人各得 5% ⚡；满足消费门槛后可申请成为合伙人，审核通过后使用动态链接分配固定 10% 奖励池。
            </p>
            <router-link to="/legal/affiliate-program" class="mt-4 inline-flex text-sm font-semibold text-primary-700 hover:underline dark:text-primary-300">
              查看完整联盟计划规则
            </router-link>
          </div>
          <div class="rounded-2xl border border-primary-200 bg-white/80 p-4 shadow-sm backdrop-blur dark:border-primary-900 dark:bg-dark-900/80">
            <p class="text-xs text-gray-600 dark:text-dark-300">{{ primaryInviteLabel }}</p>
            <div class="mt-2 flex gap-2">
              <input :value="primaryInviteURL" readonly class="input min-w-0 flex-1 text-sm" :aria-label="primaryInviteLabel">
              <button class="btn btn-primary shrink-0" :disabled="!primaryInviteURL" @click="copyPrimaryInvite">
                {{ copied ? '已复制' : '复制' }}
              </button>
            </div>
            <p class="mt-2 text-xs leading-5 text-gray-600 dark:text-dark-300">
              {{ primaryInviteHint }}
            </p>
          </div>
        </div>
      </header>

      <section
        v-else-if="!loading && qualification"
        class="relative overflow-hidden rounded-3xl border border-primary-200 bg-primary-50 px-6 py-14 text-center dark:border-primary-900 dark:bg-primary-950 sm:px-10"
      >
        <p class="text-xs font-semibold uppercase tracking-[0.22em] text-primary-700 dark:text-primary-300">联盟计划</p>
        <h1 class="mt-4 text-3xl font-bold tracking-tight text-gray-950 dark:text-white sm:text-4xl">
          {{ unavailableTitle }}
        </h1>
        <p class="mx-auto mt-4 max-w-xl text-sm leading-7 text-gray-600 dark:text-dark-300">
          {{ unavailableDescription }}
        </p>
        <div class="mt-7 flex flex-wrap justify-center gap-3">
          <router-link to="/dashboard" class="btn btn-primary">返回控制台</router-link>
          <router-link to="/legal/affiliate-program" class="btn btn-secondary">查看联盟计划规则</router-link>
        </div>
      </section>

      <div v-if="error" class="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
        {{ error }}
      </div>

      <div
        v-if="programIsLive && partnerAccessCopy.banner"
        class="rounded-2xl border px-4 py-3 text-sm"
        :class="partnerAccessState === 'under_review'
          ? 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200'
          : 'border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300'"
      >
        {{ partnerAccessCopy.banner }}
      </div>

      <section v-if="loading" class="grid gap-4 md:grid-cols-3" aria-label="加载中">
        <div v-for="item in 3" :key="item" class="card h-40 animate-pulse bg-gray-100 dark:bg-dark-800" />
      </section>

      <template v-else-if="programIsLive">
        <section v-if="qualification" class="card overflow-hidden">
          <div class="border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p class="text-xs font-semibold uppercase tracking-wider text-primary-700 dark:text-primary-300">合伙人资格</p>
                <h2 class="mt-1 text-xl font-semibold text-gray-950 dark:text-white">合伙人资格进度</h2>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">仅统计计划启用后的真实消费，永久直属关系不会因升级改变。</p>
              </div>
              <span class="rounded-full border px-3 py-1 text-xs font-medium" :class="qualificationBadgeClass">
                {{ qualificationStatusLabel }}
              </span>
            </div>
          </div>

          <div class="grid gap-4 p-6 lg:grid-cols-2">
            <article class="rounded-2xl border p-5" :class="qualification.direct_route_qualified ? 'border-green-300 bg-green-50 dark:border-green-900 dark:bg-green-950' : 'border-gray-200 dark:border-dark-700'">
              <div class="flex items-center justify-between gap-3">
                <h3 class="font-semibold text-gray-900 dark:text-white">路线 A · 直属团队</h3>
                <span class="text-xs font-medium text-gray-600 dark:text-dark-300">{{ qualification.direct_route_qualified ? '已达成' : '进行中' }}</span>
              </div>
              <dl class="mt-5 space-y-4">
                <ProgressRow
                  label="有效直属用户"
                  :value="qualification.valid_direct_user_count"
                  :target="qualification.required_direct_user_count"
                  suffix=" 人"
                />
                <ProgressRow
                  label="直属团队确认消费"
                  :value="microsToYuan(qualification.direct_team_consumption_micros)"
                  :target="microsToYuan(qualification.required_direct_team_micros)"
                  prefix="¥"
                />
              </dl>
              <p class="mt-4 text-xs leading-5 text-gray-600 dark:text-dark-300">
                每位有效用户需确认消费至少 ¥{{ formatAmount(microsToYuan(qualification.required_per_user_micros)) }}。
              </p>
            </article>

            <article class="rounded-2xl border p-5" :class="qualification.combined_route_qualified ? 'border-green-300 bg-green-50 dark:border-green-900 dark:bg-green-950' : 'border-gray-200 dark:border-dark-700'">
              <div class="flex items-center justify-between gap-3">
                <h3 class="font-semibold text-gray-900 dark:text-white">路线 B · 高质量直属消费</h3>
                <span class="text-xs font-medium text-gray-600 dark:text-dark-300">{{ qualification.combined_route_qualified ? '已达成' : '进行中' }}</span>
              </div>
              <dl class="mt-5">
                <ProgressRow
                  label="直属团队确认消费"
                  :value="microsToYuan(qualification.combined_consumption_micros)"
                  :target="microsToYuan(qualification.required_combined_micros)"
                  prefix="¥"
                />
              </dl>
              <p class="mt-4 text-xs leading-5 text-gray-600 dark:text-dark-300">
                本人消费不计入申请门槛；达到路线 B 后仍需提交申请并由平台审核。
              </p>
            </article>
          </div>

          <div v-if="qualification.can_apply" class="border-t border-gray-100 bg-gray-50 px-6 py-4 dark:border-dark-800 dark:bg-dark-900">
            <div class="flex flex-wrap items-end justify-between gap-4">
              <label class="min-w-0 flex-1">
                <span class="mb-1 block text-sm font-medium text-gray-800 dark:text-dark-100">资格已达成，可以申请成为合伙人</span>
                <textarea v-model.trim="applicationNote" maxlength="500" class="input min-h-20" placeholder="可选：简单介绍你的客户和推广方式" />
              </label>
              <button class="btn btn-primary" :disabled="applying" @click="applyForPartner">
                {{ applying ? '正在提交…' : '提交合伙人申请' }}
              </button>
            </div>
          </div>
          <div v-else-if="qualification.agent_status === 'pending_review'" class="border-t border-amber-200 bg-amber-50 px-6 py-4 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200">
            申请已进入审核中。审核通过后，这里会自动切换为合伙人中心。
          </div>
        </section>

        <template v-if="isPartnerAvailable">
          <section v-if="unreadNotices.length" class="space-y-3">
            <article v-for="notice in unreadNotices" :key="notice.id" class="flex gap-4 rounded-2xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-900 dark:bg-primary-950">
              <div class="min-w-0 flex-1">
                <h3 class="font-semibold text-gray-900 dark:text-white">{{ notice.title }}</h3>
                <p class="mt-1 whitespace-pre-line text-sm leading-6 text-gray-600 dark:text-dark-300">{{ notice.message }}</p>
              </div>
              <button class="btn btn-secondary btn-sm shrink-0 self-start" @click="markNoticeRead(notice.id)">知道了</button>
            </article>
          </section>

          <section class="grid gap-6 lg:grid-cols-[1.25fr_0.75fr]">
            <div class="card overflow-hidden">
              <div class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
                <div>
                  <p class="text-xs font-semibold uppercase tracking-wider text-primary-700 dark:text-primary-300">固定 10% 奖励池</p>
                  <h2 class="mt-1 text-xl font-semibold text-gray-950 dark:text-white">动态返利链接</h2>
                </div>
                <button class="btn btn-secondary btn-sm" :disabled="links.length >= 6" @click="showCreateLink = !showCreateLink">
                  新建活动链接
                </button>
              </div>

              <form v-if="showCreateLink" class="grid gap-3 border-b border-gray-100 bg-gray-50 p-5 dark:border-dark-800 dark:bg-dark-900 sm:grid-cols-[1fr_1fr_auto]" @submit.prevent="createLink">
                <input v-model.trim="newLink.name" required maxlength="80" class="input" placeholder="活动名称">
                <input v-model.trim="newLink.channel" maxlength="80" class="input" placeholder="渠道（可选）">
                <button class="btn btn-primary" :disabled="linkSaving">创建</button>
                <div class="sm:col-span-3">
                  <label class="mb-2 flex justify-between text-xs text-gray-600 dark:text-dark-300">
                    <span>客户返利</span><span>{{ newLink.rate }}% ⚡ / {{ 10 - newLink.rate }}% 现金佣金</span>
                  </label>
                  <input v-model.number="newLink.rate" type="range" min="0" max="10" step="1" class="w-full accent-primary-600">
                </div>
              </form>

              <div class="divide-y divide-gray-100 dark:divide-dark-800">
                <article v-for="link in links" :key="link.id" class="p-5">
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div class="flex flex-wrap items-center gap-2">
                        <h3 class="font-semibold text-gray-900 dark:text-white">{{ link.name }}</h3>
                        <span v-if="link.is_default" class="rounded-full bg-primary-100 px-2 py-0.5 text-[11px] font-medium text-primary-700 dark:bg-primary-950 dark:text-primary-300">默认</span>
                        <span class="rounded-full px-2 py-0.5 text-[11px] font-medium" :class="link.status === 'active' ? 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-400'">
                          {{ link.status === 'active' ? '使用中' : '已停用' }}
                        </span>
                      </div>
                      <p class="mt-1 text-xs text-gray-600 dark:text-dark-300">{{ link.channel || '通用渠道' }} · {{ affiliateURL(link.code) }}</p>
                    </div>
                    <div class="flex gap-2">
                      <button class="btn btn-secondary btn-sm" @click="copyLink(link)">复制</button>
                      <button v-if="!link.is_default" class="btn btn-secondary btn-sm" @click="toggleLink(link)">{{ link.status === 'active' ? '停用' : '启用' }}</button>
                    </div>
                  </div>

                  <div class="mt-5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                    <div class="flex h-9 text-xs font-semibold">
                      <div class="flex items-center justify-center bg-primary-500 text-white transition-[width]" :style="{ width: `${link.customer_rebate_rate_bps / 1000 * 100}%` }">
                        <span v-if="link.customer_rebate_rate_bps > 0">客户 {{ link.customer_rebate_rate_bps / 100 }}%</span>
                      </div>
                      <div class="flex flex-1 items-center justify-center bg-gray-800 text-white dark:bg-gray-200 dark:text-gray-900">
                        合伙人 {{ link.agent_commission_rate_bps / 100 }}%
                      </div>
                    </div>
                  </div>

                  <div class="mt-4 flex items-center gap-3">
                    <input
                      :value="link.customer_rebate_rate_bps / 100"
                      type="range"
                      min="0"
                      max="10"
                      step="1"
                      class="min-w-0 flex-1 accent-primary-600"
                      :aria-label="`${link.name} 客户返利率`"
                      @change="changeLinkRate(link, $event)"
                    >
                    <span class="w-14 text-right text-sm font-semibold text-gray-900 dark:text-white">{{ link.customer_rebate_rate_bps / 100 }}%</span>
                  </div>
                </article>
                <div v-if="!links.length" class="p-10 text-center text-sm text-gray-600 dark:text-dark-300">暂无动态链接</div>
              </div>
              <p class="border-t border-gray-100 px-5 py-4 text-xs leading-5 text-gray-500 dark:border-dark-800 dark:text-dark-400">
                奖励池始终为 10%。返给客户的比例可按 1% 步进动态调整；新比例只用于之后绑定的客户，已经绑定的客户保持原比例。
              </p>
            </div>

            <aside class="space-y-6">
              <section class="card p-5">
                <p class="text-xs font-semibold tracking-wider text-primary-700 dark:text-primary-300">佣金与提现</p>
                <h2 class="mt-1 text-xl font-semibold text-gray-950 dark:text-white">佣金钱包</h2>
                <div class="mt-5 grid grid-cols-2 gap-3">
                  <MetricTile label="可提现" :value="formatMicros(wallet?.available_cash_micros, '¥')" />
                  <MetricTile label="处理中" :value="formatMicros(wallet?.processing_withdrawal_micros, '¥')" />
                  <MetricTile label="累计佣金" :value="formatMicros(wallet?.lifetime_earned_micros, '¥')" />
                  <MetricTile label="转额度倍率" :value="`${(wallet?.conversion_multiplier_millis ?? 1200) / 1000}×`" />
                </div>

                <div class="mt-5 space-y-4 border-t border-gray-100 pt-5 dark:border-dark-800">
                  <label class="block">
                    <span class="mb-1 block text-xs text-gray-600 dark:text-dark-300">提现金额（¥）</span>
                    <div class="flex gap-2">
                      <input v-model.number="withdrawAmount" min="0" step="0.01" type="number" class="input min-w-0 flex-1">
                      <button class="btn btn-primary shrink-0" :disabled="walletBusy || !wallet?.can_withdraw" @click="requestWithdrawal">申请提现</button>
                    </div>
                  </label>
                  <p class="text-xs leading-5 text-gray-600 dark:text-dark-300">
                    最低 {{ formatMicros(wallet?.withdrawal_minimum_micros, '¥') }}；提交即显示“处理中”，预计 {{ wallet?.withdrawal_sla_hours ?? 24 }} 小时内到账（北京时间）。
                  </p>

                  <label class="block">
                    <span class="mb-1 block text-xs text-gray-600 dark:text-dark-300">转为 ⚡平台额度</span>
                    <div class="flex gap-2">
                      <input v-model.number="convertAmount" min="0" step="0.01" type="number" class="input min-w-0 flex-1">
                      <button class="btn btn-secondary shrink-0" :disabled="walletBusy" @click="convertCommission">立即转换</button>
                    </div>
                  </label>
                </div>

                <div v-if="withdrawals.length" class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-800">
                  <p class="mb-2 text-xs font-medium text-gray-600 dark:text-dark-300">最近提现</p>
                  <div v-for="item in withdrawals.slice(0, 4)" :key="item.id" class="flex items-center justify-between py-2 text-sm">
                    <span class="text-gray-700 dark:text-dark-200">{{ formatMicros(item.amount_micros, '¥') }}</span>
                    <span :class="item.status === 'paid' ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'">
                      {{ item.status === 'paid' ? '已到账' : '处理中' }}
                    </span>
                  </div>
                </div>
              </section>

              <section class="card p-5">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <p class="text-xs font-semibold tracking-wider text-primary-700 dark:text-primary-300">收款信息</p>
                    <h2 class="mt-1 text-lg font-semibold text-gray-950 dark:text-white">支付宝收款资料</h2>
                  </div>
                  <span class="rounded-full px-2 py-1 text-xs font-medium" :class="paymentVerificationClass">{{ paymentVerificationLabel }}</span>
                </div>
                <form class="mt-4 space-y-3" @submit.prevent="savePaymentProfile">
                  <input v-model.trim="paymentForm.alipay_real_name" required class="input" placeholder="支付宝实名">
                  <input v-model.trim="paymentForm.alipay_account" required class="input" placeholder="支付宝账号">
                  <input v-model.trim="paymentForm.contact_phone" required class="input" placeholder="联系电话">
                  <textarea v-model.trim="paymentForm.payment_note" class="input min-h-20" placeholder="打款备注（可选）" />
                  <div class="rounded-xl border border-primary-200 bg-primary-50 p-3 text-xs leading-5 text-gray-700 dark:border-primary-900 dark:bg-primary-950 dark:text-dark-200">
                    我们仅为审核合伙人身份、支付宝打款、风控与争议处理使用上述资料。支付宝账号和收款码属于敏感个人信息。
                    <router-link to="/legal/affiliate-payment-privacy" class="font-semibold text-primary-700 hover:underline dark:text-primary-300">
                      查看《合伙人收款资料隐私告知》
                    </router-link>
                  </div>
                  <label class="flex items-start gap-2 text-xs leading-5 text-gray-700 dark:text-dark-200">
                    <input v-model="paymentPrivacyConsent" type="checkbox" class="mt-1 rounded border-gray-300 text-primary-600 focus:ring-primary-500">
                    <span>我已阅读并单独同意平台按上述告知处理我的支付宝收款资料，用于审核和佣金打款。</span>
                  </label>
                  <label class="block rounded-xl border border-dashed border-gray-300 p-3 text-center text-sm text-gray-600 hover:border-primary-400 dark:border-dark-600 dark:text-dark-300">
                    <input type="file" accept="image/png,image/jpeg,image/webp" class="sr-only" :disabled="paymentSaving || !paymentPrivacyConsent" @change="uploadPaymentQR">
                    {{ paymentQRPreview ? '更换支付宝收款码' : '上传支付宝收款码' }}
                  </label>
                  <img v-if="paymentQRPreview" :src="paymentQRPreview" alt="支付宝收款码预览" class="mx-auto max-h-48 rounded-xl border border-gray-200 p-2 dark:border-dark-700">
                  <button class="btn btn-primary w-full" :disabled="paymentSaving || !paymentPrivacyConsent">保存并提交审核</button>
                </form>
                <p v-if="paymentProfile?.verification_note" class="mt-3 text-xs text-red-600 dark:text-red-400">{{ paymentProfile.verification_note }}</p>
              </section>

              <section v-if="community?.enabled" class="card p-5">
                <p class="text-xs font-semibold tracking-wider text-primary-700 dark:text-primary-300">合伙人社群</p>
                <h2 class="mt-1 text-lg font-semibold text-gray-950 dark:text-white">{{ community.title }}</h2>
                <p class="mt-2 whitespace-pre-line text-sm leading-6 text-gray-600 dark:text-dark-300">{{ community.message }}</p>
                <img v-if="communityQRPreview" :src="communityQRPreview" alt="合伙人社群二维码" class="mx-auto mt-4 max-h-56 rounded-xl border border-gray-200 p-2 dark:border-dark-700">
              </section>
            </aside>
          </section>
        </template>
      </template>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import {
  AGENT_PAYMENT_PRIVACY_NOTICE_VERSION,
  applyAffiliateAgent,
  convertAffiliateCommission,
  createAffiliateLink,
  getAffiliateCommunity,
  getAffiliateCommunityQRCode,
  getAffiliateQualification,
  getAffiliateWallet,
  getAgentPaymentProfile,
  getAgentPaymentQRCode,
  getMyInviteCode,
  listAffiliateLinks,
  listAffiliateNotices,
  listAffiliateWithdrawals,
  readAffiliateNotice,
  requestAffiliateWithdrawal,
  updateAffiliateLinkRate,
  updateAffiliateLinkStatus,
  updateAgentPaymentProfile,
  uploadAgentPaymentQRCode,
  type AffiliateAgentNotice,
  type AffiliateAgentQualification,
  type AffiliateCommunity,
  type AffiliateLink,
  type AffiliateWallet,
  type AffiliateWithdrawal,
  type AgentPaymentProfile,
  type AgentPaymentProfileUpdate
} from '@/api/agent'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useAffiliateProgramStore } from '@/stores/affiliateProgram'
import { useClipboard } from '@/composables/useClipboard'
import { buildAuthErrorMessage } from '@/utils/authError'
import { getPartnerAccessCopy, resolvePartnerAccessState } from '@/features/affiliate/partnerAccess'

const ProgressRow = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: Number, required: true },
    target: { type: Number, required: true },
    prefix: { type: String, default: '' },
    suffix: { type: String, default: '' }
  },
  setup(props) {
    return () => h('div', [
      h('div', { class: 'mb-2 flex items-center justify-between gap-3 text-sm' }, [
        h('dt', { class: 'text-gray-600 dark:text-dark-300' }, props.label),
        h('dd', { class: 'font-semibold text-gray-900 dark:text-white' }, `${props.prefix}${formatAmount(props.value)} / ${props.prefix}${formatAmount(props.target)}${props.suffix}`)
      ]),
      h('div', { class: 'h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800' }, [
        h('div', {
          class: 'h-full rounded-full bg-primary-500',
          style: { width: `${Math.min(100, props.target > 0 ? props.value / props.target * 100 : 100)}%` }
        })
      ])
    ])
  }
})

const MetricTile = defineComponent({
  props: { label: { type: String, required: true }, value: { type: String, required: true } },
  setup(props) {
    return () => h('div', { class: 'rounded-xl bg-gray-50 p-3 dark:bg-dark-900' }, [
      h('p', { class: 'text-xs text-gray-600 dark:text-dark-300' }, props.label),
      h('p', { class: 'mt-1 text-lg font-bold text-gray-950 dark:text-white' }, props.value)
    ])
  }
})

const appStore = useAppStore()
const authStore = useAuthStore()
const affiliateProgramStore = useAffiliateProgramStore()
const { copied, copyToClipboard } = useClipboard()
const loading = ref(true)
const error = ref('')
const applying = ref(false)
const linkSaving = ref(false)
const walletBusy = ref(false)
const paymentSaving = ref(false)
const inviteCode = ref('')
const qualification = ref<AffiliateAgentQualification | null>(null)
const links = ref<AffiliateLink[]>([])
const wallet = ref<AffiliateWallet | null>(null)
const withdrawals = ref<AffiliateWithdrawal[]>([])
const notices = ref<AffiliateAgentNotice[]>([])
const community = ref<AffiliateCommunity | null>(null)
const paymentProfile = ref<AgentPaymentProfile | null>(null)
const paymentPrivacyConsent = ref(false)
const paymentQRPreview = ref('')
const communityQRPreview = ref('')
const showCreateLink = ref(false)
const applicationSubmitted = ref(false)
const applicationNote = ref('')
const newLink = reactive({ name: '', channel: '', rate: 5 })
const withdrawAmount = ref<number | null>(null)
const convertAmount = ref<number | null>(null)
const paymentForm = reactive({
  alipay_real_name: '',
  alipay_account: '',
  contact_phone: '',
  payment_note: ''
})
let refreshTimer: ReturnType<typeof setInterval> | null = null
let refreshInFlight = false

const ordinaryInviteURL = computed(() => inviteCode.value ? `${window.location.origin}/register?ref=${inviteCode.value}` : '')
const programIsLive = computed(() => qualification.value?.program_mode === 'live')
const unavailableTitle = computed(() =>
  qualification.value?.program_mode === 'shadow' ? '联盟计划即将开放' : '联盟计划暂未开放'
)
const unavailableDescription = computed(() =>
  qualification.value?.program_mode === 'shadow'
    ? '我们正在完成正式开放前的最后检查。开放后，控制台会显示邀请入口和完整规则。'
    : '当前暂不接受邀请与合伙人申请。正式开放时间以后续通知为准。'
)
const partnerAccessState = computed(() => resolvePartnerAccessState(
  qualification.value?.agent_status,
  qualification.value?.risk_status
))
const partnerAccessCopy = computed(() => getPartnerAccessCopy(partnerAccessState.value))
const isApprovedPartner = computed(() => qualification.value?.agent_status === 'active')
const isPartnerAvailable = computed(() => partnerAccessState.value === 'available')
const defaultAgentLink = computed(() => links.value.find(item => item.is_default && item.status === 'active'))
const primaryInviteURL = computed(() =>
  !programIsLive.value
    ? ''
    : isPartnerAvailable.value && defaultAgentLink.value
    ? affiliateURL(defaultAgentLink.value.code)
    : isApprovedPartner.value
      ? ''
      : ordinaryInviteURL.value
)
const primaryInviteLabel = computed(() => partnerAccessCopy.value.inviteLabel)
const primaryInviteHint = computed(() => partnerAccessCopy.value.inviteHint)
const unreadNotices = computed(() => notices.value.filter(item => !item.read_at))
const qualificationStatusLabel = computed(() => {
  if (isApprovedPartner.value) return partnerAccessCopy.value.badge
  if (qualification.value?.agent_status === 'pending_review' || applicationSubmitted.value) return '审核中'
  if (qualification.value?.can_apply) return '可以申请'
  if (qualification.value?.program_mode === 'off') return '计划尚未开放'
  return '资格积累中'
})
const qualificationBadgeClass = computed(() => {
  if (partnerAccessState.value === 'under_review') {
    return 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300'
  }
  if (partnerAccessState.value === 'suspended') {
    return 'border-red-300 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300'
  }
  return isPartnerAvailable.value || qualification.value?.can_apply
    ? 'border-green-300 bg-green-50 text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-300'
    : 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300'
})
const paymentVerificationLabel = computed(() => {
  const status = paymentProfile.value?.verification_status
  if (paymentProfile.value?.verified) return '已验证'
  if (status === 'verified' && !paymentProfile.value?.privacy_consent_current) return '需重新确认'
  return status === 'pending_review' ? '审核中' : status === 'rejected' ? '需修改' : '未提交'
})
const paymentVerificationClass = computed(() => (
  paymentProfile.value?.verified
    ? 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300'
    : paymentProfile.value?.verification_status === 'rejected'
      ? 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300'
      : 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300'
))

function affiliateURL(code: string) {
  return `${window.location.origin}/register?ref=${code}`
}

function microsToYuan(value?: number) {
  return (value ?? 0) / 1_000_000
}

function formatAmount(value?: number) {
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(value ?? 0)
}

function formatMicros(value: number | undefined, symbol: string) {
  return `${symbol}${formatAmount(microsToYuan(value))}`
}

function setBlobPreview(target: typeof paymentQRPreview, blob: Blob) {
  if (target.value.startsWith('blob:')) URL.revokeObjectURL(target.value)
  target.value = URL.createObjectURL(blob)
}

function applyPaymentProfile(profile: AgentPaymentProfile) {
  paymentProfile.value = profile
  paymentForm.alipay_real_name = profile.alipay_real_name || ''
  paymentForm.alipay_account = profile.alipay_account || ''
  paymentForm.contact_phone = profile.contact_phone || ''
  paymentForm.payment_note = profile.payment_note || ''
  paymentPrivacyConsent.value = profile.privacy_consent_current
}

function paymentProfilePayload(): AgentPaymentProfileUpdate {
  return {
    ...paymentForm,
    privacy_consent_accepted: true as const,
    privacy_consent_version: AGENT_PAYMENT_PRIVACY_NOTICE_VERSION
  }
}

async function loadAgentData() {
  const [agentLinks, agentWallet, agentWithdrawals, agentNotices, agentCommunity, profile] = await Promise.all([
    listAffiliateLinks(),
    getAffiliateWallet(),
    listAffiliateWithdrawals(),
    listAffiliateNotices(),
    getAffiliateCommunity(),
    getAgentPaymentProfile()
  ])
  links.value = agentLinks
  wallet.value = agentWallet
  withdrawals.value = agentWithdrawals
  notices.value = agentNotices
  community.value = agentCommunity
  applyPaymentProfile(profile)
  void loadOptionalPreviews(profile, agentCommunity)
}

function clearPartnerData() {
  links.value = []
  wallet.value = null
  withdrawals.value = []
  notices.value = []
  community.value = null
  paymentProfile.value = null
  if (paymentQRPreview.value.startsWith('blob:')) URL.revokeObjectURL(paymentQRPreview.value)
  if (communityQRPreview.value.startsWith('blob:')) URL.revokeObjectURL(communityQRPreview.value)
  paymentQRPreview.value = ''
  communityQRPreview.value = ''
}

async function loadOptionalPreviews(profile: AgentPaymentProfile, agentCommunity: AffiliateCommunity) {
  if (profile.has_alipay_qr) {
    try { setBlobPreview(paymentQRPreview, await getAgentPaymentQRCode()) } catch { /* optional preview */ }
  }
  if (agentCommunity.enabled && agentCommunity.has_qr_code) {
    try { setBlobPreview(communityQRPreview, await getAffiliateCommunityQRCode()) } catch { /* optional preview */ }
  }
}

async function loadPage() {
  loading.value = true
  error.value = ''
  try {
    const currentQualification = await affiliateProgramStore.refresh(true)
    qualification.value = currentQualification
    if (currentQualification.program_mode !== 'live') {
      inviteCode.value = ''
      clearPartnerData()
      return
    }
    inviteCode.value = (await getMyInviteCode()).invite_code
    if (resolvePartnerAccessState(currentQualification.agent_status, currentQualification.risk_status) === 'available') {
      await loadAgentData()
    }
  } catch (cause: unknown) {
    error.value = buildAuthErrorMessage(cause, { fallback: '联盟计划加载失败，请稍后重试。' })
  } finally {
    loading.value = false
  }
}

async function refreshLiveData() {
  if (document.hidden || refreshInFlight) return
  refreshInFlight = true
  try {
    const latest = await getAffiliateQualification()
    affiliateProgramStore.setQualification(latest)
    const previousAccessState = partnerAccessState.value
    qualification.value = latest
    if (latest.program_mode !== 'live') {
      inviteCode.value = ''
      clearPartnerData()
      return
    }
    if (!inviteCode.value) inviteCode.value = (await getMyInviteCode()).invite_code
    const latestAccessState = resolvePartnerAccessState(latest.agent_status, latest.risk_status)
    if (latestAccessState === 'available') {
      if (previousAccessState !== 'available') await authStore.refreshUser()
      await loadAgentData()
    } else if (latest.agent_status === 'active') {
      clearPartnerData()
    }
  } catch {
    // Background refresh must not replace the last usable page with an error.
  } finally {
    refreshInFlight = false
  }
}

async function copyPrimaryInvite() {
  if (!primaryInviteURL.value) return
  await copyToClipboard(primaryInviteURL.value, isPartnerAvailable.value ? '默认合伙人链接已复制' : '普通邀请链接已复制')
}

async function applyForPartner() {
  applying.value = true
  try {
    await applyAffiliateAgent(applicationNote.value)
    applicationSubmitted.value = true
    qualification.value = await getAffiliateQualification()
    appStore.showSuccess('申请已提交，审核通过后会自动开通合伙人中心')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '合伙人申请提交失败' }))
  } finally {
    applying.value = false
  }
}

async function copyLink(link: AffiliateLink) {
  await copyToClipboard(affiliateURL(link.code), `${link.name}链接已复制`)
}

async function createLink() {
  linkSaving.value = true
  try {
    links.value.push(await createAffiliateLink({
      name: newLink.name,
      channel: newLink.channel,
      customer_rebate_rate_bps: newLink.rate * 100
    }))
    newLink.name = ''
    newLink.channel = ''
    newLink.rate = 5
    showCreateLink.value = false
    appStore.showSuccess('活动链接已创建')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '创建链接失败' }))
  } finally {
    linkSaving.value = false
  }
}

async function changeLinkRate(link: AffiliateLink, event: Event) {
  const rate = Number((event.target as HTMLInputElement).value)
  try {
    const updated = await updateAffiliateLinkRate(link.id, rate * 100)
    links.value = links.value.map(item => item.id === link.id ? updated : item)
    appStore.showSuccess('返利比例已更新，新绑定按新比例执行')
  } catch (cause: unknown) {
    const rateInput = event.target as HTMLInputElement
    rateInput.value = String(link.customer_rebate_rate_bps / 100)
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '比例更新失败' }))
  }
}

async function toggleLink(link: AffiliateLink) {
  try {
    const updated = await updateAffiliateLinkStatus(link.id, link.status === 'active' ? 'paused' : 'active')
    links.value = links.value.map(item => item.id === link.id ? updated : item)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '链接状态更新失败' }))
  }
}

async function refreshWallet() {
  const [summary, items] = await Promise.all([getAffiliateWallet(), listAffiliateWithdrawals()])
  wallet.value = summary
  withdrawals.value = items
}

async function requestWithdrawal() {
  if (!withdrawAmount.value || withdrawAmount.value <= 0) return
  walletBusy.value = true
  try {
    await requestAffiliateWithdrawal(Math.round(withdrawAmount.value * 1_000_000))
    withdrawAmount.value = null
    await refreshWallet()
    appStore.showSuccess('提现已进入处理中，预计 24 小时内到账')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '提现申请失败' }))
  } finally {
    walletBusy.value = false
  }
}

async function convertCommission() {
  if (!convertAmount.value || convertAmount.value <= 0) return
  walletBusy.value = true
  try {
    const result = await convertAffiliateCommission(Math.round(convertAmount.value * 1_000_000))
    convertAmount.value = null
    await Promise.all([refreshWallet(), authStore.refreshUser()])
    appStore.showSuccess(`已转换为 ${formatMicros(result.credit_amount_micros, '⚡')} 平台额度`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '佣金转换失败' }))
  } finally {
    walletBusy.value = false
  }
}

async function savePaymentProfile() {
  if (!paymentPrivacyConsent.value) {
    appStore.showError('请先阅读并单独同意《合伙人收款资料隐私告知》')
    return
  }
  if (!paymentProfile.value?.has_alipay_qr) {
    appStore.showError('请先上传支付宝收款码，再提交审核')
    return
  }
  paymentSaving.value = true
  try {
    applyPaymentProfile(await updateAgentPaymentProfile(paymentProfilePayload()))
    wallet.value = await getAffiliateWallet()
    appStore.showSuccess('收款资料已保存，正在等待审核')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '收款资料保存失败' }))
  } finally {
    paymentSaving.value = false
  }
}

async function uploadPaymentQR(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!paymentPrivacyConsent.value) {
    appStore.showError('请先阅读并单独同意《合伙人收款资料隐私告知》')
    input.value = ''
    return
  }
  paymentSaving.value = true
  try {
    applyPaymentProfile(await updateAgentPaymentProfile(paymentProfilePayload()))
    applyPaymentProfile(await uploadAgentPaymentQRCode(file))
    setBlobPreview(paymentQRPreview, file)
    appStore.showSuccess('收款码已上传，资料进入审核')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '收款码上传失败' }))
  } finally {
    paymentSaving.value = false
    input.value = ''
  }
}

async function markNoticeRead(id: number) {
  try {
    await readAffiliateNotice(id)
    notices.value = notices.value.map(item => item.id === id ? { ...item, read_at: new Date().toISOString() } : item)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '通知状态更新失败' }))
  }
}

onMounted(async () => {
  await loadPage()
  refreshTimer = setInterval(() => { void refreshLiveData() }, 5000)
})
onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (paymentQRPreview.value.startsWith('blob:')) URL.revokeObjectURL(paymentQRPreview.value)
  if (communityQRPreview.value.startsWith('blob:')) URL.revokeObjectURL(communityQRPreview.value)
})
</script>
