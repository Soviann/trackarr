export type TitleType = 'movie' | 'series' | 'anime'
export type TitleStatus = 'watching' | 'completed' | 'dropped' | 'plan_to_watch'
export type SeriesStatus = 'returning' | 'ended' | 'cancelled' | 'in_production'
export type MatchStatus = 'confirmed' | 'pending_review' | 'unconfirmed'

export interface Title {
  id: number
  type: TitleType
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
  names: TitleName[]
  seasons: Season[]
  matched_name?: string
  matched_language?: string
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
  episodes: Episode[]
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
