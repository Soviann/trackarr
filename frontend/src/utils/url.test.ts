import { describe, it, expect } from 'vitest'
import { isUrl, detectUrlType } from './url'

describe('isUrl', () => {
  it('accepts an IMDb share link with locale and tracking query', () => {
    // The exact shape Android's IMDb share sends; previously rejected because of
    // the `?ref_=ext_shr` suffix, which made the share flow store the raw URL as
    // the title name.
    expect(isUrl('https://www.imdb.com/fr/title/tt31974288/?ref_=ext_shr')).toBe(true)
  })

  it('accepts plain and scheme-less URLs', () => {
    expect(isUrl('https://www.imdb.com/title/tt31974288/')).toBe(true)
    expect(isUrl('imdb.com/title/tt0111161')).toBe(true)
    expect(isUrl('https://anilist.co/anime/12345/Title/?x=1')).toBe(true)
    expect(isUrl('https://thetvdb.com/series/foo#frag')).toBe(true)
  })

  it('rejects plain titles', () => {
    expect(isUrl('The Matrix')).toBe(false)
    expect(isUrl('Spirited Away')).toBe(false)
  })
})

describe('detectUrlType', () => {
  it('detects an IMDb link with a locale segment', () => {
    expect(detectUrlType('https://www.imdb.com/fr/title/tt31974288/?ref_=ext_shr')).toBe('imdb')
    expect(detectUrlType('https://www.imdb.com/title/tt0111161/')).toBe('imdb')
  })

  it('detects tvdb and anilist', () => {
    expect(detectUrlType('https://thetvdb.com/series/foo')).toBe('tvdb')
    expect(detectUrlType('https://anilist.co/anime/12345')).toBe('anilist')
  })

  it('returns null for non-media input', () => {
    expect(detectUrlType('The Matrix')).toBeNull()
  })
})
