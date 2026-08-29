import { describe, it, expect } from 'vitest'
import { aniListMediaUrl, computeAniListUrl, getName, getAlternativeNames, getTypeLabel, getStatusLabel, formatMatchSource, formatDate, formatDateTime24h, hexToRgba, watchedCount, totalEpisodes, getCoverUrl } from './utils'
import type { Title, TitleName, Season, Episode, TitleType } from './types'

function makeTitle(overrides: Partial<Title> = {}): Title {
  return {
    id: 1, type: 'movie', is_anime: false, year: 2024, cover_url: null, accent_hex: null, imdb_id: null,
    simkl_id: null, simkl_slug: null, anilist_id: null,
    tmdb_id: null, tvdb_id: null, my_rating: null, status: 'watching', series_status: null,
    match_status: 'confirmed', original_title: null, match_source: null, names: [], seasons: [],
    overview: null, genres: null, runtime: null, tmdb_rating: null, credits: null,
    anilist_rating: null, release_date: null, total_watch_minutes: 0, created_at: '', updated_at: '',
    ...overrides,
  }
}

function makeName(name: string, lang: string, primary: boolean): TitleName {
  return { id: 1, title_id: 1, name, language: lang, is_primary: primary }
}

function makeEpisode(watched: boolean): Episode {
  return { id: 1, season_id: 1, episode: 1, name: null, air_date: null, watched, first_watched_at: null, last_watched_at: null }
}

function makeSeason(episodes: Episode[]): Season {
  return { id: 1, title_id: 1, season_number: 1, total_episodes: null, episodes }
}

describe('getName', () => {
  it('prefers French over English by default', () => {
    const t = makeTitle({ names: [makeName('Eng', 'en', true), makeName('Fr', 'fr', false)] })
    expect(getName(t, 'fr')).toBe('Fr')
  })

  it('prefers English when preferredLang is en', () => {
    const t = makeTitle({ names: [makeName('Eng', 'en', true), makeName('Fr', 'fr', false)] })
    expect(getName(t, 'en')).toBe('Eng')
  })

  it('prefers German when preferredLang is de', () => {
    const t = makeTitle({ names: [makeName('Eng', 'en', true), makeName('Fr', 'fr', false), makeName('De', 'de', false)] })
    expect(getName(t, 'de')).toBe('De')
  })

  it('falls back to English when preferred language is not present', () => {
    const t = makeTitle({ names: [makeName('Eng', 'en', false), makeName('Alt', 'it', false)] })
    expect(getName(t, 'de')).toBe('Eng')
  })

  it('falls back to romaji for anime when no preferred/en/fr', () => {
    const t = makeTitle({
      is_anime: true,
      names: [makeName('Shingeki no Kyojin', 'x-romaji', false), makeName('進撃の巨人', 'ja', false)],
    })
    expect(getName(t)).toBe('Shingeki no Kyojin')
  })

  it('falls back to ja when anime has no romaji', () => {
    const t = makeTitle({ is_anime: true, names: [makeName('進撃の巨人', 'ja', false)] })
    expect(getName(t)).toBe('進撃の巨人')
  })

  it('does not use romaji fallback for non-anime', () => {
    const t = makeTitle({ is_anime: false, names: [makeName('Something', 'x-romaji', false), makeName('Alt', 'de', false)] })
    expect(getName(t)).toBe('Something')
  })

  it('falls back to first name when nothing matches', () => {
    const t = makeTitle({ names: [makeName('De', 'de', false)] })
    expect(getName(t)).toBe('De')
  })

  it('returns (untitled) for empty names', () => {
    expect(getName(makeTitle())).toBe('(untitled)')
    expect(getName(makeTitle({ names: undefined as unknown as TitleName[] }))).toBe('(untitled)')
  })
})

describe('getAlternativeNames', () => {
  it('returns alternative names ordered with preferred language first', () => {
    const t = makeTitle({
      names: [
        makeName('Inception', 'en', true),
        makeName('Origine', 'fr', false),
        makeName('Inception (DE)', 'de', false),
      ],
    })
    const altFR = getAlternativeNames(t, 'fr')
    // Primary was 'Origine', alternatives should be 'Inception' (en) and 'Inception (DE)' (de)
    expect(altFR.map(a => a.name)).toEqual(['Inception', 'Inception (DE)'])

    const altEN = getAlternativeNames(t, 'en')
    // Primary was 'Inception', alternatives should be 'Origine' (fr) and 'Inception (DE)' (de)
    expect(altEN.map(a => a.name)).toEqual(['Origine', 'Inception (DE)'])
  })
})

