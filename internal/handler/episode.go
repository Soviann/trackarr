package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type EpisodeHandler struct {
	titles   *repository.TitleRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
	push     *service.PushService
}

func NewEpisodeHandler(titles *repository.TitleRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, push *service.PushService) *EpisodeHandler {
	return &EpisodeHandler{titles: titles, episodes: episodes, events: events, push: push}
}

func (h *EpisodeHandler) ToggleWatched(w http.ResponseWriter, r *http.Request) {
	titleID, err := strconv.ParseInt(chi.URLParam(r, "titleID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid title ID", http.StatusBadRequest)
		return
	}

	episodeID, err := strconv.ParseInt(chi.URLParam(r, "episodeID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid episode ID", http.StatusBadRequest)
		return
	}

	ep, err := h.episodes.ToggleWatched(episodeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	writeJSON(w, title)
}

func (h *EpisodeHandler) BatchMarkWatched(w http.ResponseWriter, r *http.Request) {
	titleID, err := strconv.ParseInt(chi.URLParam(r, "titleID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid title ID", http.StatusBadRequest)
		return
	}

	var body struct {
		EpisodeIDs []int64 `json:"episode_ids"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	if err := h.episodes.BatchMarkWatched(body.EpisodeIDs, now); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log watch events
	for _, epID := range body.EpisodeIDs {
		id := epID
		_, _ = h.events.Create(&model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &id,
			Source:    model.WatchEventSourceManual,
		})
	}

	title, _ := h.titles.GetByID(titleID)

	// Push rating prompt if all episodes of a season are now watched
	if title != nil {
		h.maybePromptRating(title)
	}

	writeJSON(w, title)
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
