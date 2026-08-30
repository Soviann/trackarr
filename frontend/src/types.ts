export interface WatchProvider {
  id: number
  name: string
}

export interface Settings {
  anilist_connected: boolean
  anilist_token_invalid: boolean
  push_subscribed: boolean
  tvdb_connected: boolean
  jellyfin_configured: boolean
  prowlarr_configured?: boolean
  jellyfin_last_scrobble_at?: string | null
  enabled_watch_providers?: string
}

export interface PublicConfig {
  google_client_id?: string
  google_auth_enabled: boolean
  password_auth_enabled: boolean
  auth_mode: string
  setup_required: boolean
  vapid_public_key?: string
  metadata_language?: string
  enabled_watch_providers?: string
}

export type TitleType = 'movie' | 'series'
export type TitleStatus = 'watching' | 'completed' | 'dropped' | 'plan_to_watch'
export type SeriesStatus = 'returning' | 'ended' | 'cancelled' | 'in_production'
export type MatchStatus = 'confirmed' | 'pending_review' | 'unconfirmed'

export interface Title {
  id: number
  type: TitleType
  is_anime: boolean
  year: number
  cover_url: string | null
  accent_hex: string | null
  imdb_id: string | null
  simkl_id: number | null
  simkl_slug: string | null
  anilist_id: number | null
  tmdb_id: number | null
  tvdb_id: number | null
  radarr_id?: number | null
  sonarr_id?: number | null
  my_rating: number | null
  status: TitleStatus
  series_status: SeriesStatus | null
  match_status: MatchStatus
  original_title: string | null
  match_source: string | null
  overview: string | null
  genres: string[] | null
  watch_providers?: WatchProvider[]
  runtime: number | null
  total_watch_minutes: number
  tmdb_rating: number | null
  credits: string | null
  anilist_rating: number | null
  release_date: string | null
  next_air_date?: string | null
  next_air_episode?: string | null
  last_watched_at?: string
  last_refreshed_at?: string
  caught_up?: boolean
  created_at: string
  updated_at: string
  names: TitleName[]
  seasons: Season[]
  relations?: TitleRelation[]
  next_episode?: NextEpisode
  personal_notes?: string | null
  matched_name?: string
  matched_language?: string
}

export interface TitleRelation {
  id: number
  title_id: number
  season_id?: number | null
  season_number?: number | null
  provider: string
  external_id: number
  relation_type: string
  format: string
  title: string
  romaji_title?: string | null
  cover_url?: string | null
  year?: number | null
  score?: number | null
  episode_count?: number | null
  duration?: number | null
  overview?: string | null
  sort_order: number
  created_at: string
  updated_at: string

  matched_title_id?: number | null
  matched_status?: TitleStatus | null
  matched_rating?: number | null
  matched_type?: TitleType | null
  matched_radarr_id?: number | null
  matched_sonarr_id?: number | null
}

export interface GenreCount {
  genre: string
  count: number
}

export interface CountryCount {
  country: string
  count: number
}

export interface NextEpisode {
  id: number
  season_id: number
  episode: number
  season_number: number
}

export interface TitleName {
  id: number
  title_id: number
  name: string
  language: string
  is_primary: boolean
}

export interface AniListPart {
  external_id: string
  score: number | null
  episode_count: number | null
  start_date: string | null
  sort_order: number | null
}

export interface Season {
  id: number
  title_id: number
  season_number: number
  total_episodes: number | null
  anilist_id?: string | null
  anilist_community_score?: number | null
  anilist_parts?: AniListPart[]
  episode_count?: number
  watched_count?: number
  episodes: Episode[]
}

export interface MatchResult {
  imdb_id: string | null
  tmdb_id: number | null
  tvdb_id: number | null
  anilist_id: number | null
  match_status: MatchStatus
  match_source: string
  names: TitleName[]
  cover_file: string | null
  type: TitleType
  is_anime: boolean
  overview: string | null
  genres: string[] | null
  runtime: number | null
  tmdb_rating: number | null
  credits: string | null
  anilist_rating: number | null
  release_date: string | null
}

export interface PaginatedResponse {
  titles: Title[]
  total: number
  has_more: boolean
  counts?: StatusCounts
}

export interface StatusCounts {
  pending_review: number
  unconfirmed: number
}

