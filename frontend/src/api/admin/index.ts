/**
 * Admin API barrel export
 * Centralized exports for all admin API modules
 */

import dashboardAPI from './dashboard'
import usersAPI from './users'
import agentsAPI from './agents'
import groupsAPI from './groups'
import accountsAPI from './accounts'
import proxiesAPI from './proxies'
import redeemAPI from './redeem'
import promoAPI from './promo'
import announcementsAPI from './announcements'
import changelogAPI from './changelog'
import feedbacksAPI from './feedbacks'
import financeTransactionsAPI from './financeTransactions'
import settingsAPI from './settings'
import systemAPI from './system'
import subscriptionsAPI from './subscriptions'
import usageAPI from './usage'
import geminiAPI from './gemini'
import antigravityAPI from './antigravity'
import grokAPI from './grok'
import userAttributesAPI from './userAttributes'
import opsAPI from './ops'
import errorPassthroughAPI from './errorPassthrough'
import dataManagementAPI from './dataManagement'
import apiKeysAPI from './apiKeys'
import scheduledTestsAPI from './scheduledTests'
import backupAPI from './backup'
import channelsAPI from './channels'
import suppliersAPI from './suppliers'
import monthlyUpstreamsAPI from './monthlyUpstreams'
import costAccountingAPI from './costAccounting'
import tlsFingerprintProfilesAPI from './tlsFingerprintProfile'
import invoiceAPI from './invoice'
import rbacAPI from './rbac'

/**
 * Unified admin API object for convenient access
 */
export const adminAPI = {
  dashboard: dashboardAPI,
  agents: agentsAPI,
  users: usersAPI,
  groups: groupsAPI,
  accounts: accountsAPI,
  proxies: proxiesAPI,
  redeem: redeemAPI,
  promo: promoAPI,
  announcements: announcementsAPI,
  changelog: changelogAPI,
  feedbacks: feedbacksAPI,
  financeTransactions: financeTransactionsAPI,
  settings: settingsAPI,
  system: systemAPI,
  subscriptions: subscriptionsAPI,
  usage: usageAPI,
  gemini: geminiAPI,
  antigravity: antigravityAPI,
  grok: grokAPI,
  userAttributes: userAttributesAPI,
  ops: opsAPI,
  errorPassthrough: errorPassthroughAPI,
  dataManagement: dataManagementAPI,
  apiKeys: apiKeysAPI,
  scheduledTests: scheduledTestsAPI,
  backup: backupAPI,
  channels: channelsAPI,
  suppliers: suppliersAPI,
  monthlyUpstreams: monthlyUpstreamsAPI,
  costAccounting: costAccountingAPI,
  invoice: invoiceAPI,
  rbac: rbacAPI,
  tlsFingerprintProfiles: tlsFingerprintProfilesAPI
}

export {
  dashboardAPI,
  agentsAPI,
  usersAPI,
  groupsAPI,
  accountsAPI,
  proxiesAPI,
  redeemAPI,
  promoAPI,
  announcementsAPI,
  changelogAPI,
  feedbacksAPI,
  financeTransactionsAPI,
  settingsAPI,
  systemAPI,
  subscriptionsAPI,
  usageAPI,
  geminiAPI,
  antigravityAPI,
  grokAPI,
  userAttributesAPI,
  opsAPI,
  errorPassthroughAPI,
  dataManagementAPI,
  apiKeysAPI,
  scheduledTestsAPI,
  backupAPI,
  channelsAPI,
  suppliersAPI,
  monthlyUpstreamsAPI,
  invoiceAPI,
  rbacAPI,
  tlsFingerprintProfilesAPI
}

export default adminAPI

// Re-export types used by components
export type { BalanceHistoryItem } from './users'
export type { AdminAgentSummary, AgentSettlement, CommissionRates, InviteActivityConfig } from './agents'
export type { ErrorPassthroughRule, CreateRuleRequest, UpdateRuleRequest } from './errorPassthrough'
export type { BackupAgentHealth, DataManagementConfig } from './dataManagement'
export type { TLSFingerprintProfile, CreateProfileRequest, UpdateProfileRequest } from './tlsFingerprintProfile'
export type {
  Supplier,
  SupplierProbeSnapshot,
  SupplierHourlyStability,
  CreateSupplierRequest,
  UpdateSupplierRequest
} from './suppliers'
