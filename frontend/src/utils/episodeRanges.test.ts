import { describe, it, expect } from 'vitest'
import { groupIntoRanges, formatRangeLabel, type RangableEpisode, type EpisodeRangeGroup } from './episodeRanges'

function ep(season: number | null, episode: number | null, name: string | null = null): RangableEpisode {
  return { season_number: season, episode_number: episode, episode_name: name }
}

describe('groupIntoRanges', () => {
  it('returns empty array for empty input', () => {
    expect(groupIntoRanges([])).toEqual([])
  })

  it('groups all consecutive episodes into a single range', () => {
    const items = [ep(1, 1, 'Pilot'), ep(1, 2, 'Second'), ep(1, 3, 'Third')]
    const groups = groupIntoRanges(items)
    expect(groups).toHaveLength(1)
    expect(groups[0].startEp).toBe(1)
    expect(groups[0].endEp).toBe(3)
    expect(groups[0].episodeName).toBeNull()
    expect(groups[0].items).toHaveLength(3)
  })

  it('splits at gaps into separate ranges', () => {
    const items = [ep(1, 1), ep(1, 2), ep(1, 5), ep(1, 6)]
    const groups = groupIntoRanges(items)
    expect(groups).toHaveLength(2)
    expect(groups[0]).toMatchObject({ startEp: 1, endEp: 2 })
    expect(groups[1]).toMatchObject({ startEp: 5, endEp: 6 })
  })

  it('does not merge across seasons', () => {
    const items = [ep(1, 1), ep(2, 2)]
    const groups = groupIntoRanges(items)
    expect(groups).toHaveLength(2)
    expect(groups[0].seasonNumber).toBe(1)
    expect(groups[1].seasonNumber).toBe(2)
  })

  it('keeps movie (null episode_number) as standalone group', () => {
    const items = [ep(null, null)]
    const groups = groupIntoRanges(items)
    expect(groups).toHaveLength(1)
    expect(groups[0].startEp).toBeNull()
  })

  it('preserves episode name for single-episode groups', () => {
    const items = [ep(1, 5, 'The One')]
    const groups = groupIntoRanges(items)
    expect(groups[0].episodeName).toBe('The One')
  })

  it('clears episode name for multi-episode ranges', () => {
    const items = [ep(1, 1, 'First'), ep(1, 2, 'Second')]
    const groups = groupIntoRanges(items)
    expect(groups[0].episodeName).toBeNull()
  })

  it('does not merge null episode_number items together', () => {
    const items = [ep(null, null, 'Movie A'), ep(null, null, 'Movie B')]
    const groups = groupIntoRanges(items)
    expect(groups).toHaveLength(2)
  })
})

describe('formatRangeLabel', () => {
  it('returns "Movie" for null season/episode', () => {
    expect(formatRangeLabel({ seasonNumber: null, startEp: null, endEp: null, episodeName: null, items: [] })).toBe('Movie')
  })

  it('returns single episode label', () => {
    expect(formatRangeLabel({ seasonNumber: 2, startEp: 5, endEp: 5, episodeName: 'X', items: [] })).toBe('S2 E5')
  })

  it('returns range label', () => {
    expect(formatRangeLabel({ seasonNumber: 1, startEp: 1, endEp: 10, episodeName: null, items: [] })).toBe('S1 E1-10')
  })
})
