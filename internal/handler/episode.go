package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type EpisodeHandler struct {
	titles   *repository.TitleRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
	push     service.PushNotifier
}

func NewEpisodeHandler(titles *repository.TitleRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, push service.PushNotifier) *EpisodeHandler {
	return &EpisodeHandler{titles: titles, episodes: episodes, events: events, push: push}
}

func (h *EpisodeHandler) ToggleWatched(w http.ResponseWriter, r *http.Request) error {
	titleID, err := httputil.ParseIDParam(r, "titleID")
	if err != nil {
		return httputil.BadRequest("Invalid title ID")
	}

	episodeID, err := httputil.ParseIDParam(r, "episodeID")
	if err != nil {
		return httputil.BadRequest("Invalid episode ID")
	}

	ep, err := h.episodes.ToggleWatched(episodeID)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	// Log watch event if toggled on
	if ep.Watched {
		_, _ = h.events.Create(&model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &episodeID,
			Source:    model.WatchEventSourceManual,
		})
	}

	// Return updated title for status auto-update
	title, _ := h.titles.GetByID(titleID)

	// Push rating prompt if all episodes of a season are now watched
	if ep.Watched && title != nil {
		h.maybePromptRating(title)
	}

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}

func (h *EpisodeHandler) BatchMarkWatched(w http.ResponseWriter, r *http.Request) error {
	titleID, err := httputil.ParseIDParam(r, "titleID")
	if err != nil {
		return httputil.BadRequest("Invalid title ID")
	}

	var body struct {
		EpisodeIDs []int64 `json:"episode_ids"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	now := time.Now().UTC()
	if err := h.episodes.BatchMarkWatched(body.EpisodeIDs, now); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	// Log watch events
	watchEvents := make([]model.WatchEvent, len(body.EpisodeIDs))
	for i, epID := range body.EpisodeIDs {
		id := epID
		watchEvents[i] = model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &id,
			Source:    model.WatchEventSourceManual,
		}
	}
	_ = h.events.BatchCreate(watchEvents)

	title, _ := h.titles.GetByID(titleID)

	// Push rating prompt if all episodes of a season are now watched
	if title != nil {
		h.maybePromptRating(title)
	}

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}

// maybePromptRating sends a push notification if any season has all episodes watched
// and the title has no rating yet.
func (h *EpisodeHandler) maybePromptRating(title *model.Title) {
	if title.MyRating != nil {
		return
	}
	for _, season := range title.Seasons {
		if len(season.Episodes) == 0 {
			continue
		}
		allWatched := true
		for _, ep := range season.Episodes {
			if !ep.Watched {
				allWatched = false
				break
			}
		}
		if allWatched {
			_ = h.push.SendNotification(
				fmt.Sprintf("Note %s ?", title.PrimaryName()),
				fmt.Sprintf("Tu as terminé la saison %d", season.SeasonNumber),
				fmt.Sprintf("/title/%d", title.ID),
			)
			return // One notification per batch
		}
	}
}
