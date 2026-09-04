<template>
  <BaseDialog :show="show" title="一键配置客户端" width="extra-wide" @close="$emit('close')">
    <div class="client-setup">
      <p class="setup-intro">使用「{{ keyName }}」已授权的分组。选一个工具，自动检测安装、填写配置并测试；不需要重新选择分组。</p>
      <div class="setup-os" role="group" aria-label="安装系统">
        <button v-for="item in systems" :key="item.id" type="button" :aria-pressed="os === item.id" @click="os = item.id">{{ item.name }}</button>
      </div>
      <p v-if="loading" role="status">正在读取分组权限与兼容模型…</p>
      <p v-if="error" role="alert" class="setup-warning">{{ error }} <button type="button" @click="load">重新读取</button></p>
      <div class="setup-tools" role="group" aria-label="选择客户端">
        <button v-for="client in clientMatrix" :key="client.id" type="button" :data-client="client.id" :aria-pressed="activeClient === client.id" @click="activeClient = client.id">
          <img :src="client.icon" alt="" /><strong>{{ client.name }}</strong>
          <small>{{ planFor(client.id)?.available ? `${planFor(client.id)!.models.length} 个兼容模型` : '查看接入条件' }}</small>
        </button>
      </div>
      <template v-if="plan && !loading && !error">
        <div class="setup-detail">
          <div><strong>{{ selectedName }}</strong><span>{{ plan.client_version_key }} · {{ plan.group_ids.length }} 个授权分组</span></div>
          <p v-if="!plan.available" class="setup-warning" role="status">{{ plan.reason }}<br />{{ clientLimitation }}</p>
          <template v-else>
            <p>缺少客户端时安装已核验版本；已安装且版本匹配时跳过。已有配置先备份，只修改本站管理的部分。</p>
            <p>默认模型：<code>{{ plan.default_model }}</code>。支持列表的工具导入兼容模型；固定模型槽位按工具规则配置。</p>
          </template>
          <details v-if="plan.models.length"><summary>{{ plan.models.length }} 个兼容模型</summary><ul><li v-for="model in plan.models" :key="model.id"><code>{{ model.id }}</code><span>{{ model.protocol }}</span></li></ul></details>
        </div>
        <p class="setup-footnote">同名模型按 Key 的分组顺序路由，按实际分组计费。测试会产生少量用量；不会自动关闭正在使用的客户端。</p>
        <button v-if="plan.available" type="button" class="btn btn-primary" :disabled="generating" @click="generate">{{ generating ? '正在生成…' : command ? '重新生成命令' : '生成一键配置命令' }}</button>
        <DocsTerminalCommand v-if="command || previewCommand" :command="preview ? '' : command" :display-command="preview ? previewCommand : command" :label="preview ? '界面预览 · 无真实票据，不可执行' : `${os === 'windows' ? 'PowerShell' : 'Terminal'} · 一次性命令`" />
        <p v-if="command" class="setup-footnote">命令有效期 {{ remainingSeconds }} 秒，仅可使用一次。像 Key 一样保管，不要分享；执行前会重新检查权限和模型，变化时请重新生成。只有测试通过才表示配置成功。</p>
      </template>
    </div>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DocsTerminalCommand from '@/components/docs/DocsTerminalCommand.vue'
