<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('team.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('team.description') }}</p>
        </div>
        <button class="btn btn-secondary" :disabled="refreshing" @click="refreshAll">
          <Icon name="refresh" size="sm" :class="refreshing ? 'animate-spin' : ''" />
          <span class="ml-2">{{ t('common.refresh') }}</span>
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-20"><LoadingSpinner /></div>

      <section v-else-if="!teamContext" class="card mx-auto max-w-xl p-6">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('team.createTitle') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('team.createDescription') }}</p>
        <form v-if="selfServiceEnabled" class="mt-5 flex gap-3" @submit.prevent="createTeam">
          <input v-model="createName" class="input flex-1" required maxlength="100" :placeholder="t('team.name')" />
          <button class="btn btn-primary" :disabled="submitting">{{ t('team.create') }}</button>
        </form>
        <p v-else class="mt-5 rounded-lg bg-gray-50 p-3 text-sm text-gray-600 dark:bg-dark-800 dark:text-gray-300">
          {{ t('team.noTeam') }}
        </p>
      </section>

      <template v-else>
        <section class="card p-5">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div>
              <div class="flex items-center gap-3">
                <h2 class="text-xl font-semibold text-gray-900 dark:text-white">{{ teamContext.team.name }}</h2>
                <span :class="['badge', teamContext.team.status === 'active' ? 'badge-success' : 'badge-warning']">
                  {{ t(teamContext.team.status === 'active' ? 'team.statusActive' : 'team.statusSuspended') }}
                </span>
              </div>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ isOwner ? t('team.owner') : t('team.member') }} · {{ t('team.memberCount', { count: teamContext.team.member_count }) }}
              </p>
            </div>
            <button class="btn btn-primary" @click="openTeamKeys">
              <Icon name="key" size="sm" class="mr-2" />{{ t('team.createKey') }}
            </button>
          </div>
        </section>

        <section class="grid gap-4 md:grid-cols-3">
          <div v-for="limit in myLimits" :key="limit.label" class="card p-4">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ limit.label }}</p>
            <p class="mt-2 font-semibold text-gray-900 dark:text-white">
              {{ formatUSD(limit.used) }} / {{ limit.limit > 0 ? formatUSD(limit.limit) : t('team.unlimited') }}
            </p>
            <div class="mt-3 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
              <div class="h-full rounded-full bg-primary-500" :style="{ width: `${limit.percent}%` }" />
            </div>
          </div>
        </section>

        <template v-if="isOwner && teamContext.team.status === 'active'">
          <section class="card p-5">
            <div class="flex items-center justify-between gap-3">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('team.members') }}</h3>
              <span class="text-sm text-gray-500">{{ teamContext.team.member_count }} / {{ teamContext.team.member_limit }}</span>
            </div>
            <div class="mt-4 divide-y divide-gray-100 dark:divide-dark-700">
              <div v-for="member in members" :key="member.id" class="flex flex-wrap items-center justify-between gap-3 py-4">
                <div class="min-w-0">
                  <p class="truncate font-medium text-gray-900 dark:text-white">{{ member.username || member.email }}</p>
                  <p class="truncate text-xs text-gray-500">{{ member.email }} · {{ t(`team.${member.role}`) }}</p>
                  <p class="mt-1 text-xs text-gray-500">
                    {{ t('team.daily') }} {{ formatUSD(member.daily_usage_usd) }} / {{ member.daily_limit_usd > 0 ? formatUSD(member.daily_limit_usd) : t('team.unlimited') }}
                  </p>
                </div>
                <div v-if="member.role === 'member'" class="flex flex-wrap gap-2">
                  <button class="btn btn-secondary" @click="editMember(member)">{{ t('team.editLimits') }}</button>
                  <button class="btn btn-secondary" @click="transferOwnership(member)">{{ t('team.transfer') }}</button>
                  <button class="btn btn-danger" @click="removeMember(member)">{{ t('team.remove') }}</button>
                </div>
              </div>
              <p v-if="members.length === 0" class="py-8 text-center text-sm text-gray-500">{{ t('team.noMembers') }}</p>
            </div>
          </section>

          <section class="card p-5">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('team.invitations') }}</h3>
            <form class="mt-4 flex flex-col gap-3 sm:flex-row" @submit.prevent="sendInvitation">
              <input v-model="inviteEmail" type="email" required class="input flex-1" :placeholder="t('team.inviteEmail')" />
              <button class="btn btn-primary" :disabled="submitting">{{ t('team.sendInvite') }}</button>
            </form>
            <div v-if="latestShareURL" class="mt-4 rounded-lg border border-primary-200 bg-primary-50/60 p-3 dark:border-primary-800 dark:bg-primary-900/20">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ latestShareLabel }}</p>
              <div class="mt-2 flex gap-2">
                <input :value="latestShareURL" readonly class="input flex-1 font-mono text-xs" />
                <button class="btn btn-secondary" @click="copyShareURL">{{ t('common.copy') }}</button>
              </div>
              <p class="mt-2 text-xs text-gray-500">{{ t('team.manualShareNotice') }}</p>
            </div>
            <div class="mt-4 space-y-3">
              <div v-for="invite in invitations" :key="invite.id" class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <div><p class="font-medium text-gray-900 dark:text-white">{{ invite.email }}</p><p class="text-xs text-gray-500">{{ invite.status }} · {{ formatDateTime(invite.expires_at) }}</p></div>
                <div v-if="invite.status === 'pending'" class="flex gap-2">
                  <button class="btn btn-secondary" @click="reissueInvitation(invite.id)">{{ t('team.reissue') }}</button>
                  <button class="btn btn-danger" @click="revokeInvitation(invite.id)">{{ t('team.revoke') }}</button>
                </div>
              </div>
              <p v-if="invitations.length === 0" class="py-4 text-center text-sm text-gray-500">{{ t('team.noInvitations') }}</p>
            </div>
          </section>

          <section class="card p-5">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('team.keys') }}</h3>
            <div class="mt-4 space-y-3">
              <div v-for="key in teamKeys" :key="key.id" class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <div><p class="font-medium text-gray-900 dark:text-white">{{ key.name }}</p><p class="font-mono text-xs text-gray-500">{{ key.masked_key }} · {{ key.user_email }}</p></div>
                <div class="flex gap-2">
                  <button class="btn btn-secondary" @click="toggleKey(key)">{{ key.status === 'active' ? t('team.disable') : t('team.enable') }}</button>
                  <button class="btn btn-danger" @click="deleteTeamKey(key)">{{ t('common.delete') }}</button>
                </div>
              </div>
              <p v-if="teamKeys.length === 0" class="py-4 text-center text-sm text-gray-500">{{ t('team.noKeys') }}</p>
            </div>
          </section>
        </template>

        <section v-if="isOwner" class="card p-5">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('team.settings') }}</h3>
          <div class="mt-4 grid gap-5 lg:grid-cols-2">
            <form class="space-y-3" @submit.prevent="renameTeam">
              <label class="input-label">{{ t('team.name') }}</label>
              <input v-model="renameName" class="input" required maxlength="100" />
              <button class="btn btn-primary" :disabled="submitting">{{ t('team.rename') }}</button>
            </form>
            <form class="space-y-3" @submit.prevent="saveDefaultLimits">
              <p class="input-label">{{ t('team.defaultMemberLimits') }}</p>
              <div class="grid grid-cols-3 gap-2">
                <input v-model.number="defaultLimits.daily" class="input" type="number" min="0" step="0.01" :placeholder="t('team.daily')" />
                <input v-model.number="defaultLimits.weekly" class="input" type="number" min="0" step="0.01" :placeholder="t('team.weekly')" />
                <input v-model.number="defaultLimits.monthly" class="input" type="number" min="0" step="0.01" :placeholder="t('team.monthly')" />
              </div>
              <button class="btn btn-primary" :disabled="submitting">{{ t('team.saveDefaultLimits') }}</button>
            </form>
          </div>
          <div class="mt-6 flex flex-wrap gap-3 border-t border-gray-100 pt-5 dark:border-dark-700">
            <button class="btn btn-secondary" @click="toggleStatus">{{ teamContext.team.status === 'active' ? t('team.pause') : t('team.resume') }}</button>
            <button class="btn btn-danger" @click="dissolveTeam">{{ t('team.dissolve') }}</button>
          </div>
        </section>

        <section v-else class="card p-5">
          <button class="btn btn-danger" @click="leaveTeam">{{ t('team.leave') }}</button>
        </section>
      </template>
    </div>

    <TeamInvitationDialog
      :show="Boolean(invitationToken)"
      :loading="invitationLoading"
      :preview="invitationPreview"
      :error="invitationError"
      :resolving="resolving"
      @close="clearTokenQuery('invitation')"
      @resolve="resolveInvitation"
    />

    <BaseDialog :show="Boolean(memberTarget)" :title="t('team.editLimits')" @close="memberTarget = null">
      <div class="grid grid-cols-3 gap-3">
        <input v-model.number="memberLimits.daily" class="input" type="number" min="0" step="0.01" :placeholder="t('team.daily')" />
        <input v-model.number="memberLimits.weekly" class="input" type="number" min="0" step="0.01" :placeholder="t('team.weekly')" />
        <input v-model.number="memberLimits.monthly" class="input" type="number" min="0" step="0.01" :placeholder="t('team.monthly')" />
      </div>
      <template #footer><button class="btn btn-primary" @click="saveMemberLimits">{{ t('team.saveLimits') }}</button></template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import TeamInvitationDialog from '@/components/team/TeamInvitationDialog.vue'
