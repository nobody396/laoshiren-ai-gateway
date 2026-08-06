<template>
  <aside
    class="sidebar"
    :class="[
      sidebarCollapsed ? 'w-[72px]' : 'w-64',
      { '-translate-x-full lg:translate-x-0': !mobileOpen }
    ]"
  >
    <!-- Logo/Brand -->
    <div class="sidebar-header">
      <!-- Custom Logo or Default Logo -->
      <div class="flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl shadow-glow">
        <img v-if="settingsLoaded" :src="siteLogo || '/laoshirenai-icon.jpg'" alt="Logo" class="h-full w-full object-cover" />
      </div>
      <transition name="fade">
        <div v-if="!sidebarCollapsed" class="flex flex-col">
          <span class="text-lg font-bold text-gray-900 dark:text-white">
            {{ siteName }}
          </span>
        </div>
      </transition>
    </div>

    <!-- Navigation -->
    <nav class="sidebar-nav scrollbar-hide">
      <!-- Admin View: Admin menu first, then personal menu -->
      <template v-if="isAdmin">
        <!-- Admin Section -->
        <div class="sidebar-section">
          <router-link
            v-for="item in adminNavItems"
            :key="item.path"
            :to="item.path"
            class="sidebar-link mb-1"
            :class="{ 'sidebar-link-active': isActive(item.path) }"
            :title="sidebarCollapsed ? item.label : undefined"
            :id="
              item.path === '/admin/accounts'
                ? 'sidebar-channel-manage'
                : item.path === '/admin/groups'
                  ? 'sidebar-group-manage'
                  : item.path === '/admin/redeem'
                    ? 'sidebar-wallet'
                    : undefined
            "
            @click="handleMenuItemClick(item.path)"
          >
            <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
            <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
            <transition name="fade">
              <span v-if="!sidebarCollapsed">{{ item.label }}</span>
            </transition>
            <span
              v-if="item.badge && item.badge > 0"
              class="ml-auto inline-flex min-w-5 items-center justify-center rounded-full bg-red-100 px-1.5 py-0.5 text-xs font-bold text-red-700 dark:bg-red-950 dark:text-red-300"
              :aria-label="`${item.badge} 个待处理事项`"
            >{{ item.badge > 99 ? '99+' : item.badge }}</span>
            <span
              v-if="item.showDot"
              class="ml-auto h-2 w-2 flex-shrink-0 rounded-full bg-primary-500 ring-2 ring-primary-100 dark:ring-primary-900"
              :aria-label="t('changelog.newUpdate')"
            ></span>
          </router-link>
        </div>

        <!-- Personal Section for Admin (hidden in simple mode) -->
        <div v-if="!authStore.isSimpleMode" class="sidebar-section">
          <div v-if="!sidebarCollapsed" class="sidebar-section-title">
            {{ t('nav.myAccount') }}
          </div>
          <div v-else class="mx-3 my-3 h-px bg-gray-200 dark:bg-dark-700"></div>

          <component
            :is="item.external ? 'a' : 'router-link'"
            v-for="item in personalNavItems"
            :key="item.path"
            :to="item.external ? undefined : item.path"
            :href="item.external ? item.path : undefined"
            :target="item.external ? '_blank' : undefined"
            :rel="item.external ? 'noopener noreferrer' : undefined"
            class="sidebar-link mb-1"
            :class="{ 'sidebar-link-active': !item.external && isActive(item.path) }"
            :title="sidebarCollapsed ? item.label : undefined"
            :data-tour="getNavTourAttr(item.path)"
            @click="handleMenuItemClick(item.path)"
          >
            <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
            <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
            <transition name="fade">
              <span v-if="!sidebarCollapsed">{{ item.label }}</span>
            </transition>
            <span
              v-if="item.showDot"
              class="ml-auto h-2 w-2 flex-shrink-0 rounded-full bg-primary-500 ring-2 ring-primary-100 dark:ring-primary-900"
              :aria-label="t('changelog.newUpdate')"
            ></span>
          </component>
        </div>
      </template>

      <!-- Regular User View -->
      <template v-else-if="!appStore.backendModeEnabled">
        <!-- Agent section (shown when role=agent) -->
        <div v-if="isAgent" class="sidebar-section">
          <div v-if="!sidebarCollapsed" class="sidebar-section-title">
            {{ t('nav.agentConsole') }}
          </div>
          <div v-else class="mx-3 my-3 h-px bg-gray-200 dark:bg-dark-700"></div>
          <router-link
            v-for="item in agentNavItems"
            :key="item.path"
            :to="item.path"
            class="sidebar-link mb-1"
            :class="{ 'sidebar-link-active': isActive(item.path) }"
            :title="sidebarCollapsed ? item.label : undefined"
            @click="handleMenuItemClick(item.path)"
          >
            <component :is="item.icon" class="h-5 w-5 flex-shrink-0" />
            <transition name="fade">
              <span v-if="!sidebarCollapsed">{{ item.label }}</span>
            </transition>
            <span
              v-if="item.showDot"
              class="ml-auto h-2 w-2 flex-shrink-0 rounded-full bg-primary-500 ring-2 ring-primary-100 dark:ring-primary-900"
              :aria-label="t('changelog.newUpdate')"
            ></span>
          </router-link>
        </div>
        <div class="sidebar-section">
          <component
            :is="item.external ? 'a' : 'router-link'"
            v-for="item in userNavItems"
            :key="item.path"
            :to="item.external ? undefined : item.path"
            :href="item.external ? item.path : undefined"
            :target="item.external ? '_blank' : undefined"
            :rel="item.external ? 'noopener noreferrer' : undefined"
            class="sidebar-link mb-1"
            :class="{ 'sidebar-link-active': !item.external && isActive(item.path) }"
            :title="sidebarCollapsed ? item.label : undefined"
            :data-tour="getNavTourAttr(item.path)"
            @click="handleMenuItemClick(item.path)"
          >
            <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
            <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
            <transition name="fade">
              <span v-if="!sidebarCollapsed">{{ item.label }}</span>
            </transition>
            <span
              v-if="item.showDot"
              class="ml-auto h-2 w-2 flex-shrink-0 rounded-full bg-primary-500 ring-2 ring-primary-100 dark:ring-primary-900"
              :aria-label="t('changelog.newUpdate')"
            ></span>
          </component>
        </div>
      </template>
    </nav>

    <!-- Bottom Section -->
    <div class="mt-auto border-t border-gray-100 p-3 dark:border-dark-800">
      <!-- Theme Toggle -->
      <button
        @click="toggleTheme"
        class="sidebar-link mb-2 w-full"
        :title="sidebarCollapsed ? (isDark ? t('nav.lightMode') : t('nav.darkMode')) : undefined"
      >
        <SunIcon v-if="isDark" class="h-5 w-5 flex-shrink-0 text-amber-500" />
        <MoonIcon v-else class="h-5 w-5 flex-shrink-0" />
        <transition name="fade">
          <span v-if="!sidebarCollapsed">{{
            isDark ? t('nav.lightMode') : t('nav.darkMode')
          }}</span>
        </transition>
      </button>

      <!-- Collapse Button -->
      <button
        @click="toggleSidebar"
        class="sidebar-link w-full"
        :title="sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
      >
        <ChevronDoubleLeftIcon v-if="!sidebarCollapsed" class="h-5 w-5 flex-shrink-0" />
        <ChevronDoubleRightIcon v-else class="h-5 w-5 flex-shrink-0" />
        <transition name="fade">
          <span v-if="!sidebarCollapsed">{{ t('nav.collapse') }}</span>
        </transition>
      </button>
    </div>
  </aside>

  <!-- Mobile Overlay -->
  <transition name="fade">
    <div
      v-if="mobileOpen"
      class="fixed inset-0 z-30 bg-black/50 lg:hidden"
      @click="closeMobile"
    ></div>
  </transition>
