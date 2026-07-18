package widget

import (
	"context"
	"strconv"
	"time"
)

type CalendarWidget struct{}

func (w *CalendarWidget) Render(ctx context.Context, rCtx *RenderContext) error {
	loc, err := time.LoadLocation(rCtx.Timezone)
	if err != nil {
		loc = time.Local
	}

	now := time.Now().In(loc)
	year, month, today := now.Date()

	// Title
	rCtx.Ctx.SetHexColor(rCtx.ColorFG)
	titleStr := now.Format("January 2006")
	rCtx.Ctx.DrawStringAnchored(titleStr, float64(rCtx.Width)/2, 20, 0.5, 0.5)

	// Weekdays
	weekdays := []string{"S", "M", "T", "W", "T", "F", "S"}
	cellWidth := float64(rCtx.Width) / 7
	calendarTop := 40.0
	
	// Draw weekday labels in red if BWR supported, otherwise foreground
	accentColor := "#FF0000"
	if rCtx.ColorMode == "mono" {
		accentColor = rCtx.ColorFG
	}
	rCtx.Ctx.SetHexColor(accentColor)
	for i, wd := range weekdays {
		cx := float64(i)*cellWidth + cellWidth/2
		rCtx.Ctx.DrawStringAnchored(wd, cx, calendarTop, 0.5, 0.5)
	}

	// Calculate monthly grid
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	startWeekday := int(first.Weekday()) // 0: Sunday, 1: Monday...

	nextMonth := first.AddDate(0, 1, 0)
	lastDay := nextMonth.AddDate(0, 0, -1)
	numDays := lastDay.Day()

	rowHeight := 24.0
	if float64(rCtx.Height) > 180 {
		rowHeight = 28.0
	}

	for d := 1; d <= numDays; d++ {
		cellIdx := d - 1 + startWeekday
		col := cellIdx % 7
		row := cellIdx / 7

		cx := float64(col)*cellWidth + cellWidth/2
		cy := calendarTop + 20 + float64(row)*rowHeight

		if cy > float64(rCtx.Height)-10 {
			break
		}

		if d == today {
			// Highlight today
			rCtx.Ctx.SetHexColor(accentColor)
			rCtx.Ctx.DrawCircle(cx, cy, 12)
			rCtx.Ctx.Fill()

			rCtx.Ctx.SetHexColor("#FFFFFF")
			rCtx.Ctx.DrawStringAnchored(strconv.Itoa(d), cx, cy, 0.5, 0.5)
		} else {
			rCtx.Ctx.SetHexColor(rCtx.ColorFG)
			rCtx.Ctx.DrawStringAnchored(strconv.Itoa(d), cx, cy, 0.5, 0.5)
		}
	}

	return nil
}
