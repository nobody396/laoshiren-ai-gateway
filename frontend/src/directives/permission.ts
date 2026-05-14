import type { Directive, DirectiveBinding } from "vue";
import { usePermissionStore } from "@/stores/permission";

/**
 * v-permission 权限指令
 *
 * 用法:
 * ```html
 * <button v-permission="'user:create'">创建用户</button>
 * <button v-permission="['user:update', 'user:delete']">操作</button>
 * ```
 *
 * 无权限时元素会被从 DOM 中移除 (类似 v-if)
 */
export const vPermission: Directive = {
  mounted(el: HTMLElement, binding: DirectiveBinding<string | string[]>) {
    const permStore = usePermissionStore();
    const required = Array.isArray(binding.value)
      ? binding.value
      : [binding.value];

    if (!permStore.hasAnyPermission(required)) {
      el.parentNode?.removeChild(el);
    }
  },
};
