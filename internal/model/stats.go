package model

// StatsResponse is the full response for GET /api/stats.
type StatsResponse struct {
	Overview          StatsOverview  `json:"overview"`
	Ratings           StatsRatings   `json:"ratings"`
	Breakdown         StatsBreakdown `json:"breakdown"`
	FunStats          []FunStat      `json:"fun_stats"`
	Year              StatsYear      `json:"year_summary"`
	Genres            interface{}    `json:"genres"`
	Streaks           StatsStreaks   `json:"streaks"`
	TotalWatchMinutes int            `json:"total_watch_minutes"`
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

type StatsYear struct {
	TitlesAdded     int `json:"titles_added"`
	EpisodesWatched int `json:"episodes_watched"`
	Completions     int `json:"completions"`
}
