package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"epaper-display/go-server/internal/config"
	"epaper-display/go-server/internal/db"
	"epaper-display/go-server/internal/weather"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Email struct {
	Sender  string `json:"sender"`
	Subject string `json:"subject"`
}

type CalendarEvent struct {
	Title string    `json:"title"`
	Time  string    `json:"time"` // e.g. "09:00 - 10:00" or "All Day"
	Start time.Time `json:"start,omitempty"`
}

type Client struct {
	client        mqtt.Client
	cfg           *config.Config
	db            *db.DB
	weatherClient *weather.WeatherClient

	mu          sync.RWMutex
	notes       []string
	emails      []Email
	calendar    []CalendarEvent
	lastUpdated time.Time
}

func NewClient(cfg *config.Config, database *db.DB, weatherClient *weather.WeatherClient) *Client {
	c := &Client{
		cfg:           cfg,
		db:            database,
		weatherClient: weatherClient,
		notes:         []string{"No notes received yet. Publish to " + cfg.NotesTopic},
		emails:        []Email{{Sender: "System", Subject: "No emails received yet. Publish to " + cfg.EmailsTopic}},
		calendar:      []CalendarEvent{{Title: "No events received yet. Publish to " + cfg.CalendarTopic, Time: "All Day"}},
		lastUpdated:   time.Now(),
	}

	// Pre-populate weather cache from SQLite DB
	if cachedWeather, err := database.GetSetting("cached_weather"); err == nil && cachedWeather != "" {
		var parsedWeather weather.WeatherData
		if err := json.Unmarshal([]byte(cachedWeather), &parsedWeather); err == nil {
			weatherClient.SetWeather(&parsedWeather)
			log.Println("Loaded cached weather from SQLite database cache")
		}
	}

	// Pre-populate caches from SQLite DB for offline resilience
	if dbNotes, err := database.GetCachedNotes(); err == nil && len(dbNotes) > 0 {
		c.notes = dbNotes
		log.Printf("Loaded %d notes from SQLite database cache", len(dbNotes))
	}
	if dbEmails, err := database.GetCachedEmails(); err == nil && len(dbEmails) > 0 {
		c.emails = make([]Email, len(dbEmails))
		for i, e := range dbEmails {
			c.emails[i] = Email{Sender: e.Sender, Subject: e.Subject}
		}
		log.Printf("Loaded %d emails from SQLite database cache", len(dbEmails))
	}

	return c
}

func (c *Client) Connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.cfg.MQTTBroker)
	opts.SetClientID(c.cfg.MQTTClientID)
	
	if c.cfg.MQTTUsername != "" {
		opts.SetUsername(c.cfg.MQTTUsername)
	}
	if c.cfg.MQTTPassword != "" {
		opts.SetPassword(c.cfg.MQTTPassword)
	}

	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(5 * time.Minute)
	opts.SetCleanSession(false)

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v. Reconnecting automatically...", err)
	})

	opts.SetReconnectingHandler(func(client mqtt.Client, options *mqtt.ClientOptions) {
		log.Println("MQTT client attempting to reconnect...")
	})

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Println("MQTT client connected to broker")
		
		// Subscribe to notes
		if token := client.Subscribe(c.cfg.NotesTopic, 1, c.handleNotesMsg); token.Wait() && token.Error() != nil {
			log.Printf("Failed to subscribe to notes topic %s: %v", c.cfg.NotesTopic, token.Error())
		} else {
			log.Printf("Subscribed to notes topic: %s", c.cfg.NotesTopic)
		}

		// Subscribe to emails
		if token := client.Subscribe(c.cfg.EmailsTopic, 1, c.handleEmailsMsg); token.Wait() && token.Error() != nil {
			log.Printf("Failed to subscribe to emails topic %s: %v", c.cfg.EmailsTopic, token.Error())
		} else {
			log.Printf("Subscribed to emails topic: %s", c.cfg.EmailsTopic)
		}

		// Subscribe to calendar
		if token := client.Subscribe(c.cfg.CalendarTopic, 1, c.handleCalendarMsg); token.Wait() && token.Error() != nil {
			log.Printf("Failed to subscribe to calendar topic %s: %v", c.cfg.CalendarTopic, token.Error())
		} else {
			log.Printf("Subscribed to calendar topic: %s", c.cfg.CalendarTopic)
		}

		// Subscribe to weather
		weatherTopic := "home/eink/weather"
		if token := client.Subscribe(weatherTopic, 1, c.handleWeatherMsg); token.Wait() && token.Error() != nil {
			log.Printf("Failed to subscribe to weather topic %s: %v", weatherTopic, token.Error())
		} else {
			log.Printf("Subscribed to weather topic: %s", weatherTopic)
		}
	})

	c.client = mqtt.NewClient(opts)

	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT connection failed: %w", token.Error())
	}

	return nil
}

