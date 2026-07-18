package widget

import (
	"context"
	"encoding/json"
	"fmt"
)

type WeatherWidget struct{}

type WeatherPayload struct {
	Temp      float64 `json:"temp"`
	Condition string  `json:"condition"`
	Humidity  int     `json:"humidity"`
	Pressure  float64 `json:"pressure"`
	WindSpeed float64 `json:"wind_speed"`
}

func (w *WeatherWidget) Render(ctx context.Context, rCtx *RenderContext) error {
	rCtx.Ctx.SetHexColor(rCtx.ColorFG)

	if rCtx.LatestData == "" {
		rCtx.Ctx.DrawString("No weather data.", 5, 20)
		return nil
	}

	var data WeatherPayload
	if err := json.Unmarshal([]byte(rCtx.LatestData), &data); err != nil {
		rCtx.Ctx.DrawString("JSON error.", 5, 20)
		return nil
	}

	// 1. Draw weather icon on the left (e.g. x: 5, size: 64)
	iconSize := 60.0
	if float64(rCtx.Height) < iconSize {
		iconSize = float64(rCtx.Height) - 10
	}
	DrawWeatherIcon(rCtx.Ctx, 5, 5, iconSize, data.Condition, rCtx.ColorFG)

	// 2. Draw temperature and conditions on the right
	textX := iconSize + 15
	rCtx.Ctx.DrawString(fmt.Sprintf("%.1f°C", data.Temp), textX, 28)
	rCtx.Ctx.DrawString(data.Condition, textX, 48)

	// 3. Draw extra stats if widget height allows
	if rCtx.Height > 70 {
		statsStr := fmt.Sprintf("Hum: %d%%  Pres: %.0f hPa  Wind: %.1f m/s", data.Humidity, data.Pressure, data.WindSpeed)
		rCtx.Ctx.DrawString(statsStr, 5, float64(rCtx.Height)-10)
	}

	return nil
}
