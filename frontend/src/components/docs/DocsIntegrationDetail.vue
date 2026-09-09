<template>
  <article class="integration-detail" :data-client-status="simpleGuide ? 'manual' : legacyIntegrationDetailsVisible ? client.one_click_status : 'paused'">
    <router-link to="/docs/category/integrations" class="back-link">← 返回工具集成</router-link>

    <header class="detail-hero">
      <span class="client-icon"><b aria-hidden="true">{{ clientInitials(client.name) }}</b><img :src="client.icon" :alt="`${client.name} 官方图标`" @error="hideBrokenIcon" /></span>
      <div class="hero-copy">
        <small>CLIENT INTEGRATION</small>
        <h1>{{ client.name }}</h1>
        <p>{{ simpleGuide ? `配置教程 · ${simpleGuide.checkedVersion}` : `配置基线 · ${client.version}` }}</p>
      </div>
      <div class="hero-meta">
        <div v-if="legacyIntegrationDetailsVisible" class="protocol-list" aria-label="原生协议">
          <span v-for="protocol in client.protocols" :key="protocol">{{ protocolLabel(protocol) }}</span>
          <em v-if="client.protocols.length === 0">尚无可公开协议</em>
        </div>
        <span class="status-badge" :class="simpleGuide ? 'status-manual' : legacyIntegrationDetailsVisible ? `status-${client.one_click_status}` : 'status-paused'">{{ statusLabel }}</span>
      </div>
    </header>

    <DocsSimpleClientGuide v-if="simpleGuide" :guide="simpleGuide" />

    <section v-else-if="!legacyIntegrationDetailsVisible" class="documentation-refresh-notice" aria-label="教程整理状态">
      <strong>基础配置教程正在整理</strong>
      <p>旧版内容已暂时隐藏，但仍完整保留在代码中。新版将只说明配置位置、Base URL、API Key、模型 ID 和验证方法。</p>
      <p>一键配置继续暂停；教程确认无误后再逐个开放。</p>
    </section>

    <section v-if="legacyIntegrationDetailsVisible" class="availability" :class="`availability-${client.one_click_status}`" aria-label="集成开放状态">
      <strong>{{ availabilityTitle }}</strong>
      <p>{{ availabilityDescription }}</p>
    </section>

    <section v-if="legacyIntegrationDetailsVisible" class="files">
      <h2>配置会修改哪些文件</h2>
      <p class="files-intro">自动配置和上面的手动写入命令只会修改以下客户端配置文件。</p>
      <div v-if="hasConfigFiles" class="file-columns">
        <div>
          <h3>macOS / Linux</h3>
          <code v-for="file in client.files.unix" :key="file">{{ file }}</code>
          <p v-if="client.files.unix.length === 0">没有可安全修改的公开路径</p>
        </div>
        <div>
          <h3>Windows</h3>
          <code v-for="file in client.files.windows" :key="file">{{ file }}</code>
          <p v-if="client.files.windows.length === 0">没有可安全修改的公开路径</p>
        </div>
      </div>
      <p v-else class="no-files">没有可安全修改的公开配置文件。不要猜测路径或直接修改客户端内部数据库。</p>
    </section>

    <section v-if="legacyIntegrationDetailsVisible && client.one_click_status === 'ready'" class="steps">
      <h2>自动配置</h2>
      <ol>
        <li>
          <b>01</b>
          <div>
            <h3>创建 Key / 选择分组</h3>
            <p>创建 API Key，并选择准备使用的分组。</p>
            <a class="step-link" href="https://laoshirenai.com/keys" target="_blank" rel="noreferrer">打开 API 密钥页面 ↗</a>
          </div>
        </li>
        <li><b>02</b><div><h3>选择导入客户端</h3><p>找到刚创建的 Key，选择 <strong>{{ client.name }}</strong>。</p></div></li>
        <li><b>03</b><div><h3>复制真实命令</h3><p>只复制 API 密钥页面为当前 Key 生成的一次性命令。</p></div></li>
        <li>
          <b>04</b>
          <div>
            <h3>打开终端并执行</h3>
            <div class="platform-list">
              <span><strong>Windows</strong>PowerShell</span>
              <span><strong>macOS</strong>Terminal</span>
              <span><strong>Linux</strong>Terminal</span>
            </div>
            <p>粘贴刚才复制的命令，按 Enter 执行。</p>
          </div>
        </li>
        <li>
          <b>05</b>
          <div>
            <h3>验证配置</h3>
            <p v-if="isClaude">执行下面的命令，返回 <code>CLAUDE_CODE_OK</code> 即配置成功。</p>
            <p v-else>{{ client.config_contract.verification }}</p>
            <DocsTerminalCommand v-if="isClaude" class="step-terminal" label="验证" :command="claudeVerificationCommand" />
          </div>
        </li>
      </ol>
    </section>

    <section v-else-if="legacyIntegrationDetailsVisible" class="automatic-unavailable">
      <h2>自动配置</h2>
      <strong>暂不提供一键命令</strong>
      <p>当前版本还没有通过安全写入和回滚验收，请使用下面的手动配置；页面不会生成占位命令。</p>
    </section>

    <ClaudeCodeManualConfig v-if="legacyIntegrationDetailsVisible && isClaude" />
    <ClientManualConfig v-else-if="legacyIntegrationDetailsVisible" :client="client" />

    <section v-if="legacyIntegrationDetailsVisible" class="errors">
      <h2>常见错误</h2>
      <ul>
        <li>模型未出现：当前 Key 的分组没有开放该模型，必须以该 Key 的 <code>GET /v1/models</code> 为准。</li>
        <li>客户端或协议不可选：协议只是候选，模型与客户端的真实闭环尚未完成。</li>
        <li>配置被拒绝：客户端没有公开安全的凭证或配置写入合同，不要绕过 UI 修改内部数据库。</li>
        <li>验证失败：恢复备份，保留明确错误，再重新核对 Base URL、模型 ID 和协议。</li>
      </ul>
    </section>

    <router-link v-if="legacyIntegrationDetailsVisible" to="/docs/models" class="model-link">查看模型能力与客户端证据 →</router-link>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ClientMatrixEntry, ClientMatrixProtocol } from '@/generated/clientMatrix'
