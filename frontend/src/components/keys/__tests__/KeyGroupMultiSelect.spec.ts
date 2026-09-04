import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import KeyGroupMultiSelect from '../KeyGroupMultiSelect.vue'
import type { Group } from '@/types'

const groups = [{ id: 6, name: 'GPT 标准', platform: 'openai', rate_multiplier: 1, subscription_type: 'standard' }, { id: 5, name: 'Claude 月卡', platform: 'anthropic', rate_multiplier: 1, subscription_type: 'subscription' }] as Group[]
describe('KeyGroupMultiSelect', () => {
  it('shows explicit priority and allows shrinking authorization', async () => {
    const wrapper = mount(KeyGroupMultiSelect, { props: { modelValue: [6, 5], groups } })
    expect(wrapper.get('[aria-label="分组优先级"]').text()).toContain('GPT')
    await wrapper.findAll('input[type=checkbox]')[0].setValue(false)
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[5]])
  })
  it('selects all eligible groups and leaves explicit empty selection empty', async () => {
    const wrapper = mount(KeyGroupMultiSelect, { props: { modelValue: [], groups } })
    await wrapper.findAll('button').find(button => button.text() === '全选可用分组')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[5, 6]])
    await wrapper.findAll('button').find(button => button.text() === '清空')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[]])
  })
  it('preserves unavailable saved group IDs visibly instead of silently dropping them', () => {
    const wrapper = mount(KeyGroupMultiSelect, { props: { modelValue: [99, 6], groups } })
    expect(wrapper.text()).toContain('原分组 #99')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
