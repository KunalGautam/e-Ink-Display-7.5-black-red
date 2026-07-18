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
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"epaper-display/go-server/internal/canvas"
	"epaper-display/go-server/internal/config"
	"epaper-display/go-server/internal/db"
	"epaper-display/go-server/internal/fonts"
	"epaper-display/go-server/internal/mqtt"
)

func main() {
	// Parse CLI flags
	configPath := flag.String("config", "config.yaml", "Path to YAML configuration file")
	flag.Parse()

	// Initialize SQLite Database
	dbPath := "./epaper.db"
	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Error initializing SQLite database: %v", err)
	}
	defer database.Close()

	// Load configuration from DB
	cfg, err := loadConfigWithDB(database, *configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	log.Printf("Configuration loaded successfully (Port: %s)", cfg.Port)

	// Pre-populate default canvas if database is empty/clean
	initDefaultCanvas(database)

	// Initialize dynamic connection MQTT registry
	mqttRegistry := mqtt.NewRegistry()
	defer mqttRegistry.Close()

	// Pre-load MQTT subscriptions for existing widgets
	widgets, err := database.GetAllWidgets()
	if err == nil {
		for _, w := range widgets {
			if w.MQTTTopic != "" {
				go func(broker, topic string) {
					_ = mqttRegistry.Subscribe(broker, topic, "", "")
				}(w.MQTTBroker, w.MQTTTopic)
			}
		}
	}

	// Initialize Google Fonts downloader and caching engine
	fontCache := fonts.NewCache("./assets/fonts")

	// Initialize Canvas drawing and binarization renderer
	canvasRenderer := canvas.NewRenderer(database, mqttRegistry, fontCache)

	// HTTP Routing Registry
	mux := http.NewServeMux()

	// Health Check / Container Live-probing endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// 1. GET /canvas/{id}/preview — returns standard debugging PNG preview
	mux.HandleFunc("/canvas/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		canvasID := parts[2]
		action := parts[3]

		if action == "preview" {
			log.Printf("Generating preview for canvas: %s", canvasID)
			img, err := canvasRenderer.RenderCanvas(r.Context(), canvasID)
			if err != nil {
				log.Printf("Render error: %v", err)
				http.Error(w, "Failed to render canvas image: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if img == nil {
				http.Error(w, "Canvas not found", http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "image/png")
			if err := png.Encode(w, img); err != nil {
				log.Printf("PNG encoding error: %v", err)
			}
			return
		}

		if action == "render" {
			log.Printf("Rendering packed binary stream for canvas: %s", canvasID)
			cRec, err := database.GetCanvas(canvasID)
			if err != nil || cRec == nil {
				http.Error(w, "Canvas profile not found", http.StatusNotFound)
				return
			}

			img, err := canvasRenderer.RenderCanvas(r.Context(), canvasID)
			if err != nil {
				log.Printf("Render error: %v", err)
				http.Error(w, "Failed to render canvas image: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Convert and pack pixels into active-low raw display formats
			payload := canvas.PackBuffers(img, cRec.ColorMode)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			_, _ = w.Write(payload)
			return
		}

		http.Error(w, "Not Found", http.StatusNotFound)
	})

	// Basic Auth REST endpoints to manage canvases and widgets
	// 2. GET /api/canvases & POST /api/canvas — canvases CRUD
	mux.HandleFunc("/api/canvas", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			list, err := database.ListCanvases()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
			return
		}

		if r.Method == http.MethodPost {
			var input struct {
				ID           string `json:"id"`
				Width        int    `json:"width"`
				Height       int    `json:"height"`
				ColorMode    string `json:"color_mode"`
				Timezone     string `json:"timezone"`
				DeviceType   string `json:"device_type"`
				MQTTBroker   string `json:"mqtt_broker"`
				MQTTUsername string `json:"mqtt_username"`
				MQTTPassword string `json:"mqtt_password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
				return
			}

			if input.ID == "" {
				http.Error(w, "Canvas ID is required", http.StatusBadRequest)
				return
			}

			// Pre-configure options based on selected device types
			if input.DeviceType != "" {
				switch input.DeviceType {
				case "waveshare_7in5_v2":
					input.Width = 800
					input.Height = 480
					input.ColorMode = "bwr"
				case "waveshare_7in5_mono":
					input.Width = 800
					input.Height = 480
					input.ColorMode = "mono"
				case "waveshare_4in2":
					input.Width = 400
					input.Height = 300
					input.ColorMode = "mono"
				case "waveshare_2in9_bwr":
					input.Width = 296
					input.Height = 128
					input.ColorMode = "bwr"
				case "waveshare_2in9_mono":
					input.Width = 296
					input.Height = 128
					input.ColorMode = "mono"
				}
			}

			if input.Width <= 0 || input.Height <= 0 {
				http.Error(w, "Invalid display dimensions", http.StatusBadRequest)
				return
			}
			if input.ColorMode == "" {
				input.ColorMode = "mono"
			}
			if input.Timezone == "" {
				input.Timezone = "Asia/Kolkata"
			}

			c := db.CanvasRecord{
				ID:           input.ID,
				Width:        input.Width,
				Height:       input.Height,
				ColorMode:    input.ColorMode,
				Timezone:     input.Timezone,
				MQTTBroker:   input.MQTTBroker,
				MQTTUsername: input.MQTTUsername,
				MQTTPassword: input.MQTTPassword,
			}

			if err := database.SaveCanvas(c); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "canvas_id": c.ID})
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}))

	// DELETE /api/canvas/{id}
	mux.HandleFunc("/api/canvas/", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		canvasID := parts[3]

		if r.Method == http.MethodDelete {
			if err := database.DeleteCanvas(canvasID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			return
		}

		if r.Method == http.MethodGet {
			// Retrieve widgets for a canvas
			widgets, err := database.GetWidgetsForCanvas(canvasID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(widgets)
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}))

	// 3. POST /api/canvas/{id}/widget — adds/updates a widget
	mux.HandleFunc("/api/widget", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var input db.WidgetRecord
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
				return
			}

			if input.ID == "" || input.CanvasID == "" || input.Type == "" {
				http.Error(w, "ID, CanvasID, and Type are required parameters", http.StatusBadRequest)
				return
			}

			// Validate target canvas profile exists
			canvasExists, err := database.GetCanvas(input.CanvasID)
			if err != nil || canvasExists == nil {
				http.Error(w, "Target Canvas ID profile does not exist", http.StatusBadRequest)
				return
			}

			if err := database.SaveWidget(input); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Dynamically trigger subscription connect for MQTT bound widgets
			if input.MQTTTopic != "" {
				go func(broker, topic string) {
					_ = mqttRegistry.Subscribe(broker, topic, "", "")
				}(input.MQTTBroker, input.MQTTTopic)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "widget_id": input.ID})
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}))

	// DELETE /api/widget/{id}
	mux.HandleFunc("/api/widget/", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		widgetID := parts[3]

		if r.Method == http.MethodDelete {
			if err := database.DeleteWidget(widgetID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}))

	// Serves settings admin layout control portal
	mux.HandleFunc("/settings", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			http.ServeFile(w, r, "static/settings.html")
			return
		}

		if r.Method == http.MethodPost {
			var model struct {
				AuthUsername string `json:"auth_username"`
				AuthPassword string `json:"auth_password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			if len(model.AuthUsername) < 3 {
				http.Error(w, "Username must be at least 3 characters", http.StatusBadRequest)
				return
			}

			if err := database.SaveAuthCredentials(model.AuthUsername, model.AuthPassword); err != nil {
				http.Error(w, "Failed to save login credentials", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			return
		}
	}))

	// Protect credentials API queries
	mux.HandleFunc("/api/settings", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			user, _, _ := database.GetAuthCredentials()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"auth_username": user})
			return
		}
	}))

	// Restart Daemon endpoint
	mux.HandleFunc("/restart", basicAuth(database, "Dashboard Admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		log.Println("Restart requested. Initiating shutdown...")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Shutting down..."))
		go func() {
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}()
	}))

	// Start HTTP Server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("E-Ink Dashboard REST API listening on port %s...", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Signal intercept for graceful exits
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