import ClaudeCodeManualConfig from './ClaudeCodeManualConfig.vue'
import ClientManualConfig from './ClientManualConfig.vue'
import DocsTerminalCommand from './DocsTerminalCommand.vue'
import DocsSimpleClientGuide from './DocsSimpleClientGuide.vue'
import { claudeVerificationCommand } from '@/utils/claudeCodeManualConfig'
import { simpleClientGuideById } from '@/docs/guides/simpleClientGuides'

const props = defineProps<{ client: ClientMatrixEntry }>()
const legacyIntegrationDetailsVisible = false
const simpleGuide = computed(() => simpleClientGuideById[props.client.id])
const isClaude = computed(() => props.client.slug === 'integration-claude-code')
const hasConfigFiles = computed(() => props.client.files.unix.length > 0 || props.client.files.windows.length > 0)
const statusLabel = computed(() => {
  if (simpleGuide.value) return '手动配置'
  if (!legacyIntegrationDetailsVisible) return '教程整理中'
  if (props.client.one_click_status === 'ready') return '一键导入可用'
  if (props.client.one_click_status === 'prototype') return '手动配置'
  return '集成未开放'
})
const availabilityTitle = computed(() => {
  if (props.client.one_click_status === 'ready') return '可以从 API 密钥页面生成一次性导入命令'
  if (props.client.one_click_status === 'prototype') return '已提供手动配置步骤'
  return '当前不提供导入命令'
})
const availabilityDescription = computed(() => {
  if (props.client.one_click_status === 'ready') return '命令必须绑定当前 Key、分组、模型、协议和客户端版本；不要保存或转发。'
  if (props.client.one_click_status === 'prototype') return '可以读取当前 Key 的模型并按下方步骤配置；尚未通过的一键命令不会显示。'
  return '本页只公开已知边界。缺少安全写入或真实闭环证据时，不把客户端描述为可用。'
})

function protocolLabel(protocol: ClientMatrixProtocol): string {
  if (protocol === 'responses') return 'Responses'
  if (protocol === 'chat_completions') return 'Chat Completions'
  if (protocol === 'messages') return 'Messages'
  return 'GenerateContent'
}

function clientInitials(name: string): string {
  return name.match(/[A-Za-z]+/g)?.map(part => part[0]).join('').slice(0, 2).toUpperCase() || name.slice(0, 1)
}

function hideBrokenIcon(event: Event): void {
  const image = event.currentTarget as HTMLImageElement
  image.style.display = 'none'
}
</script>

