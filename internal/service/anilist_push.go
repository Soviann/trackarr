package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

const (
	settingAniListToken        = "anilist_token"
	settingAniListTokenInvalid = "anilist_token_invalid"
	providerAniList            = "anilist"
)

// aniListPushClient is the subset of matching.AniListClient needed for pushes.
// Narrowing the dependency lets tests inject a fake without wiring the real
// HTTP client.
type aniListPushClient interface {
	SaveMediaListEntry(ctx context.Context, in matching.SaveMediaListEntryInput, accessToken string) error
}

// AniListPushService pushes a season (or movie) state to AniList.
//
// Callers are responsible for deciding *when* to push (watch events, status
// changes, rating updates) — this service only handles the "one push" unit.
// It derives status + progress from DB state, enforces the rating guard, and
// flags the token invalid on 401 so the next call skips silently instead of
// hammering the API.
type AniListPushService struct {
	writeDB   *sql.DB
	client    aniListPushClient
	log       *slog.Logger
	seasons   *repository.SeasonRepository
	seasonIDs *repository.SeasonExternalIDRepository
	titles    *repository.TitleRepository
	settings  *repository.SettingRepository
}

func NewAniListPushService(writeDB *sql.DB, client aniListPushClient, log *slog.Logger) *AniListPushService {
	return &AniListPushService{
		writeDB:   writeDB,
		client:    client,
		log:       log,
		seasons:   repository.NewSeasonRepository(writeDB),
		seasonIDs: repository.NewSeasonExternalIDRepository(writeDB),
		titles:    repository.NewTitleRepository(writeDB),
		settings:  repository.NewSettingRepository(writeDB),
	}
}

// PushSeasonState pushes the current state of a season to its mapped AniList
// media entry. Silently skips when:
//   - no AniList token is configured
//   - the stored token has been flagged invalid (awaiting user reconnect)
//   - the season has no AniList mapping in season_external_ids
//
// Errors from AniList are logged and returned; a 401 is swallowed after
// flagging the token invalid (so the calling task handler doesn't retry).
func (s *AniListPushService) PushSeasonState(ctx context.Context, seasonID int64) error {
	token, skip, err := s.tokenOrSkip(ctx, "season_id", seasonID)
	if err != nil || skip {
		return err
	}

	parts, err := s.seasonIDs.ListParts(ctx, seasonID, providerAniList)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		s.log.Debug("anilist push skipped: no season mapping", "season_id", seasonID)
		return nil
	}

	season, err := s.seasons.GetWithProgress(ctx, seasonID)
	if err != nil {
		return err
	}
	title, err := s.titles.GetByID(season.TitleID)
	if err != nil {
		return err
	}
	watched, err := s.seasons.WatchedEpisodeNumbers(ctx, seasonID)
	if err != nil {
		return err
	}

	for _, ps := range DerivePartStates(string(title.Status), parts, watched) {
		var score *int
		if ps.Rating && title.MyRating != nil {
			score = title.MyRating
		}
		if err := s.send(ctx, matching.SaveMediaListEntryInput{
			MediaID:  ps.MediaID,
			Status:   ps.Status,
			Progress: ps.Progress,
			Score:    score,
		}, token, "season_id", seasonID); err != nil {
			return err // a 401 is already swallowed inside send; a real error stops the rest
		}
	}
	return nil
}

// PushMovieState pushes the current state of an anime movie to AniList using
// titles.anilist_id directly (movies have no season mapping). Silently skips
// when the title has no AniList ID, when there's no token, or when the token
// is flagged invalid — same escape hatches as PushSeasonState.
func (s *AniListPushService) PushMovieState(ctx context.Context, titleID int64) error {
	token, skip, err := s.tokenOrSkip(ctx, "title_id", titleID)
	if err != nil || skip {
		return err
	}

	title, err := s.titles.GetByID(titleID)
	if err != nil {
		return err
	}
	if title.AniListID == nil || *title.AniListID == 0 {
		s.log.Debug("anilist push skipped: movie has no AniList id", "title_id", titleID)
		return nil
	}

	var status string
	switch title.Status {
	case "completed", "watching":
		status = "COMPLETED"
	case "dropped":
		status = "DROPPED"
	case "plan_to_watch":
		status = "PLANNING"
	default:
		return nil
	}

	var score *int
	if (status == "COMPLETED" || status == "DROPPED") && title.MyRating != nil {
		score = title.MyRating
	}

	return s.send(ctx, matching.SaveMediaListEntryInput{
		MediaID:  *title.AniListID,
		Status:   status,
		Progress: 1,
		Score:    score,
	}, token, "title_id", titleID)
}

// tokenOrSkip returns (token, false, nil) when the push should proceed, or
// ("", true, nil) when the caller should silently skip (no token / token
// flagged invalid). The logKey/logVal pair names the entity in skip logs.
func (s *AniListPushService) tokenOrSkip(_ context.Context, logKey string, logVal int64) (string, bool, error) {
	token, _ := s.settings.Get(settingAniListToken)
	if token == "" {
		s.log.Debug("anilist push skipped: no token", logKey, logVal)
		return "", true, nil
	}
	if invalid, _ := s.settings.Get(settingAniListTokenInvalid); invalid == "true" {
		s.log.Debug("anilist push skipped: token flagged invalid", logKey, logVal)
		return "", true, nil
	}
	return token, false, nil
}

