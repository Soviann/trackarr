package service

import (
	"context"
	"database/sql"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	plexwebhooks "github.com/hekmon/plexwebhooks"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

// NewJellyfinService returns a webhook ingest service that records events with
// source=jellyfin. It deliberately reuses the entire Plex ingest pipeline
// (find/create/match/enrich); only the recorded WatchEvent source differs. The
// Jellyfin payload is normalised into the internal Plex shape by
// ProcessJellyfinWebhook before reaching the shared logic.
func NewJellyfinService(db *sql.DB, pipeline *matching.Pipeline, titleSvc *TitleService, libSvc *LibraryService) *PlexService {
	s := NewPlexService(db, pipeline, titleSvc, libSvc)
	s.log = slog.With("subsystem", "jellyfin")
	s.source = model.WatchEventSourceJellyfin
	return s
}

// ProcessJellyfinWebhook ingests a Jellyfin Webhook-plugin notification. It
// normalises the payload and, when the event represents a completed playback,
// runs it through the shared Plex pipeline.
func (s *PlexService) ProcessJellyfinWebhook(ctx context.Context, jf *model.JellyfinPayload, rawPayload string) error {
	payload, ok := normalizeJellyfinPayload(jf)
	if !ok {
		return nil
	}
	return s.ProcessWebhook(ctx, payload, rawPayload)
}

// normalizeJellyfinPayload converts a Jellyfin webhook into the internal Plex
// payload shape. ok=false means the event must be ignored — anything that is not
// a *completed* playback of a movie or episode (PlaybackStart, paused/aborted
// stops, unsupported item types). Completion-based ingestion mirrors Plex's
// media.scrobble semantics: a title counts as watched only when actually
// finished, and every completion (including a rewatch) yields one scrobble.
func normalizeJellyfinPayload(jf *model.JellyfinPayload) (*plexwebhooks.Payload, bool) {
	// Only completed stops count. Jellyfin's PlayedToCompletion flag is the
	// equivalent of Plex's ~90% scrobble threshold.
	if !strings.EqualFold(jf.NotificationType, "PlaybackStop") || !parseJellyfinBool(jf.PlayedToCompletion) {
		return nil, false
	}

	meta := plexwebhooks.Metadata{
		Year: atoiSafe(jf.Year),
	}

	switch strings.ToLower(jf.ItemType) {
	case "movie":
		meta.Type = plexwebhooks.MediaTypeMovie
		meta.Title = jf.Name
		meta.RatingKey = jf.ItemID
		// Movie provider IDs identify the film and dedupe against Plex-created titles.
		meta.GUIDExternal = jellyfinGUIDs(jf.ProviderIMDB, jf.ProviderTMDB, jf.ProviderTVDB)
	case "episode":
		meta.Type = plexwebhooks.MediaTypeEpisode
		meta.Title = jf.Name
		meta.GrandparentTitle = jf.SeriesName
		// SeriesID is the stable per-series key (Plex's GrandparentRatingKey
		// equivalent). Episode-level provider IDs are NOT the series' IDs, so they
		// are intentionally omitted: the series is matched via this key plus the
		// name/year search in the matching pipeline.
		meta.GrandparentRatingKey = jf.SeriesID
		meta.ParentIndex = atoiSafe(jf.Season)
		meta.Index = atoiSafe(jf.Episode)
	default:
		return nil, false
	}

	return &plexwebhooks.Payload{Event: plexwebhooks.EventTypeScrobble, Metadata: meta}, true
}

// jellyfinGUIDs builds the Plex-style GUID list (imdb://, tmdb://, tvdb://) that
// ParseGUIDs consumes, skipping any empty provider ID.
func jellyfinGUIDs(imdb, tmdb, tvdb string) []*url.URL {
	var out []*url.URL
	add := func(scheme, val string) {
		if val == "" {
			return
		}
		if u, err := url.Parse(scheme + "://" + val); err == nil {
			out = append(out, u)
		}
	}
	add("imdb", imdb)
	add("tmdb", tmdb)
	add("tvdb", tvdb)
	return out
}

// parseJellyfinBool accepts the .NET Handlebars boolean rendering ("True"/"False")
// as well as the usual "true"/"1" forms. Anything else (including empty) is false.
func parseJellyfinBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1":
		return true
	default:
		return false
	}
}

// atoiSafe parses an integer, returning 0 for empty or non-numeric input (a
// Handlebars variable absent for the item type renders as "").
func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