<style scoped>
.integration-detail{max-width:54rem;margin:0 auto;padding:3.5rem 1rem 5rem;color:#18181b}.back-link{color:#71717a;font-size:.82rem;text-decoration:none}.detail-hero{display:flex;align-items:center;gap:1rem;margin:2.5rem 0 1.1rem;padding:1.25rem;border:1px solid #e7dfcf;border-radius:1rem;background:linear-gradient(110deg,#fbfaf6,#f4f7ff)}.client-icon{position:relative;display:grid;place-items:center;width:3.5rem;height:3.5rem;flex:0 0 auto;border-radius:.75rem;background:#fff;color:#64748b;font:800 .8rem ui-monospace,monospace;overflow:hidden}.client-icon img{position:absolute;inset:0;width:100%;height:100%;object-fit:contain;background:#fff}.hero-copy{flex:1;min-width:0}.detail-hero small{color:#1267d6;font:700 .65rem ui-monospace,monospace;letter-spacing:.14em}.detail-hero h1{margin:.25rem 0 0;font-size:2rem}.detail-hero p{margin:.25rem 0 0;color:#78716c;font-size:.75rem}.hero-meta{display:flex;flex-direction:column;align-items:flex-end;gap:.55rem}.status-badge{flex:0 0 auto;border-radius:999px;padding:.4rem .65rem;font-size:.68rem;font-weight:800}.status-ready{background:#dcfce7;color:#166534}.status-prototype{background:#fef3c7;color:#92400e}.status-disabled{background:#e4e4e7;color:#52525b}.protocol-list{display:flex;justify-content:flex-end;flex-wrap:wrap;gap:.35rem}.protocol-list span{border-radius:999px;background:#fff;padding:.3rem .55rem;color:#52525b;font-size:.65rem;font-weight:700;box-shadow:0 0 0 1px #e4e4e7}.protocol-list em{color:#a1a1aa;font-size:.7rem;font-style:normal}.availability{margin-top:1rem;border-radius:.8rem;padding:1rem}.availability strong{font-size:.82rem}.availability p{margin:.35rem 0 0;font-size:.74rem;line-height:1.55}.availability-ready{border:1px solid #bbf7d0;background:#f0fdf4;color:#166534}.availability-prototype{border:1px solid #fde68a;background:#fffbeb;color:#854d0e}.availability-disabled{border:1px solid #e4e4e7;background:#fafafa;color:#52525b}.files,.steps,.automatic-unavailable,.errors{margin-top:2rem}.files h2,.steps h2,.automatic-unavailable h2,.errors h2{font-size:1.2rem}.files-intro{margin:-.35rem 0 .8rem;color:#71717a;font-size:.74rem;line-height:1.55}.automatic-unavailable{border:1px solid #fde68a;border-radius:.85rem;background:#fffbeb;padding:1rem;color:#854d0e}.automatic-unavailable h2{margin:0 0 .7rem}.automatic-unavailable strong{font-size:.82rem}.automatic-unavailable p{margin:.35rem 0 0;font-size:.74rem;line-height:1.55}.file-columns{display:grid;grid-template-columns:1fr 1fr;gap:.75rem}.file-columns>div,.no-files{border:1px solid #e4e4e7;border-radius:.75rem;padding:.9rem}.file-columns h3{margin:0 0 .55rem;font-size:.78rem}.file-columns code{display:block;margin:.3rem 0;color:#4f4639;font-size:.7rem;overflow-wrap:anywhere}.file-columns p,.no-files{color:#71717a;font-size:.72rem;line-height:1.55}.steps ol{list-style:none;margin:1rem 0 0;padding:0;border:1px solid #e4e4e7;border-radius:.9rem;overflow:hidden}.steps li{min-width:0;display:grid;grid-template-columns:2.5rem minmax(0,1fr);gap:.7rem;padding:1rem;border-bottom:1px solid #eee}.steps li>div{min-width:0}.steps li:last-child{border:0}.steps li>b{color:#1267d6;font:700 .7rem ui-monospace,monospace}.steps h3{margin:0;font-size:.9rem}.steps p{margin:.35rem 0 0;color:#71717a;font-size:.78rem;line-height:1.6}.step-link{display:inline-flex;margin-top:.65rem;border-radius:.55rem;background:#1267d6;color:#fff;padding:.5rem .72rem;font-size:.7rem;font-weight:700;text-decoration:none}.platform-list{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:.45rem;margin-top:.65rem}.platform-list span{display:flex;flex-direction:column;gap:.18rem;border:1px solid #e4e4e7;border-radius:.55rem;background:#fafafa;padding:.55rem .65rem;color:#71717a;font-size:.65rem}.platform-list strong{color:#27272a;font-size:.72rem}.step-terminal{margin-top:.65rem}.errors ul{margin:0;padding:1rem 1rem 1rem 2rem;border:1px solid #e4e4e7;border-radius:.75rem;color:#57534e;font-size:.78rem;line-height:1.8}.model-link{display:inline-block;margin-top:1.5rem;color:#1267d6;text-decoration:none;font-weight:700;font-size:.8rem}.dark .integration-detail{color:#f4f4f5}.dark .detail-hero{background:#18181b;border-color:#3f3f46}.dark .file-columns>div,.dark .steps ol,.dark .errors ul{border-color:#3f3f46}@media(max-width:700px){.integration-detail{padding-top:2rem}.detail-hero{display:grid;grid-template-columns:3.5rem minmax(0,1fr);align-items:start}.hero-meta{grid-column:2;align-items:flex-start}.protocol-list{justify-content:flex-start}.file-columns,.platform-list{grid-template-columns:1fr}}@media(prefers-reduced-motion:reduce){*{scroll-behavior:auto!important}}
.status-paused{background:#fef3c7;color:#92400e}.status-manual{background:#dbeafe;color:#1d4ed8}.documentation-refresh-notice{margin-top:1rem;border:1px solid #fde68a;border-radius:.85rem;background:#fffbeb;padding:1rem;color:#854d0e}.documentation-refresh-notice strong{font-size:.86rem}.documentation-refresh-notice p{margin:.4rem 0 0;font-size:.76rem;line-height:1.65}
</style>
