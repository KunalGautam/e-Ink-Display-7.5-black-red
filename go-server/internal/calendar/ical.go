package calendar

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"epaper-display/go-server/internal/config"
	"epaper-display/go-server/internal/mqtt"
)

type ICalClient struct {
	url         string
	timezone    *time.Location
	mu          sync.RWMutex
	events      []mqtt.CalendarEvent
	lastUpdated time.Time
}

func NewICalClient(cfg *config.Config) *ICalClient {
	tz := time.Local
	if cfg.Timezone != "" && cfg.Timezone != "Local" {
		if loc, err := time.LoadLocation(cfg.Timezone); err == nil {
			tz = loc
		} else {
			log.Printf("Warning: Failed to load timezone %s: %v. Defaulting to local.", cfg.Timezone, err)
		}
	}

	return &ICalClient{
		url:         cfg.ICalURL,
		timezone:    tz,
		events:      []mqtt.CalendarEvent{},
		lastUpdated: time.Now(),
	}
}

func (c *ICalClient) Sync() error {
	if c.url == "" {
		return nil // No iCal URL configured, skip sync
	}

	log.Printf("Fetching iCal feed from Google Calendar: %s", c.url)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(c.url)
	if err != nil {
		return fmt.Errorf("failed to fetch iCal feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("iCal feed returned HTTP status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read iCal body: %w", err)
	}

	events, err := c.parseICS(string(data))
	if err != nil {
		return fmt.Errorf("failed to parse ICS: %w", err)
	}

	c.mu.Lock()
	c.events = events
	c.lastUpdated = time.Now()
	c.mu.Unlock()

	log.Printf("iCal sync complete. Cached %d events for today.", len(events))
	return nil
}

func (c *ICalClient) GetEvents() []mqtt.CalendarEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	eventsCopy := make([]mqtt.CalendarEvent, len(c.events))
	copy(eventsCopy, c.events)
	return eventsCopy
}

func (c *ICalClient) GetTimezone() *time.Location {
	return c.timezone
}

func (c *ICalClient) GetLastUpdated() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdated
}

// parseICS parses the ICS file text and returns events happening today
func (c *ICalClient) parseICS(text string) ([]mqtt.CalendarEvent, error) {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")

	// Unfold lines (ICS folds lines by starting the next line with a space or tab)
	lines := strings.Split(text, "\n")
	var unfolded []string
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			if len(unfolded) > 0 {
				unfolded[len(unfolded)-1] += line[1:]
			}
		} else {
			unfolded = append(unfolded, line)
		}
	}

	now := time.Now().In(c.timezone)
	// Boundaries for 7 days
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, c.timezone)
	sevenDaysEnd := todayStart.Add(7 * 24 * time.Hour)

	var todayEvents []struct {
		event mqtt.CalendarEvent
		start time.Time
	}

	var inEvent bool
	var eventData map[string]string

	for _, line := range unfolded {
		if line == "BEGIN:VEVENT" {
			inEvent = true
			eventData = make(map[string]string)
			continue
		}

		if line == "END:VEVENT" {
			inEvent = false
			summary := eventData["SUMMARY"]
			dtstart := eventData["DTSTART"]
			dtend := eventData["DTEND"]

			if summary == "" || dtstart == "" {
				continue
			}

			start, startParsed := c.parseTime(dtstart)
			end, endParsed := c.parseTime(dtend)

			if !startParsed {
				continue
			}

			// If DTEND is missing, default to start time + 1 hour (or same day)
			if !endParsed {
				end = start.Add(1 * time.Hour)
			}

			// Check if event overlaps with the next 7 days:
			// Event overlaps if start < sevenDaysEnd && end > todayStart
			if start.Before(sevenDaysEnd) && end.After(todayStart) {
				// Format display time
				timeStr := ""
				// Check if it's an all-day event
				// In ICS, all day events start dates are represented as 8 chars (YYYYMMDD) without 'T'
				isAllDay := !strings.Contains(dtstart, "T")
				
				// Compute date prefix timezone-sensitively
				eventDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, c.timezone)
				prefix := ""
				if eventDay.Equal(todayStart) {
					prefix = "Today, "
				} else if eventDay.Equal(todayStart.Add(24 * time.Hour)) {
					prefix = "Tom, "
				} else {
					prefix = start.Format("Mon 02, ")
				}

				if isAllDay {
					timeStr = prefix + "All Day"
				} else {
					timeStr = prefix + fmt.Sprintf("%s - %s", start.Format("15:04"), end.Format("15:04"))
				}

				todayEvents = append(todayEvents, struct {
					event mqtt.CalendarEvent
					start time.Time
				}{
					event: mqtt.CalendarEvent{
						Title: summary,
						Time:  timeStr,
						Start: start,
					},
					start: start,
				})
			}
			continue
		}

		if inEvent {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := parts[0]
				// Strip parameter qualifiers, e.g. DTSTART;TZID=America/New_York
				if idx := strings.Index(key, ";"); idx != -1 {
					key = key[:idx]
				}
				eventData[key] = parts[1]
			}
		}
	}

	// Sort events chronologically
	sort.Slice(todayEvents, func(i, j int) bool {
		return todayEvents[i].start.Before(todayEvents[j].start)
	})

	// Convert to slice of calendar events
	res := make([]mqtt.CalendarEvent, len(todayEvents))
	for i, te := range todayEvents {
		res[i] = te.event
	}

	return res, nil
}

func (c *ICalClient) parseTime(val string) (time.Time, bool) {
	// ICS format types:
	// 1. UTC: 20260712T140000Z
	// 2. Floating/Local: 20260712T140000
	// 3. Date only (all-day): 20260712

	val = strings.TrimSpace(val)
	if len(val) == 0 {
		return time.Time{}, false
	}

	// UTC Format (ends with Z)
	if strings.HasSuffix(val, "Z") {
		t, err := time.Parse("20060102T150405Z", val)
		if err == nil {
			return t.In(c.timezone), true
		}
		return time.Time{}, false
	}

	// Contains time divider 'T' (Floating local time)
	if strings.Contains(val, "T") {
		t, err := time.ParseInLocation("20060102T150405", val, c.timezone)
		if err == nil {
			return t, true
		}
		return time.Time{}, false
	}

	// Date only (All Day Event)
	if len(val) >= 8 {
		t, err := time.ParseInLocation("20060102", val[:8], c.timezone)
		if err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}
