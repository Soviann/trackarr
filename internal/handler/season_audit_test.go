package handler_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSeasonAuditHandler(t *testing.T) (*handler.SeasonAuditHandler, *sql.DB, *repository.TitleRepository) {
	t.Helper()
	db := testutil.NewTestDB(t)
	titlesRepo := repository.NewTitleRepository(db)
	tasksRepo := repository.NewTaskRepository(db)
	titleSvc := service.NewTitleService(db, titlesRepo, tasksRepo, nil)
	auditRepo := repository.NewSeasonAuditRepository(db)
	auditSvc := service.NewSeasonAuditService(db, titlesRepo, auditRepo, nil, titleSvc)
	return handler.NewSeasonAuditHandler(auditSvc), db, titlesRepo
}

func TestSeasonAuditHandler_Endpoints(t *testing.T) {
	h, db, titlesRepo := setupSeasonAuditHandler(t)

	t.Run("List returns empty proposals when no duplicates exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/season-audit", nil)
		rr := httptest.NewRecorder()

		require.NoError(t, h.List(rr, req))
		assert.Equal(t, http.StatusOK, rr.Code)

		var res struct {
			Proposals []any `json:"proposals"`
		}
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Empty(t, res.Proposals)
	})

	t.Run("Accept validates payload", func(t *testing.T) {
		// Invalid JSON
		req1 := httptest.NewRequest(http.MethodPost, "/api/admin/season-audit/accept", bytes.NewReader([]byte("invalid")))
		rr1 := httptest.NewRecorder()
		assert.Error(t, h.Accept(rr1, req1))

		// Missing IDs
		req2 := httptest.NewRequest(http.MethodPost, "/api/admin/season-audit/accept", bytes.NewReader([]byte(`{"source_title_id":0,"target_title_id":0}`)))
		rr2 := httptest.NewRecorder()
		assert.Error(t, h.Accept(rr2, req2))

		// Same ID
		req3 := httptest.NewRequest(http.MethodPost, "/api/admin/season-audit/accept", bytes.NewReader([]byte(`{"source_title_id":5,"target_title_id":5}`)))
		rr3 := httptest.NewRecorder()
		assert.Error(t, h.Accept(rr3, req3))
	})

	t.Run("Dismiss records dismissal and returns ok", func(t *testing.T) {
		t1 := testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeSeries,
			Year:        2020,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
		}, []model.TitleName{{Name: "Series S1", Language: "en", IsPrimary: true}})

		t2 := testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeSeries,
			Year:        2021,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
		}, []model.TitleName{{Name: "Series S2", Language: "en", IsPrimary: true}})

		body, _ := json.Marshal(map[string]int64{
			"source_title_id": t1,
			"target_title_id": t2,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/admin/season-audit/dismiss", bytes.NewReader(body))
		rr := httptest.NewRecorder()

		require.NoError(t, h.Dismiss(rr, req))
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]string
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "ok", res["status"])
	})

	_ = titlesRepo
}
