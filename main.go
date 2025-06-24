package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	Untis "untislogger/Bot"

	"github.com/joho/godotenv"
)

type NamedTimetableEntry struct {
	ID           int      `json:"id"`
	Date         string   `json:"date"`
	StartTime    string   `json:"startTime"`
	EndTime      string   `json:"endTime"`
	Code         string   `json:"code,omitempty"`
	Statflags    string   `json:"statflags,omitempty"`
	Kl           []string `json:"kl"`
	Su           []string `json:"su"`
	Ro           []string `json:"ro"`
	ActivityType string   `json:"activityType"`
}

// init and main//
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
}

func main() {
	go Untis.Main() //starting API calls function
	//checking for time
	log.Println("starting Timetable checks")
	go checkTimetableChanges()
	go notifyUpcomingLessons()
	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Program is running. Press Ctrl+C to stop.")

	// Block until we receive a signal
	<-sigChan
	log.Println("Shutting down...")
}

func loadTimetableFilled(path string) ([]NamedTimetableEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []NamedTimetableEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func checkTimetableChanges() {
	var lastTimetable []NamedTimetableEntry

	for {
		current, err := loadTimetableFilled("timetableFilled.json")
		if err != nil {
			log.Println("Error loading timetable:", err)
			time.Sleep(time.Hour)
			continue
		}

		// Compare with last timetable (simple string comparison)
		log.Println("Checking if something changed")
		if !equalTimetables(lastTimetable, current) && len(lastTimetable) > 0 {
			// Find and send changes
			changes := diffTimetables(lastTimetable, current)
			oldMap := make(map[int]NamedTimetableEntry)
			for _, e := range lastTimetable {
				oldMap[e.ID] = e
			}
			for _, newLesson := range changes {
				oldLesson, found := oldMap[newLesson.ID]
				if found {
					sendTimetableChangeWebhook(oldLesson, newLesson)
				} else {
					// If it's a new lesson, you can send a simple notification or skip
					sendTimetableChangeWebhook(NamedTimetableEntry{}, newLesson)
				}
			}
		}
		lastTimetable = current
		time.Sleep(time.Hour)
	}
}

func equalTimetables(a, b []NamedTimetableEntry) bool {

	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return bytes.Equal(aj, bj)
}

func diffTimetables(old, new []NamedTimetableEntry) []NamedTimetableEntry {
	// Simple diff: return lessons in new that are not in old
	var diff []NamedTimetableEntry
	oldMap := make(map[int]NamedTimetableEntry)
	for _, e := range old {
		oldMap[e.ID] = e
	}
	for _, e := range new {
		if _, found := oldMap[e.ID]; !found {
			diff = append(diff, e)
		}
	}
	return diff
}

func notifyUpcomingLessons() {
	log.Println("checking for new Lesson")
	for {
		entries, err := loadTimetableFilled("timetableFilled.json")
		if err != nil {
			log.Println("Error loading timetable:", err)
			time.Sleep(time.Minute)
			continue
		}
		now := time.Now()
		for _, lesson := range entries {
			// Parse date and time
			lessonTime, err := time.Parse("02-01-2006 15:04", lesson.Date+" "+lesson.StartTime)
			if err != nil {
				continue
			}
			if lessonTime.Sub(now) > 4*time.Minute && lessonTime.Sub(now) < 6*time.Minute {
				sendNextLessonWebhook(lesson)
			}
		}
		time.Sleep(time.Minute)
	}
}

// Discord webhook configuration
var discordWebhookURL string // Webhook URL from environment variable

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

func sendTimetableChangeWebhook(oldLesson, newLesson NamedTimetableEntry) {
	var changes []string
	if !equalStringSlices(oldLesson.Su, newLesson.Su) {
		changes = append(changes, fmt.Sprintf("Subject: %v → %v", strings.Join(oldLesson.Su, ", "), strings.Join(newLesson.Su, ", ")))
	}
	if !equalStringSlices(oldLesson.Ro, newLesson.Ro) {
		changes = append(changes, fmt.Sprintf("Room: %v → %v", strings.Join(oldLesson.Ro, ", "), strings.Join(newLesson.Ro, ", ")))
	}
	if oldLesson.StartTime != newLesson.StartTime {
		changes = append(changes, fmt.Sprintf("Time: %s → %s", oldLesson.StartTime, newLesson.StartTime))
	}
	if oldLesson.Code != newLesson.Code {
		changes = append(changes, fmt.Sprintf("Code: %s → %s", oldLesson.Code, newLesson.Code))
	}
	if len(changes) == 0 {
		changes = append(changes, "Other details changed")
	}

	content := fmt.Sprintf("**Timetable Change**\nLesson: %v\nTime: %s\nChanged: %s",
		strings.Join(newLesson.Su, ", "),
		newLesson.StartTime,
		strings.Join(changes, "; "),
	)

	sendDiscordWebhook(content)
}

func sendNextLessonWebhook(lesson NamedTimetableEntry) {
	content := fmt.Sprintf(
		"**Next Lesson Reminder**\nLesson: %v\nTime: %s\nRoom: %v",
		strings.Join(lesson.Su, ", "),
		lesson.StartTime,
		strings.Join(lesson.Ro, ", "),
	)
	sendDiscordWebhook(content)
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
