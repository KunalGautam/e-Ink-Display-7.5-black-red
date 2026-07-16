package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MQTTBroker    string `yaml:"mqtt_broker"`
	MQTTClientID  string `yaml:"mqtt_client_id"`
	MQTTUsername  string `yaml:"mqtt_username"`
	MQTTPassword  string `yaml:"mqtt_password"`
	NotesTopic    string `yaml:"notes_topic"`
	EmailsTopic   string `yaml:"emails_topic"`
	CalendarTopic string `yaml:"calendar_topic"`
	ICalURL       string `yaml:"ical_url"`
	Timezone      string `yaml:"timezone"`
	Port          string `yaml:"port"`
	Width         int    `yaml:"width"`
	Height        int    `yaml:"height"`
	FontFamily    string `yaml:"font_family"`
}

// LoadConfig loads configuration from a file, then overrides with environment variables
func LoadConfig(path string) (*Config, error) {
	// Start with default values
	cfg := &Config{
		MQTTBroker:    "tcp://localhost:1883",
		MQTTClientID:  "epaper-display-server",
		NotesTopic:    "home/eink/notes",
		EmailsTopic:   "home/eink/emails",
		CalendarTopic: "home/eink/calendar",
		Timezone:      "Local",
		Port:          "8080",
		Width:         800,
		Height:        480,
		FontFamily:    "Noto Sans Devanagari",
	}

	// Try reading from file if it exists
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Override with environment variables if set
	if env := os.Getenv("MQTT_BROKER"); env != "" {
		cfg.MQTTBroker = env
	}
	if env := os.Getenv("MQTT_CLIENT_ID"); env != "" {
		cfg.MQTTClientID = env
	}
	if env := os.Getenv("MQTT_USERNAME"); env != "" {
		cfg.MQTTUsername = env
	}
	if env := os.Getenv("MQTT_PASSWORD"); env != "" {
		cfg.MQTTPassword = env
	}
	if env := os.Getenv("NOTES_TOPIC"); env != "" {
		cfg.NotesTopic = env
	}
	if env := os.Getenv("EMAILS_TOPIC"); env != "" {
		cfg.EmailsTopic = env
	}
	if env := os.Getenv("CALENDAR_TOPIC"); env != "" {
		cfg.CalendarTopic = env
	}
	if env := os.Getenv("ICAL_URL"); env != "" {
		cfg.ICalURL = env
	}
	if env := os.Getenv("TIMEZONE"); env != "" {
		cfg.Timezone = env
	}
	if env := os.Getenv("PORT"); env != "" {
		cfg.Port = env
	}
	if env := os.Getenv("DISPLAY_WIDTH"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			cfg.Width = val
		}
	}
	if env := os.Getenv("DISPLAY_HEIGHT"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			cfg.Height = val
		}
	}
	if env := os.Getenv("FONT_FAMILY"); env != "" {
		cfg.FontFamily = env
	}

	return cfg, nil
}
