package widget

import (
	"context"
	"encoding/json"
	"time"
)

type DateTimeWidget struct{}

type DateTimeConfig struct {
	Format string `json:"format"` // e.g. "15:04", "03:04 PM", "2006-01-02", "Monday, Jan _2"
}

func (w *DateTimeWidget) Render(ctx context.Context, rCtx *RenderContext) error {
	loc, err := time.LoadLocation(rCtx.Timezone)
	if err != nil {
		loc = time.Local
	}

	now := time.Now().In(loc)

	// Read format parameter from CustomConfig
	format := "03:04 PM"
	if rCtx.CustomConfig != "" {
		var cfg DateTimeConfig
		if err := json.Unmarshal([]byte(rCtx.CustomConfig), &cfg); err == nil && cfg.Format != "" {
			format = cfg.Format
		}
	}

	displayStr := now.Format(format)

	rCtx.Ctx.SetHexColor(rCtx.ColorFG)
	
	// Draw string centered in widget block bounds
	rCtx.Ctx.DrawStringAnchored(displayStr, float64(rCtx.Width)/2, float64(rCtx.Height)/2, 0.5, 0.5)
	
	return nil
}
