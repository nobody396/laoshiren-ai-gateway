<template>
  <div class="integrations-home">
    <router-link to="/docs" class="back-link">← 返回文档首页</router-link>

    <header class="hero">
      <p>CLIENT INTEGRATIONS</p>
      <h1>工具集成</h1>
      <span>旧版工具配置内容已暂时隐藏。我们正在按“配置位置、Base URL、API Key、模型 ID、验证方法”重写最简教程。</span>
    </header>

    <section class="refresh-notice" aria-label="教程整理状态">
      <strong>教程整理中</strong>
      <p>旧版内容仍保留在代码中，没有删除；确认每个工具的最简配置后再逐个开放。</p>
    </section>

    <section v-if="legacyIntegrationOverviewVisible" class="flow" aria-labelledby="integration-flow-title">
      <div class="section-heading">
        <small>CONFIGURATION FLOW</small>
        <h2 id="integration-flow-title">接入流程</h2>
      </div>
      <ol>
        <li v-for="(step, index) in flowSteps" :key="step"><b>{{ index + 1 }}</b><span>{{ step }}</span></li>
      </ol>
      <p class="flow-note">协议支持只是候选条件。最终必须同时满足当前 Key 的模型目录、模型协议合同、客户端版本和真实 Agent 闭环。</p>
    </section>

    <section class="clients" aria-labelledby="clients-title">
      <div class="section-heading clients-heading">
        <div><small>CLIENT MATRIX</small><h2 id="clients-title">全部 {{ clientMatrix.length }} 个客户端</h2></div>
        <div v-if="legacyIntegrationOverviewVisible" class="status-legend" aria-label="客户端集成状态">
          <span class="status-ready">{{ statusCounts.ready }} 一键导入可用</span>
          <span class="status-prototype">{{ statusCounts.prototype }} 手动配置</span>
          <span class="status-disabled">{{ statusCounts.disabled }} 未开放</span>
        </div>
      </div>

      <div class="client-grid">
        <router-link
          v-for="client in clientMatrix"
          :key="client.slug"
          :to="`/docs/${client.slug}`"
          class="client-card"
          :data-client-status="legacyIntegrationOverviewVisible ? client.one_click_status : 'paused'"
        >
          <div class="client-card-head">
            <span class="client-icon"><b aria-hidden="true">{{ clientInitials(client.name) }}</b><img :src="client.icon" :alt="`${client.name} 官方图标`" @error="hideBrokenIcon" /></span>
            <span><b>{{ client.name }}</b><small>配置基线 {{ client.version }}</small></span>
            <i aria-hidden="true">→</i>
          </div>
          <div v-if="legacyIntegrationOverviewVisible" class="protocols">
            <span v-for="protocol in client.protocols" :key="protocol">{{ protocolLabel(protocol) }}</span>
            <em v-if="client.protocols.length === 0">尚无可公开协议</em>
          </div>
          <footer>
            <span :class="legacyIntegrationOverviewVisible ? `status-${client.one_click_status}` : 'status-paused'">{{ statusLabel(client.one_click_status) }}</span>
            <b>查看教程状态</b>
          </footer>
        </router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { clientMatrix, type ClientMatrixEntry, type ClientMatrixProtocol } from '@/generated/clientMatrix'

const legacyIntegrationOverviewVisible = false
const flowSteps = ['创建 Key / 选择分组', '读取当前 Key 的模型', '核对协议和客户端状态', '按安全合同配置', '完成工具续轮验证']
const statusCounts = computed(() => ({
  ready: clientMatrix.filter(client => client.one_click_status === 'ready').length,
  prototype: clientMatrix.filter(client => client.one_click_status === 'prototype').length,
  disabled: clientMatrix.filter(client => client.one_click_status === 'disabled').length,
}))

