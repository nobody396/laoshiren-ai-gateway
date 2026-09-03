<template>
  <section class="manual-config">
    <header>
      <small>MANUAL CONFIGURATION</small>
      <h2>手动配置</h2>
    </header>

    <ol>
      <li>
        <b>01</b>
        <div class="step-body">
          <h3>配置 Base URL 和 Key</h3>
          <div class="field-grid">
            <label><span>Base URL</span><input v-model.trim="baseUrl" type="url" autocomplete="url" /></label>
            <label><span>API Key</span><input v-model="apiKey" type="password" autocomplete="off" placeholder="在这里粘贴 API Key" /></label>
          </div>
          <p>点击运行时，Key 只发送到上面的 Base URL 用于读取模型；本站不会保存。手动命令会包含你的 Key。</p>
        </div>
      </li>

      <li>
        <b>02</b>
        <div class="step-body">
          <h3>读取当前分组模型</h3>
          <p>点击运行，直接读取这把 Key 可以使用的模型；也可以复制命令到终端执行。</p>
          <DocsTerminalCommand label="读取模型" :command="modelListCommand" :display-command="modelListDisplayCommand" empty-text="先填写 API Key" runnable :running="modelLoading" @run="runModelDiscovery" />
          <p v-if="modelError" class="model-error">{{ modelError }}</p>
          <div v-if="discoveredModels.length" class="model-results">
            <header><b>当前分组模型</b><span>{{ discoveredModels.length }} 个</span></header>
            <div v-for="model in discoveredModels" :key="model" class="model-result-row" :class="{ selected: mainModel === model }">
              <span class="model-name"><code>{{ model }}</code><button type="button" class="copy-model" :aria-label="`复制 ${model}`" :title="`复制 ${model}`" @click="copyModel(model)"><svg v-if="copiedModel !== model" viewBox="0 0 24 24" aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M15 9V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h3"/></svg><svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6"/></svg></button></span>
              <button type="button" class="set-main-button" @click="mainModel = model">{{ mainModel === model ? '已选择' : '设为主模型' }}</button>
            </div>
          </div>
        </div>
      </li>

      <li>
        <b>03</b>
        <div class="step-body">
          <h3>选择主模型</h3>
          <label class="model-field"><span>主模型 ID</span><input v-model.trim="mainModel" placeholder="从上一步返回结果中复制" /><small>主模型是 Claude Code 每次新建会话时默认使用的模型。</small></label>
          <DocsReasoningSlider
            v-model="effortLevel"
            :options="effortOptions"
            :mapping-text="effectiveEffortNote"
            :support-text="reasoningProfile ? `该模型可用：${availableReasoningLabel}` : ''"
            :warning-text="!reasoningProfile && mainModel.trim() ? '推理矩阵中没有这个模型，已使用自动，避免写入未验证档位。' : ''"
          />
          <p class="context-note">上下文窗口和自动压缩由 Claude Code 根据模型处理，不需要填写。</p>
        </div>
      </li>

      <li>
        <b>04</b>
        <div class="step-body">
          <h3>填充模型槽位</h3>
          <div class="slot-grid">
            <label><span>Opus</span><input v-model.trim="opusModel" placeholder="留空则使用主模型" /></label>
            <label><span>Sonnet</span><input v-model.trim="sonnetModel" placeholder="留空则使用主模型" /></label>
            <label><span>Haiku</span><input v-model.trim="haikuModel" placeholder="留空则使用主模型" /></label>
            <label><span>Fable</span><input v-model.trim="fableModel" placeholder="留空则使用主模型" /></label>
          </div>
        </div>
      </li>

      <li>
        <b>05</b>
        <div class="step-body">
          <h3>写入配置并测试</h3>
          <div class="platform-tabs" role="tablist" aria-label="操作系统">
            <button v-for="item in platforms" :key="item.id" type="button" :class="{ active: platform === item.id }" @click="platform = item.id">{{ item.label }}</button>
          </div>
          <h4>写入配置</h4>
          <DocsTerminalCommand :label="`${platformLabel} · 写入配置`" :command="settingsCommand" :display-command="settingsDisplayCommand" empty-text="填写 API Key 和主模型后生成" />
          <h4>验证配置</h4>
          <DocsTerminalCommand label="验证" :command="verificationCommand" />
          <p>返回 <code>CLAUDE_CODE_OK</code> 即配置成功。</p>
        </div>
      </li>
    </ol>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import DocsTerminalCommand from './DocsTerminalCommand.vue'
