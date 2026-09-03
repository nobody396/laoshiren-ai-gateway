<template>
  <section class="manual-config" :data-client="client.slug">
    <header>
      <small>MANUAL CONFIGURATION</small>
      <h2>手动配置</h2>
    </header>

    <div v-if="client.protocols.length === 0" class="no-protocol-manual">
      <strong>当前版本没有可公开接入的协议</strong>
      <p>{{ client.name }} {{ client.version }} 的候选协议尚未通过公共网关 Agent 闭环终态验证，或已确认不支持。因此本页不显示 Key、模型槽位或写入命令，避免制造看似可用的假配置。</p>
      <p>下面的配置合同只用于说明已知文件与边界；客户端后续版本通过验证后，才会自动恢复五步配置。</p>
    </div>

    <ol v-else>
      <li>
        <b>01</b>
        <div class="step-body">
          <h3>配置 Base URL 和 Key</h3>
          <div class="field-grid">
            <label><span>Base URL</span><input v-model.trim="baseUrl" type="url" autocomplete="url" /></label>
            <label><span>API Key</span><input v-model="apiKey" type="password" autocomplete="off" placeholder="在这里粘贴 API Key" /></label>
          </div>
          <p v-if="manualTarget">Key 只在当前浏览器中用于读取模型和生成手动写入命令，本站不会保存。复制的手动命令会包含你的 Key，请只在自己的终端执行。</p>
          <p v-else>Key 只在当前浏览器中用于读取模型，本站不会保存。当前客户端未通过安全写入验收，所以本页不会生成包含 Key 的配置命令。</p>
        </div>
      </li>

      <li>
        <b>02</b>
        <div class="step-body">
          <h3>读取当前分组模型</h3>
          <p>点击运行，直接读取这把 Key 可以使用的模型；也可以复制命令到终端执行。</p>
          <div class="platform-tabs" role="tablist" aria-label="读取模型命令的操作系统">
            <button v-for="item in platforms" :key="item.id" type="button" :class="{ active: platform === item.id }" @click="platform = item.id">{{ item.label }}</button>
          </div>
          <DocsTerminalCommand label="读取模型" :command="modelListCommand" :display-command="modelListDisplayCommand" empty-text="先填写 API Key" runnable :running="modelLoading" @run="runModelDiscovery" />
          <p v-if="modelError" class="model-error">{{ modelError }}</p>
          <div v-if="discoveredModels.length" class="model-results">
            <header><b>当前分组模型</b><span>{{ discoveredModels.length }} 个</span></header>
            <div v-for="model in discoveredModels" :key="model" class="model-result-row" :class="{ selected: mainModel === model }">
              <span class="model-name"><code>{{ model }}</code><button type="button" class="copy-model" :aria-label="`复制 ${model}`" @click="copyModel(model)"><svg v-if="copiedModel !== model" viewBox="0 0 24 24" aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M15 9V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h3"/></svg><svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6"/></svg></button></span>
              <button type="button" class="set-main-button" @click="mainModel = model">{{ mainModel === model ? '已选择' : '设为主模型' }}</button>
            </div>
          </div>
        </div>
      </li>

      <li>
        <b>03</b>
        <div class="step-body">
          <h3>选择主模型</h3>
          <label class="model-field"><span>主模型 ID</span><input v-model.trim="mainModel" placeholder="从上一步返回结果中选择" /><small>主模型是 {{ client.name }} 每次新建会话时默认使用的模型。</small></label>
          <div v-if="mainModel" class="protocol-choice">
            <span>配置协议</span>
            <div v-if="compatibleProtocols.length" class="protocol-options">
              <button v-for="protocol in compatibleProtocols" :key="protocol" type="button" :class="{ active: selectedProtocol === protocol }" @click="selectedProtocol = protocol">{{ protocolLabel(protocol) }}</button>
            </div>
            <small v-if="compatibleProtocols.length === 1">当前模型与 {{ client.name }} 只有这一种共同协议。</small>
            <small v-else-if="!compatibleProtocols.length">当前模型与客户端没有共同的已验证协议，不能生成配置。</small>
          </div>
          <DocsReasoningSlider
            v-if="reasoningControlAvailable"
            v-model="selectedReasoning"
            :options="reasoningSliderOptions"
            :mapping-text="isCodex ? 'Ultracode 是 Claude Code 的单次工作流模式，不是 Codex 推理档位。' : ''"
            :support-text="mainModel && reasoningOptions.length ? `该模型可用：${reasoningOptions.map(effortLabel).join(' / ')}` : ''"
            :warning-text="mainModel && !reasoningOptions.length ? '当前模型与客户端没有已验证的显式推理档位，保持客户端默认值。' : ''"
          />
          <div v-else class="reasoning-unavailable">
            <strong>推理强度由模型与客户端决定</strong>
            <p>当前版本没有已验证的持久化字段或命令参数，本页不会显示一个实际无法写入的滑块。</p>
          </div>
        </div>
      </li>

      <li>
        <b>04</b>
        <div class="step-body">
          <h3>填充模型槽位</h3>
          <div v-if="isCodex" class="codex-slot-layout">
            <label>
              <span>review_model</span>
              <input v-model.trim="reviewModel" placeholder="留空则使用主模型" />
              <small>可选的代码审查模型，只用于 Codex 的 <code>/review</code> 和 <code>codex review</code>；留空时与主模型相同。</small>
            </label>
            <article class="catalog-entry-card">
              <span>模型目录条目</span>
              <strong>{{ mainModel || '选择主模型后自动生成' }}</strong>
              <small>不是需要手填的参数。写入命令会生成 <code>laoshirenai-model-catalog.json</code>，告诉 Codex 这个模型的上下文、自动压缩阈值和可用推理档位。</small>
            </article>
          </div>
          <div v-else-if="isGrok" class="generated-slot-card">
            <strong>{{ mainModel || '选择主模型后自动生成' }}</strong>
            <p><code>[models].default</code> 会写入主模型；命令还会自动生成对应的 <code>[model."模型ID"]</code> Provider 条目，并填入 Base URL、Key 和 {{ protocolLabel(selectedProtocol) }} 协议。</p>
          </div>
          <div v-else-if="isGemini" class="generated-slot-card">
            <strong>{{ mainModel || '选择主模型后自动生成' }}</strong>
            <p><code>GEMINI_MODEL</code> 与 <code>settings.model.name</code> 都会使用主模型；命令会同时安全合并 <code>~/.gemini/.env</code> 和 <code>settings.json</code>。</p>
          </div>
          <div v-else-if="client.config_contract.model_slots.length" class="slot-grid">
            <label v-for="slot in client.config_contract.model_slots" :key="slot"><span>{{ slot }}</span><input v-model.trim="slotValues[slot]" :placeholder="slot.toLowerCase() === 'model' ? '使用主模型' : '留空则按客户端默认规则'" /><small>{{ slotExplanation(slot) }}</small></label>
          </div>
          <p v-else>这个客户端没有需要单独填写的模型槽位。</p>
        </div>
      </li>

      <li>
        <b>05</b>
        <div class="step-body">
          <h3>写入配置并测试</h3>
          <template v-if="manualTarget">
            <div class="platform-tabs" role="tablist" aria-label="写入配置命令的操作系统">
              <button v-for="item in platforms" :key="item.id" type="button" :class="{ active: platform === item.id }" @click="platform = item.id">{{ item.label }}</button>
            </div>
            <h4>写入配置</h4>
            <DocsTerminalCommand :label="`${platformLabel} · 写入配置`" :command="settingsCommand" :display-command="settingsDisplayCommand" empty-text="填写 API Key 和主模型后生成" />
            <h4>验证配置</h4>
            <DocsTerminalCommand :label="`${platformLabel} · ${client.name} 验证`" :command="clientVerificationCommand" />
            <p>先输出客户端版本，再发起一次真实请求；输出中出现 <code>{{ verificationMarker }}</code> 才表示 Base URL、Key、模型、协议和推理配置都已生效。</p>
          </template>
          <template v-else>
            <p v-if="client.one_click_status === 'ready'">当前客户端的一键配置入口位于 API 密钥页面；本页暂未生成手动写入命令。</p>
            <p v-else>当前没有通过安全写入验收的一键命令。请按本页列出的配置文件、字段和最小变更规则手动填写，不要执行占位命令。</p>
            <a v-if="client.one_click_status === 'ready'" class="key-link" href="https://laoshirenai.com/keys" target="_blank" rel="noreferrer">打开 API 密钥页面 ↗</a>
          </template>
          <template v-if="!manualTarget && verificationCommand">
            <h4>检查客户端版本</h4>
            <DocsTerminalCommand :label="`${platformLabel} · 版本检查`" :command="verificationCommand" />
          </template>
          <p><strong>最终验收：</strong>{{ client.config_contract.verification }}</p>
        </div>
      </li>
    </ol>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { ClientMatrixEntry } from '@/generated/clientMatrix'