function statusLabel(status: ClientMatrixEntry['one_click_status']): string {
  if (!legacyIntegrationOverviewVisible) return '教程整理中'
  if (status === 'ready') return '一键导入可用'
  if (status === 'prototype') return '手动配置'
  return '集成未开放'
}

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
.integrations-home{max-width:62rem;margin:0 auto;padding:3.5rem 1rem 5rem;color:#18181b}.back-link{color:#71717a;font-size:.82rem;text-decoration:none}.hero{margin:3rem 0 2.5rem}.hero p,.section-heading small{color:#1267d6;font:700 .68rem ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:.16em}.hero h1{margin:.7rem 0 0;font-size:clamp(2.6rem,6vw,4rem);letter-spacing:-.05em}.hero span{display:block;max-width:42rem;margin-top:.8rem;color:#71717a;line-height:1.65}.flow,.clients{margin-top:2rem}.section-heading h2{margin:.35rem 0 0;font-size:1.35rem}.flow{border:1px solid #e7dfcf;border-radius:1rem;padding:1.25rem;background:#fbfaf7}.flow ol{display:grid;grid-template-columns:repeat(5,1fr);gap:.6rem;padding:0;margin:1rem 0;list-style:none}.flow li{display:flex;gap:.55rem;align-items:center;font-size:.76rem}.flow li b{display:grid;place-items:center;flex:0 0 auto;width:1.55rem;height:1.55rem;border-radius:50%;background:#1267d6;color:#fff}.flow-note{margin:0;color:#756b5c;font-size:.75rem;line-height:1.6}.clients-heading{display:flex;align-items:flex-end;justify-content:space-between;gap:1rem}.status-legend{display:flex;flex-wrap:wrap;justify-content:flex-end;gap:.35rem}.status-legend span,.client-card footer span{border-radius:999px;padding:.28rem .5rem;font-size:.62rem;font-weight:750}.status-ready{background:#dcfce7;color:#166534}.status-prototype{background:#fef3c7;color:#92400e}.status-disabled{background:#f4f4f5;color:#71717a}.client-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:.85rem;margin-top:1rem}.client-card{border:1px solid #e4e4e7;border-radius:.85rem;background:#fff;padding:1rem;color:inherit;text-decoration:none;transition:transform .15s,border-color .15s}.client-card:hover{transform:translateY(-1px);border-color:#1267d6}.client-card[data-client-status=disabled]{background:#fafafa}.client-card-head{display:flex;align-items:center;gap:.7rem}.client-icon{position:relative;display:grid;place-items:center;width:2.25rem;height:2.25rem;flex:0 0 auto;border-radius:.55rem;background:#f1f5f9;color:#64748b;font:800 .7rem ui-monospace,monospace;overflow:hidden}.client-icon img{position:absolute;inset:0;width:100%;height:100%;object-fit:contain;background:#fff}.client-card-head>span:not(.client-icon){flex:1;min-width:0}.client-card-head b,.client-card-head small{display:block}.client-card-head small{margin-top:.15rem;color:#8b8172;font-size:.65rem}.client-card-head i{font-style:normal;color:#aaa}.protocols{display:flex;flex-wrap:wrap;gap:.3rem;min-height:1.5rem;margin:.8rem 0}.protocols span{border-radius:999px;background:#f3f4f6;padding:.25rem .5rem;color:#52525b;font-size:.62rem}.protocols em{color:#a1a1aa;font-size:.67rem;font-style:normal}.client-card footer{display:flex;align-items:center;justify-content:space-between;gap:.5rem;color:#78716c;font-size:.67rem}.client-card footer b{color:#1267d6}.dark .integrations-home{color:#f4f4f5}.dark .client-card{background:#18181b;border-color:#3f3f46}.dark .client-card[data-client-status=disabled],.dark .flow{background:#202020;border-color:#3f3f46}@media(max-width:760px){.flow ol{grid-template-columns:1fr 1fr}.client-grid{grid-template-columns:1fr}.clients-heading{align-items:flex-start;flex-direction:column}.status-legend{justify-content:flex-start}.integrations-home{padding-top:2rem}}@media(prefers-reduced-motion:reduce){.client-card{transition:none}}
.status-paused{background:#fef3c7;color:#92400e}.refresh-notice{margin:1rem 0 2rem;border:1px solid #fde68a;border-radius:.85rem;background:#fffbeb;padding:1rem;color:#854d0e}.refresh-notice strong{font-size:.86rem}.refresh-notice p{margin:.35rem 0 0;font-size:.76rem;line-height:1.65}
</style>
