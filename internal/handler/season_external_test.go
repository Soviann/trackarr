package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSeasonParam attaches seasonID as a chi URL param to the request.
func withSeasonParam(r *http.Request, seasonID int64) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("seasonID", strconv.FormatInt(seasonID, 10))
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestSetSeasonAniListID_PersistsAndEnqueues(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	body, _ := json.Marshal(map[string]string{"anilist_id": "145064"})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = withSeasonParam(req, seasonID)
	rr := httptest.NewRecorder()

	err := h.SetAniListID(rr, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	got, err := testutil.GetSeasonExternalID(t, db, seasonID, repository.ProviderAniList)
	require.NoError(t, err)
	assert.Equal(t, "145064", got)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeAniListPushSeason, tasks[0].TaskType)

	var p service.AniListPushSeasonPayload
	require.NoError(t, json.Unmarshal([]byte(tasks[0].Payload), &p))
	assert.Equal(t, seasonID, p.SeasonID)
}

func TestSetSeasonAniListID_RejectsEmpty(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	body, _ := json.Marshal(map[string]string{"anilist_id": ""})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = withSeasonParam(req, seasonID)
	rr := httptest.NewRecorder()

	err := h.SetAniListID(rr, req)
	require.Error(t, err)

	var apiErr *httputil.APIError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestDeleteSeasonAniListID_RemovesMappingNoEnqueue(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.InsertSeasonExternalID(t, db, seasonID, repository.ProviderAniList, "145064")

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = withSeasonParam(req, seasonID)
	rr := httptest.NewRecorder()

	err := h.ClearAniListID(rr, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	got, err := testutil.GetSeasonExternalID(t, db, seasonID, repository.ProviderAniList)
	require.NoError(t, err)
	assert.Equal(t, "", got)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	assert.Empty(t, tasks)
}
