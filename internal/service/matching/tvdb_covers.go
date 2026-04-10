package matching

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const tvdbArtworkBaseURL = "https://artworks.thetvdb.com"

// DownloadCover downloads a cover image from TVDB and saves it to destDir.
// imageURL is the full URL returned by the TVDB API (e.g. https://artworks.thetvdb.com/...).
// Returns the local filename (not the full path), prefixed with "tvdb_<id>".
func (c *TVDBClient) DownloadCover(imageURL string, tvdbID int64, destDir string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("empty TVDB image URL")
	}

	// In test mode, rewrite the URL to use the mock server
	if c.baseURL != tvdbBaseURL && strings.HasPrefix(imageURL, tvdbArtworkBaseURL) {
		imageURL = c.baseURL + "/artwork" + imageURL[len(tvdbArtworkBaseURL):]
	}

	resp, err := c.httpClient.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("tvdb download cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tvdb download cover: status %d", resp.StatusCode)
	}

	ext := filepath.Ext(imageURL)
	if ext == "" {
		ext = ".jpg"
	}
	// Keep only the extension without query params
	if idx := strings.Index(ext, "?"); idx != -1 {
		ext = ext[:idx]
	}

	filename := fmt.Sprintf("tvdb_%d%s", tvdbID, ext)
	destPath := filepath.Join(destDir, filename)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create cover dir: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create tvdb cover file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write tvdb cover: %w", err)
	}

	return filename, nil
}
