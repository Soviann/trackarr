package matching

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadCover downloads a poster from TMDB and saves it to destDir.
// Returns the local filename (not the full path). The ctx must be the caller's
// (e.g. webhook request, taskqueue worker, refresh tick) so a slow CDN or a
// shutdown cancels the request body in flight rather than pinning the only
// writeDB connection until the HTTP client's own timeout fires.
func (c *TMDBClient) DownloadCover(ctx context.Context, posterPath string, destDir string) (string, error) {
	if posterPath == "" {
		return "", fmt.Errorf("empty poster path")
	}

	imageURL := tmdbImageURL + posterPath
	if c.baseURL != tmdbBaseURL {
		// Test mode: use baseURL for image requests too
		imageURL = c.baseURL + "/image" + posterPath
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("build cover request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download cover: status %d", resp.StatusCode)
	}

	filename := filepath.Base(posterPath)
	destPath := filepath.Join(destDir, filename)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create cover dir: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create cover file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = os.Remove(destPath)
		return "", fmt.Errorf("write cover: %w", err)
	}

	return filename, nil
}
