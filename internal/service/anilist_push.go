package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/nicolasvasse/plextracker/internal/database"
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

	mediaIDStr, err := s.seasonIDs.Get(ctx, seasonID, providerAniList)
	if err != nil {
		return err
	}
	if mediaIDStr == "" {
		s.log.Debug("anilist push skipped: no season mapping", "season_id", seasonID)
		return nil
	}
	mediaID, err := strconv.ParseInt(mediaIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("anilist push: invalid media id %q: %w", mediaIDStr, err)
	}

	season, err := s.seasons.GetWithProgress(ctx, seasonID)
	if err != nil {
		return err
	}
	title, err := s.titles.GetByID(season.TitleID)
	if err != nil {
		return err
	}

	derivedStatus, progress := DeriveSeasonState(string(title.Status), season.TotalEpisodes, season.WatchedEpisodes)

	var score *int
	if ShouldPushRating(derivedStatus) && title.MyRating != nil {
		score = title.MyRating
	}

	return s.send(ctx, matching.SaveMediaListEntryInput{
		MediaID:  mediaID,
		Status:   derivedStatus,
		Progress: progress,
		Score:    score,
	}, token, "season_id", seasonID)
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

// ShouldPushRating returns true when the derived status warrants a rating
// push — the user has formed an opinion (finished or abandoned).
func ShouldPushRating(derivedStatus string) bool {
	return derivedStatus == "COMPLETED" || derivedStatus == "DROPPED"
}
