import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import KeyGroupSelector from '../KeyGroupSelector.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const options = [
  {
    value: 6,
    label: 'GPT 标准线路',
    description: 'OpenAI GPT 标准线路',
    rate: 0.5,
    userRate: null,
    subscriptionType: 'standard' as const,
    platform: 'openai' as const,
    cacheHitRatePct: null,
    cacheWindowDays: 7,
    familyKey: 'openai' as const
  },
  {
    value: 65,
    label: 'Claude 经济线路',
    description: 'Claude 日常开发线路',
    rate: 1,
    userRate: null,
    subscriptionType: 'standard' as const,
    platform: 'anthropic' as const,
    cacheHitRatePct: null,
    cacheWindowDays: 7,
    familyKey: 'claude' as const
  }
]

afterEach(() => {
  document.body.innerHTML = ''
})

describe('KeyGroupSelector', () => {
  it.each(['field', 'inline'] as const)('uses the same bounded popup in %s mode', async (variant) => {
    const wrapper = mount(KeyGroupSelector, {
      attachTo: document.body,
      props: {
        modelValue: 6,
        options,
        variant,
        placeholder: '选择分组',
        searchPlaceholder: '搜索分组...'
      }
    })

    await wrapper.get('[data-testid="key-group-selector-trigger"]').trigger('click')

    const popup = document.body.querySelector<HTMLElement>('[data-testid="key-group-selector-popup"]')
    expect(popup).not.toBeNull()
    expect(popup!.className).toContain('w-[min(560px,calc(100vw-24px))]')
    expect(popup!.style.left).toMatch(/px$/)

    const claudeFamily = [...popup!.querySelectorAll<HTMLButtonElement>('button')]
      .find((button) => button.textContent?.includes('keys.groupFamilies.claude'))
    expect(claudeFamily).toBeDefined()
    claudeFamily!.click()
    await wrapper.vm.$nextTick()

    const option = document.body.querySelector<HTMLButtonElement>('[data-option-value="65"]')
    expect(option).not.toBeNull()
    await option!.click()
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([65])

    wrapper.unmount()
  })

  it('filters the shared option list without changing its viewport bounds', async () => {
    const wrapper = mount(KeyGroupSelector, {
      attachTo: document.body,
      props: {
        modelValue: null,
        options,
        searchPlaceholder: '搜索分组...'
      }
    })

    await wrapper.get('[data-testid="key-group-selector-trigger"]').trigger('click')
    const search = document.body.querySelector<HTMLInputElement>('[data-testid="key-group-selector-search"]')!
    search.value = 'Claude'
    search.dispatchEvent(new Event('input'))
    await wrapper.vm.$nextTick()

    expect(document.body.querySelector('[data-option-value="65"]')).not.toBeNull()
    expect(document.body.querySelector('[data-option-value="6"]')).toBeNull()

    wrapper.unmount()
  })
})
