import { usePermissionStore } from "@/stores/permission";

/**
 * 权限检查 composable
 *
 * 用法:
 * ```vue
 * <script setup>
 * const { hasPermission, hasAnyPermission, isSuperAdmin } = usePermission()
 * </script>
 *
 * <template>
 *   <button v-if="hasPermission('user:create')">创建用户</button>
 * </template>
 * ```
 */
export function usePermission() {
  const permStore = usePermissionStore();

  function hasPermission(key: string): boolean {
    return permStore.hasPermission(key);
  }

  function hasAnyPermission(keys: string[]): boolean {
    return permStore.hasAnyPermission(keys);
  }

  return {
    hasPermission,
    hasAnyPermission,
    isSuperAdmin: permStore.isSuperAdmin,
    loaded: permStore.loaded,
  };
}