import { authAPI } from '@/api'
import { teamAPI, type TeamAPIKey, type TeamContext, type TeamInvitation, type TeamInvitationPreview, type TeamMembership } from '@/api/team'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { absoluteTeamShareURL } from '@/utils/teamShareURL'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(true)
const refreshing = ref(false)
const submitting = ref(false)
const resolving = ref(false)
const selfServiceEnabled = ref(true)
const teamContext = ref<TeamContext | null>(null)
const members = ref<TeamMembership[]>([])
const invitations = ref<TeamInvitation[]>([])
const teamKeys = ref<TeamAPIKey[]>([])
const createName = ref('')
const renameName = ref('')
const inviteEmail = ref('')
const defaultLimits = reactive({ daily: 0, weekly: 0, monthly: 0 })
const memberTarget = ref<TeamMembership | null>(null)
const memberLimits = reactive({ daily: 0, weekly: 0, monthly: 0 })
const invitationPreview = ref<TeamInvitationPreview | null>(null)
const invitationLoading = ref(false)
const invitationError = ref('')
const latestShareURL = ref('')
const latestShareLabel = ref('')

const invitationToken = computed(() => typeof route.query.invitation === 'string' ? route.query.invitation : '')
const transferToken = computed(() => typeof route.query.transfer === 'string' ? route.query.transfer : '')
const isOwner = computed(() => teamContext.value?.membership.role === 'owner')
const myLimits = computed(() => {
  const m = teamContext.value?.membership
  if (!m) return []
  return [
    { label: t('team.daily'), used: m.daily_usage_usd, limit: m.daily_limit_usd },
    { label: t('team.weekly'), used: m.weekly_usage_usd, limit: m.weekly_limit_usd },
    { label: t('team.monthly'), used: m.monthly_usage_usd, limit: m.monthly_limit_usd }
  ].map((item) => ({ ...item, percent: item.limit > 0 ? Math.min(100, item.used / item.limit * 100) : 0 }))
})

