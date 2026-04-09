import { Title, TitleType, TitleStatus } from './types'

/** Returns the best display name for a title (primary name, or first available). */
export function getName(title: Title): string {
  if (!title.names || title.names.length === 0) return '(untitled)'
  const primary = title.names.find((n) => n.is_primary)
  if (primary) return primary.name
  const fr = title.names.find((n) => n.language === 'fr')
  if (fr) return fr.name
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
