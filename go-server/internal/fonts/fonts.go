package fonts

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/golang/freetype/truetype"
)

type Cache struct {
	mu       sync.Mutex
	cacheDir string
}

func NewCache(cacheDir string) *Cache {
	_ = os.MkdirAll(cacheDir, 0755)
	return &Cache{cacheDir: cacheDir}
}

func (c *Cache) LoadFont(fontURL string) (*truetype.Font, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean/sanitize URL to make it a filename
	safeFilename := sanitizeFilename(fontURL) + ".ttf"
	localPath := filepath.Join(c.cacheDir, safeFilename)

	// Check if already cached
	if _, err := os.Stat(localPath); err == nil {
		return readFontFile(localPath)
	}

	// Retrieve TTF URL (might be a CSS stylesheet URL)
	ttfURL := fontURL
	if strings.Contains(fontURL, "fonts.googleapis.com") {
		var err error
		ttfURL, err = extractTTFURLFromCSS(fontURL)
		if err != nil {
			return nil, fmt.Errorf("failed to extract TTF from Google Fonts CSS: %w", err)
		}
	}

	// Download and save
	resp, err := http.Get(ttfURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download font from %s: %w", ttfURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("font download URL returned status %d", resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create local font file: %w", err)
	}
	defer out.Close()

	if _, err = io.Copy(out, resp.Body); err != nil {
		return nil, fmt.Errorf("failed to write font file payload: %w", err)
	}

	return readFontFile(localPath)
}

func readFontFile(path string) (*truetype.Font, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return truetype.Parse(bytes)
}

func sanitizeFilename(urlStr string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return reg.ReplaceAllString(urlStr, "_")
}

func extractTTFURLFromCSS(cssURL string) (string, error) {
	// Request with a User-Agent that gets standard TTF files from Google Fonts (MSIE 9.0)
	req, err := http.NewRequest("GET", cssURL, nil)
	if err != nil {
		return "", err
	}
	// By default, not specifying a modern browser User-Agent forces Google Fonts to return .ttf urls

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Google Fonts CSS returned status %d", resp.StatusCode)
	}

	cssBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Regex to locate standard TTF url inside CSS rule: url(https://...ttf)
	re := regexp.MustCompile(`url\((https://[^\)]+\.ttf)\)`)
	matches := re.FindStringSubmatch(string(cssBytes))
	if len(matches) < 2 {
		return "", fmt.Errorf("no TTF font URL matches found in CSS payload")
	}

	return matches[1], nil
}
