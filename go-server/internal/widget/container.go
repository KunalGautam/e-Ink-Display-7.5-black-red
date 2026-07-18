package widget

import (
	"context"
	"encoding/json"
	"image/color"
	"log"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

type ContainerWidget struct{}

type ChildWidgetConfig struct {
	Type         string  `json:"type"`
	ColorFG      string  `json:"color_fg"`
	ColorBG      string  `json:"color_bg"`
	FontURL      string  `json:"font_url"`
	FontSize     float64 `json:"font_size"`
	FontWeight   string  `json:"font_weight"`
	MQTTTopic    string  `json:"mqtt_topic"`
	MQTTBroker   string  `json:"mqtt_broker"`
	CustomConfig string  `json:"custom_config"`
}

type ContainerConfig struct {
	GridCols int                 `json:"grid_cols"`
	GridRows int                 `json:"grid_rows"`
	Gap      int                 `json:"gap"`
	Children []ChildWidgetConfig `json:"children"`
}

func (w *ContainerWidget) Render(ctx context.Context, rCtx *RenderContext) error {
	// Parse configurations
	cols := 1
	rows := 1
	gap := 4
	var children []ChildWidgetConfig

	if rCtx.CustomConfig != "" {
		var cfg ContainerConfig
		if err := json.Unmarshal([]byte(rCtx.CustomConfig), &cfg); err == nil {
			if cfg.GridCols > 0 {
				cols = cfg.GridCols
			}
			if cfg.GridRows > 0 {
				rows = cfg.GridRows
			}
			gap = cfg.Gap
			children = cfg.Children
		}
	}

	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}

	// Calculate cell dimensions
	cellW := (float64(rCtx.Width) - float64(gap*(cols-1))) / float64(cols)
	cellH := (float64(rCtx.Height) - float64(gap*(rows-1))) / float64(rows)

	if cellW <= 0 || cellH <= 0 {
		return nil
	}

	for idx, child := range children {
		if idx >= cols*rows {
			break // Exceeded grid capacity
		}

		col := idx % cols
		row := idx / cols

		cellX := float64(col) * (cellW + float64(gap))
		cellY := float64(row) * (cellH + float64(gap))

		// Instantiate child widget
		var childRenderer Widget
		switch child.Type {
		case "calendar":
			childRenderer = &CalendarWidget{}
		case "datetime":
			childRenderer = &DateTimeWidget{}
		case "text":
			childRenderer = &TextWidget{}
		case "notes":
			childRenderer = &ListWidget{IsEmailList: false}
		case "emails":
			childRenderer = &ListWidget{IsEmailList: true}
		case "weather":
			childRenderer = &WeatherWidget{}
		case "image":
			childRenderer = &ImageWidget{}
		default:
			log.Printf("Warning: Unknown child widget type inside container: %s", child.Type)
			continue
		}

		// Create sub-sub-context for the child grid cell
		childDc := gg.NewContext(int(cellW), int(cellH))
		childDc.SetColor(color.Transparent)
		childDc.Clear()

		// Configure font for child
		var childFace font.Face
		fontURL := child.FontURL
		if fontURL == "" {
			fontURL = rCtx.FontURL
		}

		if fontURL != "" && rCtx.FontCache != nil {
			fontAsset, err := rCtx.FontCache.LoadFont(fontURL)
			if err == nil && fontAsset != nil {
				fontSize := child.FontSize
				if fontSize == 0 {
					fontSize = 12
				}
				face := truetype.NewFace(fontAsset, &truetype.Options{
					Size: fontSize,
				})
				if face != nil {
					childFace = face
					childDc.SetFontFace(face)
				}
			}
		}

		// Fallback to parent FontFace if child face was not resolved
		if childFace == nil && rCtx.FontFace != nil {
			childFace = rCtx.FontFace
			childDc.SetFontFace(rCtx.FontFace)
		}

		childFG := child.ColorFG
		if childFG == "" {
			childFG = rCtx.ColorFG
		}
		childBG := child.ColorBG
		if childBG == "" {
			childBG = rCtx.ColorBG
		}

		if child.ColorBG != "" {
			childDc.SetHexColor(child.ColorBG)
			childDc.Clear()
		}

		// Resolve child MQTT topic payload
		childLatestData := rCtx.LatestData
		if child.MQTTTopic != "" && rCtx.MqttReg != nil {
			broker := child.MQTTBroker
			if broker == "" {
				broker = rCtx.MQTTBroker
			}
			childLatestData = rCtx.MqttReg.GetPayload(broker, child.MQTTTopic)
		}

		childRCtx := &RenderContext{
			Ctx:          childDc,
			Width:        int(cellW),
			Height:       int(cellH),
			ColorMode:    rCtx.ColorMode,
			Timezone:     rCtx.Timezone,
			ColorFG:      childFG,
			ColorBG:      childBG,
			LatestData:   childLatestData,
			CustomConfig: child.CustomConfig,
			FontCache:    rCtx.FontCache,
			FontFace:     childFace,
			MqttReg:      rCtx.MqttReg,
			MQTTBroker:   rCtx.MQTTBroker,
			FontURL:      fontURL,
		}

		if err := childRenderer.Render(ctx, childRCtx); err != nil {
			log.Printf("Error rendering child widget %s in container: %v", child.Type, err)
		}

		// Draw child grid cell onto parent context
		rCtx.Ctx.DrawImage(childDc.Image(), int(cellX), int(cellY))
	}

	return nil
}
