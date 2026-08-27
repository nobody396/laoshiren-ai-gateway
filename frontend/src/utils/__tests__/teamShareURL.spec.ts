import { describe, expect, it } from 'vitest'
import { absoluteTeamShareURL } from '../teamShareURL'

describe('absoluteTeamShareURL', () => {
  it('converts a relative one-time link to the current site', () => {
    expect(absoluteTeamShareURL('/team?invitation=token', 'https://laoshirenai.com')).toBe(
      'https://laoshirenai.com/team?invitation=token',
    )
  })

  it('preserves a configured absolute URL and rejects missing values', () => {
    expect(absoluteTeamShareURL('https://portal.example/team?transfer=token', 'https://laoshirenai.com')).toBe(
      'https://portal.example/team?transfer=token',
    )
    expect(absoluteTeamShareURL(undefined, 'https://laoshirenai.com')).toBe('')
  })
})
