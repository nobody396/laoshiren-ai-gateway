import { defineStore } from "pinia";
import { ref, computed } from "vue";
import rbacAPI from "@/api/admin/rbac";
import type { MenuTreeNode } from "@/api/admin/rbac";

// Module-scope in-flight Promise, shared across all concurrent
// fetchPermissions() callers so that a request triggered at login time
// and a request triggered by the router guard resolve on the same Promise
// instead of racing. See: admin 首次进入带 meta.permission 路由的竞态修复。
let fetchPermissionsInFlight: Promise<void> | null = null;

export const usePermissionStore = defineStore("permission", () => {
  // State
  const menuTree = ref<MenuTreeNode[]>([]);
  const permissionKeys = ref<string[]>([]);
  const isSuperAdmin = ref(false);
  const loaded = ref(false);
  const loading = ref(false);

  // Getters
  const hasLoaded = computed(() => loaded.value);

  // Actions
  async function fetchMenuTree(): Promise<void> {
    try {
      const tree = await rbacAPI.getMyMenuTree();
      menuTree.value = tree;
    } catch {
      // 权限加载失败不阻断应用
      menuTree.value = [];
    }
  }

  async function fetchPermissions(): Promise<void> {
    // 已加载过, 直接返回
    if (loaded.value) return;
    // 已有请求在飞, 复用同一个 Promise, 避免竞态与重复请求
    if (fetchPermissionsInFlight) {
      return fetchPermissionsInFlight;
    }

    loading.value = true;
    fetchPermissionsInFlight = (async () => {
      try {
        const [keys, tree] = await Promise.all([
          rbacAPI.getMyPermissions(),
          rbacAPI.getMyMenuTree(),
        ]);
        permissionKeys.value = keys;
        menuTree.value = tree;
        isSuperAdmin.value = keys.includes("*");
        loaded.value = true;
      } catch {
        // 权限加载失败, 降级为全量权限 (向后兼容)
        permissionKeys.value = ["*"];
        isSuperAdmin.value = true;
        loaded.value = true;
      } finally {
        loading.value = false;
        fetchPermissionsInFlight = null;
      }
    })();

    return fetchPermissionsInFlight;
  }

  function hasPermission(key: string): boolean {
    if (isSuperAdmin.value) return true;
    return permissionKeys.value.includes(key);
  }

  function hasAnyPermission(keys: string[]): boolean {
    if (isSuperAdmin.value) return true;
    return keys.some((k) => permissionKeys.value.includes(k));
  }

  function reset(): void {
    menuTree.value = [];
    permissionKeys.value = [];
    isSuperAdmin.value = false;
    loaded.value = false;
    loading.value = false;
    // 同步丢弃正在飞的 Promise 引用, 避免 logout 后仍返回旧请求结果
    fetchPermissionsInFlight = null;
  }

  return {
    menuTree,
    permissionKeys,
    isSuperAdmin,
    loaded,
    loading,
    hasLoaded,
    fetchMenuTree,
    fetchPermissions,
    hasPermission,
    hasAnyPermission,
    reset,
  };
});
