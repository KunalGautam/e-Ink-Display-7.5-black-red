package main

import (
	"encoding/json"
	"flag"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Email struct {
	Sender  string `json:"sender"`
	Subject string `json:"subject"`
}

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	notesTopic := flag.String("notes-topic", "home/eink/notes", "MQTT topic for notes")
	emailsTopic := flag.String("emails-topic", "home/eink/emails", "MQTT topic for emails")
	flag.Parse()

	log.Printf("Connecting to MQTT broker at %s...", *broker)

	opts := mqtt.NewClientOptions().AddBroker(*broker)
	opts.SetClientID("mock-publisher")
	opts.SetCleanSession(true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to broker: %v", token.Error())
	}
	defer client.Disconnect(250)

	log.Println("Connected. Publishing mock data...")

	// 1. Mock Emails Payload
	emails := []Email{
		{Sender: "Alice Cooper <alice@example.com>", Subject: "Meeting reschedule request for next Monday"},
		{Sender: "Github Notifications", Subject: "[PR #402] Merged: Add layout engine functionality"},
		{Sender: "Tech Newsletter", Subject: "Top 10 Raspberry Pi hardware projects of 2026"},
		{Sender: "Family Group", Subject: "Sunday dinner plans and updates"},
	}

	emailsData, err := json.Marshal(emails)
	if err != nil {
		log.Fatalf("Failed to marshal emails: %v", err)
	}

	token := client.Publish(*emailsTopic, 1, true, emailsData) // publish as retained
	token.Wait()
	if token.Error() != nil {
		log.Printf("Error publishing emails: %v", token.Error())
	} else {
		log.Printf("Published %d emails to %s (retained)", len(emails), *emailsTopic)
	}

	// 2. Mock Notes Payload
	notes := []string{
		"Buy fresh groceries and milk from the local store",
		"Review the implementation plan for epaper display system and sign off",
		"Schedule a brief call with Alice to discuss layout changes next Wednesday",
		"Water the backyard plants at 6:00 PM today",
	}

	notesData, err := json.Marshal(notes)
	if err != nil {
		log.Fatalf("Failed to marshal notes: %v", err)
	}

	token = client.Publish(*notesTopic, 1, true, notesData) // publish as retained
	token.Wait()
	if token.Error() != nil {
		log.Printf("Error publishing notes: %v", token.Error())
	} else {
		log.Printf("Published %d notes to %s (retained)", len(notes), *notesTopic)
	}

	// Give a small pause to let messages dispatch completely
	time.Sleep(500 * time.Millisecond)
	log.Println("Mock data published successfully. Exiting.")
}
