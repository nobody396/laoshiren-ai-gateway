<template>
  <div class="text-guide" data-docs-mode="text-preview">
    <header class="guide-header">
      <a class="brand" href="#start" @click.prevent="select('start')"><img src="/laoshirenai-icon.jpg" alt="" /><strong>老实人AI</strong><span>/ 配置文档</span></a>
      <span class="preview-label"><i />文字版预览 · 未发布</span>
      <a class="console-link" href="https://laoshirenai.com/keys" target="_blank" rel="noreferrer">我的 API Key ↗</a>
    </header>

    <div class="guide-layout">
      <aside class="guide-navigation">
        <p class="nav-label">从这里开始</p>
        <button class="guide-nav-item start-item" :aria-current="activeId === 'start' ? 'page' : undefined" @click="select('start')"><span class="nav-symbol">↗</span>快速开始</button>
        <div class="nav-heading"><p class="nav-label">按工具配置</p><span>{{ textGuideClients.length }}</span></div>
        <nav aria-label="工具配置文档">
          <button v-for="item in textGuideClients" :key="item.client.id" :data-client-id="item.client.id" class="guide-nav-item" :aria-current="activeId === item.client.id ? 'page' : undefined" @click="select(item.client.id)">
            <span class="nav-icon" aria-hidden="true"><b>{{ item.client.name.slice(0, 1) }}</b><img :src="item.client.icon" alt="" @error="hideIcon" /></span>
            <span>{{ item.client.id === 'vscode-local-agent' ? 'VS Code Local Agent' : item.client.name }}</span>
            <small v-if="item.guide?.mode === 'pending'">待补</small>
          </button>
        </nav>
        <div class="navigation-note">只看你用的工具。<br />不用读完所有教程。</div>
      </aside>

      <main ref="mainElement" class="guide-main" tabindex="-1">
        <template v-if="!selected">
          <div class="eyebrow">GET CONNECTED</div>
          <h1>把模型接到<br class="hero-break" />你正在用的工具。</h1>
          <p class="hero-description">一把 Key，三步配置。<br />选你的工具，照着对应的文字教程操作。</p>
          <div class="overview-steps" aria-label="接入三步">
            <a href="#get-key"><b>01</b><span>创建 Key<small>选好授权分组</small></span></a>
            <a href="#choose-tool"><b>02</b><span>填写配置<small>复制地址与模型 ID</small></span></a>
            <a href="#verify"><b>03</b><span>启动验证<small>收到第一条回复</small></span></a>
          </div>
          <section id="get-key" class="overview-section">
            <div class="section-title"><span>01</span><h2>先准备一把 Key</h2></div>
            <p>登录控制台，确认账户有余额或可用套餐。打开 <a href="https://laoshirenai.com/keys" target="_blank" rel="noreferrer">API 密钥</a>，创建「多分组 Key」，勾选需要的分组并复制 Key。</p>
            <p class="secondary">同名模型按分组顺序选择，按实际分组计费；月卡额度不足不会自动转扣其他分组。图片接口仍使用单分组 Key。</p>
          </section>
          <section id="choose-tool" class="overview-section">
            <div class="section-title"><span>02</span><h2>打开对应工具的教程</h2></div>
            <p>每篇都写清楚：<strong>Base URL、配置文件位置、模型 ID 和填写内容。</strong>地址不能一套通用，配置也不能互相照搬。</p>
            <div class="featured-tools">
              <button v-for="id in featuredIds" :key="id" @click="select(id)"><span>{{ clientName(id) }}</span><b>查看教程 →</b></button>
            </div>
            <p class="secondary">还有 Grok Build、Kimi、ZCode 等工具，全部在左侧。标为「待补」的条目尚不能作为可用配置使用。</p>
          </section>
          <section id="verify" class="overview-section">
            <div class="section-title"><span>03</span><h2>发出第一条请求</h2></div>
            <p>重新启动工具，选择配置好的模型，发送「你好」。有正常回复后，再检查控制台的使用记录。编程工具还需要验证一次文件读取或工具调用。</p>
          </section>
          <aside class="reading-note"><strong>不用在这个网页填 Key。</strong><p>这里是文字配置文档，只提供说明和复制功能；不会读取你的凭证，也不会替你修改电脑。本站图形化配置生成器不在本预览内。</p></aside>
        </template>

        <article v-else :key="selected.client.id" :data-guide-id="selected.client.id">
          <div class="breadcrumb"><button @click="select('start')">快速开始</button><span>/</span><span>{{ selected.client.name }}</span></div>
          <div class="client-title"><img :src="selected.client.icon" alt="" @error="hideIcon" /><div><div class="eyebrow">CLIENT SETUP</div><h1>{{ selected.client.name }}</h1></div></div>
          <p class="client-description">{{ guide.scope }}</p>
          <div class="reference-meta"><span>本文参考 {{ referenceVersionLabel }}</span><span>{{ modeLabel }}</span><span>{{ guide.verification?.[os] ?? '初稿 · 待按实际版本验收' }}</span></div>
          <div class="os-switch" role="group" aria-label="文档操作系统">
            <button v-for="item in operatingSystems" :key="item.id" type="button" :aria-pressed="os === item.id" :class="{ selected: os === item.id }" @click="changeOS(item.id)">{{ item.label }}</button>
          </div>
          <aside v-if="unsupportedOS" class="reading-note warning" role="status"><strong>此工具暂不提供 Linux 配置</strong><p>不要将 Windows 或 macOS 的配置路径直接套过来。请选择受支持系统的教程。</p></aside>
          <template v-else>
            <aside v-if="usesWSL" class="reading-note warning"><strong>Windows：请先进入 WSL</strong><p>下面的命令在 WSL 的 Linux 终端执行，不在 PowerShell。配置文件也属于 WSL 用户目录；这不是 Windows 原生客户端的验收声明。</p></aside>
            <section id="step-key" class="guide-step">
              <div class="section-title"><span>01</span><h2>准备 Key 和模型 ID</h2></div>
              <p>在 <a href="https://laoshirenai.com/keys" target="_blank" rel="noreferrer">API 密钥页面</a>创建 Key，确认勾选了目标模型所在分组。</p>
              <template v-if="usesTerminalCredentials">
                <p>在你准备启动工具的终端执行下面这行，再粘贴 Key。输入不显示，Key 只保留在这个终端会话里。</p>
                <DocsTerminalCommand class="guide-terminal credential-window" :label="`${shellName} · 设置本次会话的 Key`" :command="credentialCommand" />
                <details class="more-detail"><summary>不知道模型 ID？读取这把 Key 的模型列表</summary><p>执行后复制返回的 <code>id</code>，不是模型的展示名称。列表反映 Key 授权，不代表 {{ selected.client.name }} 支持列表里的所有模型。</p><DocsTerminalCommand class="guide-terminal" :label="`${shellName} · 读取模型 ID`" :command="modelsCommand(commandOS)" /></details>
              </template>
              <p v-else>Key 先妥善保管，不用粘贴到这个网页。模型 ID 可从 <a href="https://laoshirenai.com/models" target="_blank" rel="noreferrer">模型与价格页面</a>查看；使用前确认 Key 和工具都支持它。</p>
              <div class="inline-note">下文的 <code>YOUR_MODEL_ID</code> 是占位符，不是模型名。请将文件和启动命令中的占位符一起替换。</div>
            </section>
            <section id="step-config" class="guide-step">
              <div class="section-title"><span>02</span><h2>{{ guide.mode === 'native' ? '在工具自己的设置里填写' : guide.mode === 'pending' ? '先看当前接入限制' : guide.mode === 'environment' ? '在当前终端设置地址和模型' : '修改这个配置文件' }}</h2></div>
              <div class="connection-facts">
                <div><span>Base URL</span><code>{{ guide.baseUrl }}</code></div>
                <div><span>本篇接口</span><strong>{{ guide.protocol }}</strong></div>
                <div><span>{{ guide.mode === 'native' ? '配置保存位置' : guide.mode === 'environment' ? '原配置保留' : '文件位置' }}</span><div><code v-for="file in files" :key="file">{{ file }}</code><span v-if="!files.length">不提供手动编辑路径</span></div></div>
              </div>
              <div v-if="guide.mode === 'pending'" class="reading-note warning"><strong>这篇配置模板还在核对</strong><p>入口保留，但不会用猜测的路径和命令充当完成的教程。请先选择已有配置示例的工具。</p></div>
              <ol class="instruction-list"><li v-for="line in guide.instructions" :key="line">{{ line }}</li></ol>
              <p v-if="guide.mode === 'file' && visibleBlocks.length" class="secondary">文件不存在时先创建对应文件夹。已有文件请先备份，只合并下面的字段，不要覆盖其他供应商、MCP 或权限设置。</p>
              <DocsTerminalCommand v-for="block in visibleBlocks" :key="block.label" class="guide-terminal config-example" :label="block.label" :command="block.candidate ? '' : block.content" :display-command="block.content" />
              <details class="more-detail" open><summary>这几个地方别填错</summary><ul><li v-for="note in guide.notes" :key="note">{{ note }}</li></ul></details>
            </section>
            <section id="step-verify" class="guide-step">
              <div class="section-title"><span>03</span><h2>启动并确认能用</h2></div>
              <template v-if="guide.mode !== 'pending'">
                <p v-if="guide.launch">配置完成后，从刚才设置 Key 的同一个终端启动。先确认版本，再开始新会话。</p>
                <p v-else>保存设置后重新打开工具，创建新会话，并选择刚添加的模型。</p>
                <DocsTerminalCommand v-if="guide.launch" class="guide-terminal" :label="`${shellName} · 先替换命令中的模型 ID`" :command="guide.launch" />
                <div class="success-check"><span aria-hidden="true">✓</span><div><strong>先发一句「你好」</strong><p>收到正常回复、使用记录里出现对应调用，说明基础接入成功。需要编码时，再试一次读取测试文件；不要直接拿重要项目做第一次测试。</p></div></div>
                <p class="secondary">不通时先查 Key、余额或套餐、模型 ID。仍有问题，提供工具版本和报错截图，记得遮住 Key。</p>
              </template>
              <p v-else>等入口和配置字段确认后再做实际调用验证。现在不要执行其他工具的配置命令来代替。</p>
            </section>
          </template>
          <footer class="guide-footer"><span>配置来源</span><a v-for="source in guide.sources" :key="source.url" :href="source.url" target="_blank" rel="noreferrer">{{ source.label }} ↗</a><p>此预览未运行你的客户端，也未验证所有模型、版本和系统组合。</p></footer>
        </article>
      </main>
      <aside class="guide-toc"><DocsToc :items="toc" /><div class="toc-tip">照着三步做。<br />不用先学全部协议。</div></aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import DocsTerminalCommand from '@/components/docs/DocsTerminalCommand.vue'
