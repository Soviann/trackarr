package model

// JellyfinPayload is the JSON body PlexTracker expects from the Jellyfin
// Webhook plugin's "Generic Destination". The plugin renders values through a
// Handlebars template; a missing variable renders as an empty string. To keep
// the body valid JSON for any item type (a Movie has no SeasonNumber, etc.),
// EVERY field in the template is emitted as a quoted string and parsed here —
// numeric and boolean coercion happens in the service layer, never in the
// template. The matching template lives in docs/user-guide.md.
type JellyfinPayload struct {
	NotificationType   string `json:"notification_type"` // "PlaybackStart" | "PlaybackStop" | ...
	ItemType           string `json:"item_type"`         // "Movie" | "Episode"
	Name               string `json:"name"`              // movie title or episode name
	Year               string `json:"year"`
	PlayedToCompletion string `json:"played_to_completion"` // "True"/"False" — only on PlaybackStop

	// External provider IDs. For an Episode these are the EPISODE's own IDs, not
	// the series' — so episode matching relies on series_id + series_name, never
	// on these. For a Movie they identify the movie and dedupe against Plex.
	ProviderIMDB string `json:"provider_imdb"`
	ProviderTMDB string `json:"provider_tmdb"`
	ProviderTVDB string `json:"provider_tvdb"`

	ItemID     string `json:"item_id"`     // Jellyfin GUID of the played item
	SeriesName string `json:"series_name"` // episode only
	SeriesID   string `json:"series_id"`   // episode only — stable per-series Jellyfin GUID
	Season     string `json:"season"`      // episode only
	Episode    string `json:"episode"`     // episode only
}