</template>

<script setup lang="ts">
import { applyThemeClass, isDarkTheme, setTheme } from '@/utils/theme'
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  useAdminSettingsStore,
  useAffiliateProgramStore,
  useAppStore,
  useAuthStore,
  useOnboardingStore
} from '@/stores'
import { usePermissionStore } from '@/stores/permission'
import { sanitizeSvg } from '@/utils/sanitize'
import { useChangelogFreshness } from '@/composables/useChangelogFreshness'
import { getAffiliateOperationsSummary } from '@/api/admin/agents'

interface NavItem {
  path: string
  label: string
  icon: unknown
  iconSvg?: string
  hideInSimpleMode?: boolean
  external?: boolean
  showDot?: boolean
  badge?: number
}

interface AdminMenuOverride {
  name: string
  nameEn?: string
  permissionKey?: string
}

const { t, locale } = useI18n()

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const onboardingStore = useOnboardingStore()
const adminSettingsStore = useAdminSettingsStore()
const affiliateProgramStore = useAffiliateProgramStore()
const permStore = usePermissionStore()
const { hasNewChangelog, refreshChangelogFreshness } = useChangelogFreshness()

const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const mobileOpen = computed(() => appStore.mobileOpen)
const isAdmin = computed(() => authStore.isAdmin)
const isAgent = computed(() => authStore.user?.role === 'agent')
const showAffiliateEntry = computed(() => affiliateProgramStore.isLive)
const isDark = ref(document.documentElement.classList.contains('dark'))
const affiliateActionCount = ref(0)
let affiliateSummaryTimer: ReturnType<typeof setInterval> | undefined

