package canvas

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"log"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"

	"epaper-display/go-server/internal/db"
	"epaper-display/go-server/internal/fonts"
	"epaper-display/go-server/internal/mqtt"
	"epaper-display/go-server/internal/widget"
)

type Renderer struct {
	db        *db.DB
	mqttReg   *mqtt.Registry
	fontCache *fonts.Cache
}

func NewRenderer(database *db.DB, mqttReg *mqtt.Registry, fontCache *fonts.Cache) *Renderer {
	return &Renderer{
		db:        database,
		mqttReg:   mqttReg,
		fontCache: fontCache,
	}
}

// RenderCanvas draws the canvas widgets and returns the composite image
func (r *Renderer) RenderCanvas(ctx context.Context, canvasID string) (image.Image, error) {
	cRec, err := r.db.GetCanvas(canvasID)
	if err != nil {
		return nil, err
	}
	if cRec == nil {
		return nil, nil // Canvas not found
	}

	wRecs, err := r.db.GetWidgetsForCanvas(canvasID)
	if err != nil {
		return nil, err
	}

	// Create main canvas drawing context
	dc := gg.NewContext(cRec.Width, cRec.Height)
	dc.SetColor(color.White)
	dc.Clear()

	// Loop and render each widget inside an isolated sub-context
	for _, w := range wRecs {
		// Initialize the widget renderer instance based on type
		var wRenderer widget.Widget
		switch w.Type {
		case "calendar":
			wRenderer = &widget.CalendarWidget{}
		case "datetime":
			wRenderer = &widget.DateTimeWidget{}
		case "text":
			wRenderer = &widget.TextWidget{}
		case "notes":
			wRenderer = &widget.ListWidget{IsEmailList: false}
		case "emails":
			wRenderer = &widget.ListWidget{IsEmailList: true}
		case "weather":
			wRenderer = &widget.WeatherWidget{}
		case "image":
			wRenderer = &widget.ImageWidget{}
		case "container":
			wRenderer = &widget.ContainerWidget{}
		default:
			log.Printf("Warning: Unknown widget type: %s", w.Type)
			continue
		}

		// Create sub-context for widget boundaries
		subDc := gg.NewContext(w.Width, w.Height)
		subDc.SetColor(color.Transparent)
		subDc.Clear()

		// Configure font if URL is provided
		var face font.Face
		if w.FontURL != "" {
			fontFace, err := r.fontCache.LoadFont(w.FontURL)
			if err == nil && fontFace != nil {
				fontSize := w.FontSize
				if fontSize == 0 {
					fontSize = 14
				}
				face = truetype.NewFace(fontFace, &truetype.Options{
					Size: fontSize,
				})
				subDc.SetFontFace(face)
			} else {
				log.Printf("Warning: Failed to load font face from %s: %v", w.FontURL, err)
			}
		}

		// Resolve MQTT broker details with fallback
		broker := w.MQTTBroker
		username := ""
		password := ""
		if broker == "" && cRec.MQTTBroker != "" {
			broker = cRec.MQTTBroker
			username = cRec.MQTTUsername
			password = cRec.MQTTPassword
		}

		// Fetch latest MQTT cached payload if bound
		latestData := ""
		if w.MQTTTopic != "" {
			latestData = r.mqttReg.GetPayload(broker, w.MQTTTopic)
			// Trigger dynamic subscription connection in background
			go func(b, t, u, p string) {
				_ = r.mqttReg.Subscribe(b, t, u, p)
			}(broker, w.MQTTTopic, username, password)
		}

		if w.Type == "container" && w.CustomConfig != "" {
			var containerCfg struct {
				Children []struct {
					MQTTTopic string `json:"mqtt_topic"`
				} `json:"children"`
			}
			if err := json.Unmarshal([]byte(w.CustomConfig), &containerCfg); err == nil {
				for _, child := range containerCfg.Children {
					if child.MQTTTopic != "" {
						go func(b, t, u, p string) {
							_ = r.mqttReg.Subscribe(b, t, u, p)
						}(broker, child.MQTTTopic, username, password)
					}
				}
			}
		}

		// Prepare render context for widget
		rCtx := &widget.RenderContext{
			Ctx:          subDc,
			Width:        w.Width,
			Height:       w.Height,
			ColorMode:    cRec.ColorMode,
			Timezone:     cRec.Timezone,
			LatestData:   latestData,
			CustomConfig: w.CustomConfig,
			FontCache:    r.fontCache,
			FontFace:     face,
			MqttReg:      r.mqttReg,
			MQTTBroker:   broker,
			FontURL:      w.FontURL,
		}

		// Setup defaults colors
		rCtx.ColorFG = w.ColorFG
		if rCtx.ColorFG == "" {
			rCtx.ColorFG = "#000000"
		}
		rCtx.ColorBG = w.ColorBG
		if rCtx.ColorBG == "" {
			rCtx.ColorBG = "#FFFFFF"
		}

		// Draw background if configured
		if w.ColorBG != "" {
			subDc.SetHexColor(w.ColorBG)
			subDc.Clear()
		}

		// Execute widget renderer
		if err := wRenderer.Render(ctx, rCtx); err != nil {
			log.Printf("Error rendering widget %s: %v", w.ID, err)
		}

		// Composite widget onto main canvas context
		dc.DrawImage(subDc.Image(), w.X, w.Y)

		// Draw border if configured
		if w.BorderWidth > 0 {
			borderColor := w.BorderColor
			if borderColor == "" {
				borderColor = w.ColorFG
			}
			if borderColor == "" {
				borderColor = "#000000"
			}
			dc.SetHexColor(borderColor)
			dc.SetLineWidth(float64(w.BorderWidth))
			offset := float64(w.BorderWidth) / 2.0
			dc.DrawRectangle(float64(w.X)+offset, float64(w.Y)+offset, float64(w.Width)-float64(w.BorderWidth), float64(w.Height)-float64(w.BorderWidth))
			dc.Stroke()
		}
	}

	return dc.Image(), nil
}