import { modelReasoningProfileById } from '@/generated/clientMatrix'
import { modelDocContractById } from '@/generated/modelDocContracts'
import { getGatewayModels } from '@/api/gatewayModels'
import { buildClaudeModelListCommand, type ClaudeManualPlatform } from '@/utils/claudeCodeManualConfig'
import { buildCodexSettingsCommand, buildCodexVerificationCommand } from '@/utils/codexManualConfig'
import { buildClientManualConfigCommand, type ClientAutoConfigTarget } from '@/utils/clientAutoConfig'
import DocsTerminalCommand from './DocsTerminalCommand.vue'
import DocsReasoningSlider, { type DocsReasoningOption } from './DocsReasoningSlider.vue'

const props = defineProps<{ client: ClientMatrixEntry }>()
const baseUrl = ref('https://api.laoshirenai.com')
const apiKey = ref('')
const mainModel = ref('')
const platform = ref<ClaudeManualPlatform>('macos')
const modelLoading = ref(false)
const modelError = ref('')
const discoveredModels = ref<string[]>([])
const copiedModel = ref('')
const selectedReasoning = ref('')
const selectedProtocol = ref('')
const slotValues = reactive<Record<string, string>>({})
const isCodex = computed(() => props.client.id === 'codex')
const isGrok = computed(() => props.client.id === 'grok-build')
const isGemini = computed(() => props.client.id === 'gemini-cli')
const manualTarget = computed<ClientAutoConfigTarget | null>(() => {
  if (isCodex.value) return 'codex'
  if (isGrok.value) return 'grok'
  if (isGemini.value) return 'gemini'
  return null
})
const reasoningControlAvailable = computed(() => new Set([
  'codex', 'gemini-cli', 'antigravity', 'hermes-agent', 'workbuddy', 'vscode-local-agent',
]).has(props.client.id))
const clientProtocolSet = computed(() => new Set<string>(props.client.protocols))
const reviewModel = computed({
  get: () => slotValues.review_model ?? '',
  set: (value: string) => { slotValues.review_model = value },
})

