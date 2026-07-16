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
		RegularURL: "https://github.com/google/fonts/raw/main/ofl/poppins/Poppins-Medium.ttf",
		BoldURL:    "https://github.com/google/fonts/raw/main/ofl/poppins/Poppins-Bold.ttf",
		RegFile:    "Poppins-Medium.ttf",
		BoldFile:   "Poppins-Bold.ttf",
	},
	"Roboto": {
		RegularURL: "https://raw.githubusercontent.com/googlefonts/roboto-2/main/src/hinted/Roboto-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/googlefonts/roboto-2/main/src/hinted/Roboto-Bold.ttf",
		RegFile:    "Roboto-Medium.ttf",
		BoldFile:   "Roboto-Bold.ttf",
	},
	"Noto Sans Bengali": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansBengali/NotoSansBengali-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansBengali/NotoSansBengali-Bold.ttf",
		RegFile:    "NotoSansBengali-Medium.ttf",
		BoldFile:   "NotoSansBengali-Bold.ttf",
	},
	"Noto Sans Arabic": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansArabic/NotoSansArabic-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansArabic/NotoSansArabic-Bold.ttf",
		RegFile:    "NotoSansArabic-Medium.ttf",
		BoldFile:   "NotoSansArabic-Bold.ttf",
	},
	"Noto Sans Oriya": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansOriya/NotoSansOriya-Regular.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansOriya/NotoSansOriya-Bold.ttf",
		RegFile:    "NotoSansOriya-Regular.ttf",
		BoldFile:   "NotoSansOriya-Bold.ttf",
	},
	"Noto Sans Tirhuta": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansTirhuta/NotoSansTirhuta-Regular.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansTirhuta/NotoSansTirhuta-Regular.ttf",
		RegFile:    "NotoSansTirhuta-Regular.ttf",
		BoldFile:   "NotoSansTirhuta-Regular.ttf",
	},
	"Noto Sans Kannada": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansKannada/NotoSansKannada-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansKannada/NotoSansKannada-Bold.ttf",
		RegFile:    "NotoSansKannada-Medium.ttf",
		BoldFile:   "NotoSansKannada-Bold.ttf",
	},
	"Noto Sans Gurmukhi": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansGurmukhi/NotoSansGurmukhi-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansGurmukhi/NotoSansGurmukhi-Bold.ttf",
		RegFile:    "NotoSansGurmukhi-Medium.ttf",
		BoldFile:   "NotoSansGurmukhi-Bold.ttf",
	},
	"Noto Sans Gujarati": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansGujarati/NotoSansGujarati-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansGujarati/NotoSansGujarati-Bold.ttf",
		RegFile:    "NotoSansGujarati-Medium.ttf",
		BoldFile:   "NotoSansGujarati-Bold.ttf",
	},
	"Noto Sans Devanagari": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansDevanagari/NotoSansDevanagari-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansDevanagari/NotoSansDevanagari-Bold.ttf",
		RegFile:    "NotoSansDevanagari-Medium.ttf",
		BoldFile:   "NotoSansDevanagari-Bold.ttf",
	},
	"Noto Sans Tamil": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansTamil/NotoSansTamil-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansTamil/NotoSansTamil-Bold.ttf",
		RegFile:    "NotoSansTamil-Medium.ttf",
		BoldFile:   "NotoSansTamil-Bold.ttf",
	},
	"Noto Sans Malayalam": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansMalayalam/NotoSansMalayalam-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansMalayalam/NotoSansMalayalam-Bold.ttf",
		RegFile:    "NotoSansMalayalam-Medium.ttf",
		BoldFile:   "NotoSansMalayalam-Bold.ttf",
	},
	"Noto Sans Sinhala": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansSinhala/NotoSansSinhala-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansSinhala/NotoSansSinhala-Bold.ttf",
		RegFile:    "NotoSansSinhala-Medium.ttf",
		BoldFile:   "NotoSansSinhala-Bold.ttf",
	},
	"Noto Sans Hebrew": {
		RegularURL: "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansHebrew/NotoSansHebrew-Medium.ttf",
		BoldURL:    "https://raw.githubusercontent.com/notofonts/noto-fonts/main/hinted/ttf/NotoSansHebrew/NotoSansHebrew-Bold.ttf",
		RegFile:    "NotoSansHebrew-Medium.ttf",
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