const formatUSD = (value: number) => `$${Number(value || 0).toFixed(4)}`
const isNoTeam = (error: any) => error?.reason === 'TEAM_NOT_FOUND' || error?.reason === 'TEAM_MEMBERSHIP_REQUIRED' || error?.response?.status === 404

const loadContext = async () => {
  try {
    teamContext.value = await teamAPI.current()
    renameName.value = teamContext.value.team.name
    defaultLimits.daily = teamContext.value.team.default_daily_limit_usd
    defaultLimits.weekly = teamContext.value.team.default_weekly_limit_usd
    defaultLimits.monthly = teamContext.value.team.default_monthly_limit_usd
  } catch (error) {
    if (isNoTeam(error)) teamContext.value = null
    else throw error
  }
}

const loadTeamData = async () => {
  members.value = []; invitations.value = []; teamKeys.value = []
  if (!teamContext.value || !isOwner.value || teamContext.value.team.status !== 'active') return
  ;[members.value, invitations.value, teamKeys.value] = await Promise.all([teamAPI.members(), teamAPI.invitations(), teamAPI.keys()])
}

const refreshAll = async () => {
  refreshing.value = true
  try { await loadContext(); await loadTeamData() } catch (error: any) { appStore.showError(error?.message || t('team.loadFailed')) } finally { refreshing.value = false }
}