// initDefaultCanvas sets up initial default canvas layouts
func initDefaultCanvas(database *db.DB) {
	existing, _ := database.GetCanvas("default")
	if existing == nil {
		log.Println("Initializing default canvas profile layout...")
		c := db.CanvasRecord{
			ID:        "default",
			Width:     800,
			Height:    480,
			ColorMode: "bwr",
			Timezone:  "Asia/Kolkata",
		}
		_ = database.SaveCanvas(c)

		// Create standard 4-quadrant dynamic widget layouts
		widgets := []db.WidgetRecord{
			{
				ID:         "widget_calendar",
				CanvasID:   "default",
				Type:       "calendar",
				X:          15, Y: 15, Width: 300, Height: 240,
				ColorFG:    "#000000", ColorBG: "#FFFFFF",
				FontURL:    "https://fonts.googleapis.com/css2?family=Mukta:wght@400;700",
				FontSize:   14,
				FontWeight: "Regular",
			},
			{
				ID:         "widget_datetime",
				CanvasID:   "default",
				Type:       "datetime",
				X:          15, Y: 270, Width: 300, Height: 80,
				ColorFG:    "#FF0000", ColorBG: "#FFFFFF",
				FontURL:    "https://fonts.googleapis.com/css2?family=Mukta:wght@400;700",
				FontSize:   15,
				FontWeight: "Bold",
				CustomConfig: `{"format":"Monday, Jan _2"}`,
			},
			{
				ID:         "widget_time",
				CanvasID:   "default",
				Type:       "datetime",
				X:          15, Y: 350, Width: 300, Height: 100,
				ColorFG:    "#000000", ColorBG: "#FFFFFF",
				FontURL:    "https://fonts.googleapis.com/css2?family=Mukta:wght@400;700",
				FontSize:   28,
				FontWeight: "Bold",
				CustomConfig: `{"format":"03:04 PM"}`,
			},
			{
				ID:         "widget_emails",
				CanvasID:   "default",
				Type:       "emails",
				X:          340, Y: 15, Width: 440, Height: 220,
				MQTTTopic:  "home/eink/emails", MQTTBroker: "tcp://100.101.102.4:1883",
				ColorFG:    "#000000", ColorBG: "#FFFFFF",
				FontURL:    "https://fonts.googleapis.com/css2?family=Mukta:wght@400;700",
				FontSize:   14,
				FontWeight: "Regular",
			},
			{
				ID:         "widget_notes",
				CanvasID:   "default",
				Type:       "notes",
				X:          340, Y: 250, Width: 440, Height: 210,
				MQTTTopic:  "home/eink/notes", MQTTBroker: "tcp://100.101.102.4:1883",
				ColorFG:    "#000000", ColorBG: "#FFFFFF",
				FontURL:    "https://fonts.googleapis.com/css2?family=Mukta:wght@400;700",
				FontSize:   14,
				FontWeight: "Regular",
			},
		}

		for _, w := range widgets {
			_ = database.SaveWidget(w)
		}
	}
}

// loadConfigWithDB handles loading configuration from SQLite or config.yaml fallback
func loadConfigWithDB(database *db.DB, configPath string) (*config.Config, error) {
	port, err := database.GetSetting("port")
	if err != nil {
		return nil, err
	}

	if port == "" {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return nil, err
		}
		_ = database.SaveSetting("port", cfg.Port)
		return cfg, nil
	}

	return &config.Config{Port: port}, nil
}

// basicAuth handles HTTP Basic Authentication wrappers
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