import DocsReasoningSlider from './DocsReasoningSlider.vue'
import { clientMatrixBySlug, modelReasoningProfileById } from '@/generated/clientMatrix'
import { getGatewayModels } from '@/api/gatewayModels'
import {
  buildClaudeModelListCommand,
  buildClaudeSettingsCommand,
  buildClaudeVerificationCommand,
  type ClaudeEffortChoice,
  type ClaudeManualPlatform,
} from '@/utils/claudeCodeManualConfig'

const baseUrl = ref('https://api.laoshirenai.com')
const apiKey = ref('')
const mainModel = ref('')
const effortLevel = ref<ClaudeEffortChoice>('high')
const opusModel = ref('')
const sonnetModel = ref('')
const haikuModel = ref('')
const fableModel = ref('')
const platform = ref<ClaudeManualPlatform>('macos')
const modelLoading = ref(false)
const modelError = ref('')
const discoveredModels = ref<string[]>([])
const copiedModel = ref('')

const platforms: Array<{ id: ClaudeManualPlatform; label: string }> = [
  { id: 'windows', label: 'Windows' },
  { id: 'macos', label: 'macOS' },
  { id: 'linux', label: 'Linux' },
]
const effortOptionCatalog: Array<{ id: ClaudeEffortChoice; label: string; shortLabel: string; mode: string; description: string }> = [
  { id: 'auto', label: '自动', shortLabel: '自动', mode: '默认规则', description: '不为当前模型单独固定档位，沿用 Claude Code 当前的默认规则。' },
  { id: 'low', label: 'Low', shortLabel: 'Low', mode: '按模型保存', description: '更快、更省 Token，适合简单和范围明确的任务。' },
  { id: 'medium', label: 'Medium', shortLabel: 'Med', mode: '按模型保存', description: '降低推理消耗，适合成本敏感的日常任务。' },
  { id: 'high', label: 'High', shortLabel: 'High', mode: '按模型保存', description: '能力与消耗更均衡，是大多数支持模型的默认档位。' },
  { id: 'xhigh', label: 'XHigh', shortLabel: 'XHigh', mode: '按模型保存', description: '更深推理，适合复杂编码和架构任务。' },
  { id: 'max', label: 'Max', shortLabel: 'Max', mode: '单次会话', description: '模型最深推理档位；Claude Code 不把 Max 保存为长期默认。' },
  { id: 'ultracode', label: 'Ultracode', shortLabel: 'Ultra', mode: '单次会话', description: 'Claude Code 工作流模式：发送 XHigh，并为复杂任务编排动态工作流。' },
]
const claudeClientReasoning = clientMatrixBySlug['integration-claude-code'].reasoning
const claudeClientControls = new Set([
  ...(claudeClientReasoning.modes ?? []),
  ...claudeClientReasoning.levels,
])
const allEffortOptions = effortOptionCatalog.filter(item => claudeClientControls.has(item.id))
const reasoningRank = ['low', 'medium', 'high', 'xhigh', 'max']

