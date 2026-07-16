package renderer

import (
	"net/http"
	"testing"
	"time"
)

func TestFontURLs(t *testing.T) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for family, cfg := range FontMap {
		t.Run(family+"-Regular", func(t *testing.T) {
			resp, err := client.Head(cfg.RegularURL)
			if err != nil {
				t.Fatalf("Failed to issue HEAD request to %s: %v", cfg.RegularURL, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Regular font URL %s for %s returned status code %d", cfg.RegularURL, family, resp.StatusCode)
			}
		})

		t.Run(family+"-Bold", func(t *testing.T) {
			resp, err := client.Head(cfg.BoldURL)
			if err != nil {
				t.Fatalf("Failed to issue HEAD request to %s: %v", cfg.BoldURL, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Bold font URL %s for %s returned status code %d", cfg.BoldURL, family, resp.StatusCode)
			}
		})
	}
}