export interface Episode {
  id: number
  season_id: number
  episode: number
  name: string | null
  air_date: string | null
  watched: boolean
  first_watched_at: string | null
  last_watched_at: string | null
}

// Stats
export interface StatsResponse {
  overview: StatsOverview
  ratings: StatsRatings
  breakdown: StatsBreakdown
  fun_stats: FunStat[]
  year_summary: StatsYear
  genres: Array<{ genre: string; count: number }>
  streaks: {
    current: number
    best: number
  }
  total_watch_minutes: number
  watched_this_year: number
  avg_rating_this_year: number
  minutes_this_week: number
}

export interface ActivityEvent {
  title_id: number
  title_name: string
  cover_url: string | null
  title_type: string
  episode_id: number | null
  episode_name: string | null
  season_number: number | null
  episode_number: number | null
  watched_at: string
  is_completion: boolean
}

export interface EpisodeHistory {
  episode_id: number | null
  episode_name: string | null
  season_number: number | null
  episode_number: number | null
  watch_count: number
  last_watched_at: string
  watches: string[]
}

export interface StatsOverview {
  total_titles: number
  total_movies: number
  total_series: number
  total_anime: number
  episodes_watched: number
  completion_rate: number
  average_rating: number
}

export interface StatsRatings {
  distribution: number[]
  average_by_type: Record<string, number>
  insight: string
}

export interface StatsBreakdown {
  by_status: Record<string, number>
  by_type: Record<string, number>
}

export interface FunStat {
  id: string
  icon: string
  title: string
  value: string
  detail: string
}

export interface StatsYear {
  titles_added: number
  episodes_watched: number
  completions: number
}

export interface ContinueWatchingTitle {
  id: number
  type: TitleType
  is_anime?: boolean
  cover_url: string | null
  name: string
  next_air_episode: string | null
  watched_episodes: number
  total_episodes: number
  last_watched_at: string | null
  watch_providers?: WatchProvider[]
  sonarr_id?: number | null
  radarr_id?: number | null
}

export type MatchEventKind = 'auto_confirmed' | 'season_attached'

export interface MatchEvent {
  id: number
  title_id: number | null
  kind: MatchEventKind
  detail: string
  created_at: string
  cover_url?: string | null
}

export interface UpcomingTitle {
  id: number
  type: TitleType
  is_anime?: boolean
  cover_url: string | null
  name: string
  next_air_date: string
  next_air_episode: string | null
  status: string
  watch_providers?: WatchProvider[]
  sonarr_id?: number | null
  radarr_id?: number | null
}

export interface CalendarEvent {
  id: string
  title_id: number
  title_name: string
  type: TitleType
  is_anime: boolean
  cover_url: string | null
  air_date: string
  episode_id?: number | null
  season_number?: number | null
  episode_number?: number | null
  episode_name?: string | null
  next_air_episode?: string | null
  status: string
  watch_providers?: WatchProvider[]
  accent_hex?: string | null
  overview?: string | null
  sonarr_id?: number | null
  radarr_id?: number | null
}

export interface CalendarTokenResponse {
  token: string
  feed_url: string
  http_url: string
  webcal_url: string
}

export type CalendarViewMode = 'month' | 'week' | 'list'

export interface SeasonAuditProposal {
  source_title_id: number
  source_name: string
  source_year: number
  source_cover_url?: string | null
  source_seasons_count: number
  target_title_id: number
  target_name: string
  target_year: number
  target_cover_url?: string | null
  target_seasons_count: number
  season_number: number
  shared_id: string
}

export interface ProwlarrRelease {
  guid: string
  title: string
  clean_title: string
  year: number
  type: TitleType
  size: number
  publish_date: string
  seeders: number
  leechers: number
  grabs: number
  indexer: string
  indexer_id: number
  download_url: string
  info_url: string
  tmdb_id: number
  imdb_id: string
  poster_url: string
  existing_title_id?: number
  existing_status?: TitleStatus
}

export interface ArrTitleDetails {
  app: 'radarr' | 'sonarr'
  exists: boolean
  arr_id?: number
  title_slug?: string
  web_url?: string
  monitored: boolean
  quality_profile_id: number
  root_folder_path: string
  has_file: boolean
  size_on_disk?: number
}