async function refreshAffiliateActionCount() {
  if (!isAdmin.value || document.visibilityState !== 'visible') return
  try {
    affiliateActionCount.value = (await getAffiliateOperationsSummary()).actionable_total
  } catch {
    affiliateActionCount.value = 0
  }
}

function startAffiliateSummaryPolling() {
  if (affiliateSummaryTimer) clearInterval(affiliateSummaryTimer)
  void refreshAffiliateActionCount()
  affiliateSummaryTimer = setInterval(() => { void refreshAffiliateActionCount() }, 60_000)
}

function stopAffiliateSummaryPolling() {
  if (affiliateSummaryTimer) clearInterval(affiliateSummaryTimer)
  affiliateSummaryTimer = undefined
  affiliateActionCount.value = 0
}

// Site settings from appStore (cached, no flicker)
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => appStore.siteLogo)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const invoiceManagementEnabled = computed(
  () => appStore.cachedPublicSettings?.invoice_management_enabled === true
)
const feedbackManagementEnabled = computed(
  () => appStore.cachedPublicSettings?.feedback_management_enabled !== false
)

// SVG Icon Components
const DashboardIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z'
        })
      ]
    )
}

const KeyIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z'
        })
      ]
    )
}

const ChartIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z'
        })
      ]
    )
}

const GiftIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M21 11.25v8.25a1.5 1.5 0 01-1.5 1.5H5.25a1.5 1.5 0 01-1.5-1.5v-8.25M12 4.875A2.625 2.625 0 109.375 7.5H12m0-2.625V7.5m0-2.625A2.625 2.625 0 1114.625 7.5H12m0 0V21m-8.625-9.75h18c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125h-18c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z'
        })
      ]
    )
}

const UserIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z'
        })
      ]
    )
}

const UsersIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z'
        })
      ]
    )
}

const FolderIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z'
        })
      ]
    )
}

const CreditCardIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z'
        })
      ]
    )
}

const WalletIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M12 6v12m-3-2.818l.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
        })
      ]
    )
}

const RechargeSubscriptionIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'currentColor', viewBox: '0 0 1024 1024' },
      [
        h('path', {
          d: 'M512 992C247.3 992 32 776.7 32 512S247.3 32 512 32s480 215.3 480 480c0 84.4-22.2 167.4-64.2 240-8.9 15.3-28.4 20.6-43.7 11.7-15.3-8.8-20.5-28.4-11.7-43.7 36.4-62.9 55.6-134.8 55.6-208 0-229.4-186.6-416-416-416S96 282.6 96 512s186.6 416 416 416c17.7 0 32 14.3 32 32s-14.3 32-32 32z'
        }),
        h('path', {
          d: 'M640 512H384c-17.7 0-32-14.3-32-32s14.3-32 32-32h256c17.7 0 32 14.3 32 32s-14.3 32-32 32zM640 640H384c-17.7 0-32-14.3-32-32s14.3-32 32-32h256c17.7 0 32 14.3 32 32s-14.3 32-32 32z'
        }),
        h('path', {
          d: 'M512 480c-8.2 0-16.4-3.1-22.6-9.4l-128-128c-12.5-12.5-12.5-32.8 0-45.3s32.8-12.5 45.3 0l128 128c12.5 12.5 12.5 32.8 0 45.3-6.3 6.3-14.5 9.4-22.7 9.4z'
        }),
        h('path', {
          d: 'M512 480c-8.2 0-16.4-3.1-22.6-9.4-12.5-12.5-12.5-32.8 0-45.3l128-128c12.5-12.5 32.8-12.5 45.3 0s12.5 32.8 0 45.3l-128 128c-6.3 6.3-14.5 9.4-22.7 9.4z'
        }),
        h('path', {
          d: 'M512 736c-17.7 0-32-14.3-32-32V448c0-17.7 14.3-32 32-32s32 14.3 32 32v256c0 17.7-14.3 32-32 32zM896 992H512c-17.7 0-32-14.3-32-32s14.3-32 32-32h306.8l-73.4-73.4c-12.5-12.5-12.5-32.8 0-45.3s32.8-12.5 45.3 0l128 128c9.2 9.2 11.9 22.9 6.9 34.9S908.9 992 896 992z'
        })
      ]
    )
}

const GlobeIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418'
        })
      ]
    )
}

const ChannelIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z'
        })
      ]
    )
}

const ServerIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z'
        })
      ]
    )
}

const BellIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75V9a6 6 0 10-12 0v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0'
        })
      ]
    )
}

const FeedbackIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M7.5 8.25h9m-9 3h6m-9 8.25h15A2.25 2.25 0 0021.75 17.25V6.75A2.25 2.25 0 0019.5 4.5h-15A2.25 2.25 0 002.25 6.75v10.5A2.25 2.25 0 004.5 19.5z'
        })
      ]
    )
}

const TicketIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M16.5 6v.75m0 3v.75m0 3v.75m0 3V18m-9-5.25h5.25M7.5 15h3M3.375 5.25c-.621 0-1.125.504-1.125 1.125v3.026a2.999 2.999 0 010 5.198v3.026c0 .621.504 1.125 1.125 1.125h17.25c.621 0 1.125-.504 1.125-1.125v-3.026a2.999 2.999 0 010-5.198V6.375c0-.621-.504-1.125-1.125-1.125H3.375z'
        })
      ]
    )
}

const BookIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M12 6.042A8.967 8.967 0 006.042 3.75H4.5A2.25 2.25 0 002.25 6v12A2.25 2.25 0 004.5 20.25h1.542A8.966 8.966 0 0112 22.5m0-16.458A8.967 8.967 0 0117.958 3.75H19.5A2.25 2.25 0 0121.75 6v12a2.25 2.25 0 01-2.25 2.25h-1.542A8.966 8.966 0 0012 22.5m0-16.458v16.458'
        })
      ]
    )
}

const DownloadIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M7.5 10.5L12 15m0 0l4.5-4.5M12 15V3'
        })
      ]
    )
}

const CogIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z'
        }),
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z'
        })
      ]
    )
}

const ShieldIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z'
        })
      ]
    )
}

const SunIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z'
        })
      ]
    )
}

const MoonIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z'
        })
      ]
    )
}

const ChevronDoubleLeftIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'm18.75 4.5-7.5 7.5 7.5 7.5m-6-15L5.25 12l7.5 7.5'
        })
      ]
    )
}

const ChevronDoubleRightIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'm5.25 4.5 7.5 7.5-7.5 7.5m6-15 7.5 7.5-7.5 7.5'
        })
      ]
    )
}

const AgentIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z'
        })
      ]
    )
}