import DocsToc from '@/components/docs/DocsToc.vue'
import { keySetupCommand, modelsCommand, textGuideClients, type GuideOS } from '@/docs/guides/textClientGuides'

const activeId = ref('start')
const os = ref<GuideOS>('macos')
const mainElement = ref<HTMLElement | null>(null)
const operatingSystems: Array<{ id: GuideOS; label: string }> = [{ id: 'macos', label: 'macOS' }, { id: 'windows', label: 'Windows' }, { id: 'linux', label: 'Linux' }]
const featuredIds = ['claude-code', 'codex', 'gemini-cli', 'hermes-agent', 'opencode', 'antigravity']
const selected = computed(() => textGuideClients.find(item => item.client.id === activeId.value && item.guide))
const guide = computed(() => selected.value!.guide!)
const files = computed(() => guide.value.files[os.value] ?? [])
const unsupportedOS = computed(() => selected.value?.client.id === 'workbuddy' && os.value === 'linux')
const usesWSL = computed(() => selected.value?.client.id === 'hermes-agent' && os.value === 'windows')
const commandOS = computed<GuideOS>(() => usesWSL.value ? 'linux' : os.value)
const shellName = computed(() => usesWSL.value ? 'WSL · Bash / Zsh' : os.value === 'windows' ? 'PowerShell 5.1 / 7' : 'Bash / Zsh')
const referenceVersionLabel = computed(() => guide.value.referenceVersion.replace(/^cli:/, '').replace(/^app:|^desktop:/, '桌面 ').replace('+cli:', ' / CLI ').replace('+builtin-copilot:', ' / Copilot '))
const modeLabel = computed(() => guide.value.mode === 'file' ? '文件配置' : guide.value.mode === 'native' ? '工具自带设置' : guide.value.mode === 'environment' ? '环境变量配置 · 不改文件' : '配置待补充')
const usesTerminalCredentials = computed(() => ['file', 'environment'].includes(guide.value.mode))
const visibleBlocks = computed(() => guide.value.blocks.filter(block => !block.operatingSystems || block.operatingSystems.includes(commandOS.value)))
const credentialCommand = computed(() => keySetupCommand(commandOS.value, guide.value.keyEnv, guide.value.environment))
const toc = computed(() => selected.value ? [
  { id: 'step-key', text: '准备 Key 和模型 ID', level: 2 },
  { id: 'step-config', text: '填写配置', level: 2 },
  { id: 'step-verify', text: '启动验证', level: 2 },
] : [
  { id: 'get-key', text: '创建 Key', level: 2 },
  { id: 'choose-tool', text: '选择你的工具', level: 2 },
  { id: 'verify', text: '确认接入成功', level: 2 },
])
function clientName(id: string): string { return textGuideClients.find(item => item.client.id === id)?.client.name ?? id }
function updateAddress(): void { window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}#client=${activeId.value}&os=${os.value}`) }
function select(id: string): void { activeId.value = id; updateAddress(); window.scrollTo({ top: 0 }); mainElement.value?.focus({ preventScroll: true }) }
function changeOS(value: GuideOS): void { os.value = value; updateAddress() }
function readAddress(): void {
  const params = new URLSearchParams(window.location.hash.slice(1))
  const id = params.get('client')
  if (id === 'start' || textGuideClients.some(item => item.client.id === id)) {
    const changed = activeId.value !== id
    activeId.value = id!
    if (changed) window.scrollTo({ top: 0 })
  }
  const value = params.get('os')
  if (value === 'macos' || value === 'windows' || value === 'linux') os.value = value
}
function hideIcon(event: Event): void { (event.target as HTMLImageElement).style.visibility = 'hidden' }
onMounted(() => { readAddress(); window.addEventListener('hashchange', readAddress) })
onBeforeUnmount(() => window.removeEventListener('hashchange', readAddress))
</script>

