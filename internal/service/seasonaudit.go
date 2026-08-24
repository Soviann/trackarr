package service

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

// SeasonAuditProposal is a single suggested season attachment: merge the source
// title into the target as a given season ordinal.
type SeasonAuditProposal struct {
	SourceTitleID      int64   `json:"source_title_id"`
	SourceName         string  `json:"source_name"`
	SourceYear         int     `json:"source_year"`
	SourceCoverURL     *string `json:"source_cover_url"`
	SourceSeasonsCount int     `json:"source_seasons_count"`
	TargetTitleID      int64   `json:"target_title_id"`
	TargetName         string  `json:"target_name"`
	TargetYear         int     `json:"target_year"`
	TargetCoverURL     *string `json:"target_cover_url"`
	TargetSeasonsCount int     `json:"target_seasons_count"`
	SeasonNumber       int     `json:"season_number"`
	SharedID           string  `json:"shared_id"`
}

// SeasonAuditService scans duplicate-external-id series groups and proposes (or
// applies) season attachments that consolidate strays under their parent.
type SeasonAuditService struct {
	db       *sql.DB
	titles   *repository.TitleRepository
	audit    *repository.SeasonAuditRepository
	pipeline *matching.Pipeline
	titleSvc *TitleService
}

// NewSeasonAuditService creates a new SeasonAuditService.
func NewSeasonAuditService(db *sql.DB, titles *repository.TitleRepository, audit *repository.SeasonAuditRepository, pipeline *matching.Pipeline, titleSvc *TitleService) *SeasonAuditService {
	return &SeasonAuditService{db: db, titles: titles, audit: audit, pipeline: pipeline, titleSvc: titleSvc}
}

// Scan walks the duplicate-series groups and returns one proposal per non-target
// member. Within a group the target (parent) is the member whose AniList chain
// is the root; absent any AniList-rooted member, the member with the most
// seasons wins (lowest id breaks ties). AniList resolution is best-effort: a
// missing id or a resolve error yields a nil chain (SeasonNumber 0), never an
// aborted scan.
func (s *SeasonAuditService) Scan(ctx context.Context) ([]SeasonAuditProposal, error) {
	groups, err := s.audit.DuplicateSeriesGroups()
	if err != nil {
		return nil, fmt.Errorf("season audit: scan groups: %w", err)
	}

	proposals := []SeasonAuditProposal{}
	for _, group := range groups {
		// Resolve each member's chain once (used for both target selection and
		// the proposal season number).
		chains := make(map[int64]*matching.SeasonChain, len(group.Titles))
		for i := range group.Titles {
			m := &group.Titles[i]
			chains[m.ID] = s.resolveChain(ctx, m)
		}

		target := s.pickTarget(group.Titles, chains)
		if target == nil {
			continue
		}

		for i := range group.Titles {
			member := &group.Titles[i]
			if member.ID == target.ID {
				continue
			}

			dismissed, err := s.audit.IsDismissed(member.ID, target.ID)
			if err != nil {
				return nil, fmt.Errorf("season audit: is dismissed: %w", err)
			}
			if dismissed {
				continue
			}

			seasonNumber := 0
			if chain := chains[member.ID]; chain != nil {
				seasonNumber = chain.SeasonNumber
			}

			proposals = append(proposals, SeasonAuditProposal{
				SourceTitleID:      member.ID,
				SourceName:         member.PrimaryName(),
				SourceYear:         member.Year,
				SourceCoverURL:     member.CoverURL,
				SourceSeasonsCount: len(member.Seasons),
				TargetTitleID:      target.ID,
				TargetName:         target.PrimaryName(),
				TargetYear:         target.Year,
				TargetCoverURL:     target.CoverURL,
				TargetSeasonsCount: len(target.Seasons),
				SeasonNumber:       seasonNumber,
				SharedID:           group.SharedID,
			})
		}
	}

	slices.SortFunc(proposals, func(a, b SeasonAuditProposal) int {
		if c := strings.Compare(strings.ToLower(a.TargetName), strings.ToLower(b.TargetName)); c != 0 {
			return c
		}
		if c := cmp.Compare(a.TargetTitleID, b.TargetTitleID); c != 0 {
			return c
		}
		if c := cmp.Compare(a.SeasonNumber, b.SeasonNumber); c != 0 {
			return c
		}
		if c := strings.Compare(strings.ToLower(a.SourceName), strings.ToLower(b.SourceName)); c != 0 {
			return c
		}
		return cmp.Compare(a.SourceTitleID, b.SourceTitleID)
	})

	return proposals, nil
}

