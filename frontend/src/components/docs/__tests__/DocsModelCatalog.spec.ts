import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const getPublicModelPricing = vi.fn()

vi.mock('@/api/publicPricing', () => ({
  getPublicModelPricing: (...args: unknown[]) => getPublicModelPricing(...args),
}))

import DocsModelCatalog from '../DocsModelCatalog.vue'

describe('DocsModelCatalog', () => {
  it('renders one logical model with multiple availability plans', async () => {
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-28T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [
        {
          group_id: 10,
          name: 'GLM Plus 月卡组',
          platform: 'openai',
          rate_multiplier: 0.5,
          is_exclusive: true,
          subscription_type: 'credit',
          models: [{ model: 'glm-5.3', input_price: 1, output_price: 2, cache_read_price: 0.1 }],
        },
        {
          group_id: 11,
          name: 'GLM 公开分组',
          platform: 'openai',
          rate_multiplier: 0.35,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [
            { model: 'glm-5.3', input_price: 1, output_price: 2, cache_read_price: 0.1 },
            { model: 'gpt-hidden', input_price: 1, output_price: 2, cache_read_price: 0.1, disabled: true },
          ],
        },
      ],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    const articles = wrapper.findAll('article')
    expect(articles).toHaveLength(1)
    expect(articles[0].get('h2').text()).toBe('glm-5.3')
    expect(articles[0].findAll('[data-client-status="verified"]')).toHaveLength(0)
    expect(articles[0].find('.model-tool-recommended').exists()).toBe(false)
    expect(articles[0].text()).toContain('2 个可用分组')
    expect(articles[0].text()).toContain('月卡组')
    expect(articles[0].text()).not.toContain('GLM Plus 月卡组')
    expect(articles[0].text()).toContain('GLM 公开分组')
    expect(articles[0].text().indexOf('GLM 公开分组')).toBeLessThan(articles[0].text().indexOf('月卡组'))
    expect(wrapper.text()).not.toContain('gpt-hidden')

    expect(wrapper.find('section[aria-labelledby="monthly-plan-heading"]').exists()).toBe(false)
  })

  it('keeps the dedicated image product separate from text models', async () => {
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-28T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [
        {
          group_id: 51,
          name: 'GPT Image 2 生图分组',
          platform: 'openai',
          rate_multiplier: 4,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [],
          image_generation: {
            mode: 'token',
            text_input_price: 20,
            image_input_price: 32,
            image_output_price: 120,
          },
        },
      ],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(wrapper.get('h2').text()).toBe('gpt-image-2')
    expect(wrapper.text()).toContain('Images API')
    expect(wrapper.text()).toContain('GPT Image 2 生图分组')
    expect(wrapper.text()).toContain('文本输入 ¥20/M · 图片输入 ¥32/M · 图片输出 ¥120/M')
  })

  it('renders verified per-model protocols instead of treating every OpenAI-platform model alike', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-30T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [{
        group_id: 60,
        name: '阿里云模型',
        platform: 'openai',
        rate_multiplier: 0.95,
        is_exclusive: false,
        subscription_type: 'standard',
        models: [
          { model: 'glm-5.3', input_price: 1, output_price: 2, cache_read_price: 0.1 },
          { model: 'qwen3.8-max', input_price: 1, output_price: 2, cache_read_price: 0.1 },
          { model: 'deepseek-v4-pro-0813', input_price: 1, output_price: 2, cache_read_price: 0.1 },
        ],
      }],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    const articleCards = Object.fromEntries(wrapper.findAll('article').map(card => [card.get('h2').text(), card]))
    const cards = Object.fromEntries(Object.entries(articleCards).map(([model, card]) => [model, card.text()]))
    const glmCard = articleCards['glm-5.3']
    expect(cards['glm-5.3']).toContain('Chat Completions')
    expect(cards['glm-5.3']).toContain('模型协议')
    expect(cards['glm-5.3']).toContain('1,048,576')
    expect(cards['glm-5.3']).toContain('131,072')
    expect(cards['glm-5.3']).toContain('仅文本输出')
    expect(cards['glm-5.3']).not.toContain('OpenAI Responses')
    expect(cards['glm-5.3']).not.toContain('标准按量')
    expect(cards['glm-5.3']).toContain('推荐')
    expect(cards['glm-5.3']).toContain('https://api.laoshirenai.com/v1')
    expect(glmCard.findAll('[data-client-status="verified"]')).toHaveLength(0)
    expect(glmCard.find('.model-tool-recommended').exists()).toBe(false)
    expect(glmCard.findAll('[data-input-modality]').map(item => [
      item.attributes('data-input-modality'),
      item.attributes('data-supported'),
    ])).toEqual([
      ['text', 'true'],
      ['image', 'false'],
      ['video', 'false'],
    ])
    expect(glmCard.findAll('details[aria-label="推理强度兼容"] code').map(item => item.text())).toEqual(['low', 'high', 'max'])
    await glmCard.get('button[aria-label="复制 glm-5.3 Base URL"]').trigger('click')
    expect(writeText).toHaveBeenCalledWith('https://api.laoshirenai.com/v1')
    expect(glmCard.get('button[aria-label="复制 glm-5.3 Base URL"]').text()).toContain('已复制')
    expect(glmCard.findAll('img[alt$="图标"]')).toHaveLength(0)
    expect(cards['qwen3.8-max']).toContain('Responses')
    expect(cards['qwen3.8-max']).toContain('Chat Completions')
    expect(cards['qwen3.8-max']).toContain('Codex')
    expect(articleCards['qwen3.8-max'].get('.model-protocol-recommended').text()).toBe('推荐')
    expect(articleCards['deepseek-v4-pro-0813']).toBeDefined()
    expect(articleCards['deepseek-v4-pro-0813'].attributes('data-contract-state')).toBe('verified')
  })

  it('filters the catalog with model-family category tags', async () => {
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-30T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [
        {
          group_id: 60,
          name: '国产模型',
          platform: 'openai',
          rate_multiplier: 1,
          models: [
            { model: 'glm-5.3', input_price: 1, output_price: 2 },
            { model: 'qwen3.8-max', input_price: 1, output_price: 2 },
          ],
        },
        {
          group_id: 34,
          name: 'Grok',
          platform: 'grok',
          rate_multiplier: 1,
          models: [{ model: 'grok-4.6', input_price: 1, output_price: 2 }],
        },
        {
          group_id: 57,
          name: 'Gemini',
          platform: 'gemini',
          rate_multiplier: 1,
          models: [{ model: 'gemini-3.7-flash', input_price: 1, output_price: 2 }],
        },
      ],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    const categoryButtons = wrapper.findAll('nav[aria-label="模型分类"] button')
    expect(categoryButtons).toHaveLength(7)
    expect(categoryButtons.map(button => button.text())).toEqual([
      '全部4',
      '国产模型2',
      'GPT0',
      'Claude0',
      'Grok1',
      'Gemini1',
      '生图0',
    ])

    await categoryButtons.find(button => button.text().startsWith('国产模型'))!.trigger('click')
    expect(wrapper.findAll('article').map(card => card.get('h2').text())).toEqual(['glm-5.3', 'qwen3.8-max'])

    const domesticButtons = wrapper.findAll('nav[aria-label="国产模型分类"] button')
    expect(domesticButtons.map(button => button.text())).toEqual([
      '全部国产2',
      'GLM1',
      'Qwen1',
      'DeepSeek0',
      'Kimi0',
      'MiniMax0',
    ])

    await domesticButtons.find(button => button.text().startsWith('GLM'))!.trigger('click')
    expect(wrapper.findAll('article').map(card => card.get('h2').text())).toEqual(['glm-5.3'])

    await domesticButtons.find(button => button.text().startsWith('Qwen'))!.trigger('click')
    expect(wrapper.findAll('article').map(card => card.get('h2').text())).toEqual(['qwen3.8-max'])

    await categoryButtons.find(button => button.text().startsWith('Grok'))!.trigger('click')
    expect(wrapper.findAll('article').map(card => card.get('h2').text())).toEqual(['grok-4.6'])
  })

  it('keeps incomplete inventory models visible and labels their evidence state', async () => {
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-30T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [{
        group_id: 1,
        name: '排序测试',
        platform: 'openai',
        rate_multiplier: 1,
        models: [
          { model: 'grok-4.5', input_price: 1, output_price: 2 },
          { model: 'gpt-5.3-codex-spark', input_price: 1, output_price: 2 },
          { model: 'qwen3.6-flash', input_price: 1, output_price: 2 },
          { model: 'gpt-5.4-mini', input_price: 1, output_price: 2 },
          { model: 'grok-4.6', input_price: 1, output_price: 2 },
          { model: 'gpt-5.6-sol', input_price: 1, output_price: 2 },
          { model: 'qwen3.8-max', input_price: 1, output_price: 2 },
          { model: 'gpt-5.5', input_price: 1, output_price: 2 },
        ],
      }],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.findAll('article').map(card => card.get('h2').text())).toEqual([
      'gpt-5.6-sol',
      'gpt-5.5',
      'gpt-5.4-mini',
      'gpt-5.3-codex-spark',
      'qwen3.8-max',
      'qwen3.6-flash',
      'grok-4.6',
      'grok-4.5',
    ])
    const spark = wrapper.findAll('[data-model-card]').find(card => card.attributes('data-model-id') === 'gpt-5.3-codex-spark')!
    expect(spark.attributes('data-contract-state')).toBe('verified')
    expect(spark.text()).not.toContain('验证中')
    expect(spark.get('[data-field="max-output-tokens"]').text()).toContain('未公开')
    expect(spark.get('[data-field="max-output-tokens"]').text()).not.toContain('待确认')
    expect(spark.findAll('[data-protocol-status="verified"]').length).toBeGreaterThan(0)
    // Blocked client candidates are summarized, never rendered as supported
    // client cards beside the exact Codex receipt.
    expect(spark.findAll('[data-client-status="blocked"]')).toHaveLength(0)
    // spark 仅声明 none 档位（官方未公开推理控制）：推理区如实展示已验证的 none 档位
    expect(spark.text()).toContain('1 个档位已验证')
  })

  it('publishes only M8-final clients that also have an exact current-model receipt', async () => {
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-31T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [{
        group_id: 1,
        name: 'GPT 标准线路',
        platform: 'openai',
        rate_multiplier: 0.5,
        is_exclusive: false,
        subscription_type: 'standard',
        models: [{ model: 'gpt-5.6-sol', input_price: 1, output_price: 2 }],
      }],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    const card = wrapper.get('[data-model-id="gpt-5.6-sol"]')
    const tools = card.get('[data-field="tools"]')
    const verified = card.findAll('[data-client-status="verified"]')
    expect(verified.map(client => client.text())).toEqual(expect.arrayContaining([
      expect.stringContaining('Codex'),
      expect.stringContaining('Hermes Agent'),
      expect.stringContaining('OpenCode'),
    ]))
    // 六家全部有 verified OS 单元格（Codex/Hermes/OpenCode 为精确 receipt，Kimi/ZCode/Grok Build 为矩阵交集推导）
    expect(verified).toHaveLength(6)
    expect(tools.text()).toContain('Grok Build')
    expect(tools.text()).toContain('Kimi Code')
    expect(tools.text()).toContain('ZCode')
    expect(card.findAll('.model-tool-recommended')).toHaveLength(1)
    expect(card.get('.model-tool-recommended').element.parentElement?.textContent).toContain('Codex')
  })

  it('renders all 34 public inventory models with every required field', async () => {
    const inventoryModelIds = [
      'gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-daybreak-blue-latest', 'gpt-5.6-luna',
      'gpt-5.5', 'gpt-5.4', 'gpt-5.4-mini', 'gpt-5.3-codex-spark',
      'claude-opus-5', 'claude-sonnet-5', 'claude-fable-5', 'claude-opus-4-8',
      'claude-opus-4-7', 'claude-opus-4-6', 'claude-sonnet-4-6', 'claude-opus-4-5',
      'claude-haiku-4-5', 'glm-5.3', 'glm-5.2', 'qwen3.8-max', 'qwen3.7-max',
      'qwen3.7-plus', 'qwen3.7-flash', 'qwen3.6-plus', 'qwen3.6-flash',
      'deepseek-v4-pro-0813', 'deepseek-v4-flash-0731', 'kimi-k3', 'kimi-k2.7-code',
      'minimax-m3', 'grok-4.6', 'grok-4.5', 'gemini-3.7-flash', 'gemini-3.1-pro',
    ]
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-31T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [{
        group_id: 60,
        name: '公开模型目录',
        platform: 'openai',
        rate_multiplier: 1,
        is_exclusive: false,
        subscription_type: 'standard',
        models: inventoryModelIds.map(model => ({ model, input_price: 1, output_price: 2 })),
      }],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    const cards = wrapper.findAll('[data-model-card]')
    expect(cards).toHaveLength(34)
    expect(new Set(cards.map(card => card.attributes('data-model-id')))).toEqual(new Set(inventoryModelIds))
    const requiredFields = [
      'context-window', 'max-output-tokens', 'groups', 'model-protocols',
      'native-io', 'base-url', 'tools', 'reasoning',
    ]
    for (const card of cards) {
      for (const field of requiredFields) expect(card.find(`[data-field="${field}"]`).exists()).toBe(true)
      expect(card.get('[data-field="base-url"]').text()).toContain('https://api.laoshirenai.com')
    }
  })
})
