import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import Select from '../Select.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Select grouped options', () => {
  it('keeps the matching group header and updates its visible count while searching', async () => {
    const wrapper = mount(Select, {
      attachTo: document.body,
      props: {
        modelValue: null,
        searchable: true,
        options: [
          {
            value: 'group:monthly',
            label: 'Monthly plans',
            kind: 'group',
            groupKey: 'monthly',
            count: 2,
            disabled: true
          },
          { value: 1, label: 'Claude monthly', groupKey: 'monthly' },
          { value: 2, label: 'GPT monthly', groupKey: 'monthly' },
          {
            value: 'group:payg',
            label: 'Pay as you go',
            kind: 'group',
            groupKey: 'payg',
            count: 2,
            disabled: true
          },
          { value: 3, label: 'Grok', groupKey: 'payg', description: 'Fast model' },
          { value: 4, label: 'Claude Bedrock', groupKey: 'payg', description: 'AWS model' }
        ]
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.get('button').trigger('click')
    const listbox = document.body.querySelector<HTMLElement>('[role="listbox"]')
    expect(listbox).not.toBeNull()

    const searchInput = listbox!.querySelector<HTMLInputElement>('input')
    expect(searchInput).not.toBeNull()
    await searchInput!.focus()
    await wrapper.findComponent(Select).vm.$nextTick()
    searchInput!.value = 'Bedrock'
    searchInput!.dispatchEvent(new Event('input'))
    await wrapper.findComponent(Select).vm.$nextTick()

    const optionText = Array.from(listbox!.querySelectorAll('[role="option"]'))
      .map((element) => element.textContent?.replace(/\s+/g, ' ').trim())

    expect(optionText).toEqual(['Pay as you go', 'Claude Bedrock'])
    expect(
      listbox!.querySelector('[role="option"][data-group-key="payg"]')
        ?.getAttribute('data-option-count')
    ).toBe('1')
    expect(listbox!.textContent).not.toContain('Monthly plans')

    wrapper.unmount()
  })
})
