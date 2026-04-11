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
  imdb_id: string | null
  anilist_id: number | null
  tmdb_id: number | null
  tvdb_id: number | null
  my_rating: number | null
  status: TitleStatus
  series_status: SeriesStatus | null
  match_status: MatchStatus
  original_title: string | null
  match_source: string | null
  overview: string | null
  genres: string[] | null
  runtime: number | null
  total_watch_minutes: number
  tmdb_rating: number | null
  credits: string | null
  anilist_rating: number | null
  release_date: string | null
  last_watched_at?: string
  created_at: string
  updated_at: string
  names: TitleName[]
  seasons: Season[]
  next_episode?: NextEpisode
  matched_name?: string
  matched_language?: string
}

export interface GenreCount {
  genre: string
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

export interface Season {
  id: number
  title_id: number
  season_number: number
  total_episodes: number | null
  my_rating: number | null
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
  watched_at: string | null
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
  type: string
  cover_url: string | null
  name: string
  next_air_episode: string | null
  watched_episodes: number
  total_episodes: number
  last_watched_at: string | null
}

export interface UpcomingTitle {
  id: number
  type: string
  cover_url: string | null
  name: string
  next_air_date: string
  next_air_episode: string | null
  status: string
}
