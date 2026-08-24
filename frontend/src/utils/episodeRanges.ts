export interface RangableEpisode {
  season_number: number | null
  episode_number: number | null
  episode_name: string | null
}

export interface EpisodeRangeGroup<T> {
  seasonNumber: number | null
  startEp: number | null
  endEp: number | null
  episodeName: string | null
  items: T[]
}

/**
 * Groups a sorted (by episode_number ASC) list of episodes into consecutive ranges.
 * Items with episode_number == null are never merged (standalone groups).
 * Duplicate episode_numbers (multiple watch_events for the same episode — rewatches,
 * dual webhook firings) fold into the current group instead of creating a singleton
 * that would overlap with the next consecutive episode's range.
 */
export function groupIntoRanges<T extends RangableEpisode>(items: T[]): EpisodeRangeGroup<T>[] {
  if (items.length === 0) return []

  const groups: EpisodeRangeGroup<T>[] = []
  let current: EpisodeRangeGroup<T> = {
    seasonNumber: items[0].season_number,
    startEp: items[0].episode_number,
    endEp: items[0].episode_number,
    episodeName: items[0].episode_name,
    items: [items[0]],
  }

  for (let i = 1; i < items.length; i++) {
    const item = items[i]
    const sameSeason =
      item.episode_number != null &&
      current.endEp != null &&
      item.season_number === current.seasonNumber

    if (sameSeason && item.episode_number === current.endEp! + 1) {
      current.endEp = item.episode_number
      current.episodeName = null
      current.items.push(item)
    } else if (sameSeason && item.episode_number === current.endEp) {
      current.items.push(item)
    } else {
      groups.push(current)
      current = {
        seasonNumber: item.season_number,
        startEp: item.episode_number,
        endEp: item.episode_number,
        episodeName: item.episode_name,
        items: [item],
      }
    }
  }
  groups.push(current)
  return groups
}

export function formatRangeLabel(group: EpisodeRangeGroup<any>): string {
  if (group.seasonNumber == null || group.startEp == null) return 'Movie'
  if (group.startEp === group.endEp) return `S${group.seasonNumber} E${group.startEp}`
  return `S${group.seasonNumber} E${group.startEp}-${group.endEp}`
}
