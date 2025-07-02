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
	"sort"

	//	"strings"
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
	Untis.Main() //starting API calls function

	//checking for time

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Program is running. Press Ctrl+C to stop.")
	//Start logging
	Run()
	// Block until we receive a signal
	<-sigChan
	log.Println("Shutting down...")
}

/*
#Main Run Function
*/
func Run() {
	codeByStartTime, err := MapTimeToCode("timetableFilled.json")
	if err != nil {
		log.Println(err)
	}

	roomByStartTime, err := MapTimeToRoom("timetableFilled.json")
	if err != nil {
		log.Fatal(err)
	}
	now := time.Now().Format("15:04")
	nextTime, room, found := NextRoomForTime(roomByStartTime, now)
	if found {
		Status, found := NextCodeForTime(codeByStartTime, now)
		if found {

			fmt.Printf("Next time: %s, Room: %s\n", nextTime, room)
			sendDiscordWebhook(room, nextTime, Status)
		}
	}

}

/*
 */
func NextRoomForTime(roomByStartTime map[string]string, current string) (string, string, bool) {
	layout := "15:04"
	now, err := time.Parse(layout, current)
	if err != nil {
		return "", "", false
	}
	var times []time.Time
	timeToStr := make(map[time.Time]string)
	for t := range roomByStartTime {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		times = append(times, parsed)
		timeToStr[parsed] = t
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	for _, t := range times {
		if t.After(now) {
			return timeToStr[t], roomByStartTime[timeToStr[t]], true
		}
	}
	return "", "", false
}
func NextCodeForTime(codeByStartTime map[string]string, current string) (string, bool) {

	layout := "15:04"
	now, err := time.Parse(layout, current)
	if err != nil {
		return "", false
	}
	var times []time.Time
	timeToStr := make(map[time.Time]string)
	for t := range codeByStartTime {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		times = append(times, parsed)
		timeToStr[parsed] = t
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	for _, t := range times {
		if t.After(now) {
			return codeByStartTime[timeToStr[t]], true
		}
	}
	return "", false

}
func MapTimeToRoom(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var table []NamedTimetableEntry
	err = json.Unmarshal(data, &table)
	if err != nil {
		return nil, err
	}
	roomByStartTime := make(map[string]string)
	for _, entry := range table {
		roomByStartTime[entry.StartTime] = entry.Ro[0]
	}
	if roomByStartTime == nil {
		return nil, nil
	}
	return roomByStartTime, nil
}
func MapTimeToCode(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var table []NamedTimetableEntry
	err = json.Unmarshal(data, &table)
	if err != nil {
		return nil, err
	}
	codeByStartTime := make(map[string]string)
	for _, entry := range table {
		codeByStartTime[entry.StartTime] = entry.Code
	}
	return codeByStartTime, nil
}
func MapTimeToSubject(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var table []NamedTimetableEntry
	err = json.Unmarshal(data, &table)
	if err != nil {
		return nil, err
	}
	subjectByStartTime := make(map[string]string)
	for _, entry := range table {
		subjectByStartTime[entry.StartTime] = entry.Su[0]
	}
	return subjectByStartTime, nil
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

func sendDiscordWebhook(room string, nextTime string, Status string) {
	log.Println("Sending Discord webhook notification...")
	// Create a rich embed message
	message := fmt.Sprintf(
		"Room: %s\nStart-Time: %s\nStatus: %s",
		room, nextTime, Status,
	)
	/*embed := Embed{
		Title:       "Next Lesson ",
		Description: "The next lesson is starting soon:  ",
		Color:       3066993, // Green color
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []Field{
			{
				Name: "New Lesson",
				//	Value:  fmt.Sprintf("`%s`", ip),
				Value:  fmt.Sprintf("Room: %s", room),
				Inline: true,
			},
			{
				Name:   "Start-Time",
				Value:  fmt.Sprintf("Start time: %s", nextTime),
				Inline: true,
			},
			{
				Name:   "Status",
				Value:  fmt.Sprintf("Status: %s", Status),
				Inline: true,
			},
		},
	}
	*/
	payload := DiscordWebhookPayload{
		Content: message,
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
