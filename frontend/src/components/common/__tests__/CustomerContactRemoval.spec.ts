import { describe, expect, it } from 'vitest'
import appHeaderSource from '@/components/layout/AppHeader.vue?raw'
import docsLayoutSource from '@/components/docs/DocsLayout.vue?raw'
import profileViewSource from '@/views/user/ProfileView.vue?raw'
import redeemViewSource from '@/views/user/RedeemView.vue?raw'

describe('customer contact removal', () => {
  it('does not render configured personal contact details on customer surfaces', () => {
    for (const source of [appHeaderSource, docsLayoutSource, profileViewSource, redeemViewSource]) {
      expect(source).not.toContain('CustomerServiceButton')
      expect(source).not.toContain('contactInfo')
    }
  })
})
