import { describe, it, expect } from 'vitest'
import { aniListMediaUrl, computeAniListUrl, getName, getTypeLabel, getStatusLabel, formatDate, watchedCount, totalEpisodes } from './utils'
import type { Title, TitleName, Season, Episode, TitleType } from './types'

function makeTitle(overrides: Partial<Title> = {}): Title {
  return {
    id: 1, type: 'movie', is_anime: false, year: 2024, cover_url: null, imdb_id: null, anilist_id: null,
    tmdb_id: null, tvdb_id: null, my_rating: null, status: 'watching', series_status: null,
    match_status: 'confirmed', original_title: null, match_source: null, names: [], seasons: [],
    overview: null, genres: null, runtime: null, tmdb_rating: null, credits: null,
    anilist_rating: null, release_date: null, created_at: '', updated_at: '',
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
  it('prefers French over English', () => {
    const t = makeTitle({ names: [makeName('Eng', 'en', true), makeName('Fr', 'fr', false)] })
    expect(getName(t)).toBe('Fr')
  })

  it('falls back to English when no French', () => {
    const t = makeTitle({ names: [makeName('Eng', 'en', false), makeName('De', 'de', false)] })
    expect(getName(t)).toBe('Eng')
  })

  it('falls back to romaji for anime when no fr/en', () => {
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

describe('getTypeLabel', () => {
  const cases: [TitleType, string][] = [
    ['movie', 'Film'],
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
})
