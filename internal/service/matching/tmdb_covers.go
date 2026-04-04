package matching

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadCover downloads a poster from TMDB and saves it to destDir.
// Returns the local filename (not the full path).
func (c *TMDBClient) DownloadCover(posterPath string, destDir string) (string, error) {
	if posterPath == "" {
		return "", fmt.Errorf("empty poster path")
	}

	imageURL := tmdbImageURL + posterPath
	if c.baseURL != tmdbBaseURL {
		// Test mode: use baseURL for image requests too
		imageURL = c.baseURL + "/image" + posterPath
	}

	resp, err := c.httpClient.Get(imageURL)
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
		return "", fmt.Errorf("write cover: %w", err)
	}

	return filename, nil
}