// PackBuffers packs the composite image into Waveshare raw bitstream payload formats
func PackBuffers(img image.Image, colorMode string) []byte {
	bounds := img.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y

	// Black buffer (always generated)
	blackBuf := make([]byte, (width*height)/8)
	// Red buffer (only generated for BWR displays)
	redBuf := make([]byte, (width*height)/8)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()

			r8 := r >> 8
			g8 := g >> 8
			b8 := b >> 8

			// Identify red vs black vs white
			isRed := r8 > 150 && g8 < 100 && b8 < 100
			isDark := r8 < 120 && g8 < 120 && b8 < 120

			byteIdx := (x + y*width) / 8
			bitShift := uint(7 - (x % 8))

			// Initialize buffers to white/inactive (1) at the start of each byte if not written
			if x%8 == 0 {
				blackBuf[byteIdx] = 0xFF
				redBuf[byteIdx] = 0xFF
			}

			if colorMode == "bwr" {
				if isRed {
					// Red active (0 in red buffer), white (1) in black buffer
					blackBuf[byteIdx] |= (1 << bitShift)
					redBuf[byteIdx] &= ^(1 << bitShift)
				} else if isDark {
					// Black active (0 in black buffer), white (1) in red buffer
					blackBuf[byteIdx] &= ^(1 << bitShift)
					redBuf[byteIdx] |= (1 << bitShift)
				} else {
					// Both white (1 in both buffers)
					blackBuf[byteIdx] |= (1 << bitShift)
					redBuf[byteIdx] |= (1 << bitShift)
				}
			} else {
				// Monochrome / Mono displays
				if isDark {
					// Black active (0)
					blackBuf[byteIdx] &= ^(1 << bitShift)
				} else {
					// White inactive (1)
					blackBuf[byteIdx] |= (1 << bitShift)
				}
			}
		}
	}

	if colorMode == "bwr" {
		return append(blackBuf, redBuf...)
	}
	return blackBuf
}