const platforms: Array<{ id: ClaudeManualPlatform; label: string }> = [
  { id: 'windows', label: 'Windows' },
  { id: 'macos', label: 'macOS' },
  { id: 'linux', label: 'Linux' },
]

const modelListCommand = computed(() => apiKey.value ? buildClaudeModelListCommand(baseUrl.value, apiKey.value, platform.value) : '')
const modelListDisplayCommand = computed(() => {
  let command = modelListCommand.value
  if (apiKey.value) command = command.split(apiKey.value).join('••••••••')
  if (platform.value === 'windows') return command.replace(/ -Headers /g, '`\n  -Headers ')
  return command.replace(/ -H /g, ' \\\n  -H ').replace(/ \| python3 /g, ' \\\n  | python3 ')
})
const platformLabel = computed(() => platforms.find(item => item.id === platform.value)?.label ?? '')
const verificationCommand = computed(() => {
  for (const row of props.client.config_contract.verification_commands) {
    if (!row.operating_systems.includes(platform.value)) continue
    const versionCommand = row.commands.find(command => /--version\b/.test(command))
    if (versionCommand) return versionCommand
  }
  return ''
})

const reasoningOptions = computed(() => {
  if (!reasoningControlAvailable.value) return []
  const model = modelReasoningProfileById[mainModel.value.trim()]
  if (!model) return []
  const clientSpecific = model.client_levels.find(item => item.client === props.client.name)?.levels ?? []
  const clientControls = props.client.reasoning.levels
  let result: string[]
  if (clientSpecific.length) {
    result = !clientControls.length
      ? [...new Set(clientSpecific)]
      : [...new Set(clientSpecific.filter(level => clientControls.includes(level)))]
  } else if (!clientControls.length) {
    result = [...new Set(model.model_levels)]
  } else {
    const direct = model.model_levels.filter(level => clientControls.includes(level))
    const mapped = model.client_mappings
      .filter(mapping => mapping.client === props.client.name && model.model_levels.includes(mapping.from) && clientControls.includes(mapping.to))
      .map(mapping => mapping.to)
    result = [...new Set([...direct, ...mapped])]
  }
  return result
})