// send calls the AniList client and handles 401 by flagging the token invalid.
// Other errors are logged and returned so task handlers can retry with backoff.
func (s *AniListPushService) send(ctx context.Context, in matching.SaveMediaListEntryInput, token, logKey string, logVal int64) error {
	err := s.client.SaveMediaListEntry(ctx, in, token)

	var tokenInvalid matching.TokenInvalidError
	if errors.As(err, &tokenInvalid) {
		s.log.Warn("anilist token rejected, flagging invalid", logKey, logVal)
		if flagErr := s.flagTokenInvalid(ctx); flagErr != nil {
			s.log.Error("failed to flag anilist token invalid", "err", flagErr)
		}
		return nil
	}
	if err != nil {
		s.log.Warn("anilist push failed", logKey, logVal, "err", err)
		return err
	}
	s.log.Info("anilist push ok", logKey, logVal, "status", in.Status, "progress", in.Progress, "score", in.Score)
	return nil
}

func (s *AniListPushService) flagTokenInvalid(ctx context.Context) error {
	return database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Set(ctx, settingAniListTokenInvalid, "true")
	})
}

// DeriveSeasonState maps PlexTracker's title status + per-season episode
// counts to AniList's MediaListStatus + progress.
//
// COMPLETED wins over DROPPED: if every episode is watched we report
// COMPLETED regardless of the title's dropped status.
func DeriveSeasonState(titleStatus string, totalEpisodes, watchedEpisodes int) (status string, progress int) {
	progress = watchedEpisodes

	if totalEpisodes > 0 && watchedEpisodes >= totalEpisodes {
		return "COMPLETED", progress
	}
	if titleStatus == "dropped" {
		return "DROPPED", progress
	}
	if watchedEpisodes == 0 {
		if titleStatus == "plan_to_watch" {
			return "PLANNING", 0
		}
		return "CURRENT", 0
	}
	return "CURRENT", progress
}

// PartPush is one part's derived AniList state, ready to send. MediaID is the
// parsed AniList media id; Rating flags whether the status warrants a rating.
type PartPush struct {
	MediaID  int64
	Status   string
	Progress int
	Rating   bool
}

// DerivePartStates splits a season's watched episodes across its ordered AniList
// parts and derives each part's AniList state. Part i covers season episode
// numbers (cum, cum+count]; the last part (and any part with an unknown count)
// absorbs the remainder so a watched episode is never dropped. Parts with a
// non-numeric ExternalID are skipped (the caller logs them); they never emit.
func DerivePartStates(titleStatus string, parts []model.AniListPart, watched []int) []PartPush {
	watchedSet := make(map[int]bool, len(watched))
	maxWatched := 0
	for _, n := range watched {
		watchedSet[n] = true
		if n > maxWatched {
			maxWatched = n
		}
	}

	out := make([]PartPush, 0, len(parts))
	cum := 0
	for i, p := range parts {
		mediaID, err := strconv.ParseInt(p.ExternalID, 10, 64)
		if err != nil {
			continue // non-numeric ids are skipped; nothing to push
		}
		count := 0
		if p.EpisodeCount != nil {
			count = *p.EpisodeCount
		}
		last := i == len(parts)-1
		// Range upper bound: cum+count, or "everything remaining" for the last
		// part / a part with an unknown count.
		hi := cum + count
		if last || count == 0 {
			hi = maxWatched
			if cum+count > hi {
				hi = cum + count
			}
		}
		inRange := 0
		for ep := cum + 1; ep <= hi; ep++ {
			if watchedSet[ep] {
				inRange++
			}
		}
		status, progress := derivePartState(titleStatus, count, inRange)
		out = append(out, PartPush{MediaID: mediaID, Status: status, Progress: progress, Rating: ShouldPushRating(status)})
		cum += count
	}
	return out
}

// derivePartState mirrors DeriveSeasonState but scoped to one part's episode
// range: COMPLETED wins when the part is fully watched.
func derivePartState(titleStatus string, count, watchedInRange int) (status string, progress int) {
	progress = watchedInRange
	if count > 0 && watchedInRange >= count {
		return "COMPLETED", count
	}
	if titleStatus == "dropped" {
		return "DROPPED", progress
	}
	if watchedInRange == 0 {
		if titleStatus == "plan_to_watch" {
			return "PLANNING", 0
		}
		return "CURRENT", 0
	}
	return "CURRENT", progress
}

// ShouldPushRating returns true when the derived status warrants a rating
// push — the user has formed an opinion (finished or abandoned).
func ShouldPushRating(derivedStatus string) bool {
	return derivedStatus == "COMPLETED" || derivedStatus == "DROPPED"
}
