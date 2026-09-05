package repository

import (
	"github.com/Soviann/trackarr/internal/model"
)

type scannable interface {
	Scan(dest ...any) error
}

// titleScanDest returns the destinations for scanning the canonical 36 title columns,
// along with pointers to raw strings that require post-processing.
func titleScanDest(t *model.Title) (dests []any, lastWatchedAtStr **string, watchProvidersRaw **string) {
	var lws *string
	var wpr *string
	dests = []any{
		&t.ID, &t.Type, &t.IsAnime, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
		&t.ExternalSourceID, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
		&t.Overview, &t.Runtime, &t.TotalWatchMinutes, &t.TMDBRating, &t.Credits, &wpr, &t.AniListRating,
		&t.ReleaseDate, &t.NextAirDate, &t.NextAirEpisode, &lws, &t.AccentHex, &t.SimklID, &t.SimklSlug, &t.RadarrID, &t.SonarrID, &t.ArrIgnored, &t.CreatedAt, &t.UpdatedAt, &t.CaughtUp,
	}
	return dests, &lws, &wpr
}

// scanTitleRow scans a standard 36-column title row and populates post-processed fields.
func scanTitleRow(s scannable, t *model.Title, extraDests ...any) error {
	dests, lws, wpr := titleScanDest(t)
	if len(extraDests) > 0 {
		dests = append(dests, extraDests...)
	}
	if err := s.Scan(dests...); err != nil {
		return err
	}
	t.LastWatchedAt = parseSQLiteTime(*lws)
	t.WatchProviders = parseWatchProviders(*wpr)
	return nil
}
