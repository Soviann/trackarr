import { Title, TitleType, TitleStatus } from './types'

/** Returns the best display name for a title. Priority: fr → en → (x-romaji → ja when anime) → first. */
export function getName(title: Title): string {
  if (!title.names || title.names.length === 0) return '(untitled)'
  const pick = (lang: string) => title.names.find((n) => n.language === lang)?.name
  const fr = pick('fr')
  if (fr) return fr
  const en = pick('en')
  if (en) return en
  if (title.is_anime) {
    const romaji = pick('x-romaji') ?? pick('ja')
    if (romaji) return romaji
  }
  return title.names[0].name
}

/** Returns the display label for a title type. */
export function getTypeLabel(type: TitleType): string {
  switch (type) {
    case 'movie': return 'Film'
    case 'series': return 'Series'
    default: return type
  }
}

/** Returns the display label for a title status. */
export function getStatusLabel(status: TitleStatus): string {
  switch (status) {
    case 'watching': return 'Watching'
    case 'completed': return 'Completed'
    case 'dropped': return 'Dropped'
    case 'plan_to_watch': return 'Plan to Watch'
    default: return status
  }
}

/** Formats a date string to locale short format. */
export function formatDate(dateStr: string | null | undefined): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''
  return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}

/** Returns the total watched episodes across all seasons. */
export function watchedCount(title: Title): number {
  return (title.seasons ?? []).reduce(
    (sum, s) => sum + (s.episodes ?? []).filter((e) => e.watched).length,
    0
  )
}

/** Returns the total episode count across all seasons. */
export function totalEpisodes(title: Title): number {
  return (title.seasons ?? []).reduce(
    (sum, s) => sum + (s.episodes ?? []).length,
    0
  )
}

/** Formats a watchtime in minutes to a human-readable string (e.g. "2h 30m"). Returns null if absent or <= 0. */
export function formatWatchtime(minutes: number | null | undefined): string | null {
  if (!minutes || minutes <= 0) return null
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  if (h === 0) return `${m}m`
  if (m === 0) return `${h}h`
  return `${h}h ${m}m`
}

/**
 * Builds an AniList media URL from an ID.
 * Shared across pages/components to keep the URL shape consistent — Title.anilist_id is number,
 * Season.anilist_id is string, both coerce cleanly via template literal.
 */
export function aniListMediaUrl(id: number | string): string {
  return `https://anilist.co/anime/${id}`
}

/**
 * Resolves the AniList URL to expose for a title, or null if no usable mapping exists.
 * Movies use the title's anilist_id; single-season anime use the season's mapping (preferred
 * over the title's, since AniList tracks each season as its own entry); multi-season anime
 * resolve at season-level only and return null here.
 */
export function computeAniListUrl(title: Title): string | null {
  if (!title.is_anime) return null
  if (title.type === 'movie') {
    return title.anilist_id ? aniListMediaUrl(title.anilist_id) : null
  }
  if (title.seasons.length === 1) {
    const s1 = title.seasons[0]
    return s1?.anilist_id ? aniListMediaUrl(s1.anilist_id) : null
  }
  return null
}