const createTeam = async () => { submitting.value = true; try { teamContext.value = await teamAPI.create(createName.value); await loadTeamData(); appStore.showSuccess(t('team.created')) } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { submitting.value = false } }
const renameTeam = async () => { submitting.value = true; try { teamContext.value = await teamAPI.rename(renameName.value); appStore.showSuccess(t('team.updated')) } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { submitting.value = false } }
const saveDefaultLimits = async () => { submitting.value = true; try { teamContext.value = await teamAPI.updateDefaultMemberLimits({ default_daily_limit_usd: defaultLimits.daily, default_weekly_limit_usd: defaultLimits.weekly, default_monthly_limit_usd: defaultLimits.monthly }); appStore.showSuccess(t('team.defaultLimitsUpdated')) } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { submitting.value = false } }
const publishShareURL = async (label: string, url?: string) => { const absoluteURL = absoluteTeamShareURL(url, window.location.origin); latestShareLabel.value = label; latestShareURL.value = absoluteURL; if (absoluteURL) { try { await navigator.clipboard.writeText(absoluteURL) } catch { /* 页面仍显示可复制链接。 */ } } }
const copyShareURL = async () => { if (!latestShareURL.value) return; try { await navigator.clipboard.writeText(latestShareURL.value); appStore.showSuccess(t('common.copied')) } catch { appStore.showError(t('common.error')) } }
const sendInvitation = async () => { submitting.value = true; try { const invitation = await teamAPI.invite(inviteEmail.value); await publishShareURL(t('team.inviteActionTitle'), invitation.invitation_url); inviteEmail.value = ''; invitations.value = await teamAPI.invitations(); appStore.showSuccess(t('team.operationSuccess')) } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { submitting.value = false } }
const reissueInvitation = async (id: number) => { try { const invitation = await teamAPI.reissueInvitation(id); await publishShareURL(t('team.inviteActionTitle'), invitation.invitation_url); invitations.value = await teamAPI.invitations() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const revokeInvitation = async (id: number) => { try { await teamAPI.revokeInvitation(id); invitations.value = await teamAPI.invitations() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const editMember = (member: TeamMembership) => { memberTarget.value = member; memberLimits.daily = member.daily_limit_usd; memberLimits.weekly = member.weekly_limit_usd; memberLimits.monthly = member.monthly_limit_usd }
const saveMemberLimits = async () => { if (!memberTarget.value) return; try { await teamAPI.updateLimits(memberTarget.value.user_id, { daily_limit_usd: memberLimits.daily, weekly_limit_usd: memberLimits.weekly, monthly_limit_usd: memberLimits.monthly }); memberTarget.value = null; members.value = await teamAPI.members(); appStore.showSuccess(t('team.operationSuccess')) } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const removeMember = async (member: TeamMembership) => { if (!window.confirm(t('team.removeMessage'))) return; try { await teamAPI.removeMember(member.user_id); await refreshAll() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const transferOwnership = async (member: TeamMembership) => { if (!window.confirm(t('team.transferMessage'))) return; try { const transfer = await teamAPI.startTransfer(member.user_id); await publishShareURL(t('team.transferActionTitle'), transfer.resolution_url); appStore.showSuccess(t('team.operationSuccess')) } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const toggleKey = async (key: TeamAPIKey) => { try { if (key.status === 'active') await teamAPI.disableKey(key.id); else await teamAPI.enableKey(key.id); teamKeys.value = await teamAPI.keys() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const deleteTeamKey = async (key: TeamAPIKey) => { if (!window.confirm(t('team.deleteKeyMessage'))) return; try { await teamAPI.deleteKey(key.id); teamKeys.value = await teamAPI.keys() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const toggleStatus = async () => { if (!teamContext.value) return; const status = teamContext.value.team.status === 'active' ? 'suspended' : 'active'; try { teamContext.value = await teamAPI.setStatus(status); await loadTeamData() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const dissolveTeam = async () => { if (!window.confirm(t('team.dissolveMessage'))) return; try { await teamAPI.dissolve(); teamContext.value = null } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const leaveTeam = async () => { if (!window.confirm(t('team.leaveMessage'))) return; try { await teamAPI.leave(); teamContext.value = null } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const openTeamKeys = () => router.push({ path: '/keys', query: { scope: 'team' } })

const clearTokenQuery = async (name: 'invitation' | 'transfer') => { const query = { ...route.query }; delete query[name]; await router.replace({ query }) }
const loadInvitation = async () => { if (!invitationToken.value) return; invitationLoading.value = true; try { invitationPreview.value = await teamAPI.previewInvitation(invitationToken.value) } catch (e: any) { invitationError.value = e?.message || t('team.invitationLoadFailed') } finally { invitationLoading.value = false } }
const resolveInvitation = async (resolution: 'accepted' | 'declined') => { if (!invitationToken.value) return; resolving.value = true; try { await teamAPI.resolveInvitation(invitationToken.value, resolution); await clearTokenQuery('invitation'); await refreshAll() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { resolving.value = false } }
const resolveTransfer = async () => { if (!transferToken.value) return; const accept = window.confirm(t('team.transferActionDescription')); try { await teamAPI.resolveTransfer(transferToken.value, accept ? 'accepted' : 'declined'); await clearTokenQuery('transfer'); await refreshAll() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }

onMounted(async () => {
  try { const settings = await authAPI.getPublicSettings(); selfServiceEnabled.value = settings.team_self_service_enabled !== false } catch { selfServiceEnabled.value = true }
  await Promise.all([refreshAll(), loadInvitation()])
  if (transferToken.value) await resolveTransfer()
  loading.value = false
})
</script>
