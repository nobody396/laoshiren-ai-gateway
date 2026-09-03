import { describe, expect, it } from 'vitest'
import { clientAutoConfigOptionsForModel, preferredAutoConfigProtocol } from '../clientAutoConfigSelection'

describe('matrix-driven client auto-config selection', () => {
  it('only returns ready clients whose native protocol intersects the model', () => {
    const qwen = clientAutoConfigOptionsForModel('qwen3.7-max')
    expect(qwen.map(option => option.client.id)).toEqual(['codex', 'grok-build'])
    expect(qwen.find(option => option.client.id === 'codex')?.protocols).toEqual(['responses'])
    expect(qwen.find(option => option.client.id === 'grok-build')?.protocols).toEqual(['responses'])

    const claude = clientAutoConfigOptionsForModel('claude-opus-5')
    expect(claude.map(option => option.client.id)).toEqual(['claude-code', 'grok-build'])

    const gemini = clientAutoConfigOptionsForModel('gemini-3.7-flash')
    expect(gemini.map(option => option.client.id)).toEqual(['gemini-cli'])
  })

  it('prefers the model recommendation and never returns removed or prototype clients', () => {
    expect(preferredAutoConfigProtocol('qwen3.7-max', ['chat_completions', 'responses'])).toBe('responses')
    const all = clientAutoConfigOptionsForModel('qwen3.7-max').map(option => option.client.id)
    expect(all).not.toEqual(expect.arrayContaining(['traecode', 'cursor-desktop', 'doubao-work', 'qoder', 'workbuddy']))
    expect(clientAutoConfigOptionsForModel('does-not-exist')).toEqual([])
  })
})