// Agent navigation items (shown for role=agent)
const agentNavItems = computed((): NavItem[] => [
  ...(showAffiliateEntry.value
    ? [{ path: '/affiliate', label: t('nav.affiliateCenter'), icon: AgentIcon }]
    : []),
  { path: '/agent/users', label: t('nav.agentUsers'), icon: UsersIcon },
  { path: '/agent/commissions', label: t('nav.agentCommissions'), icon: ChartIcon }
])

/**
 * 统一生成侧边栏文档入口，避免普通用户和管理员菜单配置分叉。
 */
function createDocsNavItem(): NavItem {
  return { path: '/docs', label: t('nav.docs'), icon: BookIcon }
}

function createChangelogNavItem(): NavItem {
  return {
    path: '/changelog',
    label: t('nav.changelog'),
    icon: BookIcon,
    showDot: hasNewChangelog.value
  }
}

function createModelPricingNavItem(): NavItem {
  return {
    path: '/models',
    label: t('nav.modelPricing'),
    icon: ChartIcon,
    external: false
  }
}

// User navigation items (for regular users)
const userNavItems = computed((): NavItem[] => {
  const items: NavItem[] = [
    { path: '/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
    { path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon },
    { path: '/usage', label: t('nav.usage'), icon: ChartIcon, hideInSimpleMode: true },
    { path: '/get-subscription', label: t('nav.getSubscription'), icon: RechargeSubscriptionIcon },
    createModelPricingNavItem(),
    { path: '/topup/orders', label: t('nav.topupOrders'), icon: CreditCardIcon },
    ...(!isAgent.value && showAffiliateEntry.value
      ? [{ path: '/affiliate', label: t('nav.affiliateCenter'), icon: AgentIcon }]
      : []),
    ...(invoiceManagementEnabled.value
      ? [{ path: '/invoice', label: t('nav.invoiceManagement'), icon: TicketIcon }]
      : []),
    ...(feedbackManagementEnabled.value
      ? [{ path: '/feedbacks', label: t('nav.feedback'), icon: FeedbackIcon }]
      : []),
    { path: '/redeem', label: t('nav.redeem'), icon: GiftIcon, hideInSimpleMode: true },
    { path: '/resources', label: t('nav.resources'), icon: DownloadIcon },
    { path: '/resources/status', label: t('nav.versionStatus'), icon: ServerIcon },
    createChangelogNavItem(),
    createDocsNavItem(),
    { path: '/profile', label: t('nav.profile'), icon: UserIcon },
    ...customMenuItemsForUser.value.map((item): NavItem => ({
      path: `/custom/${item.id}`,
      label: item.label,
      icon: null,
      iconSvg: item.icon_svg,
    })),
  ]
  return authStore.isSimpleMode ? items.filter(item => !item.hideInSimpleMode) : items
})

// Personal navigation items (for admin's "My Account" section, without Dashboard)
const personalNavItems = computed((): NavItem[] => {
  const items: NavItem[] = [
    { path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon },
    { path: '/usage', label: t('nav.usage'), icon: ChartIcon, hideInSimpleMode: true },
    { path: '/get-subscription', label: t('nav.getSubscription'), icon: RechargeSubscriptionIcon },
    createModelPricingNavItem(),
    { path: '/topup/orders', label: t('nav.topupOrders'), icon: CreditCardIcon },
    ...(invoiceManagementEnabled.value
      ? [{ path: '/invoice', label: t('nav.invoiceManagement'), icon: TicketIcon }]
      : []),
    ...(feedbackManagementEnabled.value
      ? [{ path: '/feedbacks', label: t('nav.feedback'), icon: FeedbackIcon }]
      : []),
    { path: '/redeem', label: t('nav.redeem'), icon: GiftIcon, hideInSimpleMode: true },
    { path: '/resources', label: t('nav.resources'), icon: DownloadIcon },
    { path: '/resources/status', label: t('nav.versionStatus'), icon: ServerIcon },
    createDocsNavItem(),
    { path: '/profile', label: t('nav.profile'), icon: UserIcon },
    ...customMenuItemsForUser.value.map((item): NavItem => ({
      path: `/custom/${item.id}`,
      label: item.label,
      icon: null,
      iconSvg: item.icon_svg,
    })),
  ]
  return authStore.isSimpleMode ? items.filter(item => !item.hideInSimpleMode) : items
})

// Custom menu items filtered by visibility
const customMenuItemsForUser = computed(() => {
  const items = appStore.cachedPublicSettings?.custom_menu_items ?? []
  return items
    .filter((item) => item.visibility === 'user')
    .sort((a, b) => a.sort_order - b.sort_order)
})

const customMenuItemsForAdmin = computed(() => {
  return adminSettingsStore.customMenuItems
    .filter((item) => item.visibility === 'admin')
    .sort((a, b) => a.sort_order - b.sort_order)
})

const adminMenuOverridesByPath = computed(() => {
  const map = new Map<string, AdminMenuOverride>()
  for (const menu of permStore.menuTree) {
    if (!menu.path) continue
    map.set(menu.path, {
      name: menu.name,
      nameEn: menu.name_en,
      permissionKey: menu.permission_key
    })
  }
  return map
})

function resolveAdminMenuLabel(path: string, fallbackLabel: string): string {
  const menu = adminMenuOverridesByPath.value.get(path)
  if (!menu) {
    return fallbackLabel
  }
  if (locale.value === 'en') {
    const englishName = (menu.nameEn ?? '').trim()
    if (englishName) {
      return englishName
    }
    if (fallbackLabel.trim()) {
      return fallbackLabel
    }
    if (menu.name.trim()) {
      return menu.name.trim()
    }
    return fallbackLabel
  }
  if (menu.name.trim()) {
    return menu.name.trim()
  }
  return fallbackLabel
}

const navPermissionMap: Record<string, string> = {
  '/admin/dashboard': 'admin:dashboard',
  '/admin/ops': 'admin:ops',
  '/admin/monthly-upstreams': 'admin:ops',
  '/admin/cost-accounting': 'admin:ops',
  '/admin/users': 'admin:users',
  '/admin/affiliate': 'admin:agents',
  '/admin/groups': 'admin:groups',
  '/admin/channels': 'admin:channels',
  '/admin/suppliers': 'admin:suppliers',
  '/admin/subscriptions': 'admin:subscriptions',
  '/admin/accounts': 'admin:accounts',
  '/admin/announcements': 'admin:announcements',
  '/admin/changelog': 'admin:changelog',
  '/admin/feedbacks': 'admin:feedbacks',
  '/admin/finance-transactions': 'admin:finance-transactions',
  '/admin/proxies': 'admin:proxies',
  '/admin/redeem': 'admin:redeem',
  '/admin/billing': 'admin:redeem',
  '/admin/promo-codes': 'admin:promo-codes',
  '/admin/usage': 'admin:usage',
  '/admin/topup-orders': 'admin:topup-orders',
  '/admin/invoice-requests': 'admin:invoice-requests',
  '/admin/settings': 'admin:settings',
  '/admin/roles': 'admin:roles',
  '/admin/menus': 'admin:menus',
  '/admin/apis': 'admin:apis'
}

// Admin navigation items
const adminNavItems = computed((): NavItem[] => {
  const baseItems: NavItem[] = [
    {
      path: '/admin/dashboard',
      label: resolveAdminMenuLabel('/admin/dashboard', t('nav.dashboard')),
      icon: DashboardIcon
    },
    ...(adminSettingsStore.opsMonitoringEnabled
      ? [{
          path: '/admin/ops',
          label: resolveAdminMenuLabel('/admin/ops', t('nav.ops')),
          icon: ChartIcon
        }, {
          path: '/admin/monthly-upstreams',
          label: resolveAdminMenuLabel('/admin/monthly-upstreams', t('nav.monthlyUpstreams', '月卡监控')),
          icon: ServerIcon
        }, {
          path: '/admin/cost-accounting',
          label: resolveAdminMenuLabel('/admin/cost-accounting', t('nav.costAccounting', '成本核算')),
          icon: CreditCardIcon
        }]
      : []),
    {
      path: '/admin/users',
      label: resolveAdminMenuLabel('/admin/users', t('nav.users')),
      icon: UsersIcon,
      hideInSimpleMode: true
    },
    {
      path: '/admin/affiliate',
      label: resolveAdminMenuLabel('/admin/affiliate', t('nav.affiliateOperations')),
      icon: GiftIcon,
      hideInSimpleMode: true,
      badge: affiliateActionCount.value
    },
    {
      path: '/admin/groups',
      label: resolveAdminMenuLabel('/admin/groups', t('nav.groups')),
      icon: FolderIcon,
      hideInSimpleMode: true
    },
    {
      path: '/admin/channels',
      label: resolveAdminMenuLabel('/admin/channels', t('nav.channels', '渠道管理')),
      icon: ChannelIcon,
      hideInSimpleMode: true
    },
    {
      path: '/admin/suppliers',
      label: resolveAdminMenuLabel('/admin/suppliers', t('nav.suppliers', '供应商考察')),
      icon: ServerIcon,
      hideInSimpleMode: true
    },
    {
      path: '/admin/subscriptions',
      label: resolveAdminMenuLabel('/admin/subscriptions', t('nav.subscriptions')),
      icon: CreditCardIcon,
      hideInSimpleMode: true
    },
    {
      path: '/admin/accounts',
      label: resolveAdminMenuLabel('/admin/accounts', t('nav.accounts')),
      icon: GlobeIcon
    },
    {
      path: '/admin/announcements',
      label: resolveAdminMenuLabel('/admin/announcements', t('nav.announcements')),
      icon: BellIcon
    },
    {
      path: '/admin/changelog',
      label: resolveAdminMenuLabel('/admin/changelog', t('nav.changelog')),
      icon: BookIcon
    },
    {
      path: '/admin/feedbacks',
      label: resolveAdminMenuLabel('/admin/feedbacks', t('nav.feedbackAdmin')),
      icon: FeedbackIcon
    },
    {
      path: '/admin/finance-transactions',
      label: resolveAdminMenuLabel('/admin/finance-transactions', t('nav.financeTransactions', '财务记账')),
      icon: WalletIcon
    },
    {
      path: '/admin/proxies',
      label: resolveAdminMenuLabel('/admin/proxies', t('nav.proxies')),
      icon: ServerIcon
    },
    {
      path: '/admin/redeem',
      label: resolveAdminMenuLabel('/admin/redeem', t('nav.redeemCodes')),
      icon: TicketIcon,
      hideInSimpleMode: true
    },
    {
      path: '/admin/billing',
      label: resolveAdminMenuLabel('/admin/billing', t('nav.billing')),
      icon: CreditCardIcon,
      hideInSimpleMode: true
    },
    {
      path: '/admin/promo-codes',
      label: resolveAdminMenuLabel('/admin/promo-codes', t('nav.promoCodes')),
      icon: GiftIcon,
      hideInSimpleMode: true
    },
    {
      path: '/admin/usage',
      label: resolveAdminMenuLabel('/admin/usage', t('nav.usage')),
      icon: ChartIcon
    },
    {
      path: '/admin/topup-orders',
      label: resolveAdminMenuLabel('/admin/topup-orders', t('nav.topupOrderManagement')),
      icon: CreditCardIcon
    },
    {
      path: '/admin/invoice-requests',
      label: resolveAdminMenuLabel('/admin/invoice-requests', t('nav.invoiceRequestManagement')),
      icon: TicketIcon
    },
    {
      path: '/admin/roles',
      label: resolveAdminMenuLabel('/admin/roles', t('nav.roleManagement', '角色管理')),
      icon: ShieldIcon
    },
    {
      path: '/admin/menus',
      label: resolveAdminMenuLabel('/admin/menus', t('nav.menuManagement', '菜单管理')),
      icon: ShieldIcon
    },
    {
      path: '/admin/apis',
      label: resolveAdminMenuLabel('/admin/apis', t('nav.apiManagement', 'API 管理')),
      icon: ShieldIcon
    }
  ]

  // 简单模式下，在系统设置前插入 API密钥
  if (authStore.isSimpleMode) {
    const filtered = baseItems.filter(item => !item.hideInSimpleMode)
    filtered.push({ path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon })
    filtered.push(createModelPricingNavItem())
    filtered.push(createDocsNavItem())
    filtered.push({
      path: '/admin/settings',
      label: resolveAdminMenuLabel('/admin/settings', t('nav.settings')),
      icon: CogIcon
    })
    // Add admin custom menu items after settings
    for (const cm of customMenuItemsForAdmin.value) {
      filtered.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
    }
    return applyRBACFilter(filtered)
  }

  baseItems.push({
    path: '/admin/settings',
    label: resolveAdminMenuLabel('/admin/settings', t('nav.settings')),
    icon: CogIcon
  })
  // Add admin custom menu items after settings
  for (const cm of customMenuItemsForAdmin.value) {
    baseItems.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
  }
  return applyRBACFilter(baseItems)
})

function applyRBACFilter(items: NavItem[]): NavItem[] {
  if (permStore.isSuperAdmin) {
    return items
  }
  if (!permStore.loaded) {
    return items.filter(
      (item) => !item.path.startsWith('/admin') || item.path.startsWith('/custom')
    )
  }
  return items.filter((item) => {
    if (!item.path.startsWith('/admin') || item.path.startsWith('/custom')) {
      return true
    }
    const permKey = navPermissionMap[item.path]
    if (!permKey) {
      return true
    }
    return permStore.hasPermission(permKey)
  })
}

function toggleSidebar() {
  appStore.toggleSidebar()
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  setTheme(isDark.value)
}

function closeMobile() {
  appStore.setMobileOpen(false)
}

function getNavTourAttr(path: string): string | undefined {
  const pathToTour: Record<string, string> = {
    '/keys': 'sidebar-my-keys',
    '/get-subscription': 'sidebar-topup',
    '/models': 'sidebar-model-pricing',
    '/docs': 'sidebar-docs'
  }
  return pathToTour[path]
}

function handleMenuItemClick(itemPath: string) {
  if (mobileOpen.value) {
    setTimeout(() => {
      appStore.setMobileOpen(false)
    }, 150)
  }

  // Map paths to tour selectors
  const pathToSelector: Record<string, string> = {
    '/admin/groups': '#sidebar-group-manage',
    '/admin/accounts': '#sidebar-channel-manage',
    '/keys': '[data-tour="sidebar-my-keys"]',
    '/get-subscription': '[data-tour="sidebar-topup"]',
    '/docs': '[data-tour="sidebar-docs"]'
  }

  const selector = pathToSelector[itemPath]
  if (selector && onboardingStore.isCurrentStep(selector)) {
    onboardingStore.nextStep(500)
  }
}

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

// Initialize theme
isDark.value = isDarkTheme()
applyThemeClass(isDark.value)

// Fetch admin settings (for feature-gated nav items like Ops).
watch(
  isAdmin,
  (v) => {
    if (v) {
      adminSettingsStore.fetch()
      startAffiliateSummaryPolling()
    } else {
      stopAffiliateSummaryPolling()
    }
  },
  { immediate: true }
)

onMounted(() => {
  void refreshChangelogFreshness()
  if (!isAdmin.value) {
    void affiliateProgramStore.refresh().catch(() => {
      // 联盟入口失败关闭；接口不可用时不向用户展示尚未确认开放的计划。
    })
  }
  if (isAdmin.value) {
    adminSettingsStore.fetch()
  }
})

onBeforeUnmount(() => {
  stopAffiliateSummaryPolling()
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* Custom SVG icon in sidebar: inherit color, constrain size */
.sidebar-svg-icon :deep(svg) {
  width: 1.25rem;
  height: 1.25rem;
  stroke: currentColor;
  color: currentColor;
}

.sidebar-svg-icon :deep(svg[fill]:not([fill="none"])),
.sidebar-svg-icon :deep(svg:not([fill]):not([stroke])) {
  fill: currentColor;
}

.sidebar-svg-icon :deep(svg[fill="none"]) {
  fill: none;
}

.sidebar-svg-icon :deep(svg[stroke="none"]) {
  stroke: none;
}
</style>