const compatibleProtocols = computed(() => {
  const model = modelDocContractById[mainModel.value.trim()]
  if (!model) return []
  const verified = new Set(model.protocols.filter(row => row.status === 'verified').map(row => row.name))
  return props.client.protocols.filter(protocol => verified.has(protocol))
})

watch(compatibleProtocols, (protocols) => {
  if (protocols.includes(selectedProtocol.value as never)) return
  const recommended = modelDocContractById[mainModel.value.trim()]?.recommended_protocol
  selectedProtocol.value = protocols.find(protocol => protocol === recommended) ?? protocols[0] ?? ''
}, { immediate: true })

const reasoningDescriptions: Record<string, { label: string; shortLabel: string; mode: string; description: string }> = {
  auto: { label: '自动', shortLabel: '自动', mode: '模型默认', description: '不写入全局推理强度，使用模型目录中的默认档位。' },
  none: { label: '关闭', shortLabel: '关闭', mode: '写入配置', description: '关闭显式推理，优先降低延迟。' },
  minimal: { label: 'Minimal', shortLabel: 'Min', mode: '写入配置', description: '使用最少推理预算，适合简单任务。' },
  low: { label: 'Low', shortLabel: 'Low', mode: '写入配置', description: '更快、更省 Token，适合范围明确的任务。' },
  medium: { label: 'Medium', shortLabel: 'Med', mode: '写入配置', description: '能力、延迟和消耗较均衡。' },
  high: { label: 'High', shortLabel: 'High', mode: '写入配置', description: '适合复杂编码和大多数 Agent 任务。' },
  xhigh: { label: 'XHigh', shortLabel: 'XHigh', mode: '写入配置', description: '更深推理，适合复杂架构和高难度调试。' },
  max: { label: 'Max', shortLabel: 'Max', mode: '写入配置', description: '模型允许时使用最大推理预算。' },
}
const reasoningSliderOptions = computed<DocsReasoningOption[]>(() => [
  reasoningDescriptions.auto,
  ...reasoningOptions.value.map(level => ({ id: level, ...(reasoningDescriptions[level] ?? { label: level, shortLabel: level, mode: '写入配置', description: '使用模型与客户端共同支持的档位。' }) })),
].map((item, index) => ({ id: index === 0 ? 'auto' : reasoningOptions.value[index - 1], ...item })))

watch(reasoningOptions, (levels) => {
  if (levels.includes(selectedReasoning.value)) return
  selectedReasoning.value = levels.includes('high') ? 'high' : 'auto'
}, { immediate: true })