const reasoningProfile = computed(() => modelReasoningProfileById[mainModel.value.trim()])
const modelReasoningLevels = computed(() => {
  const profile = reasoningProfile.value
  if (!profile) return []
  return profile.client_levels.find(item => item.client === 'Claude Code')?.levels ?? profile.model_levels
})
const effortOptions = computed(() => {
  if (!mainModel.value.trim()) return allEffortOptions
  if (!reasoningProfile.value) return allEffortOptions.filter(item => item.id === 'auto')
  const levels = new Set(modelReasoningLevels.value)
  if (levels.size === 0) return allEffortOptions.filter(item => item.id === 'auto')
  return allEffortOptions.filter(item => item.id === 'auto' || Boolean(resolveEffectiveEffort(item.id)))
})

function resolveEffectiveEffort(choice: ClaudeEffortChoice): string | null {
  if (choice === 'auto') return null
  const requested = choice === 'ultracode' ? 'xhigh' : choice
  const levels = modelReasoningLevels.value
  if (levels.includes(requested)) return requested
  const explicit = reasoningProfile.value?.client_mappings.find(mapping =>
    mapping.client === 'Claude Code' && mapping.from === requested)
  if (explicit && levels.includes(explicit.to)) return explicit.to
  if (claudeClientReasoning.unsupported_strategy !== 'floor') return null
  const requestedRank = reasoningRank.indexOf(requested)
  if (requestedRank < 0) return null
  return [...levels]
    .filter(level => reasoningRank.includes(level) && reasoningRank.indexOf(level) <= requestedRank)
    .sort((left, right) => reasoningRank.indexOf(right) - reasoningRank.indexOf(left))[0] ?? null
}

const availableReasoningLabel = computed(() => modelReasoningLevels.value.length > 0
  ? modelReasoningLevels.value.join(' / ')
  : '不支持显式推理强度')
const effectiveEffortNote = computed(() => {
  if (!reasoningProfile.value || effortLevel.value === 'auto') return ''
  const effective = resolveEffectiveEffort(effortLevel.value)
  const requested = effortLevel.value === 'ultracode' ? 'xhigh' : effortLevel.value
  return effective && effective !== requested ? `当前模型会把 ${requested} 降级为 ${effective}。` : ''
})

watch(effortOptions, (options) => {
  if (options.some(item => item.id === effortLevel.value)) return
  effortLevel.value = options.some(item => item.id === 'high') ? 'high' : 'auto'
})

const modelListCommand = computed(() => apiKey.value
  ? buildClaudeModelListCommand(baseUrl.value, apiKey.value, platform.value)
  : '')

const settingsCommand = computed(() => apiKey.value && mainModel.value
  ? buildClaudeSettingsCommand({
      baseUrl: baseUrl.value,
      apiKey: apiKey.value,
      mainModel: mainModel.value,
      effortLevel: effortLevel.value,
      opusModel: opusModel.value,
      sonnetModel: sonnetModel.value,
      haikuModel: haikuModel.value,
      fableModel: fableModel.value,
    }, platform.value)
  : '')

const platformLabel = computed(() => platforms.find(item => item.id === platform.value)?.label ?? '')
const verificationCommand = computed(() => buildClaudeVerificationCommand(effortLevel.value))