func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		log.Println("Disconnecting from MQTT broker...")
		c.client.Disconnect(250)
	}
}

func (c *Client) GetNotes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	notesCopy := make([]string, len(c.notes))
	copy(notesCopy, c.notes)
	return notesCopy
}

func (c *Client) GetEmails() []Email {
	c.mu.RLock()
	defer c.mu.RUnlock()

	emailsCopy := make([]Email, len(c.emails))
	copy(emailsCopy, c.emails)
	return emailsCopy
}

func (c *Client) GetCalendarEvents() []CalendarEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	calCopy := make([]CalendarEvent, len(c.calendar))
	copy(calCopy, c.calendar)
	return calCopy
}

func (c *Client) GetLastUpdated() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdated
}

func (c *Client) handleNotesMsg(client mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	log.Printf("Received notes update: %s", string(payload))

	var parsedNotes []string
	if err := json.Unmarshal(payload, &parsedNotes); err != nil {
		log.Printf("Notes payload not a JSON array (%v). Storing as a single note.", err)
		parsedNotes = []string{string(payload)}
	}

	c.mu.Lock()
	c.notes = parsedNotes
	c.lastUpdated = time.Now()
	c.mu.Unlock()

	// Persist to database
	if err := c.db.SaveNotes(parsedNotes); err != nil {
		log.Printf("Failed to save notes to SQLite DB: %v", err)
	}
}

func (c *Client) handleEmailsMsg(client mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	log.Printf("Received emails update: %s", string(payload))

	var parsedEmails []Email
	if err := json.Unmarshal(payload, &parsedEmails); err != nil {
		log.Printf("Emails payload not a JSON array of Email objects: %v. Ignoring update.", err)
		return
	}

	c.mu.Lock()
	c.emails = parsedEmails
	c.lastUpdated = time.Now()
	c.mu.Unlock()

	// Convert and persist to database
	var dbEmails []db.EmailRecord
	for _, e := range parsedEmails {
		dbEmails = append(dbEmails, db.EmailRecord{Sender: e.Sender, Subject: e.Subject})
	}
	if err := c.db.SaveEmails(dbEmails); err != nil {
		log.Printf("Failed to save emails to SQLite DB: %v", err)
	}
}

func (c *Client) handleCalendarMsg(client mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	log.Printf("Received calendar update: %s", string(payload))

	var parsedCal []CalendarEvent
	if err := json.Unmarshal(payload, &parsedCal); err != nil {
		log.Printf("Calendar payload not a JSON array of CalendarEvent objects: %v. Ignoring update.", err)
		return
	}

	c.mu.Lock()
	c.calendar = parsedCal
	c.lastUpdated = time.Now()
	c.mu.Unlock()
}

func (c *Client) handleWeatherMsg(client mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	log.Printf("Received weather update via MQTT: %s", string(payload))

	var parsedWeather weather.WeatherData
	if err := json.Unmarshal(payload, &parsedWeather); err != nil {
		log.Printf("Weather payload not a valid WeatherData JSON object: %v. Ignoring update.", err)
		return
	}

	if c.weatherClient != nil {
		c.weatherClient.SetWeather(&parsedWeather)
	}

	c.mu.Lock()
	c.lastUpdated = time.Now()
	c.mu.Unlock()

	// Persist weather to database under key "cached_weather" as JSON string
	if err := c.db.SaveSetting("cached_weather", string(payload)); err != nil {
		log.Printf("Failed to save cached weather to SQLite DB: %v", err)
	}
}
