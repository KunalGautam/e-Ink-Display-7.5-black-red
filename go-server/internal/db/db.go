package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type EmailRecord struct {
	Sender  string
	Subject string
}

type DB struct {
	conn *sql.DB
	mu   sync.Mutex
}

// InitDB initializes the SQLite database at the specified path and runs tables migration
func InitDB(dbPath string) (*DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Create tables schema
	queries := []string{
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS emails (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sender TEXT,
			subject TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS canvases (
			id TEXT PRIMARY KEY,
			width INTEGER,
			height INTEGER,
			color_mode TEXT,
			timezone TEXT,
			mqtt_broker TEXT DEFAULT '',
			mqtt_username TEXT DEFAULT '',
			mqtt_password TEXT DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS widgets (
			id TEXT PRIMARY KEY,
			canvas_id TEXT,
			type TEXT,
			x INTEGER,
			y INTEGER,
			width INTEGER,
			height INTEGER,
			mqtt_topic TEXT,
			mqtt_broker TEXT,
			color_fg TEXT,
			color_bg TEXT,
			font_url TEXT,
			font_size REAL,
			font_weight TEXT,
			custom_config TEXT,
			border_width INTEGER DEFAULT 0,
			border_color TEXT DEFAULT '',
			FOREIGN KEY(canvas_id) REFERENCES canvases(id) ON DELETE CASCADE
		);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	// Migrate existing database schemas if columns are missing
	_, _ = db.Exec("ALTER TABLE widgets ADD COLUMN border_width INTEGER DEFAULT 0")
	_, _ = db.Exec("ALTER TABLE widgets ADD COLUMN border_color TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE canvases ADD COLUMN mqtt_broker TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE canvases ADD COLUMN mqtt_username TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE canvases ADD COLUMN mqtt_password TEXT DEFAULT ''")

	d := &DB{conn: db}
	if err := d.initDefaultSettings(); err != nil {
		db.Close()
		return nil, err
	}

	return d, nil
}

func (d *DB) Close() {
	if d.conn != nil {
		_ = d.conn.Close()
	}
}

// initDefaultSettings populates default values for configuration and basic auth credentials
func (d *DB) initDefaultSettings() error {
	defaults := map[string]string{
		"port":           "8080",
		"width":          "800",
		"height":         "480",
		"timezone":       "Asia/Kolkata",
		"mqtt_broker":    "tcp://localhost:1883",
		"mqtt_client_id": "epaper-display-server",
		"notes_topic":    "home/eink/notes",
		"emails_topic":   "home/eink/emails",
		"calendar_topic": "home/eink/calendar",
		"weather_topic":  "home/eink/weather",
		"auth_username":  "admin",
		"font_family":     "Mukta",
		"layout_style":    "default",
		"show_calendar":   "true",
		"show_schedule":   "true",
		"show_inbox":      "true",
		"show_notes":      "true",
		"show_weather":    "true",
		"show_sensors":    "true",
		"weather_api_key": "",
		"weather_city":    "New Delhi,IN",
	}

	for k, v := range defaults {
		exists, err := d.hasSetting(k)
		if err != nil {
			return err
		}
		if !exists {
			if err := d.SaveSetting(k, v); err != nil {
				return err
			}
		}
	}

	// Check and initialize default Basic Auth credentials
	authExists, err := d.hasSetting("auth_password_hash")
	if err != nil {
		return err
	}
	if !authExists {
		// Hash default password "admin"
		hashed, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash default admin password: %w", err)
		}
		if err := d.SaveSetting("auth_password_hash", string(hashed)); err != nil {
			return err
		}
	}

	return nil
}

func (d *DB) hasSetting(key string) (bool, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", key).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) GetSetting(key string) (string, error) {
	var val string
	err := d.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (d *DB) SaveSetting(key, val string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, val)
	return err
}

func (d *DB) GetAuthCredentials() (string, string, error) {
	user, err := d.GetSetting("auth_username")
	if err != nil {
		return "", "", err
	}
	hash, err := d.GetSetting("auth_password_hash")
	if err != nil {
		return "", "", err
	}
	return user, hash, nil
}

func (d *DB) SaveAuthCredentials(username, plainPassword string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", "auth_username", username)
	if err != nil {
		return err
	}

	if plainPassword != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		_, err = tx.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", "auth_password_hash", string(hashed))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) SaveNotes(notes []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert all new notes
	for _, note := range notes {
		_, err := tx.Exec("INSERT INTO notes (content) VALUES (?)", note)
		if err != nil {
			return err
		}
	}

	// Prune notes to keep only the last 10 entries
	_, err = tx.Exec(`DELETE FROM notes WHERE id NOT IN (
		SELECT id FROM notes ORDER BY id DESC LIMIT 10
	)`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *DB) GetCachedNotes() ([]string, error) {
	rows, err := d.conn.Query("SELECT content FROM notes ORDER BY id DESC LIMIT 10")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []string
	for rows.Next() {
		var note string
		if err := rows.Scan(&note); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}

	// Reverse slice order so the most recent is drawn first/last appropriately
	// (usually when fetched we keep them as is or in order. Since query ordered DESC, index 0 is most recent)
	return notes, nil
}

func (d *DB) SaveEmails(emails []EmailRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert all new emails
	for _, email := range emails {
		_, err := tx.Exec("INSERT INTO emails (sender, subject) VALUES (?, ?)", email.Sender, email.Subject)
		if err != nil {
			return err
		}
	}

	// Prune emails to keep only the last 10 entries
	_, err = tx.Exec(`DELETE FROM emails WHERE id NOT IN (
		SELECT id FROM emails ORDER BY id DESC LIMIT 10
	)`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *DB) GetCachedEmails() ([]EmailRecord, error) {
	rows, err := d.conn.Query("SELECT sender, subject FROM emails ORDER BY id DESC LIMIT 10")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []EmailRecord
	for rows.Next() {
		var email EmailRecord
		if err := rows.Scan(&email.Sender, &email.Subject); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}

	return emails, nil
}

type CanvasRecord struct {
	ID           string `json:"id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	ColorMode    string `json:"color_mode"`
	Timezone     string `json:"timezone"`
	MQTTBroker   string `json:"mqtt_broker"`
	MQTTUsername string `json:"mqtt_username"`
	MQTTPassword string `json:"mqtt_password"`
}

type WidgetRecord struct {
	ID           string  `json:"id"`
	CanvasID     string  `json:"canvas_id"`
	Type         string  `json:"type"`
	X            int     `json:"x"`
	Y            int     `json:"y"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	MQTTTopic    string  `json:"mqtt_topic"`
	MQTTBroker   string  `json:"mqtt_broker"`
	ColorFG      string  `json:"color_fg"`
	ColorBG      string  `json:"color_bg"`
	FontURL      string  `json:"font_url"`
	FontSize     float64 `json:"font_size"`
	FontWeight   string  `json:"font_weight"`
	CustomConfig string  `json:"custom_config"`
	BorderWidth  int     `json:"border_width"`
	BorderColor  string  `json:"border_color"`
}

func (d *DB) SaveCanvas(c CanvasRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec("INSERT OR REPLACE INTO canvases (id, width, height, color_mode, timezone, mqtt_broker, mqtt_username, mqtt_password) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		c.ID, c.Width, c.Height, c.ColorMode, c.Timezone, c.MQTTBroker, c.MQTTUsername, c.MQTTPassword)
	return err
}

func (d *DB) GetCanvas(id string) (*CanvasRecord, error) {
	var c CanvasRecord
	err := d.conn.QueryRow("SELECT id, width, height, color_mode, timezone, mqtt_broker, mqtt_username, mqtt_password FROM canvases WHERE id = ?", id).
		Scan(&c.ID, &c.Width, &c.Height, &c.ColorMode, &c.Timezone, &c.MQTTBroker, &c.MQTTUsername, &c.MQTTPassword)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *DB) ListCanvases() ([]CanvasRecord, error) {
	rows, err := d.conn.Query("SELECT id, width, height, color_mode, timezone, mqtt_broker, mqtt_username, mqtt_password FROM canvases")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []CanvasRecord
	for rows.Next() {
		var c CanvasRecord
		if err := rows.Scan(&c.ID, &c.Width, &c.Height, &c.ColorMode, &c.Timezone, &c.MQTTBroker, &c.MQTTUsername, &c.MQTTPassword); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (d *DB) DeleteCanvas(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec("DELETE FROM canvases WHERE id = ?", id)
	return err
}

func (d *DB) SaveWidget(w WidgetRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(`INSERT OR REPLACE INTO widgets 
		(id, canvas_id, type, x, y, width, height, mqtt_topic, mqtt_broker, color_fg, color_bg, font_url, font_size, font_weight, custom_config, border_width, border_color) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.CanvasID, w.Type, w.X, w.Y, w.Width, w.Height, w.MQTTTopic, w.MQTTBroker, w.ColorFG, w.ColorBG, w.FontURL, w.FontSize, w.FontWeight, w.CustomConfig, w.BorderWidth, w.BorderColor)
	return err
}

func (d *DB) GetWidgetsForCanvas(canvasID string) ([]WidgetRecord, error) {
	rows, err := d.conn.Query(`SELECT id, canvas_id, type, x, y, width, height, mqtt_topic, mqtt_broker, color_fg, color_bg, font_url, font_size, font_weight, custom_config, border_width, border_color 
		FROM widgets WHERE canvas_id = ?`, canvasID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WidgetRecord
	for rows.Next() {
		var w WidgetRecord
		err := rows.Scan(&w.ID, &w.CanvasID, &w.Type, &w.X, &w.Y, &w.Width, &w.Height, &w.MQTTTopic, &w.MQTTBroker, &w.ColorFG, &w.ColorBG, &w.FontURL, &w.FontSize, &w.FontWeight, &w.CustomConfig, &w.BorderWidth, &w.BorderColor)
		if err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, nil
}

func (d *DB) GetAllWidgets() ([]WidgetRecord, error) {
	rows, err := d.conn.Query(`SELECT id, canvas_id, type, x, y, width, height, mqtt_topic, mqtt_broker, color_fg, color_bg, font_url, font_size, font_weight, custom_config, border_width, border_color 
		FROM widgets`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WidgetRecord
	for rows.Next() {
		var w WidgetRecord
		err := rows.Scan(&w.ID, &w.CanvasID, &w.Type, &w.X, &w.Y, &w.Width, &w.Height, &w.MQTTTopic, &w.MQTTBroker, &w.ColorFG, &w.ColorBG, &w.FontURL, &w.FontSize, &w.FontWeight, &w.CustomConfig, &w.BorderWidth, &w.BorderColor)
		if err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, nil
}

func (d *DB) DeleteWidget(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec("DELETE FROM widgets WHERE id = ?", id)
	return err
}
