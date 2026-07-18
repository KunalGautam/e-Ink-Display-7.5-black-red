package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"epaper-display/go-server/internal/config"
	"epaper-display/go-server/internal/mqtt"
	"epaper-display/go-server/internal/weather"
)

type Renderer struct {
	cfg         *config.Config
	regularFont string
	boldFont    string
}

type renderPayload struct {
	Width         int                  `json:"width"`
	Height        int                  `json:"height"`
	RegularFont   string               `json:"regular_font"`
	BoldFont      string               `json:"bold_font"`
	OutputPath    string               `json:"output_path"`
	LastUpdated   string               `json:"last_updated"`
	LayoutStyle   string               `json:"layout_style"`
	ShowCalendar  bool                 `json:"show_calendar"`
	ShowSchedule  bool                 `json:"show_schedule"`
	ShowInbox     bool                 `json:"show_inbox"`
	ShowNotes     bool                 `json:"show_notes"`
	ShowWeather   bool                 `json:"show_weather"`
	ShowSensors   bool                 `json:"show_sensors"`
	Notes         []string             `json:"notes"`
	Emails        []mqtt.Email         `json:"emails"`
	Calendar      []mqtt.CalendarEvent `json:"calendar"`
	Weather       *weather.WeatherData `json:"weather,omitempty"`
}

func NewRenderer(cfg *config.Config, regularFont, boldFont string) *Renderer {
	return &Renderer{
		cfg:         cfg,
		regularFont: regularFont,
		boldFont:    boldFont,
	}
}

// Render creates the layout image by executing the Python rendering helper
func (r *Renderer) Render(notes []string, emails []mqtt.Email, calendar []mqtt.CalendarEvent, weatherData *weather.WeatherData, lastUpdated string) (image.Image, error) {
	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("eink_layout_%d.png", os.Getpid()))

	payload := renderPayload{
		Width:         r.cfg.Width,
		Height:        r.cfg.Height,
		RegularFont:   r.regularFont,
		BoldFont:      r.boldFont,
		OutputPath:    tempPath,
		LastUpdated:   lastUpdated,
		LayoutStyle:   r.cfg.LayoutStyle,
		ShowCalendar:  r.cfg.ShowCalendar,
		ShowSchedule:  r.cfg.ShowSchedule,
		ShowInbox:     r.cfg.ShowInbox,
		ShowNotes:     r.cfg.ShowNotes,
		ShowWeather:   r.cfg.ShowWeather,
		ShowSensors:   r.cfg.ShowSensors,
		Notes:         notes,
		Emails:        emails,
		Calendar:      calendar,
		Weather:       weatherData,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal renderer payload: %w", err)
	}

	// Execute Python layout generator
	scriptPath := "./renderer.py"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		if _, err := os.Stat("../renderer.py"); err == nil {
			scriptPath = "../renderer.py"
		} else if _, err := os.Stat("../../renderer.py"); err == nil {
			scriptPath = "../../renderer.py"
		}
	}

	pythonExec := "python3"
	if _, err := os.Stat("./venv/bin/python"); err == nil {
		pythonExec = "./venv/bin/python"
	} else if _, err := os.Stat("../venv/bin/python"); err == nil {
		pythonExec = "../venv/bin/python"
	} else if _, err := os.Stat("../../venv/bin/python"); err == nil {
		pythonExec = "../../venv/bin/python"
	}

	cmd := exec.Command(pythonExec, scriptPath)
	cmd.Stdin = bytes.NewReader(jsonBytes)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Printf("Executing Python renderer.py to generate layout at: %s", tempPath)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("python renderer execution failed: %w (stderr: %s)", err, stderr.String())
	}

	// Read and decode the generated image
	f, err := os.Open(tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open generated image: %w", err)
	}
	defer func() {
		f.Close()
		// Clean up temporary image file
		if err := os.Remove(tempPath); err != nil {
			log.Printf("Warning: failed to remove temporary layout image %s: %v", tempPath, err)
		}
	}()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode generated PNG: %w", err)
	}

	return img, nil
}

// PackBuffers packs the 3-color RGBA image into Waveshare compatible black/white and red/white 1-bit byte streams.
// Width*Height must be divisible by 8.
// In e-ink representation:
// - 0 represents active pixel (black/red)
// - 1 represents inactive pixel (white background)
func PackBuffers(img image.Image) ([]byte, []byte) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	stride := width / 8

	blackBuf := make([]byte, stride*height)
	redBuf := make([]byte, stride*height)

	// Initialize all pixels to 1 (white)
	for i := range blackBuf {
		blackBuf[i] = 0xFF
		redBuf[i] = 0xFF
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()

			// Scale values to 0-255
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)

			byteIdx := y*stride + (x / 8)
			bitIdx := uint(7 - (x % 8)) // MSB first

			// Classify colors based on red-heavy or black-heavy properties
			isRed := r8 > 150 && g8 < 100 && b8 < 100
			isBlack := r8 < 100 && g8 < 100 && b8 < 100

			if isBlack {
				// Clear bit in black buffer (0 = black)
				blackBuf[byteIdx] &= ^(1 << bitIdx)
			} else if isRed {
				// Clear bit in red buffer (0 = red)
				redBuf[byteIdx] &= ^(1 << bitIdx)
			}
		}
	}

	return blackBuf, redBuf
}

// Color conversion helper for display previewing
func GetDisplayColor(c color.Color) color.RGBA {
	r, g, b, _ := c.RGBA()
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)

	if r8 > 150 && g8 < 100 && b8 < 100 {
		return color.RGBA{255, 0, 0, 255} // Red
	} else if r8 < 100 && g8 < 100 && b8 < 100 {
		return color.RGBA{0, 0, 0, 255} // Black
	}
	return color.RGBA{255, 255, 255, 255} // White
}