function maskSecret(command: string): string {
  let value = command
  if (apiKey.value) value = value.split(apiKey.value).join('••••••••')
  value = value.replace(/CLAUDE_CFG_B64='[^']+'/g, "CLAUDE_CFG_B64='••••••••'")
  value = value.replace(/FromBase64String\('[^']+'\)/g, "FromBase64String('••••••••')")
  return value
}

const modelListDisplayCommand = computed(() => {
  const command = maskSecret(modelListCommand.value)
  if (platform.value === 'windows') return command.replace(/ -Headers /g, '`\n  -Headers ')
  return command
    .replace(/ -H /g, ' \\\n  -H ')
    .replace(/ \| python3 /g, ' \\\n  | python3 ')
})

const settingsDisplayCommand = computed(() => {
  const command = maskSecret(settingsCommand.value)
  if (platform.value === 'windows') return command.replace(/; /g, ';\n')
  return command
    .replace(/ python3 -c '/, " \\\n  python3 -c '\n  ")
    .replace(/;/g, ';\n  ')
})

async function runModelDiscovery(): Promise<void> {
  if (!apiKey.value) return
  modelLoading.value = true
  modelError.value = ''
  discoveredModels.value = []
  try {
    const isLocalPreview = ['127.0.0.1', 'localhost'].includes(window.location.hostname)
    const requestBaseUrl = isLocalPreview && baseUrl.value.replace(/\/+$/, '') === 'https://api.laoshirenai.com'
      ? '/__gateway'
      : baseUrl.value
    discoveredModels.value = await getGatewayModels(requestBaseUrl, apiKey.value)
    if (discoveredModels.value.length === 1) mainModel.value = discoveredModels.value[0]
  } catch (error) {
    modelError.value = error instanceof Error ? error.message : '读取模型失败'
  } finally {
    modelLoading.value = false
  }
}

async function copyModel(model: string): Promise<void> {
  await navigator.clipboard.writeText(model)
  copiedModel.value = model
  window.setTimeout(() => { if (copiedModel.value === model) copiedModel.value = '' }, 1400)
}

watch(apiKey, () => {
  discoveredModels.value = []
  modelError.value = ''
})

</script>

<style scoped>
.manual-config{margin-top:2rem;min-width:0}.manual-config header small{color:#1267d6;font:700 .62rem ui-monospace,monospace;letter-spacing:.12em}.manual-config header h2{margin:.35rem 0 1rem;font-size:1.2rem}.manual-config ol{min-width:0;list-style:none;margin:0;padding:0;border:1px solid #e4e4e7;border-radius:.9rem;overflow:hidden}.manual-config li{min-width:0;display:grid;grid-template-columns:2.5rem minmax(0,1fr);gap:.7rem;padding:1rem;border-bottom:1px solid #eee}.manual-config li:last-child{border-bottom:0}.manual-config li>b{color:#1267d6;font:700 .7rem ui-monospace,monospace}.step-body{min-width:0}.step-body h3{margin:0;color:#27272a;font-size:.9rem}.step-body h4{margin:1rem 0 .45rem;color:#3f3f46;font-size:.72rem}.step-body p{margin:.5rem 0 0;color:#52525b;font-size:.72rem;line-height:1.6}.field-grid,.slot-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:.65rem;margin-top:.75rem}.single-field{display:block;margin-top:.75rem;max-width:28rem}label span{display:block;margin-bottom:.35rem;color:#3f3f46;font-size:.68rem;font-weight:700}label small,.effort-field small{display:block;margin-top:.5rem;color:#71717a;font-size:.66rem;line-height:1.55}.effort-field .effort-support{color:#1267d6;font-weight:600}.effort-field .effort-mapping{color:#b45309;font-weight:600}.effort-field .effort-warning{color:#fbbf24;font-weight:600}.model-field{display:block;margin-top:.75rem;max-width:32rem}input{width:100%;box-sizing:border-box;border:1px solid #d6d3d1;border-radius:.75rem;background:#fff;padding:.68rem .78rem;color:#27272a;font:400 .72rem ui-monospace,monospace;outline:none}input:focus{border-color:#1267d6;box-shadow:0 0 0 3px #1267d619}.effort-field{max-width:38rem;margin-top:1.15rem;border:1px solid #e7e5e4;border-radius:1rem;background:linear-gradient(145deg,#fff,#fafafa);padding:1rem;box-shadow:0 8px 24px #18181b0a}.effort-heading{display:flex;align-items:center;justify-content:space-between;gap:1rem}.effort-heading>span{color:#3f3f46;font-size:.7rem;font-weight:700}.effort-heading>div{display:flex;align-items:center;gap:.45rem}.effort-heading strong{color:#1267d6;font-size:.76rem}.effort-heading em{border-radius:999px;background:#eff6ff;color:#1d4ed8;padding:.24rem .48rem;font-size:.58rem;font-style:normal;font-weight:700}.effort-slider{margin-top:.8rem;padding:.1rem .15rem}.effort-slider>input{width:100%;height:1.3rem;margin:0;appearance:none;border:0;background:transparent;padding:0;box-shadow:none;cursor:pointer}.effort-slider>input::-webkit-slider-runnable-track{height:.35rem;border-radius:999px;background:linear-gradient(90deg,#1267d6 0 var(--effort-progress),#e5e7eb var(--effort-progress) 100%)}.effort-slider>input::-webkit-slider-thumb{width:1.18rem;height:1.18rem;margin-top:-.42rem;appearance:none;border:3px solid #fff;border-radius:999px;background:#1267d6;box-shadow:0 1px 6px #18181b38,0 0 0 1px #1267d6}.effort-slider>input::-moz-range-track{height:.35rem;border-radius:999px;background:#e5e7eb}.effort-slider>input::-moz-range-progress{height:.35rem;border-radius:999px;background:#1267d6}.effort-slider>input::-moz-range-thumb{width:1rem;height:1rem;border:3px solid #fff;border-radius:999px;background:#1267d6;box-shadow:0 1px 6px #18181b38}.effort-labels{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));margin-top:.35rem}.effort-labels button{border:0;background:transparent;color:#a1a1aa;padding:.2rem 0;font-size:.58rem;font-weight:700;cursor:pointer}.effort-labels button.active{color:#1267d6}.context-note{max-width:38rem;margin-top:.85rem!important;border-left:2px solid #d6d3d1;padding-left:.65rem;color:#71717a!important}.platform-tabs{display:flex;gap:.35rem;margin-top:.75rem}.platform-tabs button{border:1px solid #d6d3d1;border-radius:.65rem;background:#fff;color:#3f3f46;padding:.48rem .75rem;font-size:.68rem;font-weight:700;cursor:pointer}.platform-tabs button.active{border-color:#1267d6;background:#1267d6;color:#fff}
.effort-field{border-color:#3f3f46;background:linear-gradient(145deg,#303033,#262629);box-shadow:0 14px 32px #18181b24}.effort-heading>span{color:#f4f4f5}.effort-heading strong{color:#60a5fa}.effort-heading em{background:#3f3f46;color:#d4d4d8}.effort-slider{padding:0}.effort-range{position:relative;height:2.5rem;margin-top:.8rem}.effort-rail{position:absolute;inset:0;border-radius:999px;background:linear-gradient(90deg,#3699f5 0 var(--effort-progress),#48484a var(--effort-progress) 100%);box-shadow:inset 0 0 0 1px #ffffff0d}.effort-dots{position:absolute;inset:0 1.2rem;display:grid;grid-template-columns:repeat(7,minmax(0,1fr));align-items:center}.effort-dots i{justify-self:center;width:.38rem;height:.38rem;border-radius:999px;background:#777779}.effort-dots i.passed{background:#8bc5ff}.effort-range>input{position:absolute;inset:0;width:100%;height:100%;box-sizing:border-box;margin:0;appearance:none;border:0;background:transparent;padding:0;box-shadow:none;cursor:pointer}.effort-range>input::-webkit-slider-runnable-track{height:2.5rem;border-radius:999px;background:transparent}.effort-range>input::-webkit-slider-thumb{width:2.5rem;height:2.5rem;margin-top:0;appearance:none;border:0;border-radius:999px;background:#fafafa;box-shadow:0 2px 9px #0006,0 0 0 1px #fff}.effort-range>input::-moz-range-track{height:2.5rem;border-radius:999px;background:transparent}.effort-range>input::-moz-range-progress{background:transparent}.effort-range>input::-moz-range-thumb{width:2.35rem;height:2.35rem;border:0;border-radius:999px;background:#fafafa;box-shadow:0 2px 9px #0006}.effort-labels{margin-top:.45rem}.effort-labels button{color:#8e8e93}.effort-labels button:hover{color:#d4d4d8}.effort-labels button.active{color:#f4f4f5}.effort-field small{color:#c4c4c7}.effort-field .effort-support{color:#8bc5ff}.context-note{border-left-color:#d6d3d1}
.effort-field{max-width:28rem;border-color:#e4e4e7;border-radius:.85rem;background:#fafafa;padding:.72rem .8rem;box-shadow:none}.effort-heading>span{color:#3f3f46;font-size:.66rem}.effort-heading strong{color:#1267d6;font-size:.7rem}.effort-heading em{background:#eaf3ff;color:#1267d6;padding:.2rem .42rem;font-size:.54rem}.effort-range{height:1.7rem;margin-top:.62rem}.effort-rail{background:linear-gradient(90deg,#3699f5 0 var(--effort-progress),#e5e7eb var(--effort-progress) 100%);box-shadow:inset 0 0 0 1px #d4d4d8}.effort-dots{inset:0 .82rem}.effort-dots i{width:.28rem;height:.28rem;background:#b6b8bd}.effort-dots i.passed{background:#c7e3ff}.effort-range>input::-webkit-slider-runnable-track{height:1.7rem}.effort-range>input::-webkit-slider-thumb{width:1.7rem;height:1.7rem;border:1px solid #d4d4d8;background:#fff;box-shadow:0 1px 5px #18181b30}.effort-range>input::-moz-range-track{height:1.7rem}.effort-range>input::-moz-range-thumb{width:1.58rem;height:1.58rem;border:1px solid #d4d4d8;background:#fff;box-shadow:0 1px 5px #18181b30}.effort-labels{margin-top:.3rem}.effort-labels button{color:#a1a1aa;padding:.14rem 0;font-size:.52rem}.effort-labels button:hover{color:#52525b}.effort-labels button.active{color:#1267d6}.effort-field small{margin-top:.35rem;color:#71717a;font-size:.61rem}.effort-field .effort-support{color:#1267d6}.effort-field .effort-warning{color:#b45309}.context-note{max-width:28rem;margin-top:.65rem!important;font-size:.65rem!important}
.model-error{border-radius:.6rem;background:#fef2f2;color:#b91c1c!important;padding:.6rem .7rem}.model-results{margin-top:.7rem;overflow:hidden;border:1px solid #e4e4e7;border-radius:.7rem;background:#fff}.model-results header{display:flex;align-items:center;justify-content:space-between;background:#fafafa;padding:.55rem .7rem}.model-results header b{font-size:.68rem}.model-results header span{color:#71717a;font-size:.62rem}.model-result-row{display:flex;align-items:center;justify-content:space-between;gap:.7rem;border-top:1px solid #f1f1f3;background:#fff;padding:.5rem .7rem}.model-result-row:hover{background:#f8fbff}.model-result-row.selected{background:#eff6ff}.model-name{min-width:0;display:flex;align-items:center;gap:.4rem}.model-name code{overflow:hidden;color:#27272a;font-size:.68rem;text-overflow:ellipsis;white-space:nowrap}.copy-model{width:1.65rem;height:1.65rem;display:inline-flex;shrink:0;align-items:center;justify-content:center;border:1px solid #e4e4e7;border-radius:.45rem;background:#fff;color:#71717a;cursor:pointer}.copy-model:hover{border-color:#bfdbfe;background:#eff6ff;color:#1267d6}.copy-model svg{width:.85rem;height:.85rem;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}.set-main-button{shrink:0;border:0;background:transparent;color:#1267d6;font-size:.6rem;font-weight:700;cursor:pointer}.set-main-button:hover{text-decoration:underline}
@media(max-width:700px){.field-grid,.slot-grid{grid-template-columns:1fr}.manual-config li{grid-template-columns:2rem 1fr}}
</style>
