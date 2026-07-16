package renderer

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const fontDir = "./assets/fonts"

type FontConfig struct {
	RegularURL string
	BoldURL    string
	RegFile    string
	BoldFile   string
}

var FontMap = map[string]FontConfig{
	"Poppins": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/poppins/Poppins-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/poppins/Poppins-Bold.ttf",
		RegFile:    "Poppins-Regular.ttf",
		BoldFile:   "Poppins-Bold.ttf",
	},
	"Roboto": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/roboto/Roboto-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/roboto/Roboto-Bold.ttf",
		RegFile:    "Roboto-Regular.ttf",
		BoldFile:   "Roboto-Bold.ttf",
	},
	"Noto Sans Bengali": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosansbengali/NotoSansBengali-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosansbengali/NotoSansBengali-Bold.ttf",
		RegFile:    "NotoSansBengali-Regular.ttf",
		BoldFile:   "NotoSansBengali-Bold.ttf",
	},
	"Noto Sans Arabic": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosansarabic/NotoSansArabic-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosansarabic/NotoSansArabic-Bold.ttf",
		RegFile:    "NotoSansArabic-Regular.ttf",
		BoldFile:   "NotoSansArabic-Bold.ttf",
	},
	"Noto Sans Oriya": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosansoriya/NotoSansOriya-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosansoriya/NotoSansOriya-Bold.ttf",
		RegFile:    "NotoSansOriya-Regular.ttf",
		BoldFile:   "NotoSansOriya-Bold.ttf",
	},
	"Noto Sans Tirhuta": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosanstirhuta/NotoSansTirhuta-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosanstirhuta/NotoSansTirhuta-Regular.ttf",
		RegFile:    "NotoSansTirhuta-Regular.ttf",
		BoldFile:   "NotoSansTirhuta-Regular.ttf",
	},
	"Noto Sans Kannada": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosanskannada/NotoSansKannada-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosanskannada/NotoSansKannada-Bold.ttf",
		RegFile:    "NotoSansKannada-Regular.ttf",
		BoldFile:   "NotoSansKannada-Bold.ttf",
	},
	"Noto Sans Gurmukhi": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosansgurmukhi/NotoSansGurmukhi-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosansgurmukhi/NotoSansGurmukhi-Bold.ttf",
		RegFile:    "NotoSansGurmukhi-Regular.ttf",
		BoldFile:   "NotoSansGurmukhi-Bold.ttf",
	},
	"Noto Sans Gujarati": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosansgujarati/NotoSansGujarati-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosansgujarati/NotoSansGujarati-Bold.ttf",
		RegFile:    "NotoSansGujarati-Regular.ttf",
		BoldFile:   "NotoSansGujarati-Bold.ttf",
	},
	"Noto Sans Devanagari": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosansdevanagari/NotoSansDevanagari-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosansdevanagari/NotoSansDevanagari-Bold.ttf",
		RegFile:    "NotoSansDevanagari-Regular.ttf",
		BoldFile:   "NotoSansDevanagari-Bold.ttf",
	},
	"Noto Sans Tamil": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosanstamil/NotoSansTamil-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosanstamil/NotoSansTamil-Bold.ttf",
		RegFile:    "NotoSansTamil-Regular.ttf",
		BoldFile:   "NotoSansTamil-Bold.ttf",
	},
	"Noto Sans Malayalam": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosansmalayalam/NotoSansMalayalam-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosansmalayalam/NotoSansMalayalam-Bold.ttf",
		RegFile:    "NotoSansMalayalam-Regular.ttf",
		BoldFile:   "NotoSansMalayalam-Bold.ttf",
	},
	"Noto Sans Sinhala": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosanssinhala/NotoSansSinhala-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosanssinhala/NotoSansSinhala-Bold.ttf",
		RegFile:    "NotoSansSinhala-Regular.ttf",
		BoldFile:   "NotoSansSinhala-Bold.ttf",
	},
	"Noto Sans Hebrew": {
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/notosanshebrew/NotoSansHebrew-Regular.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/notosanshebrew/NotoSansHebrew-Bold.ttf",
		RegFile:    "NotoSansHebrew-Regular.ttf",
		BoldFile:   "NotoSansHebrew-Bold.ttf",
	},
}

// EnsureFontsDownloaded checks for selected fonts locally and downloads them if missing
func EnsureFontsDownloaded(fontFamily string) (string, string, error) {
	// Default to Poppins if empty or not in map
	fCfg, exists := FontMap[fontFamily]
	if !exists {
		log.Printf("Warning: Font family '%s' not recognized. Defaulting to 'Poppins'.", fontFamily)
		fCfg = FontMap["Poppins"]
	}

	regularPath := filepath.Join(fontDir, fCfg.RegFile)
	boldPath := filepath.Join(fontDir, fCfg.BoldFile)

	// Ensure target directory exists
	if err := os.MkdirAll(fontDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create font directory %s: %w", fontDir, err)
	}

	// Check/download Regular Font
	if _, err := os.Stat(regularPath); os.IsNotExist(err) {
		log.Printf("%s not found. Downloading from %s...", fCfg.RegFile, fCfg.RegularURL)
		if err := downloadFile(fCfg.RegularURL, regularPath); err != nil {
			return "", "", fmt.Errorf("failed to download regular font %s: %w", fCfg.RegFile, err)
		}
		log.Printf("%s downloaded successfully", fCfg.RegFile)
	}

	// Check/download Bold Font
	if _, err := os.Stat(boldPath); os.IsNotExist(err) {
		log.Printf("%s not found. Downloading from %s...", fCfg.BoldFile, fCfg.BoldURL)
		if err := downloadFile(fCfg.BoldURL, boldPath); err != nil {
			return "", "", fmt.Errorf("failed to download bold font %s: %w", fCfg.BoldFile, err)
		}
		log.Printf("%s downloaded successfully", fCfg.BoldFile)
	}

	return regularPath, boldPath, nil
}

func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received bad HTTP status code: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