watch(mainModel, (model, previous) => {
  for (const slot of props.client.config_contract.model_slots) {
    if (slot.toLowerCase() === 'model') slotValues[slot] = model
  }
  if (isCodex.value && (!reviewModel.value || reviewModel.value === previous)) reviewModel.value = model
})

const codexCompatibleModels = computed(() => discoveredModels.value.filter((model) => {
  const contract = modelDocContractById[model]
  return contract?.protocols.some(row => row.status === 'verified' && clientProtocolSet.value.has(row.name)) ?? false
}))
const settingsCommand = computed(() => isCodex.value && apiKey.value && mainModel.value
  ? buildCodexSettingsCommand({
      baseUrl: baseUrl.value,
      apiKey: apiKey.value,
      mainModel: mainModel.value,
      reviewModel: reviewModel.value,
      reasoningEffort: selectedReasoning.value,
      availableModels: codexCompatibleModels.value.length ? codexCompatibleModels.value : [mainModel.value, reviewModel.value],
    }, platform.value)
  : manualTarget.value && apiKey.value && mainModel.value && selectedProtocol.value
    ? buildClientManualConfigCommand({
        target: manualTarget.value,
        apiKey: apiKey.value,
        baseUrl: baseUrl.value,
        modelId: mainModel.value,
        protocol: selectedProtocol.value as 'responses' | 'chat_completions' | 'messages' | 'generate_content',
        reasoningEffort: selectedReasoning.value === 'auto' ? '' : selectedReasoning.value,
        isWindows: platform.value === 'windows',
      })
    : '')
const clientVerificationCommand = computed(() => {
  if (isCodex.value) return buildCodexVerificationCommand(platform.value)
  const chain = platform.value === 'windows' ? '; if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}; ' : ' && '
  if (isGrok.value) return `grok --version${chain}grok --always-approve --permission-mode bypassPermissions --single "只回复 GROK_OK"`
  if (isGemini.value) return `gemini --version${chain}gemini --yolo -p "只回复 GEMINI_OK" --output-format stream-json`
  return verificationCommand.value
})
const verificationMarker = computed(() => isCodex.value ? 'CODEX_OK' : isGrok.value ? 'GROK_OK' : 'GEMINI_OK')
const settingsDisplayCommand = computed(() => {
  if (!settingsCommand.value) return ''
  if (isCodex.value && platform.value === 'windows') {
    return "$ErrorActionPreference='Stop';\n$配置数据='••••••••';\n# 备份并增量合并 config.toml、auth.json 与模型目录"
  }
  if (isCodex.value) return "CODEX_CFG_B64='••••••••' \\\n  python3 -c '…备份、增量合并并原子写入 Codex 配置…'"
  let command = settingsCommand.value
  if (apiKey.value) command = command.split(apiKey.value).join('••••••••')
  return platform.value === 'windows' ? command.replace(/; /g, ';\n') : command.replace(/; /g, ';\n')
})

watch(apiKey, () => {
  discoveredModels.value = []
  modelError.value = ''
})

