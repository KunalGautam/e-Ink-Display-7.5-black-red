package widget

import (
	"context"
	"encoding/json"
)

type TextWidget struct{}

type TextConfig struct {
	Text string `json:"text"`
}

func (w *TextWidget) Render(ctx context.Context, rCtx *RenderContext) error {
	displayText := ""
	
	// Default to custom config static text
	if rCtx.CustomConfig != "" {
		var cfg TextConfig
		if err := json.Unmarshal([]byte(rCtx.CustomConfig), &cfg); err == nil {
			displayText = cfg.Text
		}
	}

	// Override with dynamic MQTT payload if present
	if rCtx.LatestData != "" {
		displayText = rCtx.LatestData
	}

	if displayText == "" {
		return nil
	}

	// Set text drawing parameters
	rCtx.Ctx.SetHexColor(rCtx.ColorFG)
	
	lines := WrapText(rCtx.Ctx, displayText, float64(rCtx.Width))
	
	// Draw line by line
	lineHeight := rCtx.LineHeight
	if lineHeight <= 0 {
		lineHeight = 18.0
	}
	currentY := lineHeight - 3.0
	for _, line := range lines {
		if currentY > float64(rCtx.Height) {
			break
		}
		rCtx.Ctx.DrawString(line, 5, currentY)
		currentY += lineHeight
	}
	
	return nil
}
