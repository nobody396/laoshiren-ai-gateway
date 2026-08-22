import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import PricingEntryCard from '../PricingEntryCard.vue'
import type { PricingFormEntry } from '../types'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (_key: string, fallback: string) => fallback })
  }
})

function entry(): PricingFormEntry {
  return {
    models: ['gpt-5.4'],
    billing_mode: 'token',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    fast_multiplier: 2,
    flex_multiplier: 0.5,
    fast_supported: false,
    flex_supported: false,
    fast_verified_at: null,
    flex_verified_at: null,
    image_output_price: null,
    per_request_price: null,
    intervals: []
  }
}

describe('PricingEntryCard service-tier capability controls', () => {
  it('keeps capability controls hidden for supplier-cost-only pricing rows', () => {
    const wrapper = mount(PricingEntryCard, {
      props: { entry: entry(), platform: 'openai' },
      global: { stubs: { Icon: true, Select: true, ModelTagInput: true, IntervalRow: true } }
    })

    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
  })

  it('records an explicit timestamp when Fast capability is confirmed', async () => {
    const wrapper = mount(PricingEntryCard, {
      props: { entry: entry(), platform: 'openai', serviceTierCapability: true },
      global: { stubs: { Icon: true, Select: true, ModelTagInput: true, IntervalRow: true } }
    })

    const checkbox = wrapper.findAll<HTMLInputElement>('input[type="checkbox"]')[0]
    await checkbox.setValue(true)
    const update = wrapper.emitted('update')?.[0]?.[0] as PricingFormEntry

    expect(update.fast_supported).toBe(true)
    expect(update.fast_verified_at).toMatch(/^\d{4}-\d{2}-\d{2}T/)
  })
})
