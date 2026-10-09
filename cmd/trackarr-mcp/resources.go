package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerResources(s *server.MCPServer, client *Client) {
	// 1. trackarr://library/summary
	summaryResource := mcp.NewResource(
		"trackarr://library/summary",
		"Trackarr Library Summary",
		mcp.WithResourceDescription("Live summary statistics and media counts of the Trackarr library."),
		mcp.WithMIMEType("application/json"),
	)
	s.AddResource(summaryResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		stats, err := client.GetStats(ctx)
		if err != nil {
			return nil, fmt.Errorf("read library summary: %w", err)
		}
		b, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal summary: %w", err)
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "trackarr://library/summary",
				MIMEType: "application/json",
				Text:     string(b),
			},
		}, nil
	})

	// 2. trackarr://continue-watching
	continueWatchingResource := mcp.NewResource(
		"trackarr://continue-watching",
		"Continue Watching",
		mcp.WithResourceDescription("Active series in progress with next episodes to watch."),
		mcp.WithMIMEType("application/json"),
	)
	s.AddResource(continueWatchingResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		items, err := client.GetContinueWatching(ctx)
		if err != nil {
			return nil, fmt.Errorf("read continue watching: %w", err)
		}
		b, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal continue watching: %w", err)
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "trackarr://continue-watching",
				MIMEType: "application/json",
				Text:     string(b),
			},
		}, nil
	})
}
