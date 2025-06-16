package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	//	"os/signal"
	//	"syscall"
	"time"
	Untis "untislogger/Bot"

	"github.com/joho/godotenv"
)

// Global variable to store the current public IP
var currentPubIP string

// Discord webhook configuration
var discordWebhookURL string // Webhook URL from environment variable

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}
	discordWebhookURL = os.Getenv("DISCORD_WEBHOOK_URL")
	log.Println("Initializing application...")
	log.Printf("DISCORD_WEBHOOK_URL: %q", discordWebhookURL)

	// Check if Discord webhook is configured
	if discordWebhookURL != "" {
		log.Println("Discord webhook configured")
	} else {
		log.Println("No Discord webhook provided, Discord notifications will be disabled")
	}

	// scheduleIPCheck() // Start the periodic IP check
}

func main() {
	go Untis.Auth() //starting API calls function
	// Create a channel to receive OS signals
	/*	sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		log.Println("Program is running. Press Ctrl+C to stop.")

		// Block until we receive a signal
		<-sigChan
		log.Println("Shutting down...")  */
}

func testPubIP() {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		log.Printf("Error getting public IP: %v", err)
		return
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return
	}

	log.Printf("Current public IP: %s", ip)
	if string(ip) != currentPubIP {
		log.Printf("IP changed sending notifications for new IP: %s", ip)
		currentPubIP = string(ip)
		notifyNewPubIP(currentPubIP)
	}
}

func scheduleIPCheck() {
	// Run once immediately
	testPubIP()

	// Then schedule to run every hour
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			testPubIP()
		}
	}()
}

func notifyNewPubIP(ip string) {
	log.Printf("Notifying about new IP: %s", ip)

	// Send Discord notification if webhook is configured
	if discordWebhookURL != "" {
		sendDiscordWebhook(ip)
	} else {
		log.Println("Discord webhook not configured, skipping Discord notification")
	}
}

// DiscordWebhookPayload represents the structure for Discord webhook messages
type DiscordWebhookPayload struct {
	Content string  `json:"content"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

type Embed struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Color       int     `json:"color"`
	Timestamp   string  `json:"timestamp"`
	Fields      []Field `json:"fields,omitempty"`
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func sendDiscordWebhook(ip string) {
	log.Println("Sending Discord webhook notification...")

	// Create a rich embed message
	embed := Embed{
		Title:       "🌐 Public IP Address Changed!",
		Description: "Your public IP address has been updated.",
		Color:       3066993, // Green color
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []Field{
			{
				Name:   "New IP Address",
				Value:  fmt.Sprintf("`%s`", ip),
				Inline: true,
			},
			{
				Name:   "Timestamp",
				Value:  time.Now().Format("2006-01-02 15:04:05"),
				Inline: true,
			},
		},
	}

	payload := DiscordWebhookPayload{
		Embeds: []Embed{embed},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling webhook payload: %v", err)
		return
	}

	resp, err := http.Post(discordWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending Discord webhook: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Println("Discord webhook notification sent successfully")
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Discord webhook failed with status %d: %s", resp.StatusCode, string(body))
	}
}