describe('getTypeLabel', () => {
  const cases: [TitleType, string][] = [
    ['movie', 'Movie'],
    ['series', 'Series'],
  ]
  it.each(cases)('%s → %s', (input, expected) => {
    expect(getTypeLabel(input)).toBe(expected)
  })
})

describe('getStatusLabel', () => {
  const cases: [string, string][] = [
    ['watching', 'Watching'],
    ['completed', 'Completed'],
    ['dropped', 'Dropped'],
    ['plan_to_watch', 'Plan to Watch'],
  ]
  it.each(cases)('%s → %s', (input, expected) => {
    expect(getStatusLabel(input as any)).toBe(expected)
  })
})

describe('formatDate', () => {
  it('formats a valid date', () => {
    const result = formatDate('2024-03-15')
    expect(result).toContain('Mar')
    expect(result).toContain('15')
    expect(result).toContain('2024')
  })

  it('returns empty for null/undefined', () => {
    expect(formatDate(null)).toBe('')
    expect(formatDate(undefined)).toBe('')
  })

  it('returns empty for invalid date', () => {
    expect(formatDate('not-a-date')).toBe('')
  })
})

describe('formatDateTime24h', () => {
  it('formats valid ISO date into DD/MM/YYYY, HH:mm format', () => {
    const d = new Date(2026, 7, 19, 14, 5) // August 19, 2026 at 14:05
    expect(formatDateTime24h(d.toISOString())).toBe('19/08/2026, 14:05')
  })

  it('returns empty for null/undefined/invalid', () => {
    expect(formatDateTime24h(null)).toBe('')
    expect(formatDateTime24h(undefined)).toBe('')
    expect(formatDateTime24h('invalid-date')).toBe('')
  })
})

describe('watchedCount', () => {
  it('counts watched episodes across seasons', () => {
    const t = makeTitle({
      seasons: [
        makeSeason([makeEpisode(true), makeEpisode(false)]),
        makeSeason([makeEpisode(true), makeEpisode(true)]),
      ],
    })
    expect(watchedCount(t)).toBe(3)
  })

  it('returns 0 for no seasons', () => {
    expect(watchedCount(makeTitle())).toBe(0)
  })

  it('handles null seasons/episodes', () => {
    expect(watchedCount(makeTitle({ seasons: undefined as unknown as Season[] }))).toBe(0)
    const t = makeTitle({ seasons: [{ ...makeSeason([]), episodes: null as unknown as Episode[] }] })
    expect(watchedCount(t)).toBe(0)
  })
})

describe('totalEpisodes', () => {
  it('counts all episodes', () => {
    const t = makeTitle({
      seasons: [makeSeason([makeEpisode(true), makeEpisode(false)]), makeSeason([makeEpisode(true)])],
    })
    expect(totalEpisodes(t)).toBe(3)
  })

  it('returns 0 for empty', () => {
    expect(totalEpisodes(makeTitle())).toBe(0)
  })
})

describe('hexToRgba', () => {
  it('converts well-formed hex', () => {
    expect(hexToRgba('#d4ad7a', 0.10)).toBe('rgba(212, 173, 122, 0.1)')
  })

  it('returns empty for invalid input', () => {
    expect(hexToRgba(null, 0.1)).toBe('')
    expect(hexToRgba(undefined, 0.1)).toBe('')
    expect(hexToRgba('#xyz', 0.1)).toBe('')
    expect(hexToRgba('d4ad7a', 0.1)).toBe('')
  })
})

describe('aniListMediaUrl', () => {
  it('builds URL from numeric id', () => {
    expect(aniListMediaUrl(12345)).toBe('https://anilist.co/anime/12345')
  })

  it('builds URL from string id', () => {
    expect(aniListMediaUrl('98765')).toBe('https://anilist.co/anime/98765')
  })
})

