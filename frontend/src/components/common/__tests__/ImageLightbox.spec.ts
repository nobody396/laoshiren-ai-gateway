import { afterEach, describe, expect, it } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'

import ImageLightbox from '../ImageLightbox.vue'

let wrapper: VueWrapper | null = null

const mountLightbox = () => {
  wrapper = mount(ImageLightbox, {
    attachTo: document.body,
    props: {
      show: true,
      src: 'https://example.test/receipt.jpg',
      title: 'Receipt — Full Size',
      alt: 'Payment receipt',
      closeLabel: 'Close receipt',
      openExternalLabel: 'Open in New Window',
      failedLabel: 'Failed to load receipt'
    },
    global: {
      stubs: {
        Icon: true
      }
    }
  })
  return wrapper
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
  document.body.classList.remove('modal-open')
})

describe('ImageLightbox', () => {
  it('renders a full-size responsive image and a new-window fallback', async () => {
    mountLightbox()
    await nextTick()

    const dialog = document.querySelector('[data-test="image-lightbox"]')
    const image = document.querySelector<HTMLImageElement>('[data-test="image-lightbox-image"]')
    const externalLink = document.querySelector<HTMLAnchorElement>('[data-test="image-lightbox-external"]')

    expect(dialog?.getAttribute('role')).toBe('dialog')
    expect(dialog?.getAttribute('aria-modal')).toBe('true')
    expect(image?.src).toBe('https://example.test/receipt.jpg')
    expect(image?.alt).toBe('Payment receipt')
    expect(image?.className).toContain('max-h-[calc(100dvh-6.75rem)]')
    expect(externalLink?.href).toBe('https://example.test/receipt.jpg')
    expect(externalLink?.target).toBe('_blank')
    expect(externalLink?.rel).toContain('noopener')
  })

  it('closes from the close button, backdrop, and Escape key', async () => {
    const mounted = mountLightbox()
    await nextTick()

    const closeButton = document.querySelector<HTMLButtonElement>('[data-test="image-lightbox-close"]')
    closeButton?.click()
    expect(mounted.emitted('close')).toHaveLength(1)

    const stage = document.querySelector<HTMLElement>('[data-test="image-lightbox-stage"]')
    stage?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(mounted.emitted('close')).toHaveLength(2)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(mounted.emitted('close')).toHaveLength(3)
  })

  it('keeps the external fallback available when the inline image fails', async () => {
    const mounted = mountLightbox()
    await nextTick()

    const image = document.querySelector<HTMLImageElement>('[data-test="image-lightbox-image"]')
    image?.dispatchEvent(new Event('error'))
    await nextTick()

    expect(mounted.emitted('error')).toHaveLength(1)
    expect(document.body.textContent).toContain('Failed to load receipt')
    expect(document.querySelector('[data-test="image-lightbox-external"]')).not.toBeNull()
  })
})