<style scoped>
.text-guide{min-height:100vh;background:rgb(var(--color-marble));color:rgb(var(--color-ink));font-family:var(--font-sans,ui-sans-serif,system-ui,sans-serif)}
.guide-header{height:72px;display:flex;align-items:center;gap:24px;padding:0 32px;border-bottom:1px solid rgb(var(--color-muted)/.18);background:rgb(var(--color-marble)/.95);position:sticky;top:0;z-index:20;backdrop-filter:blur(16px)}
.brand{display:flex;align-items:center;gap:10px;color:inherit;text-decoration:none;white-space:nowrap}.brand img{width:28px;height:28px;border-radius:50%}.brand strong{font-size:15px}.brand>span{color:rgb(var(--color-muted));font-size:13px}
.preview-label{font-size:11px;color:rgb(var(--color-muted));display:flex;gap:7px;align-items:center}.preview-label i{width:5px;height:5px;border-radius:50%;background:rgb(var(--color-terracotta))}.console-link{margin-left:auto;color:rgb(var(--color-ink));font-size:12px;text-decoration:none;border:1px solid rgb(var(--color-muted)/.3);padding:8px 12px;border-radius:7px}
.guide-layout{display:grid;grid-template-columns:224px minmax(0,820px) 170px;max-width:1360px;margin:auto;min-height:calc(100vh - 72px)}
.guide-navigation{padding:32px 18px 24px;border-right:1px solid rgb(var(--color-muted)/.16);position:sticky;top:72px;align-self:start;max-height:calc(100vh - 72px);overflow:auto}.nav-label{font-size:10px;letter-spacing:.11em;font-weight:650;color:rgb(var(--color-muted));margin:0 10px 10px}.nav-heading{display:flex;justify-content:space-between;align-items:center;margin-top:30px}.nav-heading>span{font:11px ui-monospace,monospace;color:rgb(var(--color-muted));margin-right:9px}.guide-nav-item{display:flex;align-items:center;gap:9px;width:100%;min-height:39px;text-align:left;border:0;background:transparent;border-radius:7px;padding:8px 10px;font-size:12px;line-height:1.45;color:rgb(var(--color-ink));cursor:pointer}.guide-nav-item:hover{background:rgb(var(--color-parchment)/.65)}.guide-nav-item[aria-current=page]{background:rgb(var(--color-terracotta)/.09);color:rgb(var(--color-terracotta));font-weight:650}.guide-nav-item small{margin-left:auto;font-size:9px;white-space:nowrap;color:rgb(var(--color-muted))}.nav-icon,.nav-symbol{width:18px;height:18px;display:grid;place-items:center;flex-shrink:0}.nav-icon{position:relative}.nav-icon b{font-size:9px;color:rgb(var(--color-muted))}.nav-icon img{position:absolute;background:rgb(var(--color-marble));max-width:17px;max-height:17px;object-fit:contain}.navigation-note{margin:28px 10px 0;padding-top:17px;border-top:1px solid rgb(var(--color-muted)/.2);font-size:11px;color:rgb(var(--color-muted));line-height:1.9}
.guide-main{min-width:0;padding:54px 56px 72px;outline:none}.eyebrow{font:600 10px ui-monospace,monospace;letter-spacing:.18em;color:rgb(var(--color-terracotta));margin-bottom:20px}h1{font-size:42px;line-height:1.23;font-weight:650;letter-spacing:-.04em;margin:0;color:rgb(var(--color-ink-deep))}.hero-description{font-size:15px;line-height:1.9;color:rgb(var(--color-muted));margin:22px 0 30px}.hero-break{display:none}.overview-steps{display:grid;grid-template-columns:repeat(3,1fr);gap:14px;padding:22px 0 28px;border-bottom:1px solid rgb(var(--color-muted)/.22)}.overview-steps a{display:flex;align-items:center;gap:11px;text-decoration:none;color:inherit}.overview-steps b{font:500 12px ui-monospace,monospace;border:1px solid rgb(var(--color-muted)/.3);border-radius:50%;width:34px;height:34px;display:grid;place-items:center}.overview-steps span{font-size:13px;font-weight:600}.overview-steps small{font-size:10px;font-weight:400;display:block;margin-top:5px;color:rgb(var(--color-muted))}
.overview-section,.guide-step{margin-top:36px;scroll-margin-top:98px}.section-title{display:flex;align-items:center;gap:11px;margin-bottom:15px}.section-title>span{font:600 11px ui-monospace,monospace;color:rgb(var(--color-terracotta));border:1px solid rgb(var(--color-terracotta)/.25);border-radius:6px;padding:5px 6px}.section-title h2{font-size:20px;margin:0;letter-spacing:-.025em;font-weight:650}p{font-size:13px;line-height:1.9;margin:10px 0}p a,.guide-footer a{color:rgb(var(--color-terracotta));text-underline-offset:3px}.secondary{font-size:12px;color:rgb(var(--color-muted))}.featured-tools{display:grid;grid-template-columns:1fr 1fr;gap:9px;margin:18px 0}.featured-tools button{display:flex;justify-content:space-between;align-items:center;gap:12px;text-align:left;border:1px solid rgb(var(--color-muted)/.2);background:rgb(var(--color-vellum));border-radius:9px;padding:14px 16px;cursor:pointer;color:rgb(var(--color-ink));font-size:12px}.featured-tools button:hover{border-color:rgb(var(--color-terracotta)/.6)}.featured-tools b{font-size:10px;color:rgb(var(--color-terracotta));font-weight:500;white-space:nowrap}.reading-note{border-left:2px solid rgb(var(--color-terracotta)/.55);background:rgb(var(--color-parchment)/.45);padding:16px 18px;margin-top:28px;border-radius:0 7px 7px 0}.reading-note strong{font-size:12px}.reading-note p{font-size:12px;margin:6px 0 0;color:rgb(var(--color-muted))}.warning{border-color:rgb(var(--color-warning))}
.breadcrumb{display:flex;align-items:center;gap:8px;font-size:11px;color:rgb(var(--color-muted));margin-bottom:30px}.breadcrumb button{padding:0;border:0;background:none;cursor:pointer;color:inherit;font:inherit}.client-title{display:flex;gap:16px;align-items:center}.client-title>img{width:42px;height:42px;object-fit:contain}.client-title .eyebrow{margin-bottom:8px}.client-title h1{font-size:35px}.client-description{margin:16px 0 10px;color:rgb(var(--color-muted))}.reference-meta{display:flex;flex-wrap:wrap;gap:7px;color:rgb(var(--color-muted));font-size:10px}.reference-meta span{padding:4px 7px;border:1px solid rgb(var(--color-muted)/.2);border-radius:5px}.os-switch{display:inline-flex;gap:4px;margin-top:27px;background:rgb(var(--color-parchment));padding:4px;border:1px solid rgb(var(--color-muted)/.15);border-radius:8px}.os-switch button{padding:7px 18px;background:transparent;border:0;border-radius:5px;color:rgb(var(--color-muted));font-size:12px;cursor:pointer}.os-switch .selected{background:rgb(var(--color-vellum));color:rgb(var(--color-ink));box-shadow:0 1px 4px rgb(var(--lacquer-base)/.08)}
.guide-terminal{margin:16px 0 20px;box-shadow:0 9px 20px rgb(var(--lacquer-base)/.1)}.guide-terminal :deep(.terminal-bar){padding:11px 14px}.guide-terminal :deep(.terminal-bar b){font-size:11px}.guide-terminal :deep(.terminal-dots i){width:9px;height:9px}.guide-terminal :deep(pre){padding:17px 19px;max-height:28rem}.guide-terminal :deep(code){font-size:12px;line-height:1.85}.credential-window :deep(code){width:100%;white-space:pre-wrap;overflow-wrap:anywhere}.guide-terminal :deep(button){font-size:10px;padding:5px 10px}.inline-note{font-size:12px;padding:12px 14px;background:rgb(var(--color-parchment)/.5);border-radius:7px;line-height:1.8}.inline-note code,li code{font-size:11px}.connection-facts{border:1px solid rgb(var(--color-muted)/.22);border-radius:9px;overflow:hidden;margin:18px 0}.connection-facts>div{display:grid;grid-template-columns:105px minmax(0,1fr);gap:12px;padding:13px 16px;font-size:12px;border-bottom:1px solid rgb(var(--color-muted)/.15)}.connection-facts>div:last-child{border-bottom:0}.connection-facts>div>span{color:rgb(var(--color-muted))}.connection-facts code{display:block;font:11.5px/1.75 ui-monospace,SFMono-Regular,monospace;overflow-wrap:anywhere}.connection-facts strong{font-weight:500}.instruction-list,.more-detail ul{padding-left:20px;font-size:12px;line-height:1.9}.instruction-list li,.more-detail li{margin:7px 0}.more-detail{padding:13px 0;border-bottom:1px solid rgb(var(--color-muted)/.16);font-size:12px}.more-detail summary{cursor:pointer;color:rgb(var(--color-ink));font-weight:600}.more-detail p{font-size:12px}.success-check{display:flex;gap:12px;border:1px solid rgb(var(--color-success)/.25);border-radius:9px;padding:16px;margin-top:20px;background:rgb(var(--color-success)/.035)}.success-check>span{width:22px;height:22px;display:grid;place-items:center;border:1px solid rgb(var(--color-success)/.3);border-radius:50%;color:rgb(var(--color-success));font-size:12px}.success-check strong{font-size:12px}.success-check p{font-size:12px;margin:4px 0 0}.guide-footer{border-top:1px solid rgb(var(--color-muted)/.22);margin-top:40px;padding-top:18px;font-size:11px}.guide-footer>span{margin-right:12px;color:rgb(var(--color-muted))}.guide-footer p{font-size:10px;color:rgb(var(--color-muted))}.guide-toc{position:sticky;top:72px;align-self:start;padding:58px 8px 0}.guide-toc :deep(a){font-size:11px}.toc-tip{margin:28px 10px;font-size:11px;line-height:1.9;color:rgb(var(--color-muted))}
button:focus-visible,a:focus-visible,summary:focus-visible{outline:2px solid rgb(var(--color-terracotta));outline-offset:3px}
@media(min-width:1440px){.guide-main{padding-top:64px}.hero-break{display:block}}
@media(max-width:1120px){.guide-layout{grid-template-columns:210px minmax(0,1fr)}.guide-toc{display:none}.guide-main{padding:40px}}
@media(max-width:720px){.guide-header{height:60px;padding:0 16px;gap:10px}.brand>span{display:none}.preview-label{font-size:9px}.console-link{font-size:10px;padding:6px 8px}.guide-layout{display:block}.guide-navigation{position:static;max-height:none;border-right:0;border-bottom:1px solid rgb(var(--color-muted)/.18);padding:12px 16px}.guide-navigation>p,.nav-heading,.navigation-note{display:none}.guide-navigation nav{display:flex;overflow:auto;gap:5px;padding-top:6px}.guide-nav-item{width:auto;flex-shrink:0;font-size:11px;min-height:34px;padding:6px 9px}.start-item{display:inline-flex}.guide-nav-item small{margin-left:4px}.nav-icon{width:15px;height:15px}.nav-icon{position:relative}.nav-icon b{font-size:9px;color:rgb(var(--color-muted))}.nav-icon img{position:absolute;background:rgb(var(--color-marble));max-width:14px;max-height:14px}.guide-main{padding:30px 20px 48px}h1{font-size:31px}.hero-description{font-size:13px}.overview-steps{gap:8px}.overview-steps a{align-items:flex-start;gap:6px}.overview-steps b{width:24px;height:24px;font-size:10px}.overview-steps span{font-size:11px}.overview-steps small{font-size:9px;line-height:1.6}.featured-tools{grid-template-columns:1fr}.section-title h2{font-size:18px}.client-title h1{font-size:29px}.connection-facts>div{grid-template-columns:77px minmax(0,1fr);padding:11px 12px;gap:8px}.connection-facts code{font-size:10.5px}.guide-terminal :deep(pre){padding:13px;max-height:24rem}.guide-terminal :deep(code){font-size:11px;width:100%;white-space:pre-wrap;overflow-wrap:anywhere}.guide-terminal :deep(pre){max-height:none}.os-switch{width:100%;display:flex}.os-switch button{flex:1;padding:8px 10px}.reference-meta{font-size:9px}.breadcrumb{margin-bottom:22px}}
</style>