// resolveChain returns the member's AniList season chain, or nil when it has no
// AniList id, the pipeline is unavailable, or resolution fails.
func (s *SeasonAuditService) resolveChain(ctx context.Context, t *model.Title) *matching.SeasonChain {
	if s.pipeline == nil || t.AniListID == nil {
		return nil
	}
	chain, err := s.pipeline.ResolveAniListSeason(ctx, *t.AniListID)
	if err != nil {
		log.Printf("season audit: resolve anilist chain for title %d (anilist %d): %v", t.ID, *t.AniListID, err)
		return nil
	}
	return chain
}

// pickTarget chooses the parent of the group: the AniList-rooted member if one
// exists, else the member with the most seasons (lowest id wins ties).
func (s *SeasonAuditService) pickTarget(titles []model.Title, chains map[int64]*matching.SeasonChain) *model.Title {
	for i := range titles {
		if chain := chains[titles[i].ID]; chain != nil && chain.IsRoot {
			return &titles[i]
		}
	}

	var best *model.Title
	for i := range titles {
		t := &titles[i]
		switch {
		case best == nil:
			best = t
		case len(t.Seasons) > len(best.Seasons):
			best = t
		case len(t.Seasons) == len(best.Seasons) && t.ID < best.ID:
			best = t
		}
	}
	return best
}

// Accept merges the source title into the target as the given season. The merge
// (which deletes the source) runs first; a season_attached match event is then
// recorded on the target. Names are captured before the merge because the
// source row is gone afterward.
func (s *SeasonAuditService) Accept(ctx context.Context, sourceID, targetID int64, seasonNumber int) error {
	if sourceID == targetID {
		return fmt.Errorf("season audit: cannot merge title %d into itself", sourceID)
	}
	source, err := s.titles.GetByID(sourceID)
	if err != nil {
		return fmt.Errorf("season audit: load source %d: %w", sourceID, err)
	}
	target, err := s.titles.GetByID(targetID)
	if err != nil {
		return fmt.Errorf("season audit: load target %d: %w", targetID, err)
	}
	sourceName := source.PrimaryName()
	targetName := target.PrimaryName()

	offset := seasonNumber - 1
	if offset < 0 {
		offset = 0
	}

	if err := s.titleSvc.Merge(ctx, s.db, targetID, sourceID, &offset); err != nil {
		return fmt.Errorf("season audit: merge %d into %d: %w", sourceID, targetID, err)
	}

	detail := fmt.Sprintf("%q attached as Season %d of %q", sourceName, seasonNumber, targetName)
	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		return repository.NewMatchEventWriter(tx).Create(ctx, targetID, model.MatchEventSeasonAttached, detail)
	}); err != nil {
		return fmt.Errorf("season audit: record event: %w", err)
	}
	return nil
}

// Dismiss records that the (source, target) attachment should not be proposed
// again.
func (s *SeasonAuditService) Dismiss(ctx context.Context, sourceID, targetID int64) error {
	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		return repository.NewSeasonAuditWriter(tx).Dismiss(ctx, sourceID, targetID)
	}); err != nil {
		return fmt.Errorf("season audit: dismiss %d->%d: %w", sourceID, targetID, err)
	}
	return nil
}
