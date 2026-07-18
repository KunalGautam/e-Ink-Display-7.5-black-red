package mqtt

import (
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Registry struct {
	mu            sync.RWMutex
	connections   map[string]mqtt.Client // key: brokerURL
	cache         map[string]string      // key: brokerURL + "||" + topic -> latest payload
	subscriptions map[string]bool        // key: brokerURL + "||" + topic -> isSubscribed
}

func NewRegistry() *Registry {
	return &Registry{
		connections:   make(map[string]mqtt.Client),
		cache:         make(map[string]string),
		subscriptions: make(map[string]bool),
	}
}

func (r *Registry) GetPayload(broker, topic string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := broker + "||" + topic
	return r.cache[key]
}

func (r *Registry) SetPayload(broker, topic, payload string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := broker + "||" + topic
	r.cache[key] = payload
}

func (r *Registry) Subscribe(broker, topic string, username, password string) error {
	if broker == "" || topic == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := broker + "||" + topic
	if r.subscriptions[key] {
		return nil // Already subscribed
	}

	client, exists := r.connections[broker]
	if !exists {
		opts := mqtt.NewClientOptions().AddBroker(broker)
		opts.SetClientID(fmt.Sprintf("epaper-server-widget-%d", time.Now().UnixNano()))
		if username != "" {
			opts.SetUsername(username)
			opts.SetPassword(password)
		}
		opts.SetAutoReconnect(true)

		client = mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to connect to broker %s: %w", broker, token.Error())
		}
		r.connections[broker] = client
		log.Printf("MQTT Connected to broker: %s", broker)
	}

	token := client.Subscribe(topic, 1, func(c mqtt.Client, msg mqtt.Message) {
		r.mu.Lock()
		r.cache[key] = string(msg.Payload())
		r.mu.Unlock()
		log.Printf("Received MQTT update for [%s]: len=%d", key, len(msg.Payload()))
	})

	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
	}

	r.subscriptions[key] = true
	log.Printf("MQTT Subscribed to topic %s on %s", topic, broker)
	return nil
}

func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for broker, client := range r.connections {
		client.Disconnect(250)
		log.Printf("Disconnected MQTT client from %s", broker)
	}
}
