package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTools(s *server.MCPServer, client *Client) {
	// 1. trackarr_search
	searchTool := mcp.NewTool("trackarr_search",
		mcp.WithDescription("Search movies and series in the Trackarr library by keyword, media type, or watch status."),
		mcp.WithString("query", mcp.Description("Search keyword, title name, or external URL")),
		mcp.WithString("type", mcp.Description("Filter by media type ('movie' or 'series')"), mcp.Enum("movie", "series")),
		mcp.WithString("status", mcp.Description("Filter by watch status ('plan_to_watch', 'watching', 'completed', 'paused', 'dropped')"), mcp.Enum("plan_to_watch", "watching", "completed", "paused", "dropped")),
		mcp.WithInteger("limit", mcp.Description("Maximum number of results to return (default: 20, max: 100)")),
	)
	s.AddTool(searchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		titleType := req.GetString("type", "")
		status := req.GetString("status", "")
		limit := req.GetInt("limit", 20)

		res, err := client.SearchTitles(ctx, query, titleType, status, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}
		return jsonResult(res)
	})

	// 2. trackarr_get_title
	getTitleTool := mcp.NewTool("trackarr_get_title",
		mcp.WithDescription("Get comprehensive details of a title by its ID, including seasons, episodes, and external IDs."),
		mcp.WithInteger("id", mcp.Required(), mcp.Description("Trackarr title ID")),
	)
	s.AddTool(getTitleTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireInt("id")
		if err != nil {
			return mcp.NewToolResultError("parameter 'id' is required and must be an integer"), nil
		}
		detail, err := client.GetTitle(ctx, int64(id))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get title failed: %v", err)), nil
		}
		return jsonResult(detail)
	})

	// 3. trackarr_get_continue_watching
	continueWatchingTool := mcp.NewTool("trackarr_get_continue_watching",
		mcp.WithDescription("Get currently in-progress series with their next unwatched episode."),
	)
	s.AddTool(continueWatchingTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		items, err := client.GetContinueWatching(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get continue watching failed: %v", err)), nil
		}
		return jsonResult(items)
	})

	// 4. trackarr_resolve_url
	resolveURLTool := mcp.NewTool("trackarr_resolve_url",
		mcp.WithDescription("Resolve an external URL (IMDb, TMDb, TVDB, AniList) to check if the title exists in the library or retrieve its metadata."),
		mcp.WithString("url", mcp.Required(), mcp.Description("External URL (e.g. https://www.imdb.com/title/tt0137523/)")),
	)
	s.AddTool(resolveURLTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawURL, err := req.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError("parameter 'url' is required"), nil
		}
		res, err := client.ResolveURL(ctx, rawURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve URL failed: %v", err)), nil
		}
		return jsonResult(res)
	})

	// 5. trackarr_get_stats
	getStatsTool := mcp.NewTool("trackarr_get_stats",
		mcp.WithDescription("Get library statistics including total movies, series, watch time, and episodes watched."),
	)
	s.AddTool(getStatsTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stats, err := client.GetStats(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get stats failed: %v", err)), nil
		}
		return jsonResult(stats)
	})

	// 6. trackarr_add_title
	addTitleTool := mcp.NewTool("trackarr_add_title",
		mcp.WithDescription("Add a new movie or TV series to Trackarr, optionally via external URL or IDs."),
		mcp.WithString("title", mcp.Description("Title name (optional if URL or external IDs provided)")),
		mcp.WithString("type", mcp.Description("Media type ('movie' or 'series')"), mcp.Enum("movie", "series")),
		mcp.WithInteger("year", mcp.Description("Release year")),
		mcp.WithString("url", mcp.Description("External URL to auto-extract IDs and metadata")),
		mcp.WithInteger("tmdb_id", mcp.Description("TMDb ID")),
		mcp.WithInteger("tvdb_id", mcp.Description("TVDB ID")),
		mcp.WithString("imdb_id", mcp.Description("IMDb ID (e.g. tt1234567)")),
		mcp.WithInteger("anilist_id", mcp.Description("AniList ID")),
		mcp.WithBoolean("is_anime", mcp.Description("Flag indicating whether title is anime")),
		mcp.WithString("status", mcp.Description("Watch status ('plan_to_watch', 'watching', 'completed', 'paused', 'dropped')"), mcp.Enum("plan_to_watch", "watching", "completed", "paused", "dropped")),
	)
	s.AddTool(addTitleTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var item BatchCreateItem
		if err := req.BindArguments(&item); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		if item.Title == "" && item.URL == "" && item.IMDBID == nil && item.TMDBID == nil && item.TVDBID == nil && item.AniListID == nil {
			return mcp.NewToolResultError("at least one identifier ('title', 'url', 'imdb_id', 'tmdb_id', 'tvdb_id', 'anilist_id') is required"), nil
		}

		res, err := client.AddTitle(ctx, item)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("add title failed: %v", err)), nil
		}
		if res.Status == "error" {
			return mcp.NewToolResultError(fmt.Sprintf("failed to add title: %s", res.Error)), nil
		}
		return jsonResult(res)
	})

	// 7. trackarr_batch_add
	batchAddTool := mcp.NewTool("trackarr_batch_add",
		mcp.WithDescription("Batch ingest up to 100 titles in a single atomic request."),
		mcp.WithArray("items", mcp.Required(), mcp.Description("Array of title items to add (each with title, url, type, year, external IDs, status)")),
	)
	s.AddTool(batchAddTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Items []BatchCreateItem `json:"items"`
		}
		if err := req.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		if len(args.Items) == 0 {
			return mcp.NewToolResultError("'items' must contain between 1 and 100 items"), nil
		}
		if len(args.Items) > 100 {
			return mcp.NewToolResultError("'items' cannot exceed 100 items"), nil
		}

		res, err := client.BatchAddTitles(ctx, args.Items)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("batch add failed: %v", err)), nil
		}
		return jsonResult(res)
	})

	// 8. trackarr_update_title
	updateTitleTool := mcp.NewTool("trackarr_update_title",
		mcp.WithDescription("Update title status, user rating (0-10), personal notes, or settings."),
		mcp.WithInteger("id", mcp.Required(), mcp.Description("Trackarr title ID")),
		mcp.WithString("status", mcp.Description("Watch status ('plan_to_watch', 'watching', 'completed', 'paused', 'dropped')"), mcp.Enum("plan_to_watch", "watching", "completed", "paused", "dropped")),
		mcp.WithInteger("my_rating", mcp.Description("User rating (0-10)")),
		mcp.WithString("personal_notes", mcp.Description("Personal notes / user review")),
		mcp.WithBoolean("is_anime", mcp.Description("Mark whether title is anime")),
		mcp.WithBoolean("arr_ignored", mcp.Description("Exclude from Sonarr/Radarr sync")),
	)
	s.AddTool(updateTitleTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireInt("id")
		if err != nil {
			return mcp.NewToolResultError("parameter 'id' is required"), nil
		}
		var payload TitleUpdatePayload
		if err := req.BindArguments(&payload); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}

		updated, err := client.UpdateTitle(ctx, int64(id), payload)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("update title failed: %v", err)), nil
		}
		return jsonResult(updated)
	})

	// 9. trackarr_set_episode_watched
	setEpisodeWatchedTool := mcp.NewTool("trackarr_set_episode_watched",
		mcp.WithDescription("Mark a specific TV series episode as watched or unwatched."),
		mcp.WithInteger("title_id", mcp.Required(), mcp.Description("Trackarr series title ID")),
		mcp.WithInteger("episode_id", mcp.Required(), mcp.Description("Episode ID")),
		mcp.WithBoolean("watched", mcp.Required(), mcp.Description("True to mark as watched, false to mark as unwatched")),
	)
	s.AddTool(setEpisodeWatchedTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		titleID, err := req.RequireInt("title_id")
		if err != nil {
			return mcp.NewToolResultError("parameter 'title_id' is required"), nil
		}
		episodeID, err := req.RequireInt("episode_id")
		if err != nil {
			return mcp.NewToolResultError("parameter 'episode_id' is required"), nil
		}
		watched, err := req.RequireBool("watched")
		if err != nil {
			return mcp.NewToolResultError("parameter 'watched' is required"), nil
		}

		if err := client.SetEpisodeWatched(ctx, int64(titleID), int64(episodeID), watched); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("set episode watched failed: %v", err)), nil
		}
		action := "watched"
		if !watched {
			action = "unwatched"
		}
		return mcp.NewToolResultText(fmt.Sprintf("Episode %d of title %d successfully marked as %s.", episodeID, titleID, action)), nil
	})

	// 10. trackarr_delete_title (Destructive, requires library:delete scope)
	deleteTitleTool := mcp.NewTool("trackarr_delete_title",
		mcp.WithDescription("Delete a title from the Trackarr library. Requires an API key with the 'library:delete' scope."),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithInteger("id", mcp.Required(), mcp.Description("Trackarr title ID to delete")),
	)
	s.AddTool(deleteTitleTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireInt("id")
		if err != nil {
			return mcp.NewToolResultError("parameter 'id' is required"), nil
		}
		if err := client.DeleteTitle(ctx, int64(id)); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("delete title failed: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Title %d successfully deleted.", id)), nil
	})

	// 11. trackarr_push_to_arr (Arr write, requires arr:write scope)
	pushToArrTool := mcp.NewTool("trackarr_push_to_arr",
		mcp.WithDescription("Push a movie or series to Radarr or Sonarr. Requires an API key with the 'arr:write' scope."),
		mcp.WithInteger("id", mcp.Required(), mcp.Description("Trackarr title ID to push")),
	)
	s.AddTool(pushToArrTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireInt("id")
		if err != nil {
			return mcp.NewToolResultError("parameter 'id' is required"), nil
		}
		arrID, err := client.PushToArr(ctx, int64(id))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("push to Arr failed: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Successfully pushed title %d to Arr (Arr ID: %d).", id, arrID)), nil
	})
}

func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
