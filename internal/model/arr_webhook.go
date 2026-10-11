package model

// ArrWebhookPayload models the webhook payloads sent by Radarr and Sonarr on
// Download, Upgrade, and Test events.
type ArrWebhookPayload struct {
	EventType    string `json:"eventType"` // "Download", "Upgrade", "Test", etc.
	InstanceName string `json:"instanceName,omitempty"`

	// Radarr fields
	Movie *ArrMoviePayload `json:"movie,omitempty"`

	// Sonarr fields
	Series   *ArrSeriesPayload    `json:"series,omitempty"`
	Episodes []ArrEpisodePayload  `json:"episodes,omitempty"`
}

type ArrMoviePayload struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Year   int    `json:"year"`
	TMDBID int64  `json:"tmdbId"`
	IMDBID string `json:"imdbId"`
}

type ArrSeriesPayload struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	TVDBID   int64  `json:"tvdbId"`
	TVMazeID int64  `json:"tvMazeId"`
	IMDBID   string `json:"imdbId"`
}

type ArrEpisodePayload struct {
	ID            int64  `json:"id"`
	EpisodeNumber int    `json:"episodeNumber"`
	SeasonNumber  int    `json:"seasonNumber"`
	Title         string `json:"title"`
	AirDate       string `json:"airDate"`
}