async function runModelDiscovery(): Promise<void> {
  if (!apiKey.value) return
  modelLoading.value = true
  modelError.value = ''
  discoveredModels.value = []
  try {
    const isLocalPreview = ['127.0.0.1', 'localhost'].includes(window.location.hostname)
    const requestBaseUrl = isLocalPreview && baseUrl.value.replace(/\/+$/, '') === 'https://api.laoshirenai.com' ? '/__gateway' : baseUrl.value
    const returnedModels = await getGatewayModels(requestBaseUrl, apiKey.value)
    discoveredModels.value = returnedModels.filter((model) => {
      const contract = modelDocContractById[model]
      return contract?.protocols.some(row => row.status === 'verified' && clientProtocolSet.value.has(row.name)) ?? false
    })
    if (!discoveredModels.value.length && returnedModels.length) {
      throw new Error(`当前 Key 返回了 ${returnedModels.length} 个模型，但没有一个支持 ${props.client.name} 的 ${props.client.protocols.map(protocolLabel).join(' / ')} 协议。`)
    }
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

function effortLabel(level: string): string {
  const labels: Record<string, string> = { none: '关闭', minimal: 'Minimal', low: 'Low', medium: 'Medium', high: 'High', xhigh: 'XHigh', max: 'Max', adaptive: '自动', always_on: '始终开启', disabled: '关闭' }
  return labels[level] ?? level
}

function protocolLabel(protocol: string): string {
  if (protocol === 'chat_completions') return 'Chat Completions'
  if (protocol === 'generate_content') return 'GenerateContent'
  return protocol === 'messages' ? 'Messages' : 'Responses'
}

function slotExplanation(slot: string): string {
  const value = slot.toLowerCase()
  if (value === 'model' || value.includes('model.main') || value.includes('models.default') || value.includes('selected main')) return '客户端每次新建会话默认使用的主模型；通常与上一步保持一致。'
  if (value.includes('review_model') || value.includes('review')) return '仅用于代码审查；留空时使用主模型。'
  if (value.includes('utilitysmall')) return '供标题、摘要等轻量辅助任务使用的小模型；不填则由客户端决定。'
  if (value.includes('utility')) return '供搜索、摘要或其他辅助任务使用；不填则由客户端决定。'
  if (value.includes('auxiliary')) return '某类辅助任务的独立模型覆盖；不是主会话模型。'
  if (value.includes('--model') || value.includes('session override')) return '只覆盖当前命令或当前会话，不改变永久默认模型。'
  if (value.includes('provider') || value.includes('models[') || value.includes('models.<')) return 'Provider 下的模型注册条目，需要同时匹配所选协议、Base URL 和模型 ID。'
  if (value.includes('reasoning') || value.includes('--effort')) return '推理档位或推理模型覆盖，只能填写上一步真实交集中的值。'
  return '客户端原生配置槽位；留空时保留客户端现有默认规则。'
}
</script>

<style scoped>
.manual-config{margin-top:2rem;min-width:0}.manual-config>header small{color:#1267d6;font:700 .62rem ui-monospace,monospace;letter-spacing:.12em}.manual-config>header h2{margin:.35rem 0 1rem;font-size:1.2rem}.no-protocol-manual{border:1px solid #fde68a;border-radius:.85rem;background:#fffbeb;padding:1rem;color:#854d0e}.no-protocol-manual strong{font-size:.82rem}.no-protocol-manual p{margin:.4rem 0 0;font-size:.7rem;line-height:1.6}.manual-config ol{min-width:0;list-style:none;margin:0;padding:0;border:1px solid #e4e4e7;border-radius:.9rem;overflow:hidden}.manual-config li{min-width:0;display:grid;grid-template-columns:2.5rem minmax(0,1fr);gap:.7rem;padding:1rem;border-bottom:1px solid #eee}.manual-config li:last-child{border-bottom:0}.manual-config li>b{color:#1267d6;font:700 .7rem ui-monospace,monospace}.step-body{min-width:0}.step-body h3{margin:0;color:#27272a;font-size:.9rem}.step-body h4{margin:1rem 0 .45rem;color:#3f3f46;font-size:.72rem}.step-body p{margin:.5rem 0 0;color:#52525b;font-size:.72rem;line-height:1.6}.field-grid,.slot-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:.65rem;margin-top:.75rem}label span{display:block;margin-bottom:.35rem;color:#3f3f46;font-size:.68rem;font-weight:700}label small{display:block;margin-top:.5rem;color:#71717a;font-size:.66rem;line-height:1.55}.model-field{display:block;margin-top:.75rem;max-width:32rem}input{width:100%;box-sizing:border-box;border:1px solid #d6d3d1;border-radius:.75rem;background:#fff;padding:.68rem .78rem;color:#27272a;font:400 .72rem ui-monospace,monospace;outline:none}input:focus{border-color:#1267d6;box-shadow:0 0 0 3px #1267d619}.platform-tabs{display:flex;gap:.35rem;margin:.75rem 0}.platform-tabs button{border:1px solid #d6d3d1;border-radius:.65rem;background:#fff;color:#3f3f46;padding:.48rem .75rem;font-size:.68rem;font-weight:700;cursor:pointer}.platform-tabs button.active{border-color:#1267d6;background:#1267d6;color:#fff}.protocol-choice{max-width:28rem;margin-top:.75rem}.protocol-choice>span{color:#3f3f46;font-size:.68rem;font-weight:700}.protocol-choice small{display:block;margin-top:.35rem;color:#71717a;font-size:.62rem}.protocol-options{display:flex;flex-wrap:wrap;gap:.35rem;margin-top:.4rem}.protocol-options button{border:1px solid #d6d3d1;border-radius:999px;background:#fff;color:#71717a;padding:.3rem .55rem;font-size:.58rem;font-weight:700;cursor:pointer}.protocol-options button.active{border-color:#1267d6;background:#eaf3ff;color:#1267d6}.reasoning-unavailable{max-width:28rem;margin-top:.85rem;border:1px solid #e4e4e7;border-radius:.75rem;background:#fafafa;padding:.7rem .8rem}.reasoning-unavailable strong{font-size:.68rem;color:#3f3f46}.reasoning-unavailable p{font-size:.64rem!important;color:#71717a!important}.model-error{border-radius:.6rem;background:#fef2f2;color:#b91c1c!important;padding:.6rem .7rem}.model-results{margin-top:.7rem;overflow:hidden;border:1px solid #e4e4e7;border-radius:.7rem;background:#fff}.model-results header{display:flex;align-items:center;justify-content:space-between;background:#fafafa;padding:.55rem .7rem}.model-results header b{font-size:.68rem}.model-results header span{color:#71717a;font-size:.62rem}.model-result-row{display:flex;align-items:center;justify-content:space-between;gap:.7rem;border-top:1px solid #f1f1f3;padding:.5rem .7rem}.model-result-row:hover{background:#f8fbff}.model-result-row.selected{background:#eff6ff}.model-name{min-width:0;display:flex;align-items:center;gap:.4rem}.model-name code{overflow:hidden;color:#27272a;font-size:.68rem;text-overflow:ellipsis;white-space:nowrap}.copy-model{width:1.65rem;height:1.65rem;display:inline-flex;align-items:center;justify-content:center;border:1px solid #e4e4e7;border-radius:.45rem;background:#fff;color:#71717a;cursor:pointer}.copy-model svg{width:.85rem;height:.85rem;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}.set-main-button{border:0;background:transparent;color:#1267d6;font-size:.6rem;font-weight:700;cursor:pointer}.codex-slot-layout{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:.7rem;margin-top:.75rem}.catalog-entry-card,.generated-slot-card{border:1px solid #e4e4e7;border-radius:.75rem;background:#fafafa;padding:.7rem .8rem}.catalog-entry-card>span,.catalog-entry-card>strong,.catalog-entry-card>small{display:block}.catalog-entry-card>span{color:#3f3f46;font-size:.68rem;font-weight:700}.catalog-entry-card>strong,.generated-slot-card>strong{display:block;margin-top:.35rem;color:#1267d6;font:700 .7rem ui-monospace,monospace}.catalog-entry-card>small{margin-top:.45rem;color:#71717a;font-size:.64rem;line-height:1.55}.generated-slot-card{max-width:38rem;margin-top:.75rem}.generated-slot-card p{font-size:.66rem!important}.key-link{display:inline-flex;margin-top:.7rem;border-radius:.55rem;background:#1267d6;color:#fff;padding:.5rem .72rem;font-size:.7rem;font-weight:700;text-decoration:none}@media(max-width:700px){.field-grid,.slot-grid,.codex-slot-layout{grid-template-columns:1fr}.manual-config li{grid-template-columns:2rem 1fr}}
</style>