describe('computeAniListUrl', () => {
  it('returns null for non-anime', () => {
    const t = makeTitle({ is_anime: false, type: 'movie', anilist_id: 42 })
    expect(computeAniListUrl(t)).toBeNull()
  })

  it('returns URL for anime movie with anilist_id', () => {
    const t = makeTitle({ is_anime: true, type: 'movie', anilist_id: 42 })
    expect(computeAniListUrl(t)).toBe('https://anilist.co/anime/42')
  })

  it('returns null for anime movie without anilist_id', () => {
    const t = makeTitle({ is_anime: true, type: 'movie', anilist_id: null })
    expect(computeAniListUrl(t)).toBeNull()
  })

  it('returns season URL for single-season anime with mapping (not title id)', () => {
    const season: Season = { ...makeSeason([]), anilist_id: '999' }
    const t = makeTitle({ is_anime: true, type: 'series', anilist_id: 1, seasons: [season] })
    expect(computeAniListUrl(t)).toBe('https://anilist.co/anime/999')
  })

  it('returns null for single-season anime without season mapping', () => {
    const season: Season = { ...makeSeason([]), anilist_id: null }
    const t = makeTitle({ is_anime: true, type: 'series', anilist_id: 1, seasons: [season] })
    expect(computeAniListUrl(t)).toBeNull()
  })

  it('returns null for multi-season anime', () => {
    const s1: Season = { ...makeSeason([]), anilist_id: '111' }
    const s2: Season = { ...makeSeason([]), id: 2, season_number: 2, anilist_id: '222' }
    const t = makeTitle({ is_anime: true, type: 'series', seasons: [s1, s2] })
    expect(computeAniListUrl(t)).toBeNull()
  })

  it('returns title URL for anime series with no seasons fetched yet (pending match review)', () => {
    const t = makeTitle({ is_anime: true, type: 'series', anilist_id: 1234, seasons: [] })
    expect(computeAniListUrl(t)).toBe('https://anilist.co/anime/1234')
  })

  it('returns null for anime series with no seasons array and no title id (unconfirmed title)', () => {
    const t = makeTitle({ is_anime: true, type: 'series', anilist_id: null, seasons: undefined as unknown as Season[] })
    expect(computeAniListUrl(t)).toBeNull()
  })
})

describe('getCoverUrl', () => {
  it('returns null for null, undefined, or empty coverUrl', () => {
    expect(getCoverUrl(null)).toBeNull()
    expect(getCoverUrl(undefined)).toBeNull()
    expect(getCoverUrl('')).toBeNull()
  })

  it('prefixes relative filenames with /api/covers/', () => {
    expect(getCoverUrl('abc123.webp')).toBe('/api/covers/abc123.webp')
  })

  it('leaves absolute URLs untouched', () => {
    expect(getCoverUrl('http://example.com/cover.jpg')).toBe('http://example.com/cover.jpg')
    expect(getCoverUrl('https://example.com/cover.jpg')).toBe('https://example.com/cover.jpg')
    expect(getCoverUrl('/custom/path/cover.jpg')).toBe('/custom/path/cover.jpg')
  })
})

describe('formatMatchSource', () => {
  it('returns empty string for null or undefined', () => {
    expect(formatMatchSource(null)).toBe('')
    expect(formatMatchSource(undefined)).toBe('')
    expect(formatMatchSource('')).toBe('')
  })

  it('maps plex_ids to Scrobble metadata', () => {
    expect(formatMatchSource('plex_ids')).toBe('Scrobble metadata')
    expect(formatMatchSource('scrobble_ids')).toBe('Scrobble metadata')
  })

  it('maps standard search sources to readable labels', () => {
    expect(formatMatchSource('tmdb')).toBe('TMDB Search')
    expect(formatMatchSource('tmdb_search')).toBe('TMDB Search')
    expect(formatMatchSource('tvdb_search')).toBe('TVDB Search')
    expect(formatMatchSource('anilist_search')).toBe('AniList Search')
    expect(formatMatchSource('crossref')).toBe('Anime Cross-Ref DB')
    expect(formatMatchSource('gemini_fuzzy')).toBe('Gemini AI Search')
    expect(formatMatchSource('manual')).toBe('Manual Match')
  })

  it('formats unknown source by replacing underscores', () => {
    expect(formatMatchSource('custom_provider_test')).toBe('custom provider test')
  })
})
