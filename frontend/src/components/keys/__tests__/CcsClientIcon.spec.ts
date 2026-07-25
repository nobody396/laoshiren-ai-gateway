import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import CcsClientIcon from '../CcsClientIcon.vue'
import type { CcsImportTarget } from '@/utils/ccSwitchImport'

const expectedSources: Record<Exclude<CcsImportTarget, 'codex'>, string> = {
  claude: '/brand/client-tools/claude.svg',
  opencode: '/brand/client-tools/opencode.svg',
  openclaw: '/brand/client-tools/openclaw.svg',
  hermes: '/brand/client-tools/hermes.png',
  gemini: '/brand/client-tools/gemini.svg'
}

describe('CcsClientIcon', () => {
  it.each(Object.entries(expectedSources))(
    'renders the official %s brand asset',
    (client, source) => {
      const wrapper = mount(CcsClientIcon, {
        props: { client: client as CcsImportTarget }
      })

      expect(wrapper.attributes('data-client-icon')).toBe(client)
      expect(wrapper.get('img').attributes('src')).toBe(source)
      expect(wrapper.get('img').attributes('alt')).toBe('')
    }
  )

  it('provides the official Codex light and dark application icons', () => {
    const wrapper = mount(CcsClientIcon, {
      props: { client: 'codex' }
    })

    expect(wrapper.findAll('img').map((image) => image.attributes('src'))).toEqual([
      '/brand/client-tools/codex-light.png',
      '/brand/client-tools/codex-dark.png'
    ])
  })
})
