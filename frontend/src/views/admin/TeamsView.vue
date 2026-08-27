<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6 p-4 sm:p-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div><h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('team.title') }}</h1><p class="mt-1 text-sm text-gray-500">{{ t('team.description') }}</p></div>
        <button class="btn btn-primary" @click="showCreate = true"><Icon name="plus" size="sm" class="mr-2" />{{ t('team.create') }}</button>
      </div>

      <section class="card p-5">
        <input v-model="search" class="input mb-4 max-w-md" :placeholder="t('team.searchPlaceholder')" />
        <div v-if="loading" class="flex justify-center py-16"><LoadingSpinner /></div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead><tr class="text-left text-xs uppercase text-gray-500"><th class="px-3 py-3">ID</th><th class="px-3 py-3">{{ t('team.name') }}</th><th class="px-3 py-3">{{ t('team.owner') }}</th><th class="px-3 py-3">{{ t('team.members') }}</th><th class="px-3 py-3">{{ t('common.status') }}</th><th class="px-3 py-3">{{ t('common.actions') }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="team in filteredTeams" :key="team.id">
                <td class="px-3 py-3 text-sm text-gray-500">{{ team.id }}</td>
                <td class="px-3 py-3 font-medium text-gray-900 dark:text-white">{{ team.name }}</td>
                <td class="px-3 py-3 text-sm text-gray-600 dark:text-gray-300">{{ team.owner_email }}</td>
                <td class="px-3 py-3 text-sm text-gray-600 dark:text-gray-300">{{ team.member_count }} / {{ team.member_limit }}</td>
                <td class="px-3 py-3"><span :class="['badge', team.status === 'active' ? 'badge-success' : 'badge-warning']">{{ team.status }}</span></td>
                <td class="px-3 py-3"><div class="flex flex-wrap gap-2"><button class="btn btn-secondary" @click="openDetails(team)">{{ t('team.viewDetails') }}</button><button class="btn btn-secondary" @click="editTeam(team)">{{ t('common.edit') }}</button><button class="btn btn-danger" @click="dissolve(team)">{{ t('team.dissolve') }}</button></div></td>
              </tr>
            </tbody>
          </table>
          <p v-if="filteredTeams.length === 0" class="py-12 text-center text-sm text-gray-500">{{ t('team.noTeams') }}</p>
        </div>
      </section>
    </div>

    <BaseDialog :show="showCreate" :title="t('team.createTitle')" @close="showCreate = false">
      <form id="admin-team-create" class="space-y-4" @submit.prevent="createTeam">
        <div><label class="input-label">{{ t('team.ownerUserID') }}</label><input v-model.number="createForm.owner_user_id" class="input" type="number" min="1" required /></div>
        <div><label class="input-label">{{ t('team.name') }}</label><input v-model="createForm.name" class="input" maxlength="100" required /></div>
        <div><label class="input-label">{{ t('team.memberCapacity') }}</label><input v-model.number="createForm.member_limit" class="input" type="number" min="0" required /></div>
      </form>
      <template #footer><button form="admin-team-create" class="btn btn-primary" :disabled="saving">{{ t('common.create') }}</button></template>
    </BaseDialog>

    <BaseDialog :show="Boolean(editing)" :title="t('team.editTitle')" @close="editing = null">
      <form id="admin-team-edit" class="space-y-4" @submit.prevent="saveEdit">
        <div><label class="input-label">{{ t('team.name') }}</label><input v-model="editForm.name" class="input" maxlength="100" required /></div>
        <div><label class="input-label">{{ t('team.memberCapacity') }}</label><input v-model.number="editForm.member_limit" class="input" type="number" min="0" required /></div>
        <div><label class="input-label">{{ t('common.status') }}</label><select v-model="editForm.status" class="input"><option value="active">active</option><option value="suspended">suspended</option></select></div>
      </form>
      <template #footer><button form="admin-team-edit" class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button></template>
    </BaseDialog>

    <BaseDialog :show="Boolean(details)" :title="details ? t('team.detailsTitle', { name: details.name }) : ''" width="wide" @close="details = null">
      <div v-if="detailsLoading" class="flex justify-center py-12"><LoadingSpinner /></div>
      <div v-else class="space-y-5">
        <div class="grid gap-3 sm:grid-cols-4">
          <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800"><p class="text-xs text-gray-500">{{ t('team.totalCost') }}</p><p class="mt-1 font-semibold">${{ Number(usage?.actual_cost || 0).toFixed(4) }}</p></div>
          <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800"><p class="text-xs text-gray-500">{{ t('team.requests') }}</p><p class="mt-1 font-semibold">{{ usage?.request_count || 0 }}</p></div>
          <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800"><p class="text-xs text-gray-500">{{ t('team.inputTokens') }}</p><p class="mt-1 font-semibold">{{ usage?.input_tokens || 0 }}</p></div>
          <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800"><p class="text-xs text-gray-500">{{ t('team.outputTokens') }}</p><p class="mt-1 font-semibold">{{ usage?.output_tokens || 0 }}</p></div>
        </div>
        <div class="space-y-2"><div v-for="member in detailMembers" :key="member.id" class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-700"><div><p class="font-medium">{{ member.username || member.email }}</p><p class="text-xs text-gray-500">{{ member.email }} · {{ member.role }}</p></div><button v-if="member.role === 'member'" class="btn btn-secondary" @click="forceTransfer(member.user_id)">{{ t('team.transfer') }}</button></div></div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { AdminTeam } from '@/api/admin/teams'
import type { TeamMembership, TeamUsageSummary } from '@/api/team'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()
const teams = ref<AdminTeam[]>([])
const loading = ref(false)
const saving = ref(false)
const search = ref('')
const showCreate = ref(false)
const editing = ref<AdminTeam | null>(null)
const details = ref<AdminTeam | null>(null)
const detailsLoading = ref(false)
const detailMembers = ref<TeamMembership[]>([])
const usage = ref<TeamUsageSummary | null>(null)
const createForm = reactive({ owner_user_id: 0, name: '', member_limit: 10 })
const editForm = reactive({ name: '', member_limit: 10, status: 'active' as 'active' | 'suspended' })
const filteredTeams = computed(() => { const q = search.value.trim().toLowerCase(); return q ? teams.value.filter((team) => team.name.toLowerCase().includes(q) || team.owner_email.toLowerCase().includes(q) || String(team.id).includes(q)) : teams.value })

const loadTeams = async () => { loading.value = true; try { teams.value = await adminAPI.teams.list() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { loading.value = false } }
const createTeam = async () => { saving.value = true; try { await adminAPI.teams.create({ ...createForm }); showCreate.value = false; createForm.owner_user_id = 0; createForm.name = ''; await loadTeams() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { saving.value = false } }
const editTeam = (team: AdminTeam) => { editing.value = team; editForm.name = team.name; editForm.member_limit = team.member_limit; editForm.status = team.status }
const saveEdit = async () => { if (!editing.value) return; saving.value = true; try { await adminAPI.teams.update(editing.value.id, { ...editForm }); editing.value = null; await loadTeams() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { saving.value = false } }
const dissolve = async (team: AdminTeam) => { if (!window.confirm(t('team.dissolveMessage'))) return; try { await adminAPI.teams.dissolve(team.id); await loadTeams() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }
const openDetails = async (team: AdminTeam) => { details.value = team; detailsLoading.value = true; try { [detailMembers.value, usage.value] = await Promise.all([adminAPI.teams.members(team.id), adminAPI.teams.usage(team.id)]) } catch (e: any) { appStore.showError(e?.message || t('common.error')) } finally { detailsLoading.value = false } }
const forceTransfer = async (userID: number) => { if (!details.value || !window.confirm(t('team.transferMessage'))) return; try { await adminAPI.teams.forceTransfer(details.value.id, userID); await openDetails(details.value); await loadTeams() } catch (e: any) { appStore.showError(e?.message || t('common.error')) } }

onMounted(loadTeams)
</script>
