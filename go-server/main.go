package main

import (
	"context"
	"encoding/json"
	"flag"
	"image/png"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"epaper-display/go-server/internal/calendar"
	"epaper-display/go-server/internal/config"
	"epaper-display/go-server/internal/db"
	"epaper-display/go-server/internal/mqtt"
	"epaper-display/go-server/internal/renderer"
	"epaper-display/go-server/internal/weather"

	"golang.org/x/crypto/bcrypt"
)

type settingsAPIModel struct {
	MQTTBroker    string `json:"mqtt_broker"`
	MQTTClientID  string `json:"mqtt_client_id"`
	MQTTUsername  string `json:"mqtt_username"`
	MQTTPassword  string `json:"mqtt_password"`
	NotesTopic    string `json:"notes_topic"`
	EmailsTopic   string `json:"emails_topic"`
	CalendarTopic string `json:"calendar_topic"`
	ICalURL       string `json:"ical_url"`
	Timezone      string `json:"timezone"`
	Port          string `json:"port"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	FontFamily    string `json:"font_family"`
	LayoutStyle   string `json:"layout_style"`
	ShowCalendar  bool   `json:"show_calendar"`
	ShowSchedule  bool   `json:"show_schedule"`
	ShowInbox     bool   `json:"show_inbox"`
	ShowNotes     bool   `json:"show_notes"`
	ShowWeather   bool   `json:"show_weather"`
	ShowSensors   bool   `json:"show_sensors"`
	WeatherAPIKey string `json:"weather_api_key"`
	WeatherCity   string `json:"weather_city"`
	AuthUsername  string `json:"auth_username"`
	AuthPassword  string `json:"auth_password,omitempty"`
}

func main() {
	// Parse CLI flags
	configPath := flag.String("config", "config.yaml", "Path to YAML configuration file")
	flag.Parse()

	log.Println("Starting E-Ink Layout Server with Devanagari & Calendar support...")

	// 1. Initialize SQLite Database
	dbPath := "epaper.db"
	if envDir := os.Getenv("DATA_DIR"); envDir != "" {
		dbPath = envDir + "/epaper.db"
	}
	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Error initializing SQLite database: %v", err)
	}
	defer database.Close()

	// 2. Load configuration from DB with file/env fallback
	cfg, err := loadConfigWithDB(database, *configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	log.Printf("Configuration loaded successfully (Port: %s, Display: %dx%d)", cfg.Port, cfg.Width, cfg.Height)

	// 3. Ensure Fonts are Downloaded (Saves selected font family locally)
	regPath, boldPath, err := renderer.EnsureFontsDownloaded(cfg.FontFamily)
	if err != nil {
		log.Fatalf("Error downloading/loading fonts: %v", err)
	}

	// 4. Initialize Weather Client
	weatherClient := weather.NewWeatherClient()

	// 5. Initialize MQTT Client (pre-populates cache from DB on creation) and Connect
	mqttClient := mqtt.NewClient(cfg, database, weatherClient)
	if err := mqttClient.Connect(); err != nil {
		log.Printf("Warning: Failed to connect to MQTT broker initially: %v. Reconnection will be attempted in the background.", err)
	}
	defer mqttClient.Disconnect()

	// 6. Start OpenWeatherMap Sync Loop if key is configured
	if cfg.WeatherAPIKey != "" {
		go func() {
			log.Println("Performing initial OpenWeatherMap fetch...")
			if err := weatherClient.FetchOpenWeatherMap(cfg.WeatherAPIKey, cfg.WeatherCity); err != nil {
				log.Printf("OpenWeatherMap fetch error: %v", err)
			}

			ticker := time.NewTicker(15 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				log.Println("Fetching weather from OpenWeatherMap...")
				if err := weatherClient.FetchOpenWeatherMap(cfg.WeatherAPIKey, cfg.WeatherCity); err != nil {
					log.Printf("OpenWeatherMap fetch error: %v", err)
				}
			}
		}()
	}

	// 7. Initialize Google Calendar iCal Client & Sync Loop
	icalClient := calendar.NewICalClient(cfg)
	if cfg.ICalURL != "" {
		go func() {
			log.Println("Performing initial Google Calendar sync...")
			if err := icalClient.Sync(); err != nil {
				log.Printf("Google Calendar sync error: %v", err)
			}

			ticker := time.NewTicker(15 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				log.Println("Syncing Google Calendar events...")
				if err := icalClient.Sync(); err != nil {
					log.Printf("Google Calendar sync error: %v", err)
				}
			}
		}()
	} else {
		log.Println("No Google Calendar ical_url configured. Skipping background sync.")
	}

	// 6. Initialize Renderer
	rend := renderer.NewRenderer(cfg, regPath, boldPath)

	// Helper to merge and sort calendar events
	mergeEvents := func() []mqtt.CalendarEvent {
		mqttEvents := mqttClient.GetCalendarEvents()
		icalEvents := icalClient.GetEvents()

		// Merge
		var merged []mqtt.CalendarEvent
		merged = append(merged, mqttEvents...)
		merged = append(merged, icalEvents...)

		// Assign default start time if zero (defaults to today's start)
		tz := icalClient.GetTimezone()
		now := time.Now().In(tz)
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tz)
		for i := range merged {
			if merged[i].Start.IsZero() {
				merged[i].Start = todayStart
			}
		}

		// Sort chronologically by start time
		sort.Slice(merged, func(i, j int) bool {
			if merged[i].Start.Equal(merged[j].Start) {
				isAllDayI := strings.Contains(merged[i].Time, "All Day")
				isAllDayJ := strings.Contains(merged[j].Time, "All Day")
				if isAllDayI && !isAllDayJ {
					return true
				}
				if isAllDayJ && !isAllDayI {
					return false
				}
			}
			return merged[i].Start.Before(merged[j].Start)
		})

		return merged
	}

	// 7. Setup HTTP router
	mux := http.NewServeMux()

	// REST endpoint to get PNG formatted image layout (no auth for physical displays)
	mux.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HTTP /image requested by %s", r.RemoteAddr)

		notes := mqttClient.GetNotes()
		emails := mqttClient.GetEmails()
		calEvents := mergeEvents()

		lastUpdated := mqttClient.GetLastUpdated()
		if cfg.ICalURL != "" {
			icalUpdated := icalClient.GetLastUpdated()
			if icalUpdated.After(lastUpdated) {
				lastUpdated = icalUpdated
			}
		}
		weatherUpdated := weatherClient.GetLastUpdated()
		if weatherUpdated.After(lastUpdated) {
			lastUpdated = weatherUpdated
		}
		lastUpdatedStr := lastUpdated.Format("2006-01-02 15:04:05")

		img, err := rend.Render(notes, emails, calEvents, weatherClient.GetWeather(), lastUpdatedStr)
		if err != nil {
			log.Printf("Render error: %v", err)
			http.Error(w, "Failed to render layout", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			log.Printf("PNG encoding error: %v", err)
			http.Error(w, "Failed to encode image", http.StatusInternalServerError)
		}
	})

	// REST endpoint to get raw packed e-paper display bytes (no auth for physical displays)
	mux.HandleFunc("/image/raw", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HTTP /image/raw requested by %s", r.RemoteAddr)

		notes := mqttClient.GetNotes()
		emails := mqttClient.GetEmails()
		calEvents := mergeEvents()

		lastUpdated := mqttClient.GetLastUpdated()
		if cfg.ICalURL != "" {
			icalUpdated := icalClient.GetLastUpdated()
			if icalUpdated.After(lastUpdated) {
				lastUpdated = icalUpdated
			}
		}
		weatherUpdated := weatherClient.GetLastUpdated()
		if weatherUpdated.After(lastUpdated) {
			lastUpdated = weatherUpdated
		}
		lastUpdatedStr := lastUpdated.Format("2006-01-02 15:04:05")

		img, err := rend.Render(notes, emails, calEvents, weatherClient.GetWeather(), lastUpdatedStr)
		if err != nil {
			log.Printf("Render error: %v", err)
			http.Error(w, "Failed to render layout", http.StatusInternalServerError)
			return
		}

		// Pack into Waveshare bits
		blackBuf, redBuf := renderer.PackBuffers(img)
		payload := append(blackBuf, redBuf...)

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", string(rune(len(payload))))
		
		if _, err := w.Write(payload); err != nil {
			log.Printf("Failed writing raw payload bytes: %v", err)
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Basic Auth Protected Settings Portal Page (GET /settings to view page, POST /settings to update)
	mux.HandleFunc("/settings", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			http.ServeFile(w, r, "static/settings.html")
			return
		}

		if r.Method == http.MethodPost {
			var model settingsAPIModel
			if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
				http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
				return
			}

			// Validate input lengths
			if len(model.AuthUsername) < 3 {
				http.Error(w, "Username must be at least 3 characters", http.StatusBadRequest)
				return
			}
			if model.AuthPassword != "" && len(model.AuthPassword) < 4 {
				http.Error(w, "Password must be at least 4 characters", http.StatusBadRequest)
				return
			}

			// Save credentials if changed
			if err := database.SaveAuthCredentials(model.AuthUsername, model.AuthPassword); err != nil {
				log.Printf("Error saving auth credentials: %v", err)
				http.Error(w, "Failed to save login credentials", http.StatusInternalServerError)
				return
			}

			// Save config fields to DB
			_ = database.SaveSetting("mqtt_broker", model.MQTTBroker)
			_ = database.SaveSetting("mqtt_client_id", model.MQTTClientID)
			_ = database.SaveSetting("mqtt_username", model.MQTTUsername)
			_ = database.SaveSetting("mqtt_password", model.MQTTPassword)
			_ = database.SaveSetting("notes_topic", model.NotesTopic)
			_ = database.SaveSetting("emails_topic", model.EmailsTopic)
			_ = database.SaveSetting("calendar_topic", model.CalendarTopic)
			_ = database.SaveSetting("ical_url", model.ICalURL)
			_ = database.SaveSetting("timezone", model.Timezone)
			_ = database.SaveSetting("port", model.Port)
			_ = database.SaveSetting("width", strconv.Itoa(model.Width))
			_ = database.SaveSetting("height", strconv.Itoa(model.Height))
			_ = database.SaveSetting("font_family", model.FontFamily)
			_ = database.SaveSetting("layout_style", model.LayoutStyle)
			_ = database.SaveSetting("show_calendar", strconv.FormatBool(model.ShowCalendar))
			_ = database.SaveSetting("show_schedule", strconv.FormatBool(model.ShowSchedule))
			_ = database.SaveSetting("show_inbox", strconv.FormatBool(model.ShowInbox))
			_ = database.SaveSetting("show_notes", strconv.FormatBool(model.ShowNotes))
			_ = database.SaveSetting("show_weather", strconv.FormatBool(model.ShowWeather))
			_ = database.SaveSetting("show_sensors", strconv.FormatBool(model.ShowSensors))
			_ = database.SaveSetting("weather_api_key", model.WeatherAPIKey)
			_ = database.SaveSetting("weather_city", model.WeatherCity)

			log.Println("Configuration successfully saved to SQLite database")

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}))

	// Basic Auth Protected JSON Settings API (used by Settings GUI)
	mux.HandleFunc("/api/settings", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		user, _, _ := database.GetAuthCredentials()
		broker, _ := database.GetSetting("mqtt_broker")
		clientId, _ := database.GetSetting("mqtt_client_id")
		username, _ := database.GetSetting("mqtt_username")
		password, _ := database.GetSetting("mqtt_password")
		notesTopic, _ := database.GetSetting("notes_topic")
		emailsTopic, _ := database.GetSetting("emails_topic")
		calTopic, _ := database.GetSetting("calendar_topic")
		icalUrl, _ := database.GetSetting("ical_url")
		timezone, _ := database.GetSetting("timezone")
		port, _ := database.GetSetting("port")
		widthStr, _ := database.GetSetting("width")
		heightStr, _ := database.GetSetting("height")
		fontFamily, _ := database.GetSetting("font_family")

		layoutStyle, _ := database.GetSetting("layout_style")
		if layoutStyle == "" {
			layoutStyle = "default"
		}
		showCalStr, _ := database.GetSetting("show_calendar")
		showCal := showCalStr != "false"
		showSchStr, _ := database.GetSetting("show_schedule")
		showSch := showSchStr != "false"
		showInbStr, _ := database.GetSetting("show_inbox")
		showInb := showInbStr != "false"
		showNotStr, _ := database.GetSetting("show_notes")
		showNot := showNotStr != "false"
		showWeaStr, _ := database.GetSetting("show_weather")
		showWea := showWeaStr != "false"
		showSenStr, _ := database.GetSetting("show_sensors")
		showSen := showSenStr != "false"
		weatherKey, _ := database.GetSetting("weather_api_key")
		weatherCity, _ := database.GetSetting("weather_city")
		if weatherCity == "" {
			weatherCity = "New Delhi,IN"
		}

		width, _ := strconv.Atoi(widthStr)
		height, _ := strconv.Atoi(heightStr)

		model := settingsAPIModel{
			MQTTBroker:    broker,
			MQTTClientID:  clientId,
			MQTTUsername:  username,
			MQTTPassword:  password,
			NotesTopic:    notesTopic,
			EmailsTopic:   emailsTopic,
			CalendarTopic: calTopic,
			ICalURL:       icalUrl,
			Timezone:      timezone,
			Port:          port,
			Width:         width,
			Height:        height,
			FontFamily:    fontFamily,
			LayoutStyle:   layoutStyle,
			ShowCalendar:  showCal,
			ShowSchedule:  showSch,
			ShowInbox:     showInb,
			ShowNotes:     showNot,
			ShowWeather:   showWea,
			ShowSensors:   showSen,
			WeatherAPIKey: weatherKey,
			WeatherCity:   weatherCity,
			AuthUsername:  user,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(model)
	}))

	// Basic Auth Protected Restart daemon endpoint
	mux.HandleFunc("/restart", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Println("Restart requested via Web Admin interface. Initiating graceful shutdown...")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Shutting down..."))

		// Asynchronously terminate process so http response sends successfully first
		go func() {
			time.Sleep(1 * time.Second)
			log.Println("Server exiting now (Code 0). Process daemon manager will restart.")
			os.Exit(0)
		}()
	}))

	// 8. Start HTTP Server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("HTTP server listening on port %s...", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Setup signal interception for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Println("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped.")
}

// loadConfigWithDB loads application config from DB settings table, falling back to config.yaml if empty
func loadConfigWithDB(database *db.DB, configPath string) (*config.Config, error) {
	broker, err := database.GetSetting("mqtt_broker")
	if err != nil {
		return nil, err
	}

	// If broker setting is empty, populate SQLite from YAML/env variables config loader
	if broker == "" {
		log.Println("SQLite database settings are empty. Initializing settings from local config file...")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return nil, err
		}

		_ = database.SaveSetting("mqtt_broker", cfg.MQTTBroker)
		_ = database.SaveSetting("mqtt_client_id", cfg.MQTTClientID)
		_ = database.SaveSetting("mqtt_username", cfg.MQTTUsername)
		_ = database.SaveSetting("mqtt_password", cfg.MQTTPassword)
		_ = database.SaveSetting("notes_topic", cfg.NotesTopic)
		_ = database.SaveSetting("emails_topic", cfg.EmailsTopic)
		_ = database.SaveSetting("calendar_topic", cfg.CalendarTopic)
		_ = database.SaveSetting("ical_url", cfg.ICalURL)
		_ = database.SaveSetting("timezone", cfg.Timezone)
		_ = database.SaveSetting("port", cfg.Port)
		_ = database.SaveSetting("width", strconv.Itoa(cfg.Width))
		_ = database.SaveSetting("height", strconv.Itoa(cfg.Height))
		_ = database.SaveSetting("font_family", cfg.FontFamily)
		_ = database.SaveSetting("layout_style", cfg.LayoutStyle)
		_ = database.SaveSetting("show_calendar", strconv.FormatBool(cfg.ShowCalendar))
		_ = database.SaveSetting("show_schedule", strconv.FormatBool(cfg.ShowSchedule))
		_ = database.SaveSetting("show_inbox", strconv.FormatBool(cfg.ShowInbox))
		_ = database.SaveSetting("show_notes", strconv.FormatBool(cfg.ShowNotes))
		_ = database.SaveSetting("show_weather", strconv.FormatBool(cfg.ShowWeather))
		_ = database.SaveSetting("show_sensors", strconv.FormatBool(cfg.ShowSensors))
		_ = database.SaveSetting("weather_api_key", cfg.WeatherAPIKey)
		_ = database.SaveSetting("weather_city", cfg.WeatherCity)

		return cfg, nil
	}

	// Otherwise load configurations straight from SQLite database
	cfg := &config.Config{
		MQTTBroker: broker,
	}

	if val, err := database.GetSetting("mqtt_client_id"); err == nil {
		cfg.MQTTClientID = val
	}
	if val, err := database.GetSetting("mqtt_username"); err == nil {
		cfg.MQTTUsername = val
	}
	if val, err := database.GetSetting("mqtt_password"); err == nil {
		cfg.MQTTPassword = val
	}
	if val, err := database.GetSetting("notes_topic"); err == nil {
		cfg.NotesTopic = val
	}
	if val, err := database.GetSetting("emails_topic"); err == nil {
		cfg.EmailsTopic = val
	}
	if val, err := database.GetSetting("calendar_topic"); err == nil {
		cfg.CalendarTopic = val
	}
	if val, err := database.GetSetting("ical_url"); err == nil {
		cfg.ICalURL = val
	}
	if val, err := database.GetSetting("timezone"); err == nil {
		cfg.Timezone = val
	}
	if val, err := database.GetSetting("port"); err == nil {
		cfg.Port = val
	}
	if val, err := database.GetSetting("width"); err == nil {
		if w, err := strconv.Atoi(val); err == nil {
			cfg.Width = w
		}
	}
	if val, err := database.GetSetting("height"); err == nil {
		if h, err := strconv.Atoi(val); err == nil {
			cfg.Height = h
		}
	}
	if val, err := database.GetSetting("font_family"); err == nil {
		cfg.FontFamily = val
	}
	cfg.LayoutStyle, _ = database.GetSetting("layout_style")
	if cfg.LayoutStyle == "" {
		cfg.LayoutStyle = "default"
	}
	if val, err := database.GetSetting("show_calendar"); err == nil && val != "" {
		cfg.ShowCalendar, _ = strconv.ParseBool(val)
	} else {
		cfg.ShowCalendar = true
	}
	if val, err := database.GetSetting("show_schedule"); err == nil && val != "" {
		cfg.ShowSchedule, _ = strconv.ParseBool(val)
	} else {
		cfg.ShowSchedule = true
	}
	if val, err := database.GetSetting("show_inbox"); err == nil && val != "" {
		cfg.ShowInbox, _ = strconv.ParseBool(val)
	} else {
		cfg.ShowInbox = true
	}
	if val, err := database.GetSetting("show_notes"); err == nil && val != "" {
		cfg.ShowNotes, _ = strconv.ParseBool(val)
	} else {
		cfg.ShowNotes = true
	}
	if val, err := database.GetSetting("show_weather"); err == nil && val != "" {
		cfg.ShowWeather, _ = strconv.ParseBool(val)
	} else {
		cfg.ShowWeather = true
	}
	if val, err := database.GetSetting("show_sensors"); err == nil && val != "" {
		cfg.ShowSensors, _ = strconv.ParseBool(val)
	} else {
		cfg.ShowSensors = true
	}
	cfg.WeatherAPIKey, _ = database.GetSetting("weather_api_key")
	cfg.WeatherCity, _ = database.GetSetting("weather_city")
	if cfg.WeatherCity == "" {
		cfg.WeatherCity = "New Delhi,IN"
	}

	return cfg, nil
}

// basicAuth HTTP Middleware wrapping logic
func basicAuth(database *db.DB, realm string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		storedUser, storedHash, err := database.GetAuthCredentials()
		if err != nil {
			log.Printf("Auth database fetch error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if username != storedUser || bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
