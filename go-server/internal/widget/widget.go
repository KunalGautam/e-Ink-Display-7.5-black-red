package widget

import (
	"context"
	"math"
	"strings"

	"github.com/fogleman/gg"
)

type RenderContext struct {
	Ctx          *gg.Context
	Width        int
	Height       int
	ColorMode    string
	Timezone     string
	ColorFG      string
	ColorBG      string
	LatestData   string // MQTT payload if bound
	CustomConfig string // JSON parameters
}

type Widget interface {
	Render(ctx context.Context, rCtx *RenderContext) error
}

// WrapText wraps text into lines using the current font configuration in gg
func WrapText(dc *gg.Context, text string, maxWidth float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	var lines []string
	currentLine := ""

	for _, word := range words {
		testLine := word
		if currentLine != "" {
			testLine = currentLine + " " + word
		}

		w, _ := dc.MeasureString(testLine)
		if w <= maxWidth {
			currentLine = testLine
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}

// DrawCloud draws a vector cloud shape
func DrawCloud(dc *gg.Context, cx, cy, size float64, outlineColor, fillColor string, strokeWidth float64) {
	r1 := size * 0.22
	r2 := size * 0.28
	r3 := size * 0.20

	ox1, oy1 := -size*0.20, size*0.08
	ox2, oy2 := 0.0, -size*0.05
	ox3, oy3 := size*0.20, size*0.08

	sw := strokeWidth

	// Draw outline shadow
	dc.SetHexColor(outlineColor)
	dc.DrawCircle(cx+ox1, cy+oy1, r1+sw)
	dc.Fill()
	dc.DrawCircle(cx+ox3, cy+oy3, r3+sw)
	dc.Fill()
	dc.DrawCircle(cx+ox2, cy+oy2, r2+sw)
	dc.Fill()
	dc.DrawRectangle(cx+ox1, cy+oy1-sw, (ox3-ox1), r3+sw*2)
	dc.Fill()

	// Draw inner fill
	dc.SetHexColor(fillColor)
	dc.DrawCircle(cx+ox1, cy+oy1, r1)
	dc.Fill()
	dc.DrawCircle(cx+ox3, cy+oy3, r3)
	dc.Fill()
	dc.DrawCircle(cx+ox2, cy+oy2, r2)
	dc.Fill()
	dc.DrawRectangle(cx+ox1, cy+oy1, (ox3 - ox1), r3)
	dc.Fill()

	// Draw solid bottom outline line
	dc.SetHexColor(outlineColor)
	dc.SetLineWidth(sw)
	dc.DrawLine(cx+ox1, cy+oy1+r1, cx+ox3, cy+oy3+r3)
	dc.Stroke()
}

// DrawWeatherIcon draws clean e-paper weather icons based on weather conditions
func DrawWeatherIcon(dc *gg.Context, x, y, size float64, condition string, fgColor string) {
	cond := strings.ToLower(condition)
	cx := x + size/2
	cy := y + size/2

	// Constrain color mapping
	outlineColor := "#000000" // Black
	accentColor := "#FF0000"  // Red (if supported, falls back in canvas packing)
	if fgColor != "" {
		outlineColor = fgColor
	}

	if strings.Contains(cond, "sun") || strings.Contains(cond, "clear") {
		// Sunny
		r := size / 4
		dc.SetHexColor(accentColor)
		dc.SetLineWidth(3)
		dc.DrawCircle(cx, cy, r)
		dc.Stroke()

		rayLen := size / 5
		for i := 0; i < 8; i++ {
			angle := float64(i) * (math.Pi / 4.0)
			x1 := cx + (r+3)*math.Cos(angle)
			y1 := cy + (r+3)*math.Sin(angle)
			x2 := cx + (r+3+rayLen)*math.Cos(angle)
			y2 := cy + (r+3+rayLen)*math.Sin(angle)
			dc.DrawLine(x1, y1, x2, y2)
			dc.Stroke()
		}

	} else if strings.Contains(cond, "cloud") || strings.Contains(cond, "overcast") || strings.Contains(cond, "mist") || strings.Contains(cond, "fog") {
		// Cloudy
		DrawCloud(dc, cx, cy, size, outlineColor, "#FFFFFF", 3)

	} else if strings.Contains(cond, "rain") || strings.Contains(cond, "shower") || strings.Contains(cond, "drizzle") {
		// Rainy
		cyShift := cy - size*0.08
		DrawCloud(dc, cx, cyShift, size*0.9, outlineColor, "#FFFFFF", 3)

		// Raindrops
		dropY := cyShift + size*0.20
		dc.SetHexColor(accentColor)
		dc.SetLineWidth(3)
		for _, dx := range []float64{-size * 0.15, 0, size * 0.15} {
			dc.DrawLine(cx+dx-2, dropY, cx+dx+1, dropY+8)
			dc.Stroke()
		}

	} else if strings.Contains(cond, "thunder") || strings.Contains(cond, "storm") {
		// Thunderstorm
		cyShift := cy - size*0.08
		DrawCloud(dc, cx, cyShift, size*0.9, outlineColor, "#FFFFFF", 3)

		// Lightning bolt
		dc.SetHexColor(accentColor)
		dc.SetLineWidth(3)
		dc.MoveTo(cx+2, cyShift+size*0.15)
		dc.LineTo(cx-8, cyShift+size*0.28)
		dc.LineTo(cx, cyShift+size*0.28)
		dc.LineTo(cx-4, cyShift+size*0.40)
		dc.Stroke()

	} else if strings.Contains(cond, "snow") || strings.Contains(cond, "ice") || strings.Contains(cond, "freeze") {
		// Snowy
		cyShift := cy - size*0.08
		DrawCloud(dc, cx, cyShift, size*0.9, outlineColor, "#FFFFFF", 3)

		// Snowflakes (crosses)
		dropY := cyShift + size*0.20
		dc.SetHexColor(accentColor)
		dc.SetLineWidth(2)
		for _, dx := range []float64{-size * 0.15, 0, size * 0.15} {
			fx, fy := cx+dx, dropY
			dc.DrawLine(fx-3, fy, fx+3, fy)
			dc.Stroke()
			dc.DrawLine(fx, fy-3, fx, fy+3)
			dc.Stroke()
		}

	} else {
		// Partly Cloudy (Default Fallback)
		cxSun := cx + size*0.16
		cySun := cy - size*0.16
		r := size * 0.18

		dc.SetHexColor(accentColor)
		dc.SetLineWidth(2)
		dc.DrawCircle(cxSun, cySun, r)
		dc.Stroke()

		rayLen := size / 8
		for i := 0; i < 8; i++ {
			angle := float64(i) * (math.Pi / 4.0)
			x1 := cxSun + (r+2)*math.Cos(angle)
			y1 := cySun + (r+2)*math.Sin(angle)
			x2 := cxSun + (r+2+rayLen)*math.Cos(angle)
			y2 := cySun + (r+2+rayLen)*math.Sin(angle)
			dc.DrawLine(x1, y1, x2, y2)
			dc.Stroke()
		}

		// Cloud in foreground
		DrawCloud(dc, cx-size*0.08, cy+size*0.08, size*0.85, outlineColor, "#FFFFFF", 3)
	}
}
