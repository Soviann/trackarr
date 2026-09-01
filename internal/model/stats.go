package model

import "time"

// PersonStat holds an actor or director name and their watched titles count.
type PersonStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// StatsResponse is the full response for GET /api/stats.
type StatsResponse struct {
	Overview          StatsOverview  `json:"overview"`
	Ratings           StatsRatings   `json:"ratings"`
	Breakdown         StatsBreakdown `json:"breakdown"`
	FunStats          []FunStat      `json:"fun_stats"`
	Year              StatsYear      `json:"year_summary"`
	Genres            any            `json:"genres"`
	TopActors         []PersonStat   `json:"top_actors"`
	TopDirectors      []PersonStat   `json:"top_directors"`
	Streaks           StatsStreaks   `json:"streaks"`
	TotalWatchMinutes int            `json:"total_watch_minutes"`
	// Library strip — compact at-a-glance figures.
	WatchedThisYear   int     `json:"watched_this_year"`
	AvgRatingThisYear float64 `json:"avg_rating_this_year"`
	MinutesThisWeek   int     `json:"minutes_this_week"`
}

// StatsStreaks holds current and best consecutive watch day streaks.
type StatsStreaks struct {
	Current int `json:"current"`
	Best    int `json:"best"`
}

type StatsOverview struct {
	TotalTitles     int     `json:"total_titles"`
	TotalMovies     int     `json:"total_movies"`
	TotalSeries     int     `json:"total_series"`
	TotalAnime      int     `json:"total_anime"`
	EpisodesWatched int     `json:"episodes_watched"`
	CompletionRate  float64 `json:"completion_rate"`
	AverageRating   float64 `json:"average_rating"`
}

type StatsRatings struct {
	Distribution  [10]int            `json:"distribution"`
	AverageByType map[string]float64 `json:"average_by_type"`
	Insight       string             `json:"insight"`
}

type StatsBreakdown struct {
	ByStatus map[string]int `json:"by_status"`
	ByType   map[string]int `json:"by_type"`
}

type FunStat struct {
	ID     string `json:"id"`
	Icon   string `json:"icon"`
	Title  string `json:"title"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

// GenreStat holds a genre name and the number of titles in that genre.
type GenreStat struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

// WrappedTitleItem represents a title highlighted in Wrapped stories.
type WrappedTitleItem struct {
	ID            int64    `json:"id"`
	Title         string   `json:"title"`
	OriginalTitle *string  `json:"original_title,omitempty"`
	Year          int      `json:"year"`
	Type          string   `json:"type"`
	IsAnime       bool     `json:"is_anime"`
	CoverURL      *string  `json:"cover_url,omitempty"`
	AccentHex     *string  `json:"accent_hex,omitempty"`
	MyRating      *int     `json:"my_rating,omitempty"`
	WatchCount    int      `json:"watch_count"`
	ReleaseDate   *string  `json:"release_date,omitempty"`
	Genres        []string `json:"genres,omitempty"`
}

// WrappedCategoryTop holds the top 3 items for Movies, Series, and Anime.
type WrappedCategoryTop struct {
	Movies []WrappedTitleItem `json:"movies"`
	Series []WrappedTitleItem `json:"series"`
	Anime  []WrappedTitleItem `json:"anime"`
}

// WrappedRewatch holds details about the champion rewatched title.
type WrappedRewatch struct {
	Title            WrappedTitleItem `json:"title"`
	TotalPlays       int              `json:"total_plays"`
	IsMovie          bool             `json:"is_movie"`
	DistinctEpisodes int              `json:"distinct_episodes,omitempty"`
	TotalEpisodes    int              `json:"total_episodes,omitempty"`
}

// WrappedAIPersona holds Gemini-generated personality and insights.
type WrappedAIPersona struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Quote    string   `json:"quote"`
	FunFacts []string `json:"fun_facts"`
	Badges   []string `json:"badges,omitempty"`
}

// WrappedRawStats is passed to Gemini for generating custom insights.
type WrappedRawStats struct {
	Year              int                `json:"year"`
	TotalTitles       int                `json:"total_titles"`
	TotalMovies       int                `json:"total_movies"`
	TotalSeries       int                `json:"total_series"`
	TotalAnime        int                `json:"total_anime"`
	EpisodesWatched   int                `json:"episodes_watched"`
	TotalWatchMinutes int                `json:"total_watch_minutes"`
	AverageRating     float64            `json:"average_rating"`
	NightOwlPct       int                `json:"night_owl_pct"`
	PeakDayOfWeek     string             `json:"peak_day_of_week"`
	PeakMonth         string             `json:"peak_month"`
	LongestBingeEps   int                `json:"longest_binge_eps"`
	LongestBingeTitle string             `json:"longest_binge_title"`
	BestStreakDays    int                `json:"best_streak_days"`
	TopGenres         []string           `json:"top_genres"`
	TopActors         []string           `json:"top_actors"`
	TopDirectors      []string           `json:"top_directors"`
	TopFavorites      WrappedCategoryTop `json:"top_favorites"`
	TopReleases       WrappedCategoryTop `json:"top_releases"`
	RewatchChampion   *WrappedRewatch    `json:"rewatch_champion,omitempty"`
}

// WrappedResponse is the response structure for GET /api/stats/wrapped.
type WrappedResponse struct {
	Year              int                `json:"year"`
	AvailableYears    []int              `json:"available_years"`
	Overview          StatsOverview      `json:"overview"`
	TotalWatchMinutes int                `json:"total_watch_minutes"`
	TopFavorites      WrappedCategoryTop `json:"top_favorites"`
	TopReleases       WrappedCategoryTop `json:"top_releases"`
	RewatchChampion   *WrappedRewatch    `json:"rewatch_champion,omitempty"`
	TopGenres         []GenreStat        `json:"top_genres"`
	TopActors         []PersonStat       `json:"top_actors"`
	TopDirectors      []PersonStat       `json:"top_directors"`
	Persona           WrappedAIPersona   `json:"persona"`
	CreatedAt         *time.Time         `json:"created_at,omitempty"`
}

// WrappedArchiveItem represents an archived Wrapped snapshot in the gallery.
type WrappedArchiveItem struct {
	Year              int       `json:"year"`
	PersonaTitle      string    `json:"persona_title"`
	PersonaBadges     []string  `json:"persona_badges,omitempty"`
	TotalWatchMinutes int       `json:"total_watch_minutes"`
	TotalTitles       int       `json:"total_titles"`
	TopCoverURL       *string   `json:"top_cover_url,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type StatsYear struct {
	TitlesAdded     int `json:"titles_added"`
	EpisodesWatched int `json:"episodes_watched"`
	Completions     int `json:"completions"`
}
