package matching

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DownloadCover downloads a cover image from a full URL (e.g. AniList CDN)
// and saves it to destDir. Returns the local filename. ctx propagates the
// caller's deadline / cancellation so a stalled CDN cannot pin the only
// writeDB connection.
func (c *AniListClient) DownloadCover(ctx context.Context, imageURL string, destDir string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("empty image URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("build anilist cover request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download anilist cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download anilist cover: status %d", resp.StatusCode)
	}

	// Use the original filename from URL if possible, otherwise hash-based
	filename := filenameFromURL(imageURL)
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

// filenameFromURL extracts a filename from an AniList CDN URL.
// e.g. "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx21459-nYh85uj2Fuwr.jpg"
// → "al-bx21459-nYh85uj2Fuwr.jpg"
// Falls back to a hash-based name if the URL is unexpected.
func filenameFromURL(imageURL string) string {
	base := filepath.Base(imageURL)
	if base != "" && base != "." && base != "/" {
		// Prefix with "al-" to avoid collisions with TMDB filenames
		return "al-" + base
	}
	// Fallback: hash the URL
	h := sha256.Sum256([]byte(imageURL))
	ext := ".jpg"
	if idx := strings.LastIndex(imageURL, "."); idx != -1 {
		ext = imageURL[idx:]
	}
	return fmt.Sprintf("al-%x%s", h[:8], ext)
}
