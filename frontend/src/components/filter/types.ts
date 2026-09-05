import type { TitleStatus, TitleType, SeriesStatus } from '../../types'
import type { SortField, SortOrder, SortState } from '../../store'

export type StatusFilter = TitleStatus | 'up_to_date' | null
export type TypeFilter = TitleType | null
export type SeriesStatusFilter = SeriesStatus | null
export type DrawerTab = 'basics' | 'genres' | 'dates'

export interface FilterState {
  status: StatusFilter
  type: TypeFilter
  isAnime: boolean
  seriesStatus: SeriesStatusFilter
  decade: string | null
  releaseFrom: string
  releaseTo: string
  includeNoRelease: boolean
  selectedGenres: string[]
  genreOp: 'AND' | 'OR'
  selectedCountries: string[]
  myRatingMin: string
  tmdbRatingMin: string
}

export interface FilterActions {
  onStatusChange: (status: StatusFilter) => void
  onTypeChange: (type: TypeFilter) => void
  onIsAnimeChange: (isAnime: boolean) => void
  onSeriesStatusChange: (seriesStatus: SeriesStatusFilter) => void
  onDecadeChange: (decade: string | null) => void
  onReleaseFromChange: (date: string) => void
  onReleaseToChange: (date: string) => void
  onIncludeNoReleaseChange: (include: boolean) => void
  onGenreToggle: (genre: string) => void
  onGenreOpChange: (op: 'AND' | 'OR') => void
  onCountryToggle: (iso: string) => void
  onMyRatingMinChange: (v: string) => void
  onTmdbRatingMinChange: (v: string) => void
}

export interface FilterDrawerProps {
  // Consolidated structure
  filter?: FilterState
  actions?: FilterActions
  sort: SortState
  onSortChange: (sort: SortState) => void
  isSearchActive: boolean
  open?: boolean
  onOpenChange?: (open: boolean) => void
  onReset?: () => void
  activeCount?: number
  defaultOpen?: boolean

  // Legacy flat props for backward compatibility
  status?: StatusFilter
  type?: TypeFilter
  isAnime?: boolean
  seriesStatus?: SeriesStatusFilter
  onStatusChange?: (status: StatusFilter) => void
  onTypeChange?: (type: TypeFilter) => void
  onIsAnimeChange?: (isAnime: boolean) => void
  onSeriesStatusChange?: (seriesStatus: SeriesStatusFilter) => void
  decade?: string | null
  releaseFrom?: string
  releaseTo?: string
  includeNoRelease?: boolean
  onDecadeChange?: (decade: string | null) => void
  onReleaseFromChange?: (date: string) => void
  onReleaseToChange?: (date: string) => void
  onIncludeNoReleaseChange?: (include: boolean) => void
  selectedGenres?: string[]
  genreOp?: 'AND' | 'OR'
  onGenreToggle?: (genre: string) => void
  onGenreOpChange?: (op: 'AND' | 'OR') => void
  selectedCountries?: string[]
  onCountryToggle?: (iso: string) => void
  myRatingMin?: string
  tmdbRatingMin?: string
  onMyRatingMinChange?: (v: string) => void
  onTmdbRatingMinChange?: (v: string) => void
}

export const statusFilters: { id: StatusFilter; label: string }[] = [
  { id: null, label: 'All' },
  { id: 'plan_to_watch', label: 'Plan' },
  { id: 'watching', label: 'Watching' },
  { id: 'up_to_date', label: 'Caught up' },
  { id: 'completed', label: 'Completed' },
  { id: 'dropped', label: 'Dropped' },
]

export const typeFilters: { id: TypeFilter; label: string }[] = [
  { id: null, label: 'All' },
  { id: 'movie', label: 'Movie' },
  { id: 'series', label: 'Series' },
]

export const sortOptions: { field: SortField; label: string; defaultOrder: SortOrder }[] = [
  { field: 'updated_at', label: 'Last updated', defaultOrder: 'desc' },
  { field: 'original_title', label: 'Title', defaultOrder: 'asc' },
  { field: 'release_date', label: 'Release date', defaultOrder: 'desc' },
  { field: 'my_rating', label: 'Rating', defaultOrder: 'desc' },
  { field: 'created_at', label: 'Date added', defaultOrder: 'desc' },
  { field: 'last_watched_at', label: 'Last watched', defaultOrder: 'desc' },
]

export const seriesStatusFilters: { id: SeriesStatusFilter; label: string }[] = [
  { id: null, label: 'All' },
  { id: 'returning', label: 'Returning' },
  { id: 'ended', label: 'Ended' },
  { id: 'cancelled', label: 'Cancelled' },
  { id: 'in_production', label: 'Not started' },
]

export const decadeOptions = [
  { value: '', label: 'Decade: All' },
  { value: '2020', label: '2020s' },
  { value: '2010', label: '2010s' },
  { value: '2000', label: '2000s' },
  { value: '1990', label: '1990s' },
  { value: '1980', label: '1980s' },
  { value: '1970', label: '1970s' },
  { value: '1960', label: '1960s' },
  { value: '1950', label: '1950s' },
  { value: '1940', label: '1940s' },
  { value: '1930', label: '1930s' },
  { value: '1920', label: '1920s' },
]
