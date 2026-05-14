<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.assignRolesTitle')"
    width="normal"
    @close="$emit('close')"
  >
    <div v-if="user" class="space-y-4">
      <!-- 用户信息 -->
      <div
        class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700"
      >
        <div
          class="flex h-10 w-10 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30"
        >
          <span
            class="text-lg font-medium text-primary-700 dark:text-primary-300"
          >
            {{ user.email.charAt(0).toUpperCase() }}
          </span>
        </div>
        <div class="flex-1 min-w-0">
          <p class="font-medium text-gray-900 dark:text-white truncate">
            {{ user.email }}
          </p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t("admin.users.columns.role") }}:
            <span class="badge badge-purple ml-1">{{
              t("admin.users.roles.admin")
            }}</span>
          </p>
        </div>
      </div>

      <!-- 角色列表 -->
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.users.selectRoles") }}
        </label>

        <div
          v-if="loading"
          class="flex items-center justify-center py-8 text-gray-400"
        >
          <div
            class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
          ></div>
        </div>

        <div
          v-else-if="allRoles.length === 0"
          class="rounded-lg border border-dashed border-gray-300 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
        >
          {{ t("admin.rbac.noRoles") }}
        </div>

        <div
          v-else
          class="max-h-80 space-y-2 overflow-y-auto rounded-lg border border-gray-200 p-2 dark:border-dark-700"
        >
          <label
            v-for="role in allRoles"
            :key="role.id"
            class="flex cursor-pointer items-start gap-3 rounded-md px-3 py-2 hover:bg-gray-50 dark:hover:bg-dark-700"
          >
            <input
              type="checkbox"
              :value="role.id"
              v-model="selectedRoleIds"
              class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ role.name }}
                </span>
                <span
                  v-if="role.is_super_admin"
                  class="badge badge-warning text-[10px]"
                  >{{ t("admin.rbac.superAdmin") }}</span
                >
              </div>
              <p
                v-if="role.description"
                class="mt-0.5 text-xs text-gray-500 dark:text-gray-400"
              >
                {{ role.description }}
              </p>
            </div>
          </label>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" @click="$emit('close')">
          {{ t("common.cancel") }}
        </button>
        <button
          class="btn btn-primary"
          :disabled="saving || loading"
          @click="handleSave"
        >
          {{ saving ? t("common.saving") : t("common.save") }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useAppStore } from "@/stores/app";
import BaseDialog from "@/components/common/BaseDialog.vue";
import rbacAPI, { type AdminRole } from "@/api/admin/rbac";
import type { AdminUser } from "@/types";

const props = defineProps<{
  show: boolean;
  user: AdminUser | null;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "success"): void;
}>();

const { t } = useI18n();
const appStore = useAppStore();

const loading = ref(false);
const saving = ref(false);
const allRoles = ref<AdminRole[]>([]);
const selectedRoleIds = ref<number[]>([]);

// 打开时加载角色列表 + 当前用户绑定角色
watch(
  () => props.show,
  async (v) => {
    if (!v || !props.user) return;
    loading.value = true;
    try {
      const [roles, assigned] = await Promise.all([
        rbacAPI.listRoles(),
        rbacAPI.getUserRoles(props.user.id),
      ]);
      // 后端空列表可能序列化为 null, 防御性关空
      allRoles.value = Array.isArray(roles) ? roles : [];
      selectedRoleIds.value = Array.isArray(assigned)
        ? assigned.map((r) => r.id)
        : [];
    } catch (e) {
      console.error("Failed to load roles:", e);
      allRoles.value = [];
      selectedRoleIds.value = [];
    } finally {
      loading.value = false;
    }
  },
);

const handleSave = async () => {
  if (!props.user) return;
  saving.value = true;
  try {
    await rbacAPI.setUserRoles(props.user.id, {
      role_ids: selectedRoleIds.value,
    });
    appStore.showSuccess(t("admin.users.rolesAssignedSuccess"));
    emit("success");
    emit("close");
  } catch (e: any) {
    console.error("Failed to assign roles:", e);
    appStore.showError(
      e.response?.data?.detail ||
        e.response?.data?.message ||
        t("admin.users.failedToAssignRoles"),
    );
  } finally {
    saving.value = false;
  }
};
</script>
