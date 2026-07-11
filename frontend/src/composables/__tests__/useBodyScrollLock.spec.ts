import { defineComponent, nextTick, ref } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { useBodyScrollLock } from '../useBodyScrollLock'

const wrappers: VueWrapper[] = []

function mountLock(locked: ReturnType<typeof ref<boolean>>): VueWrapper {
  const wrapper = mount(defineComponent({
    setup() {
      useBodyScrollLock(locked)
      return () => null
    }
  }))
  wrappers.push(wrapper)
  return wrapper
}

describe('useBodyScrollLock', () => {
  beforeEach(() => {
    document.body.style.overflow = ''
  })

  afterEach(() => {
    wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
    document.body.style.overflow = ''
  })

  it('restores scrolling after the final lock closes', async () => {
    const locked = ref(false)
    mountLock(locked)

    locked.value = true
    await nextTick()
    expect(document.body.style.overflow).toBe('hidden')

    locked.value = false
    await nextTick()
    expect(document.body.style.overflow).toBe('')
  })

  it('keeps scrolling locked while another overlay remains open', async () => {
    const first = ref(true)
    const second = ref(true)
    mountLock(first)
    mountLock(second)

    first.value = false
    await nextTick()
    expect(document.body.style.overflow).toBe('hidden')

    second.value = false
    await nextTick()
    expect(document.body.style.overflow).toBe('')
  })

  it('restores the previous inline overflow value on unmount', () => {
    document.body.style.overflow = 'clip'
    const wrapper = mountLock(ref(true))

    expect(document.body.style.overflow).toBe('hidden')
    wrapper.unmount()
    wrappers.splice(wrappers.indexOf(wrapper), 1)
    expect(document.body.style.overflow).toBe('clip')
  })
})
