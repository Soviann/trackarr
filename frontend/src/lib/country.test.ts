import { describe, it, expect } from 'vitest'
import { countryFlag, countryName, countryLabel } from './country'

describe('countryFlag', () => {
  it('builds the KR regional-indicator emoji', () => {
    expect(countryFlag('kr')).toBe('\u{1F1F0}\u{1F1F7}')
  })

  it('returns empty string for invalid codes', () => {
    expect(countryFlag('KOR')).toBe('')
  })
})

describe('countryName', () => {
  it('returns a non-empty English name for a valid code', () => {
    expect(countryName('KR')).toBe('South Korea')
  })

  it('falls back to the raw code when Intl.DisplayNames throws', () => {
    expect(countryName('123')).toBe('123')
  })
})

describe('countryLabel', () => {
  it('combines flag and name', () => {
    expect(countryLabel('KR')).toBe('\u{1F1F0}\u{1F1F7} South Korea')
  })
})
