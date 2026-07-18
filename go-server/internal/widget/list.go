package widget

import (
	"context"
	"encoding/json"
	"strings"
)

type ListWidget struct {
	IsEmailList bool
}

type EmailItem struct {
	Sender  string `json:"sender"`
	Subject string `json:"subject"`
}

func (w *ListWidget) Render(ctx context.Context, rCtx *RenderContext) error {
	rCtx.Ctx.SetHexColor(rCtx.ColorFG)

	if rCtx.LatestData == "" {
		rCtx.Ctx.DrawString("No data received.", 5, 20)
		return nil
	}

	if w.IsEmailList {
		var emails []EmailItem
		if err := json.Unmarshal([]byte(rCtx.LatestData), &emails); err != nil {
			lines := strings.Split(rCtx.LatestData, "\n")
			lineHeight := rCtx.LineHeight
			if lineHeight <= 0 {
				lineHeight = 18.0
			}
			currentY := lineHeight - 3.0
			for _, line := range lines {
				if currentY > float64(rCtx.Height)-15 {
					break
				}
				rCtx.Ctx.DrawString(line, 5, currentY)
				currentY += lineHeight
			}
			return nil
		}

		lineHeight := rCtx.LineHeight
		if lineHeight <= 0 {
			lineHeight = 18.0
		}
		currentY := lineHeight
		for _, email := range emails {
			if currentY > float64(rCtx.Height)-(lineHeight+10) {
				break
			}
			// Draw Sender in bold/accent (mock by shifting or just standard layout color)
			rCtx.Ctx.DrawString(email.Sender, 5, currentY)
			
			// Draw Subject
			rCtx.Ctx.DrawString(email.Subject, 5, currentY+lineHeight-3.0)
			currentY += lineHeight*2.0 + 2.0
		}
	} else {
		var notes []string
		if err := json.Unmarshal([]byte(rCtx.LatestData), &notes); err != nil {
			// Fallback: parse plain string list separated by newlines
			notes = strings.Split(rCtx.LatestData, "\n")
		}

		lineHeight := rCtx.LineHeight
		if lineHeight <= 0 {
			lineHeight = 18.0
		}
		currentY := lineHeight
		bulletRadius := 2.5
		for _, note := range notes {
			if currentY > float64(rCtx.Height)-15 {
				break
			}
			// Draw bullet point
			dc := rCtx.Ctx
			dc.DrawCircle(8, currentY-4, bulletRadius)
			dc.Fill()

			// Draw note text (wrapped)
			lines := WrapText(rCtx.Ctx, note, float64(rCtx.Width)-20)
			for _, line := range lines {
				if currentY > float64(rCtx.Height)-15 {
					break
				}
				dc.DrawString(line, 18, currentY)
				currentY += lineHeight
			}
			currentY += 6 // list item margin
		}
	}

	return nil
}
