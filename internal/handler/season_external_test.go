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
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSeasonParam attaches seasonID as a chi URL param to the request.
func withSeasonParam(r *http.Request, seasonID int64) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("seasonID", strconv.FormatInt(seasonID, 10))
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// withSeasonAndExternalParam attaches both seasonID and externalID chi URL params.
func withSeasonAndExternalParam(r *http.Request, seasonID int64, externalID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("seasonID", strconv.FormatInt(seasonID, 10))
	rctx.URLParams.Add("externalID", externalID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- AddAniListID ---

func TestAddAniListID_PersistsAndEnqueues(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	body, _ := json.Marshal(map[string]string{"anilist_id": "145064"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = withSeasonParam(req, seasonID)
	rr := httptest.NewRecorder()

	err := h.AddAniListID(rr, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	parts, err := repository.NewSeasonExternalIDRepository(db).ListParts(context.Background(), seasonID, repository.ProviderAniList)
	require.NoError(t, err)
	require.Len(t, parts, 1)
	assert.Equal(t, "145064", parts[0].ExternalID)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeAniListPushSeason, tasks[0].TaskType)

	var p service.AniListPushSeasonPayload
	require.NoError(t, json.Unmarshal([]byte(tasks[0].Payload), &p))
	assert.Equal(t, seasonID, p.SeasonID)
}

func TestAddAniListID_URLFormat_ExtractsIDAndSetsIsAnime(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Texhnolyze", false)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	body, _ := json.Marshal(map[string]string{"anilist_id": "https://anilist.co/anime/26/Texhnolyze/"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = withSeasonParam(req, seasonID)
	rr := httptest.NewRecorder()

	err := h.AddAniListID(rr, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	parts, err := repository.NewSeasonExternalIDRepository(db).ListParts(context.Background(), seasonID, repository.ProviderAniList)
	require.NoError(t, err)
	require.Len(t, parts, 1)
	assert.Equal(t, "26", parts[0].ExternalID)

	title, err := repository.NewTitleRepository(db).GetByID(titleID)
	require.NoError(t, err)
	assert.True(t, title.IsAnime)
}

func TestAddAniListID_Idempotent(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	for range 2 {
		body, _ := json.Marshal(map[string]string{"anilist_id": "145064"})
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req = withSeasonParam(req, seasonID)
		rr := httptest.NewRecorder()
		require.NoError(t, h.AddAniListID(rr, req))
		assert.Equal(t, http.StatusNoContent, rr.Code)
	}

	parts, err := repository.NewSeasonExternalIDRepository(db).ListParts(context.Background(), seasonID, repository.ProviderAniList)
	require.NoError(t, err)
	assert.Len(t, parts, 1)
}

func TestAddAniListID_RejectsEmpty(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	body, _ := json.Marshal(map[string]string{"anilist_id": ""})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = withSeasonParam(req, seasonID)
	rr := httptest.NewRecorder()

	err := h.AddAniListID(rr, req)
	require.Error(t, err)

	var apiErr *httputil.APIError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

// --- RemoveAniListID ---

func TestRemoveAniListID_RemovesOnlyTargetPart(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.InsertSeasonExternalID(t, db, seasonID, repository.ProviderAniList, "145064")
	testutil.InsertSeasonExternalID(t, db, seasonID, repository.ProviderAniList, "166240")

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = withSeasonAndExternalParam(req, seasonID, "145064")
	rr := httptest.NewRecorder()

	err := h.RemoveAniListID(rr, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	parts, err := repository.NewSeasonExternalIDRepository(db).ListParts(context.Background(), seasonID, repository.ProviderAniList)
	require.NoError(t, err)
	require.Len(t, parts, 1)
	assert.Equal(t, "166240", parts[0].ExternalID)
}

// --- ReorderAniList ---

func TestReorderAniList_SetsSortOrder(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := handler.NewSeasonExternalHandler(db)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.InsertSeasonExternalID(t, db, seasonID, repository.ProviderAniList, "100")
	testutil.InsertSeasonExternalID(t, db, seasonID, repository.ProviderAniList, "200")

	// Without explicit order, 100 < 200 alphabetically so 100 comes first.
	// Reorder to put 200 first.
	body, _ := json.Marshal(map[string][]string{"ordered_ids": {"200", "100"}})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = withSeasonParam(req, seasonID)
	rr := httptest.NewRecorder()

	err := h.ReorderAniList(rr, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	parts, err := repository.NewSeasonExternalIDRepository(db).ListParts(context.Background(), seasonID, repository.ProviderAniList)
	require.NoError(t, err)
	require.Len(t, parts, 2)
	assert.Equal(t, "200", parts[0].ExternalID)
	assert.Equal(t, "100", parts[1].ExternalID)
}