import { clientMatrix } from '@/generated/clientMatrix'
import { createClientSetupTicketForPlan, getClientSetupPlans, type ClientSetupOS, type ClientSetupPlan } from '@/api/resources'
import { buildClientAutoConfigCommand } from '@/utils/clientAutoConfig'
const props = defineProps<{ show: boolean; apiKeyId: number; keyName: string; preview?: boolean; previewPlans?: ClientSetupPlan[] }>()
defineEmits<{ close: [] }>()
const systems: Array<{ id: ClientSetupOS; name: string }> = [{ id: 'macos', name: 'macOS' }, { id: 'linux', name: 'Linux' }, { id: 'windows', name: 'Windows' }]
const os = ref<ClientSetupOS>(typeof navigator !== 'undefined' && /windows/i.test(navigator.userAgent) ? 'windows' : typeof navigator !== 'undefined' && /linux/i.test(navigator.userAgent) ? 'linux' : 'macos')
const activeClient = ref('claude-code')
const plans = ref<ClientSetupPlan[]>([])
const loading = ref(false), generating = ref(false), error = ref(''), command = ref(''), previewCommand = ref(''), remainingSeconds = ref(0)
let epoch = 0, generation = 0
let expiryTimer: ReturnType<typeof setInterval> | undefined
const planFor = (id: string) => plans.value.find(plan => plan.client_id === id && plan.os === os.value)
const plan = computed(() => planFor(activeClient.value))
const selectedName = computed(() => clientMatrix.find(c => c.id === activeClient.value)?.name ?? '')
const clientLimitation = computed(() => activeClient.value === 'qoder' ? 'Qoder 还需要官方登录及 Custom URL 权限，不能跳过授权。' : activeClient.value === 'minimax-code' ? 'MiniMax Code 会把 Key 保存到本机；安全存储及自动验收适配尚未开放。' : '文字教程可用不代表该工具的全自动安装链路已经验收。')
function clearCommand() { generation++; command.value = ''; previewCommand.value = ''; generating.value = false; remainingSeconds.value = 0; if (expiryTimer) clearInterval(expiryTimer) }
async function load() {
  const request = ++epoch; clearCommand(); plans.value = []; error.value = ''
  if (!props.show || props.apiKeyId <= 0) { loading.value = false; return }
  loading.value = true
  try {
    const result = props.preview ? (props.previewPlans ?? []).map(p => ({ ...p, os: os.value })) : await getClientSetupPlans(props.apiKeyId, os.value)
    if (request !== epoch) return
    plans.value = result
  } catch { if (request === epoch) error.value = '无法读取当前 Key 的配置计划，请确认 Key 和分组仍有效后重试。' }
  finally { if (request === epoch) loading.value = false }
}
async function generate() {
  const selected = plan.value
  if (!selected?.available || !selected.target || loading.value || generating.value) return
  clearCommand(); const request = generation; generating.value = true; error.value = ''
  if (props.preview) { previewCommand.value = '# 正式环境将在这里生成校验安装器完整性的一次性命令\n# 检测安装 → 导入兼容模型 → 验证配置与调用'; generating.value = false; return }
  try {
    const ticket = await createClientSetupTicketForPlan(props.apiKeyId, selected)
    if (request !== generation) return
    if (ticket.target !== selected.target) throw new Error('target mismatch')
    command.value = buildClientAutoConfigCommand({ target: selected.target, ticket: ticket.ticket, isWindows: os.value === 'windows', installMissing: true })
    const expiresAt = Date.now() + ticket.expires_in * 1000
    remainingSeconds.value = ticket.expires_in
    expiryTimer = setInterval(() => { remainingSeconds.value = Math.max(0, Math.ceil((expiresAt - Date.now()) / 1000)); if (!remainingSeconds.value) clearCommand() }, 1000)
  } catch { if (request === generation) error.value = '配置计划已变化或命令生成失败，请重新读取后再试。' }
  finally { if (request === generation) generating.value = false }
}
watch([() => props.show, () => props.apiKeyId, os], load, { immediate: true })
watch(activeClient, clearCommand)
onBeforeUnmount(() => { epoch++; clearCommand() })
</script>
<style scoped>
.client-setup{display:grid;gap:18px;color:rgb(var(--color-ink));font-size:13px;line-height:1.7}.setup-intro{margin:0;color:rgb(var(--color-muted))}.setup-os{display:flex;gap:5px;padding:5px;border-radius:10px;background:rgb(var(--color-parchment));width:fit-content}.setup-os button{padding:7px 18px;border-radius:7px}.setup-os button[aria-pressed=true]{background:rgb(var(--color-vellum));color:rgb(var(--color-terracotta))}.setup-tools{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:9px}.setup-tools button{display:grid;grid-template-columns:28px 1fr;gap:3px 9px;align-items:center;text-align:left;padding:13px 10px;border:1px solid rgb(var(--color-muted)/.24);border-radius:12px;background:rgb(var(--color-vellum))}.setup-tools img{grid-row:span 2;width:24px;height:24px;object-fit:contain}.setup-tools strong{font-size:12px}.setup-tools small{font-size:10px;color:rgb(var(--color-muted))}.setup-tools button[aria-pressed=true]{border-color:rgb(var(--color-terracotta));background:rgb(var(--color-terracotta)/.06)}.setup-detail{padding:17px;border:1px solid rgb(var(--color-muted)/.22);border-radius:12px}.setup-detail>div{display:flex;justify-content:space-between;gap:8px;flex-wrap:wrap}.setup-detail span,.setup-footnote{color:rgb(var(--color-muted));font-size:11px}.setup-detail p{margin-top:10px}.setup-detail ul{max-height:150px;overflow:auto;margin-top:8px}.setup-detail li{display:flex;justify-content:space-between;gap:10px}.setup-warning{background:rgb(var(--color-parchment));padding:12px;border-left:2px solid rgb(var(--color-terracotta));border-radius:5px}.setup-footnote{margin:0}.client-setup>.btn{justify-self:start}.client-setup :deep(pre code){width:auto;white-space:pre-wrap;overflow-wrap:anywhere}@media(max-width:650px){.setup-tools{grid-template-columns:repeat(2,minmax(0,1fr))}.setup-tools button{padding:11px 8px}.setup-os button{padding:7px 14px}}
</style>
